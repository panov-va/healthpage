-- ── incidents ──

-- name: CreateIncident :one
INSERT INTO incidents (
    status_page_id, title, current_status, impact, started_at, resolved_at, postmortem, is_visible
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetIncidentByID :one
SELECT * FROM incidents WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateIncident :one
UPDATE incidents SET
    title = $2,
    current_status = $3,
    impact = $4,
    resolved_at = $5,
    postmortem = $6,
    is_visible = $7
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteIncident :exec
UPDATE incidents SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- ── incident_components ──

-- name: AddIncidentComponent :one
INSERT INTO incident_components (incident_id, component_id, component_status_in_incident)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListIncidentComponents :many
SELECT * FROM incident_components WHERE incident_id = $1 ORDER BY created_at;

-- name: DeleteIncidentComponents :exec
DELETE FROM incident_components WHERE incident_id = $1;

-- Плоский список (component_id, статус) по всем активным (не resolved, не удалённым) инцидентам
-- страницы — основа авто-деривации статуса компонентов (DESIGN §3.3, §6).
-- name: ListActiveIncidentComponentStatuses :many
SELECT ic.component_id, ic.component_status_in_incident
FROM incident_components ic
JOIN incidents i ON i.id = ic.incident_id
WHERE i.status_page_id = $1
  AND i.deleted_at IS NULL
  AND i.current_status <> 'resolved';

-- ── incident_updates ──

-- name: AddIncidentUpdate :one
INSERT INTO incident_updates (incident_id, status, body, notify_subscribers)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListIncidentUpdates :many
SELECT * FROM incident_updates WHERE incident_id = $1 ORDER BY created_at;
