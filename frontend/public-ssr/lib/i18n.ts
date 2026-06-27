// Минимальный i18n для публичных страниц (RU/EN с первого дня — CLAUDE.md §6).
// Локаль определяется query-параметром ?lang=en (дефолт — ru). Полноценный i18n
// (по default_locale страницы, Accept-Language) — отдельная задача кастомизации.

import type { ComponentStatus } from "./api";

export type Locale = "ru" | "en";

export function resolveLocale(value: string | string[] | undefined): Locale {
  const v = Array.isArray(value) ? value[0] : value;
  return v === "en" ? "en" : "ru";
}

interface Dict {
  // Заголовок-сводка по общему статусу страницы.
  overall: Record<ComponentStatus, string>;
  // Подпись статуса отдельного компонента/группы.
  status: Record<ComponentStatus, string>;
  updatedAt: string;
  noComponents: string;
  notFoundTitle: string;
  notFoundBody: string;
  poweredBy: string;
}

const ru: Dict = {
  overall: {
    operational: "Все системы работают штатно",
    degraded_performance: "Снижение производительности",
    under_maintenance: "Идут плановые работы",
    partial_outage: "Частичный сбой",
    major_outage: "Серьёзный сбой",
  },
  status: {
    operational: "Работает",
    degraded_performance: "Снижение производительности",
    under_maintenance: "Плановые работы",
    partial_outage: "Частичный сбой",
    major_outage: "Серьёзный сбой",
  },
  updatedAt: "Обновлено",
  noComponents: "Компоненты ещё не добавлены.",
  notFoundTitle: "Страница не найдена",
  notFoundBody: "Запрашиваемая страница статуса не существует или недоступна.",
  poweredBy: "Работает на HealthPage",
};

const en: Dict = {
  overall: {
    operational: "All systems operational",
    degraded_performance: "Degraded performance",
    under_maintenance: "Scheduled maintenance in progress",
    partial_outage: "Partial outage",
    major_outage: "Major outage",
  },
  status: {
    operational: "Operational",
    degraded_performance: "Degraded performance",
    under_maintenance: "Under maintenance",
    partial_outage: "Partial outage",
    major_outage: "Major outage",
  },
  updatedAt: "Updated",
  noComponents: "No components have been added yet.",
  notFoundTitle: "Page not found",
  notFoundBody: "The requested status page does not exist or is unavailable.",
  poweredBy: "Powered by HealthPage",
};

const dicts: Record<Locale, Dict> = { ru, en };

export function dict(locale: Locale): Dict {
  return dicts[locale];
}

export function formatUpdatedAt(iso: string, locale: Locale): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) {
    return iso;
  }
  return new Intl.DateTimeFormat(locale === "en" ? "en-GB" : "ru-RU", {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: "UTC",
  }).format(d);
}
