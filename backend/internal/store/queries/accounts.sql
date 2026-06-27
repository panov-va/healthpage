-- name: CreateAccount :one
INSERT INTO accounts (name, owner_user_id)
VALUES ($1, $2)
RETURNING *;

-- name: GetAccountByID :one
SELECT * FROM accounts WHERE id = $1;

-- name: GetAccountByOwner :one
SELECT * FROM accounts WHERE owner_user_id = $1 ORDER BY created_at LIMIT 1;
