package domain

import (
	"time"

	"github.com/google/uuid"
)

// ── Биллинг и тарифы (DESIGN §4.2, §10; этап 6) ──
//
// Эффективный флаг тарифа аккаунта — Account.BillingPlan; Subscription несёт жизненный цикл
// (статус/период/триал/токен рекуррента). Нормативные наборы значений — openapi
// (SubscriptionStatus / PaymentStatus / PaymentProvider / BillingPeriod).

// SubscriptionStatus — статус подписки (openapi SubscriptionStatus).
type SubscriptionStatus string

const (
	SubStatusPending  SubscriptionStatus = "pending"  // оплата инициирована, ещё не подтверждена
	SubStatusActive   SubscriptionStatus = "active"   // оплачена/в триале, фичи Premium доступны
	SubStatusPastDue  SubscriptionStatus = "past_due" // рекуррент не прошёл, идёт dunning (grace)
	SubStatusCanceled SubscriptionStatus = "canceled" // отменена/исчерпан dunning, откат на Free
)

// IsValid сообщает, входит ли значение в нормативный набор.
func (s SubscriptionStatus) IsValid() bool {
	switch s {
	case SubStatusPending, SubStatusActive, SubStatusPastDue, SubStatusCanceled:
		return true
	default:
		return false
	}
}

// PaymentStatus — статус платежа (openapi PaymentStatus).
type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "pending"
	PaymentSucceeded PaymentStatus = "succeeded"
	PaymentFailed    PaymentStatus = "failed"
	PaymentRefunded  PaymentStatus = "refunded"
)

// IsValid сообщает, входит ли значение в нормативный набор.
func (s PaymentStatus) IsValid() bool {
	switch s {
	case PaymentPending, PaymentSucceeded, PaymentFailed, PaymentRefunded:
		return true
	default:
		return false
	}
}

// PaymentProvider — платёжный провайдер (openapi PaymentProvider). MVP: yookassa.
type PaymentProvider string

const (
	ProviderYooKassa      PaymentProvider = "yookassa"
	ProviderTinkoff       PaymentProvider = "tinkoff"
	ProviderCloudPayments PaymentProvider = "cloudpayments"
	ProviderRobokassa     PaymentProvider = "robokassa"
)

// AllPaymentProviders — нормативный набор (для валидации пути webhook'а).
var AllPaymentProviders = []PaymentProvider{
	ProviderYooKassa, ProviderTinkoff, ProviderCloudPayments, ProviderRobokassa,
}

// IsValid сообщает, входит ли значение в нормативный набор.
func (p PaymentProvider) IsValid() bool {
	switch p {
	case ProviderYooKassa, ProviderTinkoff, ProviderCloudPayments, ProviderRobokassa:
		return true
	default:
		return false
	}
}

// BillingPeriod — периодичность списания (openapi BillingPeriod).
type BillingPeriod string

const (
	PeriodMonthly BillingPeriod = "monthly"
	PeriodYearly  BillingPeriod = "yearly"
)

// IsValid сообщает, входит ли значение в нормативный набор.
func (p BillingPeriod) IsValid() bool { return p == PeriodMonthly || p == PeriodYearly }

// Advance возвращает момент окончания следующего периода от from.
func (p BillingPeriod) Advance(from time.Time) time.Time {
	if p == PeriodYearly {
		return from.AddDate(1, 0, 0)
	}
	return from.AddDate(0, 1, 0)
}

// Subscription — подписка аккаунта (одна на аккаунт).
type Subscription struct {
	ID                    uuid.UUID
	AccountID             uuid.UUID
	Plan                  BillingPlan
	Status                SubscriptionStatus
	BillingPeriod         *BillingPeriod
	Provider              *PaymentProvider
	ProviderCustomerToken *string // токен рекуррентного списания (НЕ данные карты)
	TrialEndsAt           *time.Time
	CurrentPeriodStart    *time.Time
	CurrentPeriodEnd      *time.Time
	CancelAtPeriodEnd     bool
	DunningAttempts       int
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// GrantsPremium сообщает, должны ли быть включены Premium-фичи: план premium и подписка
// не отменена (active или past_due — в past_due действует grace-период dunning'а).
func (s Subscription) GrantsPremium() bool {
	return s.Plan == PlanPremium && (s.Status == SubStatusActive || s.Status == SubStatusPastDue)
}

// InTrial сообщает, активен ли пробный период на момент now.
func (s Subscription) InTrial(now time.Time) bool {
	return s.TrialEndsAt != nil && now.Before(*s.TrialEndsAt)
}

// Payment — запись журнала платежей. Сумма в минорных единицах (копейки).
type Payment struct {
	ID                uuid.UUID
	AccountID         uuid.UUID
	SubscriptionID    *uuid.UUID
	AmountMinor       int64
	Currency          string
	Status            PaymentStatus
	Provider          *PaymentProvider
	ProviderPaymentID *string
	IdempotencyKey    *string
	ReceiptID         *string
	BillingPeriod     *BillingPeriod
	CreatedAt         time.Time
	PaidAt            *time.Time
}
