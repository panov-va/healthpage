package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/notify"
	"github.com/healthpage/backend/internal/store"
)

// Disposition — решение воркера по сообщению, применяемое к AMQP-доставке вызывающим.
// (Зеркалит email.Disposition; держим локально, чтобы пакеты каналов были независимы.)
type Disposition int

const (
	Ack     Disposition = iota // подтвердить (доставлено / нечего делать / ретрай запланирован)
	Reject                     // Nack(requeue=false) → DLQ (битое/неустранимое/исчерпан ретрай)
	Requeue                    // Nack(requeue=true) → повторить (транзиентная инфра-ошибка)
)

// Sender отправляет текст в чат Telegram. Реализуется *Client; фейк — в тестах.
type Sender interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
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

// Worker обрабатывает одно сообщение q.telegram: идемпотентность по Notification.id, рендер,
// отправка через Bot API, отметка sent или планирование ретрая (DESIGN §8.1).
type Worker struct {
	store     WorkerStore
	sender    Sender
	retrier   Retrier
	publicURL string // для ссылки на публичную страницу
	log       *slog.Logger
}

// NewWorker собирает воркера. logger=nil → slog.Default().
func NewWorker(st WorkerStore, sender Sender, retrier Retrier, publicURL string, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{store: st, sender: sender, retrier: retrier, publicURL: publicURL, log: logger}
}

// Process обрабатывает тело сообщения и возвращает решение по доставке.
func (w *Worker) Process(ctx context.Context, body []byte) Disposition {
	var msg notify.Message
	if err := json.Unmarshal(body, &msg); err != nil {
		w.log.Error("telegram: malformed message", "err", err)
		return Reject
	}
	nid, err := uuid.Parse(msg.NotificationID)
	if err != nil {
		w.log.Error("telegram: bad notification_id", "id", msg.NotificationID, "err", err)
		return Reject
	}

	n, err := w.store.NotificationByID(ctx, nid)
	if errors.Is(err, store.ErrNotFound) {
		w.log.Warn("telegram: orphan notification, dropping", "id", nid)
		return Ack
	}
	if err != nil {
		w.log.Error("telegram: load notification", "id", nid, "err", err)
		return Requeue
	}
	// Идемпотентность: уже доставлено — ничего не делаем (защита от повторной доставки брокером).
	if n.Status == domain.NotificationSent {
		w.log.Info("telegram: already sent, skipping", "id", nid)
		return Ack
	}

	chatID, err := strconv.ParseInt(msg.Address, 10, 64)
	if err != nil {
		w.log.Error("telegram: bad chat_id address", "addr", msg.Address, "err", err)
		return Reject
	}

	text, err := w.build(ctx, msg)
	if err != nil {
		w.log.Error("telegram: build", "id", nid, "event", msg.Event, "err", err)
		return Reject
	}

	if err := w.sender.SendMessage(ctx, chatID, text); err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Permanent {
			// Бот заблокирован / чат не найден — повтор бесполезен, ретрай и DLQ не помогут.
			// Дропаем доставку (подтверждаем). Авто-отписка заблокировавших — возможное улучшение.
			w.log.Warn("telegram: permanent delivery error, dropping", "id", nid, "err", err)
			return Ack
		}
		w.log.Warn("telegram: send failed, scheduling retry", "id", nid, "err", err)
		return w.handleSendFailure(ctx, msg)
	}

	if err := w.store.MarkNotificationSent(ctx, nid); err != nil {
		// Сообщение ушло, но отметку не записали: повторим — повторная доставка не критична
		// (на следующем заходе NotificationByID всё ещё pending → отметим).
		w.log.Error("telegram: mark sent", "id", nid, "err", err)
		return Requeue
	}
	w.log.Info("telegram: sent", "id", nid, "chat", chatID, "event", msg.Event)
	return Ack
}

// handleSendFailure планирует ретрай через delayed exchange либо отправляет в DLQ при исчерпании.
func (w *Worker) handleSendFailure(ctx context.Context, msg notify.Message) Disposition {
	scheduled, err := w.retrier.Retry(ctx, msg)
	if err != nil {
		w.log.Error("telegram: schedule retry", "id", msg.NotificationID, "err", err)
		return Requeue
	}
	if scheduled {
		return Ack // отложенная копия придёт позже; текущую доставку подтверждаем
	}
	w.log.Warn("telegram: retries exhausted → DLQ", "id", msg.NotificationID)
	return Reject
}

// build загружает страницу и рендерит текст сообщения по событию.
func (w *Worker) build(ctx context.Context, msg notify.Message) (string, error) {
	pageID, err := uuid.Parse(msg.StatusPageID)
	if err != nil {
		return "", fmt.Errorf("telegram: bad status_page_id: %w", err)
	}
	page, err := w.store.StatusPageByID(ctx, pageID)
	if err != nil {
		return "", fmt.Errorf("telegram: load page: %w", err)
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
			return "", fmt.Errorf("telegram: incident payload: %w", err)
		}
		in.Incident = &p
	case domain.EventMaintenanceScheduled, domain.EventMaintenanceStarted, domain.EventMaintenanceCompleted:
		var p notify.MaintenancePayload
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			return "", fmt.Errorf("telegram: maintenance payload: %w", err)
		}
		in.Maintenance = &p
	}
	return Render(in)
}
