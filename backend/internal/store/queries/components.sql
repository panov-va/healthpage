-- name: CreateComponent :one
INSERT INTO components (
    status_page_id, group_id, parent_id, name, description, position,
    current_status, is_private, show_uptime, display_state
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetComponentByID :one
SELECT * FROM components WHERE id = $1 AND deleted_at IS NULL;

-- name: ListComponentsByPage :many
SELECT * FROM components
WHERE status_page_id = $1 AND deleted_at IS NULL
ORDER BY position, name;

-- name: UpdateComponent :one
UPDATE components SET
    group_id = $2,
    parent_id = $3,
    name = $4,
    description = $5,
    position = $6,
    is_private = $7,
    show_uptime = $8,
    display_state = $9
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SetComponentStatus :one
UPDATE components SET current_status = $2
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteComponent :exec
UPDATE components SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;
