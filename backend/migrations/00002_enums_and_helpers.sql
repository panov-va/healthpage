-- Этап 1.1: базовые enum'ы и общая trigger-функция для updated_at.
-- enum component_status — нормативный (DESIGN §3.2/§5, openapi ComponentStatus).
-- enum billing_plan нужен таблице accounts уже сейчас (DESIGN §5); биллинг — этап 6.

-- +goose Up
CREATE TYPE component_status AS ENUM (
    'operational',
    'degraded_performance',
    'partial_outage',
    'major_outage',
    'under_maintenance'
);

CREATE TYPE billing_plan AS ENUM ('free', 'premium');

-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS set_updated_at();
DROP TYPE IF EXISTS billing_plan;
DROP TYPE IF EXISTS component_status;
