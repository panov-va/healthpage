-- name: CloseOpenStatusPeriod :exec
UPDATE component_status_history SET ended_at = now()
WHERE component_id = $1 AND ended_at IS NULL;

-- name: OpenStatusPeriod :one
INSERT INTO component_status_history (component_id, status, source)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListStatusHistory :many
SELECT * FROM component_status_history
WHERE component_id = $1
ORDER BY started_at;
