-- Этап 7.2: страница релизов / changelog (DESIGN §5 ChangelogEntry, §175).
-- Лента анонсов продукта, отдельно от инцидентов. Черновики (published=false) скрыты публично.
-- body хранится как текст; на публичной странице выводится экранированным (без markdown-движка).

-- +goose Up
CREATE TABLE changelog_entries (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status_page_id uuid NOT NULL REFERENCES status_pages (id) ON DELETE CASCADE,
    title          text NOT NULL,
    body           text NOT NULL DEFAULT '',
    published      boolean NOT NULL DEFAULT false,
    published_at   timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
-- Публичный список — опубликованные, новые сверху (по published_at, затем created_at).
CREATE INDEX changelog_page_idx
    ON changelog_entries (status_page_id, published_at DESC NULLS LAST, created_at DESC);

CREATE TRIGGER changelog_set_updated_at BEFORE UPDATE ON changelog_entries
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS changelog_entries;
