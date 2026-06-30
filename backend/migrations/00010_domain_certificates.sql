-- Этап 4.3.2: хранилище TLS-сертификатов кастомных доменов + ACME-аккаунт и HTTP-01 challenge'и.
--
-- domain_certificates — выпущенные сертификаты (cert+key PEM) по доменам; edge-прокси (4.3.3)
-- грузит их по SNI, tls-manager продлевает по expires_at.
-- acme_accounts — ACME-аккаунт (ключ + регистрация) для lego; одна строка на directory URL.
-- acme_challenges — активные HTTP-01 токены; tls-manager кладёт, edge отдаёт по
-- /.well-known/acme-challenge/<token>, после выпуска tls-manager удаляет.

-- +goose Up
CREATE TABLE domain_certificates (
    domain     text PRIMARY KEY,
    cert_pem   text NOT NULL,
    key_pem    text NOT NULL,
    issued_at  timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE acme_accounts (
    directory_url text PRIMARY KEY,
    email         text NOT NULL,
    private_key   text NOT NULL,
    registration  jsonb NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE acme_challenges (
    token      text PRIMARY KEY,
    key_auth   text NOT NULL,
    domain     text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER domain_certificates_set_updated_at BEFORE UPDATE ON domain_certificates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS acme_challenges;
DROP TABLE IF EXISTS acme_accounts;
DROP TABLE IF EXISTS domain_certificates;
