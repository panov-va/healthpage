package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
)

// fakeRepo — in-memory реализация Repo для юнит-тестов.
type fakeRepo struct {
	subs     map[uuid.UUID]domain.Subscription // by accountID
	payments map[uuid.UUID]domain.Payment      // by paymentID
	plans    map[uuid.UUID]domain.BillingPlan  // accounts billing_plan
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		subs:     map[uuid.UUID]domain.Subscription{},
		payments: map[uuid.UUID]domain.Payment{},
		plans:    map[uuid.UUID]domain.BillingPlan{},
	}
}

func (r *fakeRepo) EnsureSubscription(_ context.Context, accountID uuid.UUID) (domain.Subscription, error) {
	if s, ok := r.subs[accountID]; ok {
		return s, nil
	}
	s := domain.Subscription{ID: uuid.New(), AccountID: accountID, Plan: domain.PlanFree, Status: domain.SubStatusActive}
	r.subs[accountID] = s
	return s, nil
}

func (r *fakeRepo) SubscriptionByAccount(_ context.Context, accountID uuid.UUID) (domain.Subscription, error) {
	return r.subs[accountID], nil
}

func (r *fakeRepo) UpdateSubscription(_ context.Context, sub domain.Subscription) (domain.Subscription, error) {
	r.subs[sub.AccountID] = sub
	return sub, nil
}

func (r *fakeRepo) SetAccountPlan(_ context.Context, accountID uuid.UUID, plan domain.BillingPlan) error {
	r.plans[accountID] = plan
	return nil
}

func (r *fakeRepo) CreatePayment(_ context.Context, p domain.Payment) (domain.Payment, error) {
	p.ID = uuid.New()
	p.CreatedAt = time.Now()
	r.payments[p.ID] = p
	return p, nil
}

func (r *fakeRepo) PaymentByProviderID(_ context.Context, pid string) (domain.Payment, error) {
	for _, p := range r.payments {
		if p.ProviderPaymentID != nil && *p.ProviderPaymentID == pid {
			return p, nil
		}
	}
	return domain.Payment{}, errNotFound
}

func (r *fakeRepo) UpdatePaymentResult(_ context.Context, id uuid.UUID, status domain.PaymentStatus, providerPaymentID, receiptID *string, paidAt *time.Time) (domain.Payment, error) {
	p := r.payments[id]
	p.Status = status
	if providerPaymentID != nil {
		p.ProviderPaymentID = providerPaymentID
	}
	if receiptID != nil {
		p.ReceiptID = receiptID
	}
	p.PaidAt = paidAt
	r.payments[id] = p
	return p, nil
}

func (r *fakeRepo) ListDueSubscriptions(_ context.Context, before time.Time, _ int) ([]domain.Subscription, error) {
	var out []domain.Subscription
	for _, s := range r.subs {
		if s.Plan == domain.PlanPremium &&
			(s.Status == domain.SubStatusActive || s.Status == domain.SubStatusPastDue) &&
			s.CurrentPeriodEnd != nil && !s.CurrentPeriodEnd.After(before) {
			out = append(out, s)
		}
	}
	return out, nil
}

var errNotFound = &notFoundErr{}

type notFoundErr struct{}

func (*notFoundErr) Error() string { return "not found" }

func newTestService(repo Repo) *Service {
	clock := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	svc := NewService(repo, Config{
		Provider:      StubProvider{BaseURL: "http://localhost:3000"},
		Pricing:       DefaultPricing(99000, 20, 14, "RUB"),
		MaxDunning:    3,
		RetryInterval: 72 * time.Hour,
	})
	return svc.WithClock(func() time.Time { return clock })
}

func succeededWebhook(t *testing.T, providerPaymentID, token string) []byte {
	t.Helper()
	b, _ := json.Marshal(stubWebhookBody{
		ProviderPaymentID: providerPaymentID,
		Status:            "succeeded",
		CustomerToken:     token,
		ReceiptID:         "rcpt-1",
		AmountMinor:       99000,
	})
	return b
}

