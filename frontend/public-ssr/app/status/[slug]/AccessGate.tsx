// Парольный гейт приватной страницы (этап 4.2). Нативная HTML-форма (без клиентского JS):
// POST на route handler /status/[slug]/access, который проверяет пароль через backend и ставит
// HttpOnly-cookie доступа. Рендерится, когда публичный API вернул 401 (нужен пароль).

import { dict, withLang } from "../../../lib/i18n";
import type { Locale } from "../../../lib/i18n";

export function AccessGate({
  slug,
  locale,
  error,
}: {
  slug: string;
  locale: Locale;
  error: boolean;
}) {
  const t = dict(locale);
  return (
    <main className="page">
      <section className="gate">
        <h1>{t.access.title}</h1>
        <p className="gate-prompt">{t.access.prompt}</p>
        <form method="POST" action={withLang(`/status/${encodeURIComponent(slug)}/access`, locale)}>
          <input type="hidden" name="lang" value={locale} />
          <label className="gate-label" htmlFor="hp-access-password">
            {t.access.passwordLabel}
          </label>
          <input
            id="hp-access-password"
            className="gate-input"
            type="password"
            name="password"
            autoComplete="current-password"
            autoFocus
            required
          />
          {error ? <div className="gate-error">{t.access.error}</div> : null}
          <button className="gate-submit" type="submit">
            {t.access.submit}
          </button>
        </form>
      </section>
    </main>
  );
}
