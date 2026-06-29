// Package notify — движок уведомлений (DESIGN §3.5, §8.1): по доменному событию (новый инцидент,
// обновление инцидента с notify, старт/конец/анонс работ) находит подходящих подтверждённых
// подписчиков, заводит на каждого запись журнала Notification (ключ идемпотентности) и публикует
// сообщение в exchange notifications для воркера канала. Ретраи с backoff — повторной публикацией
// через delayed.events; исчерпание — пометка failed (сообщение уходит в DLQ воркером).
package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
)

// Store — то, что движку нужно от хранилища (журнал + подписчики).
type Store interface {
	ListConfirmedSubscribers(ctx context.Context, pageID uuid.UUID) ([]domain.Subscriber, error)
	CreateNotification(ctx context.Context, subscriberID uuid.UUID, event domain.EventType, payload []byte) (domain.Notification, error)
	MarkNotificationFailed(ctx context.Context, id uuid.UUID) error
	IncrementNotificationAttempts(ctx context.Context, id uuid.UUID) (int, error)
}

// Publisher — то, что движку нужно от очереди (публикация обычная и отложенная).
type Publisher interface {
	PublishNotification(ctx context.Context, channel, event string, body []byte) error
	PublishNotificationDelayed(ctx context.Context, channel, event string, body []byte, delay time.Duration) error
}

// Engine рассылает уведомления по событиям. Publisher не потокобезопасен (держит один AMQP-канал),
// поэтому публикации сериализуются мьютексом — для нагрузок MVP этого достаточно.
type Engine struct {
	store Store
	pub   Publisher
	log   *slog.Logger
	mu    sync.Mutex // защищает pub
}

// New создаёт движок. logger может быть nil — тогда используется slog.Default().
func New(store Store, pub Publisher, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{store: store, pub: pub, log: logger}
}

// IncidentCreated рассылает уведомление о новом инциденте (event incident_new). body — текст
// стартового обновления.
func (e *Engine) IncidentCreated(ctx context.Context, inc domain.Incident, body string) error {
	return e.dispatch(ctx, inc.StatusPageID, incidentComponentIDs(inc), domain.EventIncidentNew,
		incidentPayload(inc, body))
}

// IncidentUpdated рассылает уведомление об обновлении инцидента (event incident_update).
func (e *Engine) IncidentUpdated(ctx context.Context, inc domain.Incident, upd domain.IncidentUpdate) error {
	return e.dispatch(ctx, inc.StatusPageID, incidentComponentIDs(inc), domain.EventIncidentUpdate,
		incidentPayload(inc, upd.Body))
}

// MaintenanceEvent рассылает уведомление о работах. event — один из maintenance_scheduled/
// maintenance_started/maintenance_completed.
func (e *Engine) MaintenanceEvent(ctx context.Context, m domain.Maintenance, event domain.EventType) error {
	return e.dispatch(ctx, m.StatusPageID, m.ComponentIDs, event, maintenancePayload(m))
}

// dispatch — общий фан-аут: разослать событие подходящим подтверждённым push-подписчикам.
// На каждого: запись журнала (pending) + публикация Message с её id. Ошибки публикации не
// прерывают рассылку (запись остаётся pending — восстановима); собираются и возвращаются.
func (e *Engine) dispatch(
	ctx context.Context, pageID uuid.UUID, affected []uuid.UUID, event domain.EventType, payload any,
) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notify: marshal payload: %w", err)
	}

	subs, err := e.store.ListConfirmedSubscribers(ctx, pageID)
	if err != nil {
		return fmt.Errorf("notify: list subscribers: %w", err)
	}

	var errs []error
	for _, sub := range subs {
		if !sub.Channel.IsPush() || !sub.WantsEvent(affected) {
			continue
		}
		n, err := e.store.CreateNotification(ctx, sub.ID, event, body)
		if err != nil {
			errs = append(errs, err)
			e.log.Error("notify: create notification", "subscriber", sub.ID, "event", event, "err", err)
			continue
		}
		msg := Message{
			NotificationID: n.ID.String(),
			SubscriberID:   sub.ID.String(),
			Channel:        string(sub.Channel),
			Event:          string(event),
			Address:        sub.Address,
			Attempt:        0,
			Payload:        body,
			StatusPageID:   pageID.String(),
		}
		if err := e.publish(ctx, msg); err != nil {
			errs = append(errs, err)
			e.log.Error("notify: publish", "notification", n.ID, "channel", sub.Channel, "err", err)
		}
	}
	return errors.Join(errs...)
}

// publish сериализует доступ к Publisher и шлёт сообщение в exchange notifications.
func (e *Engine) publish(ctx context.Context, msg Message) error {
	raw, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("notify: marshal message: %w", err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.pub.PublishNotification(ctx, msg.Channel, msg.Event, raw)
}

// Retry повторно публикует сообщение с возрастающим backoff (DESIGN §8.1). Увеличивает
// Notification.attempts; если ретраи исчерпаны — помечает запись failed и возвращает (false, nil),
// сигнализируя воркеру отправить сообщение в DLQ (Nack без requeue). Иначе публикует копию через
// delayed.events с задержкой и возвращает (true, nil). Вызывается воркером канала (этап 3.4+) при
// неуспешной доставке.
func (e *Engine) Retry(ctx context.Context, msg Message) (scheduled bool, err error) {
	id, err := uuid.Parse(msg.NotificationID)
	if err != nil {
		return false, fmt.Errorf("notify: bad notification id %q: %w", msg.NotificationID, err)
	}
	attempt, err := e.store.IncrementNotificationAttempts(ctx, id)
	if err != nil {
		return false, err
	}
	delay, ok := RetryBackoff(attempt)
	if !ok {
		if err := e.store.MarkNotificationFailed(ctx, id); err != nil {
			return false, err
		}
		e.log.Warn("notify: retries exhausted", "notification", id, "attempts", attempt)
		return false, nil
	}
	msg.Attempt = attempt
	raw, err := json.Marshal(msg)
	if err != nil {
		return false, fmt.Errorf("notify: marshal message: %w", err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.pub.PublishNotificationDelayed(ctx, msg.Channel, msg.Event, raw, delay); err != nil {
		return false, err
	}
	return true, nil
}

func incidentComponentIDs(inc domain.Incident) []uuid.UUID {
	ids := make([]uuid.UUID, len(inc.Components))
	for i, c := range inc.Components {
		ids[i] = c.ComponentID
	}
	return ids
}