func TestCheckoutThenWebhookActivatesPremium(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	ctx := context.Background()
	acc := uuid.New()

	res, err := svc.Checkout(ctx, acc, domain.PlanPremium, domain.PeriodMonthly, "http://localhost:3000/return")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if res.ConfirmationURL == "" || res.PaymentID == uuid.Nil {
		t.Fatal("checkout must return confirmation url + payment id")
	}
	// Подписка pending, аккаунт ещё free.
	if sub := repo.subs[acc]; sub.Status != domain.SubStatusPending {
		t.Fatalf("sub status=%s want pending", sub.Status)
	}
	if repo.plans[acc] == domain.PlanPremium {
		t.Fatal("account must not be premium before payment")
	}

	pay := repo.payments[res.PaymentID]
	body := succeededWebhook(t, *pay.ProviderPaymentID, "pm_token_1")
	if err := svc.HandleWebhook(ctx, http.Header{}, body); err != nil {
		t.Fatalf("webhook: %v", err)
	}

	sub := repo.subs[acc]
	if !sub.GrantsPremium() {
		t.Fatalf("after success sub must grant premium: %+v", sub)
	}
	if repo.plans[acc] != domain.PlanPremium {
		t.Fatal("account plan must be premium")
	}
	if sub.ProviderCustomerToken == nil || *sub.ProviderCustomerToken != "pm_token_1" {
		t.Fatal("customer token must be saved")
	}
	if sub.CurrentPeriodEnd == nil {
		t.Fatal("period end must be set")
	}
	if p := repo.payments[res.PaymentID]; p.Status != domain.PaymentSucceeded || p.ReceiptID == nil {
		t.Fatalf("payment must be succeeded with receipt: %+v", p)
	}
}

func TestWebhookIdempotent(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	ctx := context.Background()
	acc := uuid.New()
	res, _ := svc.Checkout(ctx, acc, domain.PlanPremium, domain.PeriodMonthly, "u")
	pay := repo.payments[res.PaymentID]
	body := succeededWebhook(t, *pay.ProviderPaymentID, "pm_1")

	if err := svc.HandleWebhook(ctx, http.Header{}, body); err != nil {
		t.Fatal(err)
	}
	end1 := *repo.subs[acc].CurrentPeriodEnd
	// Повтор того же события не должен сдвинуть период / задвоить.
	if err := svc.HandleWebhook(ctx, http.Header{}, body); err != nil {
		t.Fatal(err)
	}
	if !repo.subs[acc].CurrentPeriodEnd.Equal(end1) {
		t.Fatal("idempotent webhook must not change period")
	}
}

func TestRecurringRenewExtendsPeriod(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	ctx := context.Background()
	acc := uuid.New()
	res, _ := svc.Checkout(ctx, acc, domain.PlanPremium, domain.PeriodMonthly, "u")
	pay := repo.payments[res.PaymentID]
	_ = svc.HandleWebhook(ctx, http.Header{}, succeededWebhook(t, *pay.ProviderPaymentID, "pm_1"))

	// Сделать период истёкшим.
	sub := repo.subs[acc]
	past := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	sub.CurrentPeriodEnd = &past
	repo.subs[acc] = sub

	n, err := svc.ProcessDue(ctx, 100)
	if err != nil || n != 1 {
		t.Fatalf("process due n=%d err=%v", n, err)
	}
	got := repo.subs[acc]
	if got.Status != domain.SubStatusActive || got.CurrentPeriodEnd == nil || !got.CurrentPeriodEnd.After(past) {
		t.Fatalf("recurring must extend period: %+v", got)
	}
	if repo.plans[acc] != domain.PlanPremium {
		t.Fatal("still premium")
	}
}

