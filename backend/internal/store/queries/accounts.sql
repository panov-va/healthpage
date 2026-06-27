-- name: CreateAccount :one
INSERT INTO accounts (name, owner_user_id)
VALUES ($1, $2)
RETURNING *;

-- name: GetAccountByID :one
SELECT * FROM accounts WHERE id = $1;
