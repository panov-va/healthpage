package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store/db"
)

// EnsureSubscription возвращает подписку аккаунта, создавая дефолтную free, если её ещё нет.
func (s *Store) EnsureSubscription(ctx context.Context, accountID uuid.UUID) (domain.Subscription, error) {
	row, err := s.q.EnsureSubscription(ctx, accountID)
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("store: ensure subscription: %w", err)
	}
	return mapSubscription(row), nil
}

// SubscriptionByAccount возвращает подписку аккаунта. ErrNotFound если нет.
func (s *Store) SubscriptionByAccount(ctx context.Context, accountID uuid.UUID) (domain.Subscription, error) {
	row, err := s.q.GetSubscriptionByAccount(ctx, accountID)
	if err != nil {
		return domain.Subscription{}, wrapNotFound(err)
	}
	return mapSubscription(row), nil
}

// UpdateSubscription сохраняет жизненный цикл подписки (по account_id).
func (s *Store) UpdateSubscription(ctx context.Context, sub domain.Subscription) (domain.Subscription, error) {
	row, err := s.q.UpdateSubscription(ctx, db.UpdateSubscriptionParams{
		AccountID:             sub.AccountID,
		Plan:                  db.BillingPlan(sub.Plan),
		Status:                string(sub.Status),
		BillingPeriod:         periodToStr(sub.BillingPeriod),
		Provider:              providerToStr(sub.Provider),
		ProviderCustomerToken: sub.ProviderCustomerToken,
		TrialEndsAt:           sub.TrialEndsAt,
		CurrentPeriodStart:    sub.CurrentPeriodStart,
		CurrentPeriodEnd:      sub.CurrentPeriodEnd,
		CancelAtPeriodEnd:     sub.CancelAtPeriodEnd,
		DunningAttempts:       int32(sub.DunningAttempts),
	})
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("store: update subscription: %w", err)
	}
	return mapSubscription(row), nil
}

