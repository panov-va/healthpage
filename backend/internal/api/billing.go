package api

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/healthpage/backend/internal/billing"
	"github.com/healthpage/backend/internal/domain"
)

// ── DTO биллинга (синхронны с openapi Subscription / Payment / CheckoutRequest / CheckoutResponse) ──

type subscriptionResponse struct {
	ID                 string  `json:"id"`
	Plan               string  `json:"plan"`
	BillingPeriod      *string `json:"billing_period"`
	Status             string  `json:"status"`
	Provider           *string `json:"provider"`
	CurrentPeriodStart *string `json:"current_period_start"`
	CurrentPeriodEnd   *string `json:"current_period_end"`
	CancelAtPeriodEnd  bool    `json:"cancel_at_period_end"`
}

func toSubscriptionResponse(s domain.Subscription) subscriptionResponse {
	out := subscriptionResponse{
		ID:                s.ID.String(),
		Plan:              string(s.Plan),
		Status:            string(s.Status),
		CancelAtPeriodEnd: s.CancelAtPeriodEnd,
	}
	if s.BillingPeriod != nil {
		v := string(*s.BillingPeriod)
		out.BillingPeriod = &v
	}
	if s.Provider != nil {
		v := string(*s.Provider)
		out.Provider = &v
	}
	out.CurrentPeriodStart = rfc3339Ptr(s.CurrentPeriodStart)
	out.CurrentPeriodEnd = rfc3339Ptr(s.CurrentPeriodEnd)
	return out
}

type paymentResponse struct {
	ID                string  `json:"id"`
	Amount            float64 `json:"amount"`
	Currency          string  `json:"currency"`
	Status            string  `json:"status"`
	Provider          *string `json:"provider"`
	ProviderPaymentID *string `json:"provider_payment_id"`
	ReceiptID         *string `json:"receipt_id"`
	CreatedAt         string  `json:"created_at"`
	PaidAt            *string `json:"paid_at"`
}

func toPaymentResponse(p domain.Payment) paymentResponse {
	out := paymentResponse{
		ID:                p.ID.String(),
		Amount:            float64(p.AmountMinor) / 100,
		Currency:          p.Currency,
		Status:            string(p.Status),
		ProviderPaymentID: p.ProviderPaymentID,
		ReceiptID:         p.ReceiptID,
		CreatedAt:         p.CreatedAt.UTC().Format(time.RFC3339),
		PaidAt:            rfc3339Ptr(p.PaidAt),
	}
	if p.Provider != nil {
		v := string(*p.Provider)
		out.Provider = &v
	}
	return out
}

type checkoutRequest struct {
	Plan          string `json:"plan"`
	BillingPeriod string `json:"billing_period"`
}

type checkoutResponse struct {
	PaymentID       string `json:"payment_id"`
	ConfirmationURL string `json:"confirmation_url,omitempty"`
	Provider        string `json:"provider,omitempty"`
}

// operatorAccount извлекает оператора и его аккаунт. При ошибке/доступе пишет ответ, ok=false.
func (s *server) operatorAccount(w http.ResponseWriter, r *http.Request) (domain.Account, bool) {
	user, ok := requireOperator(w, r)
	if !ok {
		return domain.Account{}, false
	}
	acc, err := s.store.AccountByOwner(r.Context(), user.ID)
	if err != nil {
		writeServerError(w, err)
		return domain.Account{}, false
	}
	return acc, true
}

// handleGetSubscription возвращает текущую подписку аккаунта (создавая дефолтную free).
func (s *server) handleGetSubscription(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.operatorAccount(w, r)
	if !ok {
		return
	}
	sub, err := s.store.EnsureSubscription(r.Context(), acc.ID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toSubscriptionResponse(sub))
}

// handleCheckout инициирует оплату Premium и возвращает параметры подтверждения провайдера.
func (s *server) handleCheckout(w http.ResponseWriter, r *http.Request) {
	if s.billing == nil {
		writeError(w, http.StatusServiceUnavailable, "billing_disabled", "биллинг не настроен")
		return
	}
	acc, ok := s.operatorAccount(w, r)
	if !ok {
		return
	}
	var req checkoutRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	plan := domain.BillingPlan(req.Plan)
	period := domain.BillingPeriod(req.BillingPeriod)
	if plan != domain.PlanPremium {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "оплачивается только тариф premium")
		return
	}
	if !period.IsValid() {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый billing_period (monthly|yearly)")
		return
	}
	// Куда вернуть клиента после оплаты — публичная страница биллинга админки.
	returnURL := s.baseURL + "/billing"
	res, err := s.billing.Checkout(r.Context(), acc.ID, plan, period, returnURL)
	if err != nil {
		if errors.Is(err, billing.ErrInvalidPlan) {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый план/период")
			return
		}
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, checkoutResponse{
		PaymentID:       res.PaymentID.String(),
		ConfirmationURL: res.ConfirmationURL,
		Provider:        string(res.Provider),
	})
}

// handleCancelSubscription отключает автопродление (Premium сохраняется до конца периода).
func (s *server) handleCancelSubscription(w http.ResponseWriter, r *http.Request) {
	if s.billing == nil {
		writeError(w, http.StatusServiceUnavailable, "billing_disabled", "биллинг не настроен")
		return
	}
	acc, ok := s.operatorAccount(w, r)
	if !ok {
		return
	}
	sub, err := s.billing.Cancel(r.Context(), acc.ID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toSubscriptionResponse(sub))
}

// handleListPayments возвращает историю платежей аккаунта (новые сверху, пагинация).
func (s *server) handleListPayments(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.operatorAccount(w, r)
	if !ok {
		return
	}
	_, perPage, offset := parsePagination(r)
	payments, err := s.store.ListPaymentsByAccount(r.Context(), acc.ID, perPage, offset)
	if err != nil {
		writeServerError(w, err)
		return
	}
	out := make([]paymentResponse, len(payments))
	for i, p := range payments {
		out[i] = toPaymentResponse(p)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleBillingWebhook принимает уведомление платёжного провайдера. Публичный (без JWT):
// подлинность проверяет провайдер (подпись/IP). Идемпотентно по provider_payment_id.
func (s *server) handleBillingWebhook(w http.ResponseWriter, r *http.Request) {
	provider := domain.PaymentProvider(chi.URLParam(r, "provider"))
	if !provider.IsValid() {
		writeError(w, http.StatusBadRequest, "bad_request", "неизвестный провайдер")
		return
	}
	if s.billing == nil {
		// Биллинг выключен — принимаем и игнорируем (не копим ретраи на стороне провайдера).
		w.WriteHeader(http.StatusAccepted)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "не удалось прочитать тело")
		return
	}
	if err := s.billing.HandleWebhook(r.Context(), r.Header, body); err != nil {
		if errors.Is(err, billing.ErrInvalidWebhook) {
			writeError(w, http.StatusBadRequest, "bad_request", "некорректный webhook")
			return
		}
		writeServerError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
