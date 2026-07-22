package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/notify"
	"github.com/healthpage/backend/internal/store"
)

// Disposition — решение воркера по сообщению, применяемое к AMQP-доставке вызывающим.
// (Зеркалит email/telegram; держим локально — пакеты каналов независимы.)
type Disposition int

const (
	Ack     Disposition = iota // подтвердить (доставлено / нечего делать / ретрай запланирован)
	Reject                     // Nack(requeue=false) → DLQ (битое/неустранимое/исчерпан ретрай)
	Requeue                    // Nack(requeue=true) → повторить (транзиентная инфра-ошибка)
)

// Poster отправляет payload в incoming-webhook URL. Реализуется *Client; фейк — в тестах.
type Poster interface {
	PostMessage(ctx context.Context, webhookURL string, payload []byte) error
}

// WorkerStore — то, что воркеру нужно от хранилища (журнал + страница).
type WorkerStore interface {
	NotificationByID(ctx context.Context, id uuid.UUID) (domain.Notification, error)
	MarkNotificationSent(ctx context.Context, id uuid.UUID) error
	StatusPageByID(ctx context.Context, id uuid.UUID) (domain.StatusPage, error)
}

// Retrier републикует сообщение с backoff (notify.Engine). scheduled=false — ретраи исчерпаны.
type Retrier interface {
	Retry(ctx context.Context, msg notify.Message) (scheduled bool, err error)
}

// Worker обрабатывает одно сообщение q.slack: идемпотентность по Notification.id, рендер Block Kit,
// POST в webhook URL, отметка sent или планирование ретрая (DESIGN §8.1).
type Worker struct {
	store     WorkerStore
	poster    Poster
	retrier   Retrier
	publicURL string
	log       *slog.Logger
}

// NewWorker собирает воркера. logger=nil → slog.Default().
func NewWorker(st WorkerStore, poster Poster, retrier Retrier, publicURL string, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{store: st, poster: poster, retrier: retrier, publicURL: publicURL, log: logger}
}

// Process обрабатывает тело сообщения и возвращает решение по доставке.
func (w *Worker) Process(ctx context.Context, body []byte) Disposition {
	var msg notify.Message
	if err := json.Unmarshal(body, &msg); err != nil {
		w.log.Error("slack: malformed message", "err", err)
		return Reject
	}
	nid, err := uuid.Parse(msg.NotificationID)
	if err != nil {
		w.log.Error("slack: bad notification_id", "id", msg.NotificationID, "err", err)
		return Reject
	}

	n, err := w.store.NotificationByID(ctx, nid)
	if errors.Is(err, store.ErrNotFound) {
		w.log.Warn("slack: orphan notification, dropping", "id", nid)
		return Ack
	}
	if err != nil {
		w.log.Error("slack: load notification", "id", nid, "err", err)
		return Requeue
	}
	// Идемпотентность: уже доставлено — ничего не делаем (защита от повторной доставки брокером).
	if n.Status == domain.NotificationSent {
		w.log.Info("slack: already sent, skipping", "id", nid)
		return Ack
	}
	if msg.Address == "" {
		w.log.Error("slack: empty webhook address", "id", nid)
		return Reject
	}

	payload, err := w.build(ctx, msg)
	if err != nil {
		w.log.Error("slack: build", "id", nid, "event", msg.Event, "err", err)
		return Reject
	}

	if err := w.poster.PostMessage(ctx, msg.Address, payload); err != nil {
		var perr *PostError
		if errors.As(err, &perr) && perr.Permanent {
			// Webhook отозван / канал удалён — повтор бесполезен. Дропаем доставку (Ack).
			w.log.Warn("slack: permanent delivery error, dropping", "id", nid, "err", err)
			return Ack
		}
		w.log.Warn("slack: post failed, scheduling retry", "id", nid, "err", err)
		return w.handleSendFailure(ctx, msg)
	}

	if err := w.store.MarkNotificationSent(ctx, nid); err != nil {
		// Сообщение ушло, но отметку не записали: повторим — повторная доставка не критична.
		w.log.Error("slack: mark sent", "id", nid, "err", err)
		return Requeue
	}
	w.log.Info("slack: sent", "id", nid, "event", msg.Event)
	return Ack
}

// handleSendFailure планирует ретрай через delayed exchange либо отправляет в DLQ при исчерпании.
func (w *Worker) handleSendFailure(ctx context.Context, msg notify.Message) Disposition {
	scheduled, err := w.retrier.Retry(ctx, msg)
	if err != nil {
		w.log.Error("slack: schedule retry", "id", msg.NotificationID, "err", err)
		return Requeue
	}
	if scheduled {
		return Ack // отложенная копия придёт позже; текущую доставку подтверждаем
	}
	w.log.Warn("slack: retries exhausted → DLQ", "id", msg.NotificationID)
	return Reject
}

// build загружает страницу и рендерит Block Kit payload по событию.
func (w *Worker) build(ctx context.Context, msg notify.Message) ([]byte, error) {
	pageID, err := uuid.Parse(msg.StatusPageID)
	if err != nil {
		return nil, fmt.Errorf("slack: bad status_page_id: %w", err)
	}
	page, err := w.store.StatusPageByID(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("slack: load page: %w", err)
	}

	in := RenderInput{
		Event:    domain.EventType(msg.Event),
		Locale:   page.DefaultLocale,
		PageName: page.Name,
		PageURL:  w.publicURL + "/status/" + page.Slug,
	}
	switch domain.EventType(msg.Event) {
	case domain.EventIncidentNew, domain.EventIncidentUpdate:
		var p notify.IncidentPayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			return nil, fmt.Errorf("slack: incident payload: %w", err)
		}
		in.Incident = &p
	case domain.EventMaintenanceScheduled, domain.EventMaintenanceStarted, domain.EventMaintenanceCompleted:
		var p notify.MaintenancePayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			return nil, fmt.Errorf("slack: maintenance payload: %w", err)
		}
		in.Maintenance = &p
	}
	return Render(in)
}
