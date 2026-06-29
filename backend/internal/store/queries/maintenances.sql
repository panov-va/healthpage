-- ── maintenances ──

-- name: CreateMaintenance :one
INSERT INTO maintenances (
    status_page_id, title, description, status, scheduled_start, scheduled_end, started_at, completed_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetMaintenanceByID :one
SELECT * FROM maintenances WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateMaintenance :one
UPDATE maintenances SET
    title = $2,
    description = $3,
    status = $4,
    scheduled_start = $5,
    scheduled_end = $6,
    started_at = $7,
    completed_at = $8
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteMaintenance :exec
UPDATE maintenances SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- ── maintenance_components ──

-- name: AddMaintenanceComponent :one
INSERT INTO maintenance_components (maintenance_id, component_id)
VALUES ($1, $2)
RETURNING *;

-- name: ListMaintenanceComponents :many
SELECT * FROM maintenance_components WHERE maintenance_id = $1 ORDER BY created_at;

-- name: DeleteMaintenanceComponents :exec
DELETE FROM maintenance_components WHERE maintenance_id = $1;

-- Список component_id всех активных (in_progress, не удалённых) работ страницы — основа
-- авто-перевода компонентов в under_maintenance (DESIGN §3.4, §6).
-- name: ListActiveMaintenanceComponentIDs :many
SELECT mc.component_id
FROM maintenance_components mc
JOIN maintenances m ON m.id = mc.maintenance_id
WHERE m.status_page_id = $1
  AND m.deleted_at IS NULL
  AND m.status = 'in_progress';

-- Публичный список работ страницы: не удалённые, с опциональным фильтром по статусу и пагинацией.
-- name: ListPublicMaintenances :many
SELECT * FROM maintenances
WHERE status_page_id = @status_page_id
  AND deleted_at IS NULL
  AND (sqlc.narg('status')::maintenance_status IS NULL OR status = sqlc.narg('status'))
ORDER BY scheduled_start DESC
LIMIT @lim OFFSET @off;

-- name: CountPublicMaintenances :one
SELECT count(*) FROM maintenances
WHERE status_page_id = @status_page_id
  AND deleted_at IS NULL
  AND (sqlc.narg('status')::maintenance_status IS NULL OR status = sqlc.narg('status'));

-- Активные (не завершённые: scheduled + in_progress) работы страницы — для публичной сводки;
-- ближайшие/идущие первыми.
-- name: ListActivePublicMaintenances :many
SELECT * FROM maintenances
WHERE status_page_id = $1
  AND deleted_at IS NULL
  AND status <> 'completed'
ORDER BY scheduled_start;

-- ── maintenance_updates ──

-- name: AddMaintenanceUpdate :one
INSERT INTO maintenance_updates (maintenance_id, body, notify_subscribers)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListMaintenanceUpdates :many
SELECT * FROM maintenance_updates WHERE maintenance_id = $1 ORDER BY created_at;
