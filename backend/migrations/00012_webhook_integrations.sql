-- Этап 5.3: входящие webhook-интеграции (WebhookIntegration, DESIGN §4, §5, §7.3).
-- Интеграция привязана к странице, аутентифицирует входящий webhook по HMAC-подписи (X-Signature)
-- секретом интеграции и маппит алерт на компоненты по component_mapping.
--
-- secret хранится В ОТКРЫТОМ ВИДЕ (НЕ хэш): для проверки HMAC нужно пересчитать подпись тем же
-- секретом — хэш для этого непригоден. Это webhook-signing-secret (как пароль кастомного SMTP в
-- smtp_config или webhook URL у slack-подписчика), а не аутентификационный токен из DESIGN §9.
-- API показывает секрет ЕДИНОЖДЫ при создании/ротации и не возвращает в list/get.
--
-- Идемпотентность: входящий алерт несёт dedup-ключ; на странице держим максимум ОДИН открытый
-- (resolved_at IS NULL) инцидент на dedup-ключ — повторный firing не плодит дубли, resolved
-- закрывает. Для этого incidents расширяются external_dedup_key + integration_id.

-- +goose Up
CREATE TABLE webhook_integrations (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status_page_id    uuid NOT NULL REFERENCES status_pages (id) ON DELETE CASCADE,
    source            text NOT NULL CHECK (source IN ('grafana', 'prometheus', 'pagerduty', 'generic')),
    name              text NOT NULL,
    secret            text NOT NULL,
    component_mapping jsonb NOT NULL DEFAULT '{}',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX webhook_integrations_page_idx ON webhook_integrations (status_page_id);

CREATE TRIGGER webhook_integrations_set_updated_at BEFORE UPDATE ON webhook_integrations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE incidents
    ADD COLUMN integration_id uuid REFERENCES webhook_integrations (id) ON DELETE SET NULL,
    ADD COLUMN external_dedup_key text;

-- Один открытый инцидент на (страница, dedup-ключ): обеспечивает идемпотентность повторных firing.
CREATE UNIQUE INDEX incidents_open_dedup_key
    ON incidents (status_page_id, external_dedup_key)
    WHERE external_dedup_key IS NOT NULL AND resolved_at IS NULL AND deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS incidents_open_dedup_key;
ALTER TABLE incidents DROP COLUMN IF EXISTS external_dedup_key;
ALTER TABLE incidents DROP COLUMN IF EXISTS integration_id;
DROP TABLE IF EXISTS webhook_integrations;
