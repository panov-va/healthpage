package billing

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
)

// ErrInvalidPlan — на оплату принимается только premium (free не покупается).
var ErrInvalidPlan = errors.New("billing: only premium plan is purchasable")

// Repo — доступ к данным, нужный сервису биллинга (реализуется *store.Store).
type Repo interface {
	EnsureSubscription(ctx context.Context, accountID uuid.UUID) (domain.Subscription, error)
	SubscriptionByAccount(ctx context.Context, accountID uuid.UUID) (domain.Subscription, error)
	UpdateSubscription(ctx context.Context, sub domain.Subscription) (domain.Subscription, error)
	SetAccountPlan(ctx context.Context, accountID uuid.UUID, plan domain.BillingPlan) error
	CreatePayment(ctx context.Context, p domain.Payment) (domain.Payment, error)
	PaymentByProviderID(ctx context.Context, providerPaymentID string) (domain.Payment, error)
	UpdatePaymentResult(ctx context.Context, id uuid.UUID, status domain.PaymentStatus, providerPaymentID, receiptID *string, paidAt *time.Time) (domain.Payment, error)
	ListDueSubscriptions(ctx context.Context, before time.Time, limit int) ([]domain.Subscription, error)
}

// Service оркеструет checkout, обработку webhook'ов, рекуррентные списания, dunning и отмену.
type Service struct {
	provider      Provider
	repo          Repo
	pricing       Pricing
	maxDunning    int           // число неуспешных попыток до отката на Free
	retryInterval time.Duration // через сколько повторять списание в past_due
	now           func() time.Time
}

// Config — параметры сервиса биллинга.
type Config struct {
	Provider      Provider
	Pricing       Pricing
	MaxDunning    int
	RetryInterval time.Duration
}

// NewService создаёт сервис. now по умолчанию — time.Now (инъекция для тестов).
func NewService(repo Repo, cfg Config) *Service {
	if cfg.MaxDunning <= 0 {
		cfg.MaxDunning = 3
	}
	if cfg.RetryInterval <= 0 {
		cfg.RetryInterval = 72 * time.Hour
	}
	return &Service{
		provider:      cfg.Provider,
		repo:          repo,
		pricing:       cfg.Pricing,
		maxDunning:    cfg.MaxDunning,
		retryInterval: cfg.RetryInterval,
		now:           time.Now,
	}
}

// WithClock подменяет источник времени (для тестов).
func (s *Service) WithClock(now func() time.Time) *Service { s.now = now; return s }

// Pricing возвращает текущие тарифы (для отображения/расчёта в API).
func (s *Service) Pricing() Pricing { return s.pricing }

// CheckoutResult — итог инициирования оплаты.
type CheckoutResult struct {
	PaymentID       uuid.UUID
	ConfirmationURL string
	Provider        domain.PaymentProvider
}

// Checkout инициирует оплату Premium: создаёт pending-платёж, дергает провайдера за ссылкой
// подтверждения, переводит подписку в pending. Премиум включается уже по webhook'у успеха.
func (s *Service) Checkout(ctx context.Context, accountID uuid.UUID, plan domain.BillingPlan, period domain.BillingPeriod, returnURL string) (CheckoutResult, error) {
	if plan != domain.PlanPremium {
		return CheckoutResult{}, ErrInvalidPlan
	}
	if !period.IsValid() {
		return CheckoutResult{}, fmt.Errorf("%w: bad period", ErrInvalidPlan)
	}
	sub, err := s.repo.EnsureSubscription(ctx, accountID)
	if err != nil {
		return CheckoutResult{}, err
	}

	amount := s.pricing.AmountMinor(period)
	idemKey := uuid.NewString()
	providerName := s.provider.Name()
	pay, err := s.repo.CreatePayment(ctx, domain.Payment{
		AccountID:      accountID,
		SubscriptionID: &sub.ID,
		AmountMinor:    amount,
		Currency:       s.pricing.Currency,
		Status:         domain.PaymentPending,
		Provider:       &providerName,
		IdempotencyKey: &idemKey,
		BillingPeriod:  &period,
	})
	if err != nil {
		return CheckoutResult{}, err
	}

	res, err := s.provider.CreatePayment(ctx, CreatePaymentInput{
		AmountMinor:    amount,
		Currency:       s.pricing.Currency,
		Description:    "HealthPage Premium",
		ReturnURL:      returnURL,
		IdempotencyKey: idemKey,
		AccountID:      accountID.String(),
	})
	if err != nil {
		return CheckoutResult{}, err
	}

	// Привязать платёж к идентификатору провайдера (ключ идемпотентности webhook'а).
	if _, err := s.repo.UpdatePaymentResult(ctx, pay.ID, domain.PaymentPending, &res.ProviderPaymentID, nil, nil); err != nil {
		return CheckoutResult{}, err
	}

	// Подписка переходит в pending; план остаётся прежним до подтверждения оплаты.
	sub.Status = domain.SubStatusPending
	sub.Provider = &providerName
	sub.BillingPeriod = &period
	if _, err := s.repo.UpdateSubscription(ctx, sub); err != nil {
		return CheckoutResult{}, err
	}

	return CheckoutResult{PaymentID: pay.ID, ConfirmationURL: res.ConfirmationURL, Provider: providerName}, nil
}

