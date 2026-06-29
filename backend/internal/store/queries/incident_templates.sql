-- ── incident_templates ──

-- name: CreateIncidentTemplate :one
INSERT INTO incident_templates (
    status_page_id, name, title_tmpl, body_tmpl, default_impact
)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetIncidentTemplateByID :one
SELECT * FROM incident_templates WHERE id = $1;

-- name: ListIncidentTemplatesByPage :many
SELECT * FROM incident_templates WHERE status_page_id = $1 ORDER BY name;

-- name: UpdateIncidentTemplate :one
UPDATE incident_templates SET
    name = $2,
    title_tmpl = $3,
    body_tmpl = $4,
    default_impact = $5
WHERE id = $1
RETURNING *;

-- name: DeleteIncidentTemplate :exec
DELETE FROM incident_templates WHERE id = $1;

-- ── incident_template_components ──

-- name: AddIncidentTemplateComponent :one
INSERT INTO incident_template_components (template_id, component_id, component_status_in_incident)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListIncidentTemplateComponents :many
SELECT * FROM incident_template_components WHERE template_id = $1 ORDER BY created_at;

-- name: DeleteIncidentTemplateComponents :exec
DELETE FROM incident_template_components WHERE template_id = $1;
