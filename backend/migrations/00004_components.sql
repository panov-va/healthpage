-- Этап 1.1: группы компонентов, компоненты (дерево через parent_id) и история
-- статусов компонентов (DESIGN §3.2, §5, §6).

-- +goose Up
CREATE TABLE component_groups (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status_page_id uuid NOT NULL REFERENCES status_pages (id) ON DELETE CASCADE,
    name           text NOT NULL,
    position       integer NOT NULL DEFAULT 0,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz
);
CREATE INDEX component_groups_page_idx ON component_groups (status_page_id);

CREATE TABLE components (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status_page_id uuid NOT NULL REFERENCES status_pages (id) ON DELETE CASCADE,
    group_id       uuid REFERENCES component_groups (id) ON DELETE SET NULL,
    -- дерево подкомпонентов: удаление родителя каскадом удаляет потомков.
    parent_id      uuid REFERENCES components (id) ON DELETE CASCADE,
    name           text NOT NULL,
    description    text NOT NULL DEFAULT '',
    position       integer NOT NULL DEFAULT 0,
    current_status component_status NOT NULL DEFAULT 'operational',
    is_private     boolean NOT NULL DEFAULT false,
    show_uptime    boolean NOT NULL DEFAULT true,
    display_state  boolean NOT NULL DEFAULT true,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    deleted_at     timestamptz
);
CREATE INDEX components_page_idx ON components (status_page_id);
CREATE INDEX components_group_idx ON components (group_id);
CREATE INDEX components_parent_idx ON components (parent_id);

CREATE TABLE component_status_history (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    component_id uuid NOT NULL REFERENCES components (id) ON DELETE CASCADE,
    status       component_status NOT NULL,
    started_at   timestamptz NOT NULL DEFAULT now(),
    ended_at     timestamptz,
    source       text NOT NULL DEFAULT 'manual'
        CHECK (source IN ('manual', 'incident', 'maintenance', 'api')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX csh_component_idx ON component_status_history (component_id, started_at);
-- у компонента не более одного открытого периода статуса одновременно.
CREATE UNIQUE INDEX csh_open_period_key
    ON component_status_history (component_id) WHERE ended_at IS NULL;

CREATE TRIGGER component_groups_set_updated_at BEFORE UPDATE ON component_groups
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER components_set_updated_at BEFORE UPDATE ON components
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER csh_set_updated_at BEFORE UPDATE ON component_status_history
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS component_status_history;
DROP TABLE IF EXISTS components;
DROP TABLE IF EXISTS component_groups;