// ListDueSubscriptions возвращает premium-подписки с истёкшим периодом (для worker-billing).
func (s *Store) ListDueSubscriptions(ctx context.Context, before time.Time, limit int) ([]domain.Subscription, error) {
	rows, err := s.q.ListDueSubscriptions(ctx, db.ListDueSubscriptionsParams{
		CurrentPeriodEnd: &before,
		Limit:            int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list due subscriptions: %w", err)
	}
	out := make([]domain.Subscription, len(rows))
	for i, r := range rows {
		out[i] = mapSubscription(r)
	}
	return out, nil
}

// SetAccountPlan переключает эффективный тариф аккаунта (включение Premium / откат на Free).
func (s *Store) SetAccountPlan(ctx context.Context, accountID uuid.UUID, plan domain.BillingPlan) error {
	if err := s.q.SetAccountPlan(ctx, db.SetAccountPlanParams{
		ID:          accountID,
		BillingPlan: db.BillingPlan(plan),
	}); err != nil {
		return fmt.Errorf("store: set account plan: %w", err)
	}
	return nil
}

// CreatePayment добавляет запись в журнал платежей.
func (s *Store) CreatePayment(ctx context.Context, p domain.Payment) (domain.Payment, error) {
	row, err := s.q.CreatePayment(ctx, db.CreatePaymentParams{
		AccountID:         p.AccountID,
		SubscriptionID:    p.SubscriptionID,
		AmountMinor:       p.AmountMinor,
		Currency:          p.Currency,
		Status:            string(p.Status),
		Provider:          providerToStr(p.Provider),
		ProviderPaymentID: p.ProviderPaymentID,
		IdempotencyKey:    p.IdempotencyKey,
		BillingPeriod:     periodToStr(p.BillingPeriod),
	})
	if err != nil {
		return domain.Payment{}, fmt.Errorf("store: create payment: %w", err)
	}
	return mapPayment(row), nil
}

// PaymentByID находит платёж по id. ErrNotFound если нет.
func (s *Store) PaymentByID(ctx context.Context, id uuid.UUID) (domain.Payment, error) {
	row, err := s.q.GetPayment(ctx, id)
	if err != nil {
		return domain.Payment{}, wrapNotFound(err)
	}
	return mapPayment(row), nil
}

// PaymentByProviderID находит платёж по идентификатору провайдера (идемпотентность webhook).
// ErrNotFound если нет.
func (s *Store) PaymentByProviderID(ctx context.Context, providerPaymentID string) (domain.Payment, error) {
	row, err := s.q.GetPaymentByProviderID(ctx, &providerPaymentID)
	if err != nil {
		return domain.Payment{}, wrapNotFound(err)
	}
	return mapPayment(row), nil
}

// UpdatePaymentResult обновляет исход платежа (статус/идентификатор провайдера/чек/время оплаты).
// providerPaymentID и receiptID применяются только если не nil (COALESCE).
func (s *Store) UpdatePaymentResult(ctx context.Context, id uuid.UUID, status domain.PaymentStatus, providerPaymentID, receiptID *string, paidAt *time.Time) (domain.Payment, error) {
	row, err := s.q.UpdatePaymentResult(ctx, db.UpdatePaymentResultParams{
		ID:                id,
		Status:            string(status),
		ProviderPaymentID: providerPaymentID,
		ReceiptID:         receiptID,
		PaidAt:            paidAt,
	})
	if err != nil {
		return domain.Payment{}, fmt.Errorf("store: update payment: %w", err)
	}
	return mapPayment(row), nil
}

// ListPaymentsByAccount возвращает историю платежей аккаунта (новые сверху, пагинация).
func (s *Store) ListPaymentsByAccount(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]domain.Payment, error) {
	rows, err := s.q.ListPaymentsByAccount(ctx, db.ListPaymentsByAccountParams{
		AccountID: accountID,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list payments: %w", err)
	}
	out := make([]domain.Payment, len(rows))
	for i, r := range rows {
		out[i] = mapPayment(r)
	}
	return out, nil
}

func mapSubscription(s db.Subscription) domain.Subscription {
	return domain.Subscription{
		ID:                    s.ID,
		AccountID:             s.AccountID,
		Plan:                  domain.BillingPlan(s.Plan),
		Status:                domain.SubscriptionStatus(s.Status),
		BillingPeriod:         strToPeriod(s.BillingPeriod),
		Provider:              strToProvider(s.Provider),
		ProviderCustomerToken: s.ProviderCustomerToken,
		TrialEndsAt:           s.TrialEndsAt,
		CurrentPeriodStart:    s.CurrentPeriodStart,
		CurrentPeriodEnd:      s.CurrentPeriodEnd,
		CancelAtPeriodEnd:     s.CancelAtPeriodEnd,
		DunningAttempts:       int(s.DunningAttempts),
		CreatedAt:             s.CreatedAt,
		UpdatedAt:             s.UpdatedAt,
	}
}

func mapPayment(p db.Payment) domain.Payment {
	return domain.Payment{
		ID:                p.ID,
		AccountID:         p.AccountID,
		SubscriptionID:    p.SubscriptionID,
		AmountMinor:       p.AmountMinor,
		Currency:          p.Currency,
		Status:            domain.PaymentStatus(p.Status),
		Provider:          strToProvider(p.Provider),
		ProviderPaymentID: p.ProviderPaymentID,
		IdempotencyKey:    p.IdempotencyKey,
		ReceiptID:         p.ReceiptID,
		BillingPeriod:     strToPeriod(p.BillingPeriod),
		CreatedAt:         p.CreatedAt,
		PaidAt:            p.PaidAt,
	}
}

// ── конвертеры типизированных enum-указателей ↔ *string (БД хранит TEXT) ──

func periodToStr(p *domain.BillingPeriod) *string {
	if p == nil {
		return nil
	}
	v := string(*p)
	return &v
}

func strToPeriod(s *string) *domain.BillingPeriod {
	if s == nil {
		return nil
	}
	v := domain.BillingPeriod(*s)
	return &v
}

func providerToStr(p *domain.PaymentProvider) *string {
	if p == nil {
		return nil
	}
	v := string(*p)
	return &v
}

func strToProvider(s *string) *domain.PaymentProvider {
	if s == nil {
		return nil
	}
	v := domain.PaymentProvider(*s)
	return &v
}
