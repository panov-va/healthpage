package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/healthpage/backend/internal/auth"
	"github.com/healthpage/backend/internal/billing"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
)

// Интеграционный тест биллинга (этап 6) на реальном PG со stub-провайдером:
// checkout → webhook успеха → Premium активен → premium-гейтинг открывается → cancel →
// идемпотентность webhook'а → история платежей. Запуск:
//
//	HEALTHPAGE_TEST_DB="postgres://healthpage:healthpage@localhost:5432/healthpage?sslmode=disable" \
//	  go test ./internal/api/ -run TestBillingIntegration
func TestBillingIntegration(t *testing.T) {
	dsn := mustTestDSN(t)
	ctx := context.Background()

	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	defer st.Close()
	raw, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("raw pool: %v", err)
	}
	defer raw.Close()

	billingSvc := billing.NewService(st, billing.Config{
		Provider: billing.StubProvider{BaseURL: "http://localhost:3000"},
		Pricing:  billing.DefaultPricing(99000, 20, 14, "RUB"),
	})

	tm, _ := security.NewTokenManager("test-secret", 15*time.Minute, time.Hour)
	srv := httptest.NewServer(NewRouter(Deps{
		Auth: auth.NewService(st, tm), Store: st, Billing: billingSvc,
		BaseURL: "http://localhost:8080", RefreshTTL: time.Hour,
	}))
	defer srv.Close()

	var cleanup []uuid.UUID
	t.Cleanup(func() {
		for _, uid := range cleanup {
			_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", uid)
			_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", uid)
		}
	})

	var reg authResultResponse
	doJSON(t, srv.URL+"/api/v1/auth/register", "",
		map[string]string{"email": "bill-" + uuid.NewString() + "@example.test", "password": "supersecret"},
		http.StatusCreated, &reg)
	uid, _ := uuid.Parse(reg.User.ID)
	cleanup = append(cleanup, uid)
	token := reg.AccessToken

	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", token,
		map[string]string{"name": "Bill Co", "slug": "bill-" + uuid.NewString()[:8]}, http.StatusCreated, &page)

	// 1. Дефолтная подписка — free/active.
	var sub subscriptionResponse
	doJSON(t, srv.URL+"/api/v1/billing/subscription", token, nil, http.StatusOK, &sub)
	if sub.Plan != "free" || sub.Status != "active" {
		t.Fatalf("default sub: %+v", sub)
	}

	// Домен уникален на прогон (глобальный unique-индекс среди живых страниц).
	customDomain := "status-" + uuid.NewString()[:8] + ".test"

	// 2. На Free premium-фича заблокирована (этап 6.7) → 403.
	doStatusBody(t, http.MethodPatch, srv.URL+"/api/v1/pages/"+page.ID, token,
		map[string]any{"custom_domain": customDomain}, http.StatusForbidden)

	// 3. Checkout Premium (monthly) → подписка pending, аккаунт ещё free.
	var co checkoutResponse
	doJSON(t, srv.URL+"/api/v1/billing/checkout", token,
		map[string]string{"plan": "premium", "billing_period": "monthly"}, http.StatusOK, &co)
	if co.PaymentID == "" || co.ConfirmationURL == "" || co.Provider != "yookassa" {
		t.Fatalf("checkout: %+v", co)
	}
	doJSON(t, srv.URL+"/api/v1/billing/subscription", token, nil, http.StatusOK, &sub)
	if sub.Status != "pending" {
		t.Fatalf("after checkout sub must be pending: %+v", sub)
	}

	// 4. Узнать provider_payment_id из истории платежей.
	var payments []paymentResponse
	doJSON(t, srv.URL+"/api/v1/billing/payments", token, nil, http.StatusOK, &payments)
	if len(payments) != 1 || payments[0].ProviderPaymentID == nil {
		t.Fatalf("payments after checkout: %+v", payments)
	}
	if payments[0].Status != "pending" || payments[0].Amount != 990 {
		t.Fatalf("pending payment: %+v", payments[0])
	}
	providerPaymentID := *payments[0].ProviderPaymentID

	// 5. Webhook успешной оплаты (публичный, без auth) → 202.
	webhook := map[string]any{
		"provider_payment_id": providerPaymentID,
		"status":              "succeeded",
		"customer_token":      "pm_token_1",
		"receipt_id":          "rcpt-1",
		"amount_minor":        99000,
	}
	doJSON(t, srv.URL+"/api/v1/billing/webhook/yookassa", "", webhook, http.StatusAccepted, nil)

	// 6. Подписка стала Premium/active с периодом.
	doJSON(t, srv.URL+"/api/v1/billing/subscription", token, nil, http.StatusOK, &sub)
	if sub.Plan != "premium" || sub.Status != "active" || sub.CurrentPeriodEnd == nil {
		t.Fatalf("after webhook sub: %+v", sub)
	}

	// 7. Теперь premium-гейтинг открыт: PATCH custom_domain → 200.
	patchJSON(t, srv.URL+"/api/v1/pages/"+page.ID, token,
		map[string]any{"custom_domain": customDomain}, http.StatusOK, nil)

	// 8. Платёж отмечен succeeded с чеком.
	doJSON(t, srv.URL+"/api/v1/billing/payments", token, nil, http.StatusOK, &payments)
	if payments[0].Status != "succeeded" || payments[0].ReceiptID == nil || payments[0].PaidAt == nil {
		t.Fatalf("succeeded payment: %+v", payments[0])
	}

	// 9. Идемпотентность: повтор того же webhook'а не меняет период.
	end1 := *sub.CurrentPeriodEnd
	doJSON(t, srv.URL+"/api/v1/billing/webhook/yookassa", "", webhook, http.StatusAccepted, nil)
	doJSON(t, srv.URL+"/api/v1/billing/subscription", token, nil, http.StatusOK, &sub)
	if *sub.CurrentPeriodEnd != end1 {
		t.Fatal("idempotent webhook must not change period")
	}

	// 10. Cancel → автопродление выключено (Premium до конца периода).
	doJSON(t, srv.URL+"/api/v1/billing/cancel", token, map[string]any{}, http.StatusOK, &sub)
	if !sub.CancelAtPeriodEnd || sub.Plan != "premium" {
		t.Fatalf("after cancel: %+v", sub)
	}

	// 11. Неизвестный провайдер в webhook → 400.
	doStatusBody(t, http.MethodPost, srv.URL+"/api/v1/billing/webhook/unknownpay", "",
		map[string]any{"x": 1}, http.StatusBadRequest)
}
