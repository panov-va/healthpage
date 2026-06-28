-- Этап 2.1: инциденты и плановые работы (DESIGN §3.3, §3.4, §5, §6).
-- Нормативные enum'ы incident_status / incident_impact / maintenance_status
-- (DESIGN §5, openapi) — как pg enum (по аналогии с component_status).
-- Все таблицы — с created_at/updated_at (DESIGN §5); пользовательский контент
-- (incidents, maintenances) — с soft-delete (deleted_at).

-- +goose Up
CREATE TYPE incident_status AS ENUM (
    'investigating',
    'identified',
    'monitoring',
    'resolved'
);

CREATE TYPE incident_impact AS ENUM (
    'none',
    'minor',
    'major',
    'critical'
);

CREATE TYPE maintenance_status AS ENUM (
    'scheduled',
    'in_progress',
    'completed'
);

-- ── Инциденты ───────────────────────────────────────────────────────────
CREATE TABLE incidents (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status_page_id uuid NOT NULL REFERENCES status_pages (id) ON DELETE CASCADE,
    title          text NOT NULL,
    current_status incident_status NOT NULL,
    impact         incident_impact NOT NULL,
    started_at     timestamptz NOT NULL DEFAULT now(),
    resolved_at    timestamptz,
    postmortem     text,
    is_visible     boolean NOT NULL DEFAULT true,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz
);
CREATE INDEX incidents_page_idx ON incidents (status_page_id, started_at DESC);

CREATE TABLE incident_components (
    id                           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id                  uuid NOT NULL REFERENCES incidents (id) ON DELETE CASCADE,
    component_id                 uuid NOT NULL REFERENCES components (id) ON DELETE CASCADE,
    component_status_in_incident component_status NOT NULL,
    created_at                   timestamptz NOT NULL DEFAULT now(),
    updated_at                   timestamptz NOT NULL DEFAULT now()
);
-- компонент фигурирует в инциденте не более одного раза.
CREATE UNIQUE INDEX incident_components_unique ON incident_components (incident_id, component_id);
CREATE INDEX incident_components_component_idx ON incident_components (component_id);

CREATE TABLE incident_updates (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id        uuid NOT NULL REFERENCES incidents (id) ON DELETE CASCADE,
    status             incident_status NOT NULL,
    body               text NOT NULL,
    notify_subscribers boolean NOT NULL DEFAULT true,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX incident_updates_incident_idx ON incident_updates (incident_id, created_at);

-- ── Плановые работы ─────────────────────────────────────────────────────
CREATE TABLE maintenances (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status_page_id  uuid NOT NULL REFERENCES status_pages (id) ON DELETE CASCADE,
    title           text NOT NULL,
    description     text NOT NULL DEFAULT '',
    status          maintenance_status NOT NULL DEFAULT 'scheduled',
    scheduled_start timestamptz NOT NULL,
    scheduled_end   timestamptz NOT NULL,
    started_at      timestamptz,
    completed_at    timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz
);
CREATE INDEX maintenances_page_idx ON maintenances (status_page_id, scheduled_start);

CREATE TABLE maintenance_components (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    maintenance_id uuid NOT NULL REFERENCES maintenances (id) ON DELETE CASCADE,
    component_id   uuid NOT NULL REFERENCES components (id) ON DELETE CASCADE,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX maintenance_components_unique
    ON maintenance_components (maintenance_id, component_id);
CREATE INDEX maintenance_components_component_idx ON maintenance_components (component_id);

CREATE TABLE maintenance_updates (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    maintenance_id     uuid NOT NULL REFERENCES maintenances (id) ON DELETE CASCADE,
    body               text NOT NULL,
    notify_subscribers boolean NOT NULL DEFAULT true,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX maintenance_updates_maintenance_idx
    ON maintenance_updates (maintenance_id, created_at);

-- ── Триггеры updated_at ─────────────────────────────────────────────────
CREATE TRIGGER incidents_set_updated_at BEFORE UPDATE ON incidents
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER incident_components_set_updated_at BEFORE UPDATE ON incident_components
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER incident_updates_set_updated_at BEFORE UPDATE ON incident_updates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER maintenances_set_updated_at BEFORE UPDATE ON maintenances
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER maintenance_components_set_updated_at BEFORE UPDATE ON maintenance_components
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER maintenance_updates_set_updated_at BEFORE UPDATE ON maintenance_updates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS maintenance_updates;
DROP TABLE IF EXISTS maintenance_components;
DROP TABLE IF EXISTS maintenances;
DROP TABLE IF EXISTS incident_updates;
DROP TABLE IF EXISTS incident_components;
DROP TABLE IF EXISTS incidents;
DROP TYPE IF EXISTS maintenance_status;
DROP TYPE IF EXISTS incident_impact;
DROP TYPE IF EXISTS incident_status;
