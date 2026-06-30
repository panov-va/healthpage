-- name: CreateWebhookIntegration :one
INSERT INTO webhook_integrations (status_page_id, source, name, secret, component_mapping)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetWebhookIntegration :one
SELECT * FROM webhook_integrations WHERE id = $1;

-- name: ListWebhookIntegrationsByPage :many
SELECT * FROM webhook_integrations
WHERE status_page_id = $1
ORDER BY created_at DESC;

-- name: UpdateWebhookIntegration :one
UPDATE webhook_integrations
SET name = $2, secret = $3, component_mapping = $4
WHERE id = $1
RETURNING *;

-- name: DeleteWebhookIntegration :exec
DELETE FROM webhook_integrations WHERE id = $1;
