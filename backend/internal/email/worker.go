package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/notify"
	"github.com/healthpage/backend/internal/store"
	"github.com/healthpage/backend/internal/subscription"
)

// Disposition — решение воркера по сообщению, применяемое к AMQP-доставке вызывающим.
type Disposition int

const (
	Ack     Disposition = iota // подтвердить (доставлено / нечего делать / ретрай запланирован)
	Reject                     // Nack(requeue=false) → DLQ (битое/неустранимое/исчерпан ретрай)
	Requeue                    // Nack(requeue=true) → повторить (транзиентная инфра-ошибка)
)

// WorkerStore — то, что воркеру нужно от хранилища.
type WorkerStore interface {
	NotificationByID(ctx context.Context, id uuid.UUID) (domain.Notification, error)
	MarkNotificationSent(ctx context.Context, id uuid.UUID) error
	StatusPageByID(ctx context.Context, id uuid.UUID) (domain.StatusPage, error)
}

// Retrier републикует сообщение с backoff (notify.Engine). scheduled=false — ретраи исчерпаны.
type Retrier interface {
	Retry(ctx context.Context, msg notify.Message) (scheduled bool, err error)
}

// Worker обрабатывает одно сообщение q.email: идемпотентность по Notification.id, рендер,
// отправка через Sender, отметка sent или планирование ретрая (DESIGN §8.1).
type Worker struct {
	store      WorkerStore
	sender     Sender
	retrier    Retrier
	systemSMTP SMTP   // дефолтный отправитель (если у страницы нет своего)
	baseURL    string // для ссылок confirm/unsubscribe (эндпоинты API)
	secret     string // секрет HMAC-токена отписки
	log        *slog.Logger
}

// NewWorker собирает воркера. logger=nil → slog.Default().
func NewWorker(st WorkerStore, sender Sender, retrier Retrier, systemSMTP SMTP, baseURL, secret string, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{store: st, sender: sender, retrier: retrier, systemSMTP: systemSMTP, baseURL: baseURL, secret: secret, log: logger}
}

// Process обрабатывает тело сообщения и возвращает решение по доставке.
func (w *Worker) Process(ctx context.Context, body []byte) Disposition {
	var msg notify.Message
	if err := json.Unmarshal(body, &msg); err != nil {
		w.log.Error("email: malformed message", "err", err)
		return Reject
	}
	nid, err := uuid.Parse(msg.NotificationID)
	if err != nil {
		w.log.Error("email: bad notification_id", "id", msg.NotificationID, "err", err)
		return Reject
	}

	n, err := w.store.NotificationByID(ctx, nid)
	if errors.Is(err, store.ErrNotFound) {
		w.log.Warn("email: orphan notification, dropping", "id", nid)
		return Ack
	}
	if err != nil {
		w.log.Error("email: load notification", "id", nid, "err", err)
		return Requeue
	}
	// Идемпотентность: уже доставлено — ничего не делаем (защита от повторной доставки брокером).
	if n.Status == domain.NotificationSent {
		w.log.Info("email: already sent, skipping", "id", nid)
		return Ack
	}

	content, cfg, err := w.build(ctx, msg)
	if err != nil {
		w.log.Error("email: build", "id", nid, "event", msg.Event, "err", err)
		return Reject
	}

	if err := w.sender.Send(ctx, cfg, Email{
		To: msg.Address, Subject: content.Subject, TextBody: content.TextBody, HTMLBody: content.HTMLBody,
	}); err != nil {
		w.log.Warn("email: send failed, scheduling retry", "id", nid, "err", err)
		return w.handleSendFailure(ctx, msg)
	}

	if err := w.store.MarkNotificationSent(ctx, nid); err != nil {
		// Письмо ушло, но отметку не записали: повторим — повторная доставка письма не критична,
		// а NotificationByID на следующем заходе всё равно покажет pending.
		w.log.Error("email: mark sent", "id", nid, "err", err)
		return Requeue
	}
	w.log.Info("email: sent", "id", nid, "to", msg.Address, "event", msg.Event)
	return Ack
}

