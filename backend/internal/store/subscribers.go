package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store/db"
)

// CreateSubscriber сохраняет подписчика. Используется сервисом подписки (3.4/3.5) и тестами движка.
// Токены (хэши) и confirmed задаёт вызывающий по правилам double opt-in.
func (s *Store) CreateSubscriber(ctx context.Context, sub domain.Subscriber) (domain.Subscriber, error) {
	// component_ids — NOT NULL; nil-слайс кодируется как NULL, поэтому нормализуем к пустому массиву.
	componentIDs := sub.ComponentIDs
	if componentIDs == nil {
		componentIDs = []uuid.UUID{}
	}
	row, err := s.q.CreateSubscriber(ctx, db.CreateSubscriberParams{
		StatusPageID:     sub.StatusPageID,
		Channel:          string(sub.Channel),
		Address:          sub.Address,
		Confirmed:        sub.Confirmed,
		ConfirmToken:     sub.ConfirmToken,
		UnsubscribeToken: sub.UnsubscribeToken,
		Scope:            string(sub.Scope),
		ComponentIds:     componentIDs,
	})
	if err != nil {
		return domain.Subscriber{}, fmt.Errorf("store: create subscriber: %w", err)
	}
	return mapSubscriber(row), nil
}

// SubscriberByID возвращает подписчика. ErrNotFound если нет.
func (s *Store) SubscriberByID(ctx context.Context, id uuid.UUID) (domain.Subscriber, error) {
	row, err := s.q.GetSubscriberByID(ctx, id)
	if err != nil {
		return domain.Subscriber{}, wrapNotFound(err)
	}
	return mapSubscriber(row), nil
}

// SubscriberByPageChannelAddress находит подписчика по уникальной тройке (для идемпотентной
// повторной подписки). ErrNotFound если нет.
func (s *Store) SubscriberByPageChannelAddress(
	ctx context.Context, pageID uuid.UUID, channel domain.SubscriberChannel, address string,
) (domain.Subscriber, error) {
	row, err := s.q.GetSubscriberByPageChannelAddress(ctx, db.GetSubscriberByPageChannelAddressParams{
		StatusPageID: pageID, Channel: string(channel), Address: address,
	})
	if err != nil {
		return domain.Subscriber{}, wrapNotFound(err)
	}
	return mapSubscriber(row), nil
}

// SubscribersByChannelAddress возвращает все подписки одного адреса в канале (один chat_id
// мессенджера может быть подписан на несколько страниц). Для команды бота /stop без аргумента.
func (s *Store) SubscribersByChannelAddress(
	ctx context.Context, channel domain.SubscriberChannel, address string,
) ([]domain.Subscriber, error) {
	rows, err := s.q.ListSubscribersByChannelAddress(ctx, db.ListSubscribersByChannelAddressParams{
		Channel: string(channel), Address: address,
	})
	if err != nil {
		return nil, fmt.Errorf("store: list subscribers by channel/address: %w", err)
	}
	out := make([]domain.Subscriber, len(rows))
	for i, row := range rows {
		out[i] = mapSubscriber(row)
	}
	return out, nil
}

// SubscriberByConfirmTokenHash находит подписчика по хэшу confirm-токена. ErrNotFound если нет.
func (s *Store) SubscriberByConfirmTokenHash(ctx context.Context, tokenHash string) (domain.Subscriber, error) {
	row, err := s.q.GetSubscriberByConfirmToken(ctx, &tokenHash)
	if err != nil {
		return domain.Subscriber{}, wrapNotFound(err)
	}
	return mapSubscriber(row), nil
}

// ReissueConfirmToken обновляет confirm-токен и scope/компоненты неподтверждённой подписки
// (повторная подписка тем же адресом — пользователь не получил/потерял письмо).
func (s *Store) ReissueConfirmToken(
	ctx context.Context, id uuid.UUID, tokenHash string, scope domain.SubscriberScope, componentIDs []uuid.UUID,
) (domain.Subscriber, error) {
	if componentIDs == nil {
		componentIDs = []uuid.UUID{}
	}
	row, err := s.q.SetSubscriberConfirmToken(ctx, db.SetSubscriberConfirmTokenParams{
		ID: id, ConfirmToken: &tokenHash, Scope: string(scope), ComponentIds: componentIDs,
	})
	if err != nil {
		return domain.Subscriber{}, wrapNotFound(err)
	}
	return mapSubscriber(row), nil
}

// ConfirmSubscriber переводит подписку в confirmed=true и гасит confirm-токен (одноразовый).
func (s *Store) ConfirmSubscriber(ctx context.Context, id uuid.UUID) error {
	if err := s.q.ConfirmSubscriber(ctx, id); err != nil {
		return fmt.Errorf("store: confirm subscriber: %w", err)
	}
	return nil
}

// DeleteSubscriber физически удаляет подписчика (отписка). Каскад убирает его уведомления.
func (s *Store) DeleteSubscriber(ctx context.Context, id uuid.UUID) error {
	if err := s.q.DeleteSubscriber(ctx, id); err != nil {
		return fmt.Errorf("store: delete subscriber: %w", err)
	}
	return nil
}

// ListConfirmedSubscribers возвращает подтверждённых подписчиков страницы — кандидатов на рассылку.
// Фильтрация по scope/компонентам — на стороне движка (domain.Subscriber.WantsEvent).
func (s *Store) ListConfirmedSubscribers(ctx context.Context, pageID uuid.UUID) ([]domain.Subscriber, error) {
	rows, err := s.q.ListConfirmedSubscribers(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("store: list confirmed subscribers: %w", err)
	}
	out := make([]domain.Subscriber, len(rows))
	for i, row := range rows {
		out[i] = mapSubscriber(row)
	}
	return out, nil
}

func mapSubscriber(s db.Subscriber) domain.Subscriber {
	return domain.Subscriber{
		ID:               s.ID,
		StatusPageID:     s.StatusPageID,
		Channel:          domain.SubscriberChannel(s.Channel),
		Address:          s.Address,
		Confirmed:        s.Confirmed,
		ConfirmToken:     s.ConfirmToken,
		UnsubscribeToken: s.UnsubscribeToken,
		Scope:            domain.SubscriberScope(s.Scope),
		ComponentIDs:     s.ComponentIds,
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
	}
}
