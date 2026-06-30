-- Этап 4.2.1: приватные страницы по списку email (DESIGN §3.1, §3.6).
--
-- page_allowed_emails — список адресов, которым разрешён доступ к приватной странице по
-- magic-link (письмо со ссылкой → токен доступа → cookie, как пароль в 4.2). Хранится сам
-- email (не хэш): это операторская конфигурация доступа, не учётные данные клиента; нужен для
-- сверки при запросе ссылки. Уникальность — по (странице, email без учёта регистра).

-- +goose Up
CREATE TABLE page_allowed_emails (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status_page_id uuid NOT NULL REFERENCES status_pages (id) ON DELETE CASCADE,
    email          text NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX page_allowed_emails_key ON page_allowed_emails (status_page_id, lower(email));
CREATE INDEX page_allowed_emails_page_idx ON page_allowed_emails (status_page_id);

-- +goose Down
DROP TABLE IF EXISTS page_allowed_emails;