// handleSendFailure планирует ретрай через delayed exchange либо отправляет в DLQ при исчерпании.
func (w *Worker) handleSendFailure(ctx context.Context, msg notify.Message) Disposition {
	scheduled, err := w.retrier.Retry(ctx, msg)
	if err != nil {
		w.log.Error("email: schedule retry", "id", msg.NotificationID, "err", err)
		return Requeue
	}
	if scheduled {
		return Ack // отложенная копия придёт позже; текущую доставку подтверждаем
	}
	w.log.Warn("email: retries exhausted → DLQ", "id", msg.NotificationID)
	return Reject
}

// build загружает страницу, рендерит письмо и выбирает SMTP (кастомный страницы или системный).
func (w *Worker) build(ctx context.Context, msg notify.Message) (Content, SMTP, error) {
	pageID, err := uuid.Parse(msg.StatusPageID)
	if err != nil {
		return Content{}, SMTP{}, fmt.Errorf("email: bad status_page_id: %w", err)
	}
	page, err := w.store.StatusPageByID(ctx, pageID)
	if err != nil {
		return Content{}, SMTP{}, fmt.Errorf("email: load page: %w", err)
	}

	in := RenderInput{
		Event:    domain.EventType(msg.Event),
		Locale:   page.DefaultLocale,
		PageName: page.Name,
		PageURL:  w.baseURL + "/status/" + page.Slug,
	}

	switch domain.EventType(msg.Event) {
	case domain.EventSubscriberConfirm:
		var p notify.ConfirmPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			return Content{}, SMTP{}, fmt.Errorf("email: confirm payload: %w", err)
		}
		in.ConfirmURL = w.baseURL + "/api/v1/subscribe/confirm?token=" + url.QueryEscape(p.ConfirmToken)
	case domain.EventIncidentNew, domain.EventIncidentUpdate:
		var p notify.IncidentPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			return Content{}, SMTP{}, fmt.Errorf("email: incident payload: %w", err)
		}
		in.Incident = &p
		in.UnsubscribeURL = w.unsubscribeURL(msg.SubscriberID)
	case domain.EventMaintenanceScheduled, domain.EventMaintenanceStarted, domain.EventMaintenanceCompleted:
		var p notify.MaintenancePayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			return Content{}, SMTP{}, fmt.Errorf("email: maintenance payload: %w", err)
		}
		in.Maintenance = &p
		in.UnsubscribeURL = w.unsubscribeURL(msg.SubscriberID)
	}

	content, err := Render(in)
	if err != nil {
		return Content{}, SMTP{}, err
	}
	return content, w.effectiveSMTP(page), nil
}

// unsubscribeURL строит ссылку отписки с HMAC-токеном (пустая при некорректном subscriber_id).
func (w *Worker) unsubscribeURL(subscriberID string) string {
	sid, err := uuid.Parse(subscriberID)
	if err != nil {
		return ""
	}
	return w.baseURL + "/api/v1/unsubscribe?token=" + url.QueryEscape(subscription.UnsubscribeToken(w.secret, sid))
}

// effectiveSMTP выбирает SMTP страницы (если задан её smtp_config), иначе системный.
func (w *Worker) effectiveSMTP(page domain.StatusPage) SMTP {
	if len(page.SMTPConfig) > 0 {
		var c SMTP
		if err := json.Unmarshal(page.SMTPConfig, &c); err == nil && !c.IsZero() {
			if c.From == "" && page.FromEmail != nil {
				c.From = *page.FromEmail
			}
			return c
		}
	}
	cfg := w.systemSMTP
	if page.FromEmail != nil && *page.FromEmail != "" {
		cfg.From = *page.FromEmail
	}
	return cfg
}