func TestDunningExhaustionDowngrades(t *testing.T) {
	repo := newFakeRepo()
	// Провайдер, у которого рекуррент всегда падает.
	svc := NewService(repo, Config{
		Provider:      failingProvider{},
		Pricing:       DefaultPricing(99000, 20, 14, "RUB"),
		MaxDunning:    2,
		RetryInterval: 72 * time.Hour,
	}).WithClock(func() time.Time { return time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC) })
	ctx := context.Background()
	acc := uuid.New()

	// Подготовить активную premium-подписку с токеном и истёкшим периодом.
	past := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	tok := "pm_1"
	repo.subs[acc] = domain.Subscription{
		ID: uuid.New(), AccountID: acc, Plan: domain.PlanPremium, Status: domain.SubStatusActive,
		BillingPeriod: ptrPeriod(domain.PeriodMonthly), ProviderCustomerToken: &tok, CurrentPeriodEnd: &past,
	}
	repo.plans[acc] = domain.PlanPremium

	// 1-я попытка → past_due (grace).
	if _, err := svc.ProcessDue(ctx, 100); err != nil {
		t.Fatal(err)
	}
	if got := repo.subs[acc]; got.Status != domain.SubStatusPastDue || got.DunningAttempts != 1 {
		t.Fatalf("after 1st fail: %+v", got)
	}
	if repo.plans[acc] != domain.PlanPremium {
		t.Fatal("grace: still premium after first failure")
	}

	// Снова сделать период истёкшим и прогнать 2-ю попытку → исчерпание → canceled + free.
	sub := repo.subs[acc]
	sub.CurrentPeriodEnd = &past
	repo.subs[acc] = sub
	if _, err := svc.ProcessDue(ctx, 100); err != nil {
		t.Fatal(err)
	}
	if got := repo.subs[acc]; got.Status != domain.SubStatusCanceled || got.Plan != domain.PlanFree {
		t.Fatalf("after exhaustion: %+v", got)
	}
	if repo.plans[acc] != domain.PlanFree {
		t.Fatal("must downgrade to free")
	}
}

func TestCancelAtPeriodEndDowngrades(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	ctx := context.Background()
	acc := uuid.New()
	past := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	tok := "pm_1"
	repo.subs[acc] = domain.Subscription{
		ID: uuid.New(), AccountID: acc, Plan: domain.PlanPremium, Status: domain.SubStatusActive,
		BillingPeriod: ptrPeriod(domain.PeriodMonthly), ProviderCustomerToken: &tok, CurrentPeriodEnd: &past,
	}
	repo.plans[acc] = domain.PlanPremium

	if _, err := svc.Cancel(ctx, acc); err != nil {
		t.Fatal(err)
	}
	if !repo.subs[acc].CancelAtPeriodEnd {
		t.Fatal("cancel must set cancel_at_period_end")
	}
	if _, err := svc.ProcessDue(ctx, 100); err != nil {
		t.Fatal(err)
	}
	if got := repo.subs[acc]; got.Status != domain.SubStatusCanceled || got.Plan != domain.PlanFree {
		t.Fatalf("canceled sub must downgrade at period end: %+v", got)
	}
}

func ptrPeriod(p domain.BillingPeriod) *domain.BillingPeriod { return &p }

// failingProvider — рекуррент всегда неуспешен.
type failingProvider struct{}

func (failingProvider) Name() domain.PaymentProvider { return domain.ProviderYooKassa }
func (failingProvider) CreatePayment(context.Context, CreatePaymentInput) (CreatePaymentResult, error) {
	return CreatePaymentResult{ProviderPaymentID: uuid.NewString()}, nil
}
func (failingProvider) ChargeRecurring(context.Context, ChargeInput) (ChargeResult, error) {
	return ChargeResult{Status: domain.PaymentFailed}, nil
}
func (failingProvider) ParseWebhook(context.Context, http.Header, []byte) (WebhookEvent, error) {
	return WebhookEvent{}, ErrInvalidWebhook
}
