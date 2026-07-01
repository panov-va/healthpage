package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
)

// StubProvider — провайдер для dev/тестов (без реальных денег). Используется, когда не заданы
// credentials боевого провайдера. CreatePayment выдаёт фейковую ссылку подтверждения,
// ChargeRecurring всегда успешен, ParseWebhook принимает наш нормализованный JSON.
type StubProvider struct {
	BaseURL string // для построения confirmation-URL
}

// Name возвращает yookassa — чтобы dev-флоу совпадал с боевым по имени провайдера в записях.
func (StubProvider) Name() domain.PaymentProvider { return domain.ProviderYooKassa }

func (p StubProvider) CreatePayment(_ context.Context, in CreatePaymentInput) (CreatePaymentResult, error) {
	id := uuid.NewString()
	// Confirmation-URL ведёт на dev-страницу подтверждения public-ssr (имитация виджета).
	return CreatePaymentResult{
		ProviderPaymentID: id,
		ConfirmationURL:   fmt.Sprintf("%s/billing/stub-confirm?payment=%s", in.ReturnURL, id),
	}, nil
}

func (StubProvider) ChargeRecurring(_ context.Context, _ ChargeInput) (ChargeResult, error) {
	id := uuid.NewString()
	return ChargeResult{
		ProviderPaymentID: id,
		Status:            domain.PaymentSucceeded,
		ReceiptID:         "stub-receipt-" + id,
	}, nil
}

// stubWebhookBody — нормализованное тело webhook'а stub-провайдера (имитация уведомления).
type stubWebhookBody struct {
	ProviderPaymentID string `json:"provider_payment_id"`
	Status            string `json:"status"`
	CustomerToken     string `json:"customer_token"`
	ReceiptID         string `json:"receipt_id"`
	AmountMinor       int64  `json:"amount_minor"`
}

func (StubProvider) ParseWebhook(_ context.Context, _ http.Header, body []byte) (WebhookEvent, error) {
	var b stubWebhookBody
	if err := json.Unmarshal(body, &b); err != nil {
		return WebhookEvent{}, ErrInvalidWebhook
	}
	if b.ProviderPaymentID == "" {
		return WebhookEvent{}, ErrInvalidWebhook
	}
	st := domain.PaymentStatus(b.Status)
	if !st.IsValid() {
		return WebhookEvent{}, ErrInvalidWebhook
	}
	return WebhookEvent{
		ProviderPaymentID: b.ProviderPaymentID,
		Status:            st,
		CustomerToken:     b.CustomerToken,
		ReceiptID:         b.ReceiptID,
		AmountMinor:       b.AmountMinor,
	}, nil
}
