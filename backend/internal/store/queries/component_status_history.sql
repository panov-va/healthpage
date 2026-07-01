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

-- name: ListStatusHistorySince :many
-- Периоды, пересекающие окно [since, now]: открытые (ended_at IS NULL) и завершившиеся не раньше
-- since. Захватывает и период, активный на момент since (started_at < since, ended_at >= since).
SELECT * FROM component_status_history
WHERE component_id = $1 AND (ended_at IS NULL OR ended_at >= $2)
ORDER BY started_at;