// HandleWebhook обрабатывает уведомление провайдера. Идемпотентно по provider_payment_id:
// повторная доставка того же успешного события не активирует подписку дважды.
func (s *Service) HandleWebhook(ctx context.Context, header http.Header, body []byte) error {
	ev, err := s.provider.ParseWebhook(ctx, header, body)
	if err != nil {
		return err
	}
	pay, err := s.repo.PaymentByProviderID(ctx, ev.ProviderPaymentID)
	if err != nil {
		// Неизвестный платёж — молча игнорируем (202): не наш или гонка создания.
		return nil
	}
	// Идемпотентность: платёж уже в терминальном статусе — повтор не обрабатываем.
	if pay.Status == ev.Status || pay.Status == domain.PaymentSucceeded {
		return nil
	}

	switch ev.Status {
	case domain.PaymentSucceeded:
		return s.activate(ctx, pay, ev)
	case domain.PaymentFailed, domain.PaymentRefunded:
		now := s.now()
		_, err := s.repo.UpdatePaymentResult(ctx, pay.ID, ev.Status, nil, nil, &now)
		return err
	default:
		return nil // pending и пр. — игнорируем
	}
}

// activate отмечает платёж успешным и включает Premium на следующий период.
func (s *Service) activate(ctx context.Context, pay domain.Payment, ev WebhookEvent) error {
	now := s.now()
	var receipt *string
	if ev.ReceiptID != "" {
		receipt = &ev.ReceiptID
	}
	if _, err := s.repo.UpdatePaymentResult(ctx, pay.ID, domain.PaymentSucceeded, nil, receipt, &now); err != nil {
		return err
	}

	sub, err := s.repo.SubscriptionByAccount(ctx, pay.AccountID)
	if err != nil {
		return err
	}
	period := domain.PeriodMonthly
	if pay.BillingPeriod != nil {
		period = *pay.BillingPeriod
	}
	end := period.Advance(now)
	sub.Plan = domain.PlanPremium
	sub.Status = domain.SubStatusActive
	sub.BillingPeriod = &period
	sub.CurrentPeriodStart = &now
	sub.CurrentPeriodEnd = &end
	sub.CancelAtPeriodEnd = false
	sub.DunningAttempts = 0
	if ev.CustomerToken != "" {
		sub.ProviderCustomerToken = &ev.CustomerToken
	}
	if pay.Provider != nil {
		sub.Provider = pay.Provider
	}
	if _, err := s.repo.UpdateSubscription(ctx, sub); err != nil {
		return err
	}
	return s.repo.SetAccountPlan(ctx, sub.AccountID, domain.PlanPremium)
}

// Cancel отключает автопродление: подписка остаётся Premium до конца оплаченного периода.
func (s *Service) Cancel(ctx context.Context, accountID uuid.UUID) (domain.Subscription, error) {
	sub, err := s.repo.EnsureSubscription(ctx, accountID)
	if err != nil {
		return domain.Subscription{}, err
	}
	sub.CancelAtPeriodEnd = true
	return s.repo.UpdateSubscription(ctx, sub)
}

