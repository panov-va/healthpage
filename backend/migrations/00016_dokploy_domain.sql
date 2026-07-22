-- Этап 4.3 (продолжение, прод-интеграция): вместо своего edge/tls-manager кастомные домены
-- клиентов подключаются через Dokploy API (domain.create/domain.delete) — Dokploy сам управляет
-- Traefik-роутингом и выпуском Let's Encrypt для добавленного домена. dokploy_domain_id хранит
-- ID записи Domain в Dokploy, чтобы при смене/снятии custom_domain отвязать и удалить её там же.
-- NULL — домен либо не подключён, либо ещё не прошла верификация CNAME.

-- +goose Up
ALTER TABLE status_pages ADD COLUMN dokploy_domain_id text;

-- +goose Down
ALTER TABLE status_pages DROP COLUMN dokploy_domain_id;
