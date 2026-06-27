-- name: CreateComponentGroup :one
INSERT INTO component_groups (status_page_id, name, position)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetComponentGroupByID :one
SELECT * FROM component_groups WHERE id = $1 AND deleted_at IS NULL;

-- name: ListComponentGroupsByPage :many
SELECT * FROM component_groups
WHERE status_page_id = $1 AND deleted_at IS NULL
ORDER BY position, name;

-- name: UpdateComponentGroup :one
UPDATE component_groups SET name = $2, position = $3
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteComponentGroup :exec
UPDATE component_groups SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;
