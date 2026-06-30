-- name: ListAllowedEmails :many
SELECT * FROM page_allowed_emails WHERE status_page_id = $1 ORDER BY created_at;

-- name: AddAllowedEmail :one
INSERT INTO page_allowed_emails (status_page_id, email) VALUES ($1, $2) RETURNING *;

-- name: AllowedEmailByID :one
SELECT * FROM page_allowed_emails WHERE id = $1;

-- name: DeleteAllowedEmail :exec
DELETE FROM page_allowed_emails WHERE id = $1;

-- name: IsEmailAllowed :one
SELECT EXISTS (
    SELECT 1 FROM page_allowed_emails
    WHERE status_page_id = $1 AND lower(email) = lower($2)
);
