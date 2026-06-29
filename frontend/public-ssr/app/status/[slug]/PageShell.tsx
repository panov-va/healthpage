// Оформление публичной страницы (этап 4.1): тема (цвета/тёмный режим через CSS-переменные),
// шапка с логотипом и названием, футер «Работает на …» (скрывается white-label'ом — 4.4).
// Серверный компонент; оборачивает содержимое каждой вкладки статуса.

import type { ReactNode } from "react";

import type { PublicPage } from "../../../lib/api";
import { dict } from "../../../lib/i18n";
import type { Locale } from "../../../lib/i18n";
import { parseTheme, themeVars } from "../../../lib/theme";

export function PageShell({
  page,
  locale,
  children,
}: {
  page: PublicPage;
  locale: Locale;
  children: ReactNode;
}) {
  const t = dict(locale);
  const theme = parseTheme(page.theme);

  return (
    <div className="page-shell" data-theme={theme.mode} style={themeVars(theme)}>
      <header className="brand">
        <div className="brand-inner">
          {page.logo_url ? (
            // eslint-disable-next-line @next/next/no-img-element -- произвольный внешний URL логотипа
            <img className="brand-logo" src={page.logo_url} alt={page.name} />
          ) : null}
          <span className="brand-name">{page.name}</span>
        </div>
      </header>

      {children}

      {page.hide_powered_by ? null : (
        <footer className="footer">{t.poweredBy}</footer>
      )}
    </div>
  );
}
