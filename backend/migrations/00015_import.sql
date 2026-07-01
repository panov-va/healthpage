-- Этап 7.5: каркас миграции данных из внешних сервисов (DESIGN §4.3, §7.6).
-- import_jobs — асинхронные задачи импорта (выполняет worker-import из очереди q.import).
--   api_key источника НЕ хранится (передаётся в сообщении очереди — «не дольше задачи», §4.3).
-- external_id_map — идемпотентность: маппинг внешний id → наш internal id по (странице, источнику,
--   типу сущности). Повторный импорт не плодит дубли (режимы skip/update на уровне логики).

-- +goose Up
CREATE TABLE import_jobs (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status_page_id uuid NOT NULL REFERENCES status_pages (id) ON DELETE CASCADE,
    account_id     uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    source         text NOT NULL CHECK (source IN ('statuspal', 'statuspage', 'instatus', 'statusmate')),
    region         text CHECK (region IN ('us', 'eu')),
    subdomain      text NOT NULL,
    mode           text NOT NULL DEFAULT 'skip' CHECK (mode IN ('skip', 'update')),
    status         text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    report         jsonb NOT NULL DEFAULT '{}',
    error          text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    finished_at    timestamptz,
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX import_jobs_page_idx ON import_jobs (status_page_id, created_at DESC);

CREATE TABLE external_id_map (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status_page_id uuid NOT NULL REFERENCES status_pages (id) ON DELETE CASCADE,
    source         text NOT NULL,
    entity_type    text NOT NULL, -- component | incident | maintenance | subscriber
    external_id    text NOT NULL,
    internal_id    uuid NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX external_id_map_key
    ON external_id_map (status_page_id, source, entity_type, external_id);

CREATE TRIGGER import_jobs_set_updated_at BEFORE UPDATE ON import_jobs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS external_id_map;
DROP TABLE IF EXISTS import_jobs;
