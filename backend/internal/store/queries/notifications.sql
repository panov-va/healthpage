-- ── notifications (журнал отправок / идемпотентность, DESIGN §8.1) ──

-- name: CreateNotification :one
INSERT INTO notifications (subscriber_id, event_type, payload)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetNotificationByID :one
SELECT * FROM notifications WHERE id = $1;

-- name: MarkNotificationSent :exec
-- Идемпотентно: помечает запись доставленной (sent_at — момент успеха). Повторный вызов
-- безвреден (sent_at перезапишется тем же смыслом). Воркер зовёт после успешной доставки.
UPDATE notifications SET status = 'sent', sent_at = now() WHERE id = $1;

-- name: MarkNotificationFailed :exec
-- Помечает запись окончательно неуспешной (ретраи исчерпаны, сообщение ушло в DLQ).
UPDATE notifications SET status = 'failed' WHERE id = $1;

-- name: IncrementNotificationAttempts :one
-- Увеличивает счётчик попыток и возвращает новое значение (для расчёта backoff).
UPDATE notifications SET attempts = attempts + 1 WHERE id = $1 RETURNING attempts;
