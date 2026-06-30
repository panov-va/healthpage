-- name: CreateStatusPage :one
INSERT INTO status_pages (account_id, name, description, slug, timezone, default_locale, visibility)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetStatusPageByID :one
SELECT * FROM status_pages WHERE id = $1 AND deleted_at IS NULL;

-- name: GetStatusPageBySlug :one
SELECT * FROM status_pages WHERE slug = $1 AND deleted_at IS NULL;

-- name: ListStatusPagesByAccount :many
SELECT * FROM status_pages
WHERE account_id = $1 AND deleted_at IS NULL
ORDER BY created_at;

-- name: UpdateStatusPage :one
UPDATE status_pages SET
    name = $2,
    description = $3,
    timezone = $4,
    default_locale = $5,
    visibility = $6,
    theme = $7,
    logo_url = $8,
    favicon_url = $9,
    hide_powered_by = $10,
    redirect_url = $11
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SetCustomDomain :exec
-- Задаёт/снимает собственный домен страницы (этап 4.3) и сбрасывает флаг верификации
-- (домен нужно проверить заново). NULL снимает домен. Уникальность — на уровне индекса.
UPDATE status_pages SET custom_domain = $2, domain_verified = false
WHERE id = $1 AND deleted_at IS NULL;

-- name: SetDomainVerified :exec
-- Отмечает результат проверки CNAME (этап 4.3).
UPDATE status_pages SET domain_verified = $2 WHERE id = $1 AND deleted_at IS NULL;

-- name: SetStatusPagePassword :exec
-- Задаёт/снимает пароль приватной страницы (этап 4.2). NULL снимает пароль.
-- Хранится только хэш (§9). UpdateStatusPage намеренно password_hash не трогает.
UPDATE status_pages SET password_hash = $2 WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteStatusPage :exec
UPDATE status_pages SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;
