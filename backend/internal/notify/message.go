package notify

import (
	"encoding/json"
	"time"

	"github.com/healthpage/backend/internal/domain"
)

// Message — формат сообщения в очереди уведомлений (тело AMQP-сообщения, публикуемого в
// notifications/delayed.events). Воркер канала (этап 3.4+) десериализует его, проверяет по
// NotificationID идемпотентность (не была ли запись уже sent) и доставляет на Address.
type Message struct {
	NotificationID string          `json:"notification_id"` // ключ идемпотентности (Notification.id)
	SubscriberID   string          `json:"subscriber_id"`
	Channel        string          `json:"channel"`
	Event          string          `json:"event"`
	Address        string          `json:"address"`
	Attempt        int             `json:"attempt"` // номер попытки (0 — первичная публикация)
	Payload        json.RawMessage `json:"payload"` // содержимое события (см. *Payload)
	StatusPageID   string          `json:"status_page_id"`
}

// IncidentPayload — содержимое уведомления об инциденте (incident_new / incident_update).
type IncidentPayload struct {
	IncidentID string   `json:"incident_id"`
	Title      string   `json:"title"`
	Status     string   `json:"status"`
	Impact     string   `json:"impact"`
	Body       string   `json:"body"`                 // текст обновления / стартового сообщения
	Components []string `json:"components,omitempty"` // id затронутых компонентов
}

// MaintenancePayload — содержимое уведомления о плановых работах
// (maintenance_scheduled / maintenance_started / maintenance_completed).
type MaintenancePayload struct {
	MaintenanceID  string    `json:"maintenance_id"`
	Title          string    `json:"title"`
	Description    string    `json:"description,omitempty"`
	Status         string    `json:"status"`
	ScheduledStart time.Time `json:"scheduled_start"`
	ScheduledEnd   time.Time `json:"scheduled_end"`
	Components     []string  `json:"components,omitempty"`
}

// ConfirmPayload — содержимое письма double opt-in (subscriber_confirm). Публикуется флоу
// подписки (этап 3.5), несёт plaintext-токен подтверждения (в БД хранится только его хэш, §9).
type ConfirmPayload struct {
	ConfirmToken string `json:"confirm_token"`
}

// AccessLinkPayload — содержимое письма magic-link доступа к приватной странице (4.2.1).
// Несёт токен; ссылку верификации воркер строит из публичного baseURL + slug.
type AccessLinkPayload struct {
	Token string `json:"token"`
}

func incidentPayload(inc domain.Incident, body string) IncidentPayload {
	comps := make([]string, len(inc.Components))
	for i, c := range inc.Components {
		comps[i] = c.ComponentID.String()
	}
	return IncidentPayload{
		IncidentID: inc.ID.String(),
		Title:      inc.Title,
		Status:     string(inc.CurrentStatus),
		Impact:     string(inc.Impact),
		Body:       body,
		Components: comps,
	}
}

func maintenancePayload(m domain.Maintenance) MaintenancePayload {
	comps := make([]string, len(m.ComponentIDs))
	for i, c := range m.ComponentIDs {
		comps[i] = c.String()
	}
	return MaintenancePayload{
		MaintenanceID:  m.ID.String(),
		Title:          m.Title,
		Description:    m.Description,
		Status:         string(m.Status),
		ScheduledStart: m.ScheduledStart,
		ScheduledEnd:   m.ScheduledEnd,
		Components:     comps,
	}
}
