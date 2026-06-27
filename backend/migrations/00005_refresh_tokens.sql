-- Этап 1.3: хранилище refresh-токенов для ротации и отзыва (выбор: JWT access + refresh).
-- Это вспомогательная таблица аутентификации, не доменная сущность из DESIGN §5.
-- Храним только ХЭШ refresh-токена (как и пароли/API-токены — DESIGN §9), не сам токен.

-- +goose Up
CREATE TABLE refresh_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX refresh_tokens_hash_key ON refresh_tokens (token_hash);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);

CREATE TRIGGER refresh_tokens_set_updated_at BEFORE UPDATE ON refresh_tokens
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;
