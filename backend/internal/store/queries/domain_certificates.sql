-- name: GetDomainCertificate :one
SELECT * FROM domain_certificates WHERE domain = $1;

-- name: UpsertDomainCertificate :exec
INSERT INTO domain_certificates (domain, cert_pem, key_pem, expires_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (domain) DO UPDATE
SET cert_pem = EXCLUDED.cert_pem, key_pem = EXCLUDED.key_pem, expires_at = EXCLUDED.expires_at;

-- name: DeleteDomainCertificate :exec
DELETE FROM domain_certificates WHERE domain = $1;

-- name: GetACMEAccount :one
SELECT * FROM acme_accounts WHERE directory_url = $1;

-- name: UpsertACMEAccount :exec
INSERT INTO acme_accounts (directory_url, email, private_key, registration)
VALUES ($1, $2, $3, $4)
ON CONFLICT (directory_url) DO UPDATE
SET email = EXCLUDED.email, private_key = EXCLUDED.private_key, registration = EXCLUDED.registration;

-- name: PutACMEChallenge :exec
INSERT INTO acme_challenges (token, key_auth, domain)
VALUES ($1, $2, $3)
ON CONFLICT (token) DO UPDATE SET key_auth = EXCLUDED.key_auth, domain = EXCLUDED.domain;

-- name: GetACMEChallenge :one
SELECT key_auth FROM acme_challenges WHERE token = $1;

-- name: DeleteACMEChallenge :exec
DELETE FROM acme_challenges WHERE token = $1;

-- name: ListVerifiedDomains :many
SELECT custom_domain FROM status_pages
WHERE custom_domain IS NOT NULL AND domain_verified = true AND deleted_at IS NULL;

-- name: GetSlugByCustomDomain :one
SELECT slug FROM status_pages
WHERE custom_domain = $1 AND domain_verified = true AND deleted_at IS NULL;
