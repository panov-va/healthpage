// Гейт приватной страницы: пароль (этап 4.2) ИЛИ magic-link по email (4.2.1). Нативные
// HTML-формы (без клиентского JS) → route handlers /status/[slug]/access и .../access/request-link.
// Оператор настраивает один из методов; гейт показывает оба (неиспользуемый просто не сработает).

import { dict, withLang } from "../../../lib/i18n";
import type { Locale } from "../../../lib/i18n";

export function AccessGate({
  slug,
  locale,
  error,
  linkSent,
}: {
  slug: string;
  locale: Locale;
  error: boolean;
  linkSent: boolean;
}) {
  const t = dict(locale);
  const enc = encodeURIComponent(slug);
  return (
    <main className="page">
      <section className="gate">
        <h1>{t.access.title}</h1>
        <p className="gate-prompt">{t.access.prompt}</p>

        <form method="POST" action={withLang(`/status/${enc}/access`, locale)}>
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
            required
          />
          {error ? <div className="gate-error">{t.access.error}</div> : null}
          <button className="gate-submit" type="submit">
            {t.access.submit}
          </button>
        </form>

        <div className="gate-or">{t.access.or}</div>

        <form method="POST" action={withLang(`/status/${enc}/access/request-link`, locale)}>
          <input type="hidden" name="lang" value={locale} />
          <label className="gate-label" htmlFor="hp-access-email">
            {t.access.emailLabel}
          </label>
          <input
            id="hp-access-email"
            className="gate-input"
            type="email"
            name="email"
            autoComplete="email"
            placeholder="you@example.com"
            required
          />
          {linkSent ? <div className="gate-note">{t.access.linkSent}</div> : null}
          <button className="gate-submit" type="submit">
            {t.access.requestLink}
          </button>
        </form>
      </section>
    </main>
  );
}
