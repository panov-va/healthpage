-- name: CreateImportJob :one
INSERT INTO import_jobs (status_page_id, account_id, source, region, subdomain, mode, status)
VALUES ($1, $2, $3, $4, $5, $6, 'pending')
RETURNING *;

-- name: GetImportJob :one
SELECT * FROM import_jobs WHERE id = $1;

-- name: UpdateImportJob :one
UPDATE import_jobs
SET status = $2, report = $3, error = $4, finished_at = $5
WHERE id = $1
RETURNING *;

-- name: GetExternalMapping :one
SELECT internal_id FROM external_id_map
WHERE status_page_id = $1 AND source = $2 AND entity_type = $3 AND external_id = $4;

-- name: UpsertExternalMapping :exec
INSERT INTO external_id_map (status_page_id, source, entity_type, external_id, internal_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (status_page_id, source, entity_type, external_id)
DO UPDATE SET internal_id = EXCLUDED.internal_id;
