-- name: CreateMembership :one
INSERT INTO memberships (user_id, status_page_id, role)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListMembershipsByUser :many
SELECT * FROM memberships WHERE user_id = $1;

-- name: GetMembership :one
SELECT * FROM memberships WHERE user_id = $1 AND status_page_id = $2;
