-- Этап 3.1: подписчики и журнал уведомлений (DESIGN §3.5, §4.1, §4.4, §5, §8.1).
--
-- Subscriber — внешний клиент, подписанный на обновления страницы по каналу
-- (email/telegram/rss/ical/webhook/max/slack). Подтверждение — double opt-in (confirmed +
-- confirm_token); отписка — по unsubscribe_token. scope = вся страница или набор компонентов.
-- Notification — журнал отправок для идемпотентности и ретраев (§8.1): статус, число попыток.
--
-- Решения по схеме (в рамках DESIGN §5, контракт не меняется):
--   - channel/scope/status — TEXT + CHECK, а НЕ pg-enum: их нет в нормативном списке enum'ов
--     DESIGN §5 (как role/visibility/source — см. 00003/00004). Нормативные значения совпадают с
--     openapi SubscriberChannel/SubscriberScope.
--   - Subscriber без soft-delete (в модели §5 нет deleted_at): отписка — физическое удаление строки.
--   - confirm_token/unsubscribe_token — capability-токены из писем/ссылок; по §9 хранить хэш
--     (генерация/хэширование — в сервисе на 3.4/3.5). Здесь это просто nullable-колонки + быстрый
--     уникальный поиск по токену.

-- +goose Up
CREATE TABLE subscribers (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status_page_id    uuid NOT NULL REFERENCES status_pages (id) ON DELETE CASCADE,
    channel           text NOT NULL
        CHECK (channel IN ('email', 'telegram', 'rss', 'ical', 'webhook', 'max', 'slack')),
    address           text NOT NULL,
    confirmed         boolean NOT NULL DEFAULT false,
    confirm_token     text,
    unsubscribe_token text,
    scope             text NOT NULL DEFAULT 'page'
        CHECK (scope IN ('page', 'components')),
    component_ids     uuid[] NOT NULL DEFAULT '{}',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
-- одна подписка на (страница, канал, адрес) — идемпотентность повторной подписки.
CREATE UNIQUE INDEX subscribers_page_channel_address_unique
    ON subscribers (status_page_id, channel, address);
CREATE INDEX subscribers_page_idx ON subscribers (status_page_id);
-- быстрый и уникальный поиск по токенам подтверждения/отписки.
CREATE UNIQUE INDEX subscribers_confirm_token_key
    ON subscribers (confirm_token) WHERE confirm_token IS NOT NULL;
CREATE UNIQUE INDEX subscribers_unsubscribe_token_key
    ON subscribers (unsubscribe_token) WHERE unsubscribe_token IS NOT NULL;

CREATE TABLE notifications (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subscriber_id uuid NOT NULL REFERENCES subscribers (id) ON DELETE CASCADE,
    event_type    text NOT NULL,
    payload       jsonb NOT NULL DEFAULT '{}'::jsonb,
    status        text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'sent', 'failed')),
    attempts      integer NOT NULL DEFAULT 0,
    sent_at       timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_subscriber_idx ON notifications (subscriber_id);
-- воркер выбирает необработанные (pending) записи.
CREATE INDEX notifications_status_idx ON notifications (status);

CREATE TRIGGER subscribers_set_updated_at BEFORE UPDATE ON subscribers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER notifications_set_updated_at BEFORE UPDATE ON notifications
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS subscribers;