// ProcessDue выполняет один проход worker-billing: для подписок с истёкшим периодом —
// рекуррентное списание, dunning при неуспехе, откат на Free при отмене/исчерпании попыток.
// Возвращает число обработанных подписок.
func (s *Service) ProcessDue(ctx context.Context, limit int) (int, error) {
	now := s.now()
	due, err := s.repo.ListDueSubscriptions(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	for _, sub := range due {
		if err := s.renewOne(ctx, sub); err != nil {
			return 0, err
		}
	}
	return len(due), nil
}

func (s *Service) renewOne(ctx context.Context, sub domain.Subscription) error {
	now := s.now()
	// Автопродление отменено — откат на Free по окончании периода.
	if sub.CancelAtPeriodEnd {
		return s.downgrade(ctx, sub)
	}
	period := domain.PeriodMonthly
	if sub.BillingPeriod != nil {
		period = *sub.BillingPeriod
	}
	amount := s.pricing.AmountMinor(period)
	idemKey := uuid.NewString()
	providerName := s.provider.Name()
	pay, err := s.repo.CreatePayment(ctx, domain.Payment{
		AccountID:      sub.AccountID,
		SubscriptionID: &sub.ID,
		AmountMinor:    amount,
		Currency:       s.pricing.Currency,
		Status:         domain.PaymentPending,
		Provider:       &providerName,
		IdempotencyKey: &idemKey,
		BillingPeriod:  &period,
	})
	if err != nil {
		return err
	}

	var token string
	if sub.ProviderCustomerToken != nil {
		token = *sub.ProviderCustomerToken
	}
	var charge ChargeResult
	var chargeErr error
	if token == "" {
		chargeErr = errors.New("billing: no saved payment method")
	} else {
		charge, chargeErr = s.provider.ChargeRecurring(ctx, ChargeInput{
			AmountMinor:    amount,
			Currency:       s.pricing.Currency,
			Description:    "HealthPage Premium renewal",
			CustomerToken:  token,
			IdempotencyKey: idemKey,
		})
	}

	if chargeErr == nil && charge.Status == domain.PaymentSucceeded {
		var receipt *string
		if charge.ReceiptID != "" {
			receipt = &charge.ReceiptID
		}
		if _, err := s.repo.UpdatePaymentResult(ctx, pay.ID, domain.PaymentSucceeded, &charge.ProviderPaymentID, receipt, &now); err != nil {
			return err
		}
		end := period.Advance(now)
		sub.Status = domain.SubStatusActive
		sub.CurrentPeriodStart = &now
		sub.CurrentPeriodEnd = &end
		sub.DunningAttempts = 0
		if _, err := s.repo.UpdateSubscription(ctx, sub); err != nil {
			return err
		}
		return s.repo.SetAccountPlan(ctx, sub.AccountID, domain.PlanPremium)
	}

	// Неуспех — dunning.
	if _, err := s.repo.UpdatePaymentResult(ctx, pay.ID, domain.PaymentFailed, nil, nil, &now); err != nil {
		return err
	}
	sub.DunningAttempts++
	if sub.DunningAttempts >= s.maxDunning {
		return s.downgrade(ctx, sub)
	}
	// Остаёмся Premium (grace) в past_due; следующая попытка — через retryInterval.
	sub.Status = domain.SubStatusPastDue
	retryAt := now.Add(s.retryInterval)
	sub.CurrentPeriodEnd = &retryAt
	_, err = s.repo.UpdateSubscription(ctx, sub)
	return err
}

// downgrade отменяет подписку и откатывает аккаунт на Free.
func (s *Service) downgrade(ctx context.Context, sub domain.Subscription) error {
	sub.Status = domain.SubStatusCanceled
	sub.Plan = domain.PlanFree
	if _, err := s.repo.UpdateSubscription(ctx, sub); err != nil {
		return err
	}
	return s.repo.SetAccountPlan(ctx, sub.AccountID, domain.PlanFree)
}
