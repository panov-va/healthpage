-- Этап 1.1: аккаунты, пользователи, членство, страницы статуса (DESIGN §3.1, §5).

-- +goose Up
CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text NOT NULL,
    password_hash text NOT NULL,
    name          text NOT NULL DEFAULT '',
    locale        text NOT NULL DEFAULT 'ru',
    is_active     boolean NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
-- email уникален без учёта регистра.
CREATE UNIQUE INDEX users_email_key ON users (lower(email));

CREATE TABLE accounts (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name          text NOT NULL DEFAULT '',
    billing_plan  billing_plan NOT NULL DEFAULT 'free',
    owner_user_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX accounts_owner_idx ON accounts (owner_user_id);

CREATE TABLE status_pages (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    name            text NOT NULL,
    description     text NOT NULL DEFAULT '',
    slug            text NOT NULL,
    timezone        text NOT NULL DEFAULT 'UTC',
    default_locale  text NOT NULL DEFAULT 'ru',
    visibility      text NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'private')),
    password_hash   text,
    custom_domain   text,
    domain_verified boolean NOT NULL DEFAULT false,
    theme           jsonb NOT NULL DEFAULT '{}'::jsonb,
    logo_url        text,
    favicon_url     text,
    hide_powered_by boolean NOT NULL DEFAULT false,
    smtp_config     jsonb,
    from_email      text,
    redirect_url    text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz
);
-- slug и custom_domain уникальны среди «живых» (не soft-deleted) страниц.
CREATE UNIQUE INDEX status_pages_slug_key ON status_pages (slug) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX status_pages_custom_domain_key
    ON status_pages (custom_domain) WHERE custom_domain IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX status_pages_account_idx ON status_pages (account_id);

CREATE TABLE memberships (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status_page_id uuid NOT NULL REFERENCES status_pages (id) ON DELETE CASCADE,
    role           text NOT NULL CHECK (role IN ('owner', 'admin', 'editor', 'viewer')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX memberships_user_page_key ON memberships (user_id, status_page_id);
CREATE INDEX memberships_page_idx ON memberships (status_page_id);

CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER accounts_set_updated_at BEFORE UPDATE ON accounts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER status_pages_set_updated_at BEFORE UPDATE ON status_pages
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER memberships_set_updated_at BEFORE UPDATE ON memberships
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS memberships;
DROP TABLE IF EXISTS status_pages;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS users;
