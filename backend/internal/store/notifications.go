package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store/db"
)

// CreateNotification заносит запись в журнал отправок (status=pending по дефолту схемы).
// Возвращённый ID — ключ идемпотентности уведомления (DESIGN §8.1).
func (s *Store) CreateNotification(
	ctx context.Context, subscriberID uuid.UUID, event domain.EventType, payload []byte,
) (domain.Notification, error) {
	row, err := s.q.CreateNotification(ctx, db.CreateNotificationParams{
		SubscriberID: subscriberID,
		EventType:    string(event),
		Payload:      payload,
	})
	if err != nil {
		return domain.Notification{}, fmt.Errorf("store: create notification: %w", err)
	}
	return mapNotification(row), nil
}

// NotificationByID возвращает запись журнала. ErrNotFound если нет.
func (s *Store) NotificationByID(ctx context.Context, id uuid.UUID) (domain.Notification, error) {
	row, err := s.q.GetNotificationByID(ctx, id)
	if err != nil {
		return domain.Notification{}, wrapNotFound(err)
	}
	return mapNotification(row), nil
}

// MarkNotificationSent помечает уведомление доставленным (идемпотентно). Воркер канала зовёт
// после успешной доставки (письмо отправлено / Telegram принял / webhook вернул 2xx).
func (s *Store) MarkNotificationSent(ctx context.Context, id uuid.UUID) error {
	if err := s.q.MarkNotificationSent(ctx, id); err != nil {
		return fmt.Errorf("store: mark notification sent: %w", err)
	}
	return nil
}

// MarkNotificationFailed помечает уведомление окончательно неуспешным (ретраи исчерпаны).
func (s *Store) MarkNotificationFailed(ctx context.Context, id uuid.UUID) error {
	if err := s.q.MarkNotificationFailed(ctx, id); err != nil {
		return fmt.Errorf("store: mark notification failed: %w", err)
	}
	return nil
}

// IncrementNotificationAttempts увеличивает счётчик попыток и возвращает новое значение
// (для расчёта задержки backoff перед повторной публикацией).
func (s *Store) IncrementNotificationAttempts(ctx context.Context, id uuid.UUID) (int, error) {
	n, err := s.q.IncrementNotificationAttempts(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("store: increment notification attempts: %w", err)
	}
	return int(n), nil
}

func mapNotification(n db.Notification) domain.Notification {
	return domain.Notification{
		ID:           n.ID,
		SubscriberID: n.SubscriberID,
		EventType:    domain.EventType(n.EventType),
		Payload:      n.Payload,
		Status:       domain.NotificationStatus(n.Status),
		Attempts:     int(n.Attempts),
		SentAt:       n.SentAt,
		CreatedAt:    n.CreatedAt,
		UpdatedAt:    n.UpdatedAt,
	}
}
