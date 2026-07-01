package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/healthpage/backend/internal/domain"
)

// YooKassaProvider — боевой адаптер ЮKassa (DESIGN §4.2). Реальный выпуск платежей и
// рекуррентные списания требуют Shop ID/секрета и согласования рекуррентов с менеджером.
//
// [ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ БИЛЛИНГА]: боевые вызовы (CreatePayment/ChargeRecurring) и приём
// webhook'ов проверены только структурно — реальный прогон на проде (sandbox→prod ЮKassa).
// Проверка подлинности webhook'а у ЮKassa — по IP-allowlist (HMAC не предоставляется);
// для MVP тело парсится как доверенное, ограничение IP — на уровне инфраструктуры/ingress.
type YooKassaProvider struct {
	ShopID    string
	SecretKey string
	APIBase   string // https://api.yookassa.ru/v3
	HTTP      *http.Client
}

// NewYooKassa создаёт адаптер с дефолтным API-базисом и таймаутом.
func NewYooKassa(shopID, secretKey string) *YooKassaProvider {
	return &YooKassaProvider{
		ShopID:    shopID,
		SecretKey: secretKey,
		APIBase:   "https://api.yookassa.ru/v3",
		HTTP:      &http.Client{Timeout: 30 * time.Second},
	}
}

func (YooKassaProvider) Name() domain.PaymentProvider { return domain.ProviderYooKassa }

// minorToDecimal переводит копейки в строку "X.XX" (формат суммы ЮKassa).
func minorToDecimal(minor int64) string {
	return fmt.Sprintf("%d.%02d", minor/100, minor%100)
}

type ykAmount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

type ykPaymentRequest struct {
	Amount            ykAmount          `json:"amount"`
	Capture           bool              `json:"capture"`
	Confirmation      *ykConfirmation   `json:"confirmation,omitempty"`
	SavePaymentMethod bool              `json:"save_payment_method,omitempty"`
	PaymentMethodID   string            `json:"payment_method_id,omitempty"`
	Description       string            `json:"description,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

type ykConfirmation struct {
	Type            string `json:"type"`
	ReturnURL       string `json:"return_url,omitempty"`
	ConfirmationURL string `json:"confirmation_url,omitempty"`
}

type ykPaymentResponse struct {
	ID            string          `json:"id"`
	Status        string          `json:"status"`
	Confirmation  *ykConfirmation `json:"confirmation,omitempty"`
	PaymentMethod *struct {
		ID    string `json:"id"`
		Saved bool   `json:"saved"`
	} `json:"payment_method,omitempty"`
}

func (p *YooKassaProvider) CreatePayment(ctx context.Context, in CreatePaymentInput) (CreatePaymentResult, error) {
	reqBody := ykPaymentRequest{
		Amount:            ykAmount{Value: minorToDecimal(in.AmountMinor), Currency: in.Currency},
		Capture:           true,
		Confirmation:      &ykConfirmation{Type: "redirect", ReturnURL: in.ReturnURL},
		SavePaymentMethod: true,
		Description:       in.Description,
		Metadata:          map[string]string{"account_id": in.AccountID},
	}
	var resp ykPaymentResponse
	if err := p.do(ctx, "/payments", in.IdempotencyKey, reqBody, &resp); err != nil {
		return CreatePaymentResult{}, err
	}
	res := CreatePaymentResult{ProviderPaymentID: resp.ID}
	if resp.Confirmation != nil {
		res.ConfirmationURL = resp.Confirmation.ConfirmationURL
	}
	return res, nil
}

func (p *YooKassaProvider) ChargeRecurring(ctx context.Context, in ChargeInput) (ChargeResult, error) {
	reqBody := ykPaymentRequest{
		Amount:          ykAmount{Value: minorToDecimal(in.AmountMinor), Currency: in.Currency},
		Capture:         true,
		PaymentMethodID: in.CustomerToken,
		Description:     in.Description,
	}
	var resp ykPaymentResponse
	if err := p.do(ctx, "/payments", in.IdempotencyKey, reqBody, &resp); err != nil {
		return ChargeResult{}, err
	}
	return ChargeResult{
		ProviderPaymentID: resp.ID,
		Status:            ykStatus(resp.Status),
	}, nil
}

// ykNotification — тело webhook'а ЮKassa (payment.succeeded/canceled/...).
type ykNotification struct {
	Event  string `json:"event"`
	Object struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		PaymentMethod *struct {
			ID string `json:"id"`
		} `json:"payment_method,omitempty"`
		Amount *ykAmount `json:"amount,omitempty"`
	} `json:"object"`
}

func (p *YooKassaProvider) ParseWebhook(_ context.Context, _ http.Header, body []byte) (WebhookEvent, error) {
	var n ykNotification
	if err := json.Unmarshal(body, &n); err != nil || n.Object.ID == "" {
		return WebhookEvent{}, ErrInvalidWebhook
	}
	ev := WebhookEvent{
		ProviderPaymentID: n.Object.ID,
		Status:            ykStatus(n.Object.Status),
	}
	if n.Object.PaymentMethod != nil {
		ev.CustomerToken = n.Object.PaymentMethod.ID
	}
	if n.Object.Amount != nil {
		ev.AmountMinor = decimalToMinor(n.Object.Amount.Value)
	}
	return ev, nil
}

// ykStatus переводит статус ЮKassa в нормативный PaymentStatus.
func ykStatus(s string) domain.PaymentStatus {
	switch s {
	case "succeeded":
		return domain.PaymentSucceeded
	case "canceled":
		return domain.PaymentFailed
	case "refunded":
		return domain.PaymentRefunded
	default:
		return domain.PaymentPending
	}
}

func decimalToMinor(v string) int64 {
	var rub, kop int64
	_, _ = fmt.Sscanf(v, "%d.%02d", &rub, &kop)
	return rub*100 + kop
}

// do выполняет авторизованный JSON-запрос к API ЮKassa.
func (p *YooKassaProvider) do(ctx context.Context, path, idempotencyKey string, in, out any) error {
	buf, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("billing: marshal yookassa request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.APIBase+path, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("billing: yookassa request: %w", err)
	}
	req.SetBasicAuth(p.ShopID, p.SecretKey)
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotence-Key", idempotencyKey)
	}
	resp, err := p.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("billing: yookassa http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("billing: yookassa status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("billing: decode yookassa response: %w", err)
	}
	return nil
}
