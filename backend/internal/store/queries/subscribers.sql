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
