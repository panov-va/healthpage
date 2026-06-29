// Вкладки публичной страницы статуса: Статус / Инциденты / Плановые работы (этап 2.10).
// Серверный компонент — просто ссылки, сохраняющие выбранную локаль (?lang).

import Link from "next/link";

import { dict, withLang } from "../../../lib/i18n";
import type { Locale } from "../../../lib/i18n";

export type StatusTab = "overview" | "incidents" | "maintenances";

export function StatusTabs({
  slug,
  locale,
  active,
}: {
  slug: string;
  locale: Locale;
  active: StatusTab;
}) {
  const t = dict(locale);
  const base = `/status/${encodeURIComponent(slug)}`;
  const tabs: { key: StatusTab; href: string; label: string }[] = [
    { key: "overview", href: base, label: t.tabs.overview },
    { key: "incidents", href: `${base}/incidents`, label: t.tabs.incidents },
    { key: "maintenances", href: `${base}/maintenances`, label: t.tabs.maintenances },
  ];
  return (
    <nav className="tabs">
      {tabs.map((tab) => (
        <Link
          key={tab.key}
          href={withLang(tab.href, locale)}
          className={`tab${tab.key === active ? " tab-active" : ""}`}
          aria-current={tab.key === active ? "page" : undefined}
        >
          {tab.label}
        </Link>
      ))}
    </nav>
  );
}
