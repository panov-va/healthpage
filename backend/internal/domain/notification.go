package domain

import (
	"time"

	"github.com/google/uuid"
)

// ── Статус уведомления ──

// NotificationStatus — стадия обработки записи журнала уведомлений
// (значения зеркалят CHECK в миграции 00008).
type NotificationStatus string

const (
	NotificationPending NotificationStatus = "pending" // создано, ещё не доставлено
	NotificationSent    NotificationStatus = "sent"    // успешно доставлено воркером
	NotificationFailed  NotificationStatus = "failed"  // исчерпаны ретраи (ушло в DLQ)
)

// IsValid сообщает, входит ли значение в допустимый набор.
func (s NotificationStatus) IsValid() bool {
	switch s {
	case NotificationPending, NotificationSent, NotificationFailed:
		return true
	default:
		return false
	}
}

// ── Типы событий ──

// EventType — тип доменного события, порождающего уведомления (часть routing key
// notify.<channel>.<event>, DESIGN §8.1). Хранится в notifications.event_type.
type EventType string

const (
	EventIncidentNew          EventType = "incident_new"          // создан новый инцидент
	EventIncidentUpdate       EventType = "incident_update"       // обновление инцидента (с notify)
	EventMaintenanceScheduled EventType = "maintenance_scheduled" // запланированы работы
	EventMaintenanceStarted   EventType = "maintenance_started"   // работы начались (in_progress)
	EventMaintenanceCompleted EventType = "maintenance_completed" // работы завершены
	EventSubscriberConfirm    EventType = "subscriber_confirm"    // письмо double opt-in (этап 3.5)
	EventAccessLink           EventType = "access_link"           // magic-link доступа к приватной странице (4.2.1)
)

// ── Сущность ──

// Notification — запись журнала отправок: создаётся движком на каждого подписчика по событию,
// служит ключом идемпотентности (Notification.id) и счётчиком ретраев (DESIGN §8.1).
// Воркер канала помечает Sent после успешной доставки.
type Notification struct {
	ID           uuid.UUID
	SubscriberID uuid.UUID
	EventType    EventType
	Payload      []byte // JSON содержимого события (для восстановления сообщения)
	Status       NotificationStatus
	Attempts     int
	SentAt       *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
