-- Этап 2.7: шаблоны инцидентов (DESIGN §3.3, §5 IncidentTemplate).
-- Преднастроенные заготовки для типовых ситуаций: имя, шаблон заголовка/тела,
-- impact по умолчанию и преднастроенные затронутые компоненты с их статусом.
-- Шаблоны — операторская конфигурация (не публичный контент с историей), поэтому
-- без soft-delete: удаление физическое (DELETE), как и в модели DESIGN §5.

-- +goose Up
CREATE TABLE incident_templates (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status_page_id uuid NOT NULL REFERENCES status_pages (id) ON DELETE CASCADE,
    name           text NOT NULL,
    title_tmpl     text NOT NULL DEFAULT '',
    body_tmpl      text NOT NULL DEFAULT '',
    default_impact incident_impact NOT NULL DEFAULT 'none',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX incident_templates_page_idx ON incident_templates (status_page_id);

CREATE TABLE incident_template_components (
    id                           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id                  uuid NOT NULL REFERENCES incident_templates (id) ON DELETE CASCADE,
    component_id                 uuid NOT NULL REFERENCES components (id) ON DELETE CASCADE,
    component_status_in_incident component_status NOT NULL,
    created_at                   timestamptz NOT NULL DEFAULT now(),
    updated_at                   timestamptz NOT NULL DEFAULT now()
);
-- компонент фигурирует в шаблоне не более одного раза.
CREATE UNIQUE INDEX incident_template_components_unique
    ON incident_template_components (template_id, component_id);
CREATE INDEX incident_template_components_component_idx
    ON incident_template_components (component_id);

CREATE TRIGGER incident_templates_set_updated_at BEFORE UPDATE ON incident_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER incident_template_components_set_updated_at BEFORE UPDATE ON incident_template_components
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS incident_template_components;
DROP TABLE IF EXISTS incident_templates;
