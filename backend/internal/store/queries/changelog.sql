-- name: CreateChangelogEntry :one
INSERT INTO changelog_entries (status_page_id, title, body, published, published_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetChangelogEntry :one
SELECT * FROM changelog_entries WHERE id = $1;

-- name: ListChangelogByPage :many
-- Админский список: все записи (включая черновики), новые сверху.
SELECT * FROM changelog_entries
WHERE status_page_id = $1
ORDER BY COALESCE(published_at, created_at) DESC, created_at DESC;

-- name: ListPublishedChangelog :many
-- Публичный список: только опубликованные, новые сверху, с пагинацией.
SELECT * FROM changelog_entries
WHERE status_page_id = $1 AND published = true
ORDER BY published_at DESC NULLS LAST, created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateChangelogEntry :one
UPDATE changelog_entries
SET title = $2, body = $3, published = $4, published_at = $5
WHERE id = $1
RETURNING *;

-- name: DeleteChangelogEntry :exec
DELETE FROM changelog_entries WHERE id = $1;
