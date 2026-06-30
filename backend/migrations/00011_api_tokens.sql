-- Этап 5.1: API-токены страницы (ApiToken, DESIGN §5).
-- Токен привязан к одной странице (status_page_id). Аутентификация управляющих запросов
-- заголовком `Authorization: <token>` (без префикса Bearer, Статусмейт-совместимо).
-- Храним только ХЭШ токена (DESIGN §9), сам токен показывается единожды при создании.
-- scopes — TEXT[] (нормативные значения read|write — решение человека, не pg-enum: как и
-- channel/scope подписчиков, набор задаётся контрактом openapi, а не §5-enum'ом).

-- +goose Up
CREATE TABLE api_tokens (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status_page_id uuid NOT NULL REFERENCES status_pages (id) ON DELETE CASCADE,
    token_hash     text NOT NULL,
    name           text NOT NULL,
    scopes         text[] NOT NULL DEFAULT '{}',
    last_used_at   timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX api_tokens_hash_key ON api_tokens (token_hash);
CREATE INDEX api_tokens_page_idx ON api_tokens (status_page_id);

CREATE TRIGGER api_tokens_set_updated_at BEFORE UPDATE ON api_tokens
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS api_tokens;
