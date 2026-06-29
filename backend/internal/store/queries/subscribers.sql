-- ── subscribers ──

-- name: CreateSubscriber :one
INSERT INTO subscribers (
    status_page_id, channel, address, confirmed, confirm_token, unsubscribe_token, scope, component_ids
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetSubscriberByID :one
SELECT * FROM subscribers WHERE id = $1;

-- name: ListConfirmedSubscribers :many
-- Подтверждённые подписчики страницы — кандидаты на рассылку (движок уведомлений, §8.1).
-- Фильтрация по scope/компонентам выполняется в Go (Subscriber.WantsEvent).
SELECT * FROM subscribers
WHERE status_page_id = $1 AND confirmed = true
ORDER BY created_at;

-- name: GetSubscriberByPageChannelAddress :one
SELECT * FROM subscribers
WHERE status_page_id = $1 AND channel = $2 AND address = $3;

-- name: ListSubscribersByChannelAddress :many
-- Все подписки одного адреса в канале (один chat_id в Telegram/MAX может быть подписан на
-- несколько страниц). Используется ботом для команды /stop без аргумента (отписка отовсюду).
SELECT * FROM subscribers
WHERE channel = $1 AND address = $2
ORDER BY created_at;

-- name: GetSubscriberByConfirmToken :one
-- Поиск по хэшу confirm-токена (double opt-in подтверждение).
SELECT * FROM subscribers WHERE confirm_token = $1;

-- name: SetSubscriberConfirmToken :one
-- Перевыпуск confirm-токена и обновление scope при повторной (неподтверждённой) подписке.
UPDATE subscribers SET confirm_token = $2, scope = $3, component_ids = $4
WHERE id = $1
RETURNING *;

-- name: ConfirmSubscriber :exec
-- Подтверждение подписки: confirmed=true, confirm-токен гасится (одноразовый).
UPDATE subscribers SET confirmed = true, confirm_token = NULL WHERE id = $1;

-- name: DeleteSubscriber :exec
-- Отписка — физическое удаление строки (в модели §5 у Subscriber нет soft-delete).
DELETE FROM subscribers WHERE id = $1;
