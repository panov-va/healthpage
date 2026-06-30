-- name: CreateAPIToken :one
INSERT INTO api_tokens (status_page_id, token_hash, name, scopes)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetAPITokenByHash :one
SELECT * FROM api_tokens WHERE token_hash = $1;

-- name: GetAPIToken :one
SELECT * FROM api_tokens WHERE id = $1;

-- name: ListAPITokensByPage :many
SELECT * FROM api_tokens
WHERE status_page_id = $1
ORDER BY created_at DESC;

-- name: TouchAPIToken :exec
UPDATE api_tokens SET last_used_at = now() WHERE id = $1;

-- name: DeleteAPIToken :exec
DELETE FROM api_tokens WHERE id = $1;
