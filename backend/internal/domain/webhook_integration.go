package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ── Входящая webhook-интеграция (DESIGN §5, этап 5.3) ──

// WebhookSource — источник входящего webhook'а (нормативный набор, openapi WebhookIntegrationSource;
// зеркалит CHECK в миграции 00012).
type WebhookSource string

const (
	SourceGrafana    WebhookSource = "grafana"
	SourcePrometheus WebhookSource = "prometheus"
	SourcePagerDuty  WebhookSource = "pagerduty"
	SourceGeneric    WebhookSource = "generic"
)

// AllWebhookSources — все допустимые значения.
var AllWebhookSources = []WebhookSource{SourceGrafana, SourcePrometheus, SourcePagerDuty, SourceGeneric}

// IsValid сообщает, входит ли значение в нормативный набор.
func (s WebhookSource) IsValid() bool {
	switch s {
	case SourceGrafana, SourcePrometheus, SourcePagerDuty, SourceGeneric:
		return true
	default:
		return false
	}
}

// Implemented сообщает, реализован ли парсер источника на этапе 5.3 (grafana/prometheus).
// generic и pagerduty отложены (роуты возвращают 501).
func (s WebhookSource) Implemented() bool {
	return s == SourceGrafana || s == SourcePrometheus
}

// WebhookIntegration — настройка входящей интеграции страницы.
// Secret хранится в открытом виде (нужен для проверки HMAC); API его в list/get не возвращает.
type WebhookIntegration struct {
	ID               uuid.UUID
	StatusPageID     uuid.UUID
	Source           WebhookSource
	Name             string
	Secret           string
	ComponentMapping json.RawMessage
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
