// Package billing инкапсулирует приём платежей за Premium за интерфейсом PaymentProvider
// (DESIGN §4.2 [ТРЕБОВАНИЕ]: доменный код не зависит от конкретного провайдера/статуса
// самозанятости). MVP-провайдер — ЮKassa; для dev/тестов есть stub.
package billing

import (
	"context"
	"errors"
	"net/http"

	"github.com/healthpage/backend/internal/domain"
)

// ErrInvalidWebhook — подпись/формат webhook'а провайдера не прошли проверку.
var ErrInvalidWebhook = errors.New("billing: invalid webhook")

// CreatePaymentInput — параметры инициирования первого платежа (с сохранением способа оплаты
// для последующих рекуррентных списаний).
type CreatePaymentInput struct {
	AmountMinor    int64  // сумма в копейках
	Currency       string // ISO-код, напр. RUB
	Description    string
	ReturnURL      string // куда вернуть клиента после оплаты
	IdempotencyKey string // защита от двойной оплаты
	AccountID      string // в metadata провайдера для сверки
}

// CreatePaymentResult — итог инициирования платежа.
type CreatePaymentResult struct {
	ProviderPaymentID string // идентификатор платежа у провайдера
	ConfirmationURL   string // URL редиректа/виджета оплаты
}

// ChargeInput — параметры рекуррентного списания по сохранённому способу оплаты.
type ChargeInput struct {
	AmountMinor    int64
	Currency       string
	Description    string
	CustomerToken  string // сохранённый токен способа оплаты (НЕ данные карты)
	IdempotencyKey string
}

// ChargeResult — итог рекуррентного списания.
type ChargeResult struct {
	ProviderPaymentID string
	Status            domain.PaymentStatus
	ReceiptID         string
}

// WebhookEvent — нормализованное событие провайдера (succeeded/failed/refunded).
type WebhookEvent struct {
	ProviderPaymentID string
	Status            domain.PaymentStatus
	CustomerToken     string // токен способа оплаты (приходит при сохранении)
	ReceiptID         string
	AmountMinor       int64
}

// Provider — абстракция платёжного провайдера (DESIGN §4.2).
type Provider interface {
	Name() domain.PaymentProvider
	// CreatePayment инициирует первый платёж (с сохранением способа оплаты).
	CreatePayment(ctx context.Context, in CreatePaymentInput) (CreatePaymentResult, error)
	// ChargeRecurring списывает по сохранённому токену (автопродление).
	ChargeRecurring(ctx context.Context, in ChargeInput) (ChargeResult, error)
	// ParseWebhook проверяет подпись/принадлежность и нормализует уведомление провайдера.
	ParseWebhook(ctx context.Context, header http.Header, body []byte) (WebhookEvent, error)
}

// Pricing — тарифные параметры (плейсхолдер, конфигурируется; финализируется перед запуском:
// цена Статусмейта −10%, DESIGN §10). Суммы в копейках.
type Pricing struct {
	MonthlyMinor int64
	YearlyMinor  int64
	Currency     string
	TrialDays    int
}

// AmountMinor возвращает сумму к списанию за период.
func (p Pricing) AmountMinor(period domain.BillingPeriod) int64 {
	if period == domain.PeriodYearly {
		return p.YearlyMinor
	}
	return p.MonthlyMinor
}

// SelectProvider возвращает боевой ЮKassa-адаптер при наличии credentials, иначе stub (dev).
func SelectProvider(shopID, secretKey, baseURL string) Provider {
	if shopID != "" && secretKey != "" {
		return NewYooKassa(shopID, secretKey)
	}
	return StubProvider{BaseURL: baseURL}
}

// DefaultPricing строит тарифы из месячной цены и годовой скидки (в процентах).
func DefaultPricing(monthlyMinor int64, yearlyDiscountPct, trialDays int, currency string) Pricing {
	yearly := monthlyMinor * 12 * int64(100-yearlyDiscountPct) / 100
	return Pricing{MonthlyMinor: monthlyMinor, YearlyMinor: yearly, Currency: currency, TrialDays: trialDays}
}
