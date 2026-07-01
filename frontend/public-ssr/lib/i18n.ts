// Минимальный i18n для публичных страниц (RU/EN с первого дня — CLAUDE.md §6).
// Локаль определяется query-параметром ?lang=en (дефолт — ru). Полноценный i18n
// (по default_locale страницы, Accept-Language) — отдельная задача кастомизации.

import type {
  ComponentStatus,
  IncidentImpact,
  IncidentStatus,
  MaintenanceStatus,
} from "./api";

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
  uptimeOver: string;
  notFoundTitle: string;
  notFoundBody: string;
  poweredBy: string;

  // Вкладки и разделы инцидентов/работ (этап 2.10).
  tabs: { overview: string; incidents: string; maintenances: string; changelog: string };
  changelogTitle: string;
  noChangelog: string;
  incidentStatus: Record<IncidentStatus, string>;
  impact: Record<IncidentImpact, string>;
  maintenanceStatus: Record<MaintenanceStatus, string>;
  incidentsTitle: string;
  maintenancesTitle: string;
  noIncidents: string;
  noMaintenances: string;
  impactLabel: string;
  started: string;
  resolved: string;
  scheduledWindow: string;
  affectedComponents: string;
  postmortem: string;
  updatesTitle: string;
  openIncident: string;
  prevPage: string;
  nextPage: string;

  // Парольный гейт приватной страницы (этап 4.2) + magic-link по email (4.2.1).
  access: {
    title: string;
    prompt: string;
    passwordLabel: string;
    submit: string;
    error: string;
    or: string;
    emailLabel: string;
    requestLink: string;
    linkSent: string;
  };
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
  uptimeOver: "за {days} дней",
  notFoundTitle: "Страница не найдена",
  notFoundBody: "Запрашиваемая страница статуса не существует или недоступна.",
  poweredBy: "Работает на HealthPage",

  tabs: { overview: "Статус", incidents: "Инциденты", maintenances: "Плановые работы", changelog: "Релизы" },
  changelogTitle: "Релизы",
  noChangelog: "Записей пока нет.",
  incidentStatus: {
    investigating: "Расследуем",
    identified: "Причина найдена",
    monitoring: "Наблюдаем",
    resolved: "Решён",
  },
  impact: {
    none: "Без влияния",
    minor: "Незначительное",
    major: "Существенное",
    critical: "Критическое",
  },
  maintenanceStatus: {
    scheduled: "Запланированы",
    in_progress: "Идут",
    completed: "Завершены",
  },
  incidentsTitle: "Инциденты",
  maintenancesTitle: "Плановые работы",
  noIncidents: "Инцидентов пока не было.",
  noMaintenances: "Плановых работ пока нет.",
  impactLabel: "Влияние",
  started: "Начало",
  resolved: "Решён",
  scheduledWindow: "Запланировано",
  affectedComponents: "Затронутые компоненты",
  postmortem: "Постмортем",
  updatesTitle: "Хроника обновлений",
  openIncident: "Подробнее",
  prevPage: "← Новее",
  nextPage: "Старее →",
  access: {
    title: "Приватная страница",
    prompt: "Эта страница статуса защищена. Введите пароль для доступа.",
    passwordLabel: "Пароль",
    submit: "Войти",
    error: "Неверный пароль. Попробуйте ещё раз.",
    or: "или",
    emailLabel: "Доступ по email",
    requestLink: "Прислать ссылку",
    linkSent: "Если адрес в списке доступа, мы отправили ссылку на почту.",
  },
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
  uptimeOver: "over {days} days",
  notFoundTitle: "Page not found",
  notFoundBody: "The requested status page does not exist or is unavailable.",
  poweredBy: "Powered by HealthPage",

  tabs: { overview: "Status", incidents: "Incidents", maintenances: "Maintenance", changelog: "Changelog" },
  changelogTitle: "Changelog",
  noChangelog: "No entries yet.",
  incidentStatus: {
    investigating: "Investigating",
    identified: "Identified",
    monitoring: "Monitoring",
    resolved: "Resolved",
  },
  impact: {
    none: "No impact",
    minor: "Minor",
    major: "Major",
    critical: "Critical",
  },
  maintenanceStatus: {
    scheduled: "Scheduled",
    in_progress: "In progress",
    completed: "Completed",
  },
  incidentsTitle: "Incidents",
  maintenancesTitle: "Scheduled maintenance",
  noIncidents: "No incidents reported yet.",
  noMaintenances: "No scheduled maintenance yet.",
  impactLabel: "Impact",
  started: "Started",
  resolved: "Resolved",
  scheduledWindow: "Scheduled",
  affectedComponents: "Affected components",
  postmortem: "Postmortem",
  updatesTitle: "Update history",
  openIncident: "Details",
  prevPage: "← Newer",
  nextPage: "Older →",
  access: {
    title: "Private page",
    prompt: "This status page is protected. Enter the password to continue.",
    passwordLabel: "Password",
    submit: "Enter",
    error: "Wrong password. Please try again.",
    or: "or",
    emailLabel: "Access by email",
    requestLink: "Send link",
    linkSent: "If your address is on the access list, we've emailed you a link.",
  },
};

const dicts: Record<Locale, Dict> = { ru, en };

export function dict(locale: Locale): Dict {
  return dicts[locale];
}

// withLang добавляет к пути ?lang=en, чтобы локаль не терялась при переходах (дефолт ru — без параметра).
export function withLang(path: string, locale: Locale): string {
  return locale === "en" ? `${path}?lang=en` : path;
}

// ── Лендинг (SSR для SEO) ──

interface PlanFeature {
  label: string;
  free: boolean;
  premium: boolean;
}

interface LandingDict {
  metaTitle: string;
  metaDescription: string;
  heroTitle: string;
  heroSubtitle: string;
  ctaPrimary: string;
  ctaSecondary: string;
  featuresTitle: string;
  features: { title: string; body: string }[];
  pricingTitle: string;
  planFree: string;
  planPremium: string;
  priceFree: string;
  pricePremium: string;
  featureColumn: string;
  planFeatures: PlanFeature[];
}

// Матрица фич — DESIGN §10. Конкретная цена Premium не указана: [РЕШЕНО] ориентир «Статусмейт −10%»,
// но точная сумма берётся перед запуском ([ТРЕБОВАНИЕ]), поэтому на лендинге — «уточняется».
const landingRu: LandingDict = {
  metaTitle: "HealthPage — страницы статуса для вашего продукта",
  metaDescription:
    "Создавайте публичные страницы статуса, публикуйте инциденты и плановые работы, " +
    "уведомляйте клиентов по email, Telegram, MAX и Slack. Manual-first, без сложной настройки.",
  heroTitle: "Страницы статуса для вашего продукта",
  heroSubtitle:
    "Сообщайте клиентам о состоянии сервиса, инцидентах и плановых работах — " +
    "вручную и под полным контролем. Подписки по email, RSS, Telegram, MAX и Slack.",
  ctaPrimary: "Начать бесплатно",
  ctaSecondary: "Посмотреть пример",
  featuresTitle: "Возможности",
  features: [
    { title: "Ручное управление статусом", body: "Вы сами управляете состоянием сервисов и компонентов — без ложных срабатываний автомониторинга." },
    { title: "Инциденты и плановые работы", body: "Публикуйте ленту обновлений; плановые работы не считаются простоем." },
    { title: "Подписки и уведомления", body: "Email, RSS, iCal, Telegram, MAX и Slack — без лимита на число подписчиков." },
    { title: "Брендирование", body: "Тема, тёмный режим, логотип, собственный домен и white-label." },
  ],
  pricingTitle: "Тарифы",
  planFree: "Free",
  planPremium: "Premium",
  priceFree: "Бесплатно",
  pricePremium: "Цена уточняется",
  featureColumn: "Возможность",
  planFeatures: [
    { label: "Компоненты, группы, инциденты, работы", free: true, premium: true },
    { label: "Подписчики без лимита (email/RSS/Telegram/MAX/Slack)", free: true, premium: true },
    { label: "Брендирование, тёмная тема, шаблоны", free: true, premium: true },
    { label: "Управление командой и уровни доступа", free: true, premium: true },
    { label: "Собственный домен (CNAME + TLS)", free: false, premium: true },
    { label: "Приватные страницы", free: false, premium: true },
    { label: "Custom SMTP / собственный From", free: false, premium: true },
    { label: "Скрытие «Работает на …» (white-label)", free: false, premium: true },
    { label: "Приоритетная поддержка", free: false, premium: true },
  ],
};

const landingEn: LandingDict = {
  metaTitle: "HealthPage — status pages for your product",
  metaDescription:
    "Build public status pages, publish incidents and scheduled maintenance, and notify " +
    "customers via email, Telegram, MAX and Slack. Manual-first, no complex setup.",
  heroTitle: "Status pages for your product",
  heroSubtitle:
    "Keep customers informed about service health, incidents and scheduled maintenance — " +
    "manually and fully in your control. Subscriptions via email, RSS, Telegram, MAX and Slack.",
  ctaPrimary: "Get started free",
  ctaSecondary: "See an example",
  featuresTitle: "Features",
  features: [
    { title: "Manual status control", body: "You control the state of your services and components — no false alarms from auto-monitoring." },
    { title: "Incidents & maintenance", body: "Publish an update feed; scheduled maintenance is not counted as downtime." },
    { title: "Subscriptions & notifications", body: "Email, RSS, iCal, Telegram, MAX and Slack — with no limit on subscribers." },
    { title: "Branding", body: "Theme, dark mode, logo, custom domain and white-label." },
  ],
  pricingTitle: "Pricing",
  planFree: "Free",
  planPremium: "Premium",
  priceFree: "Free",
  pricePremium: "Price TBD",
  featureColumn: "Feature",
  planFeatures: [
    { label: "Components, groups, incidents, maintenance", free: true, premium: true },
    { label: "Unlimited subscribers (email/RSS/Telegram/MAX/Slack)", free: true, premium: true },
    { label: "Branding, dark theme, templates", free: true, premium: true },
    { label: "Team management and access levels", free: true, premium: true },
    { label: "Custom domain (CNAME + TLS)", free: false, premium: true },
    { label: "Private pages", free: false, premium: true },
    { label: "Custom SMTP / custom From", free: false, premium: true },
    { label: "Hide “Powered by…” (white-label)", free: false, premium: true },
    { label: "Priority support", free: false, premium: true },
  ],
};

const landingDicts: Record<Locale, LandingDict> = { ru: landingRu, en: landingEn };

export function landing(locale: Locale): LandingDict {
  return landingDicts[locale];
}

// formatInZone форматирует ISO-время в часовом поясе страницы (этап 4.1). Метка пояса
// (timeZoneName: short, напр. «GMT+3»/«MSK») включена в вывод — отдельный суффикс не нужен.
// Невалидный пояс (operator ввёл мусор) → фолбэк на UTC, чтобы рендер не падал.
export function formatInZone(
  iso: string,
  locale: Locale,
  timeZone: string,
  hour12: boolean,
): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) {
    return iso;
  }
  const opts: Intl.DateTimeFormatOptions = {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12,
    timeZoneName: "short",
    timeZone,
  };
  const lang = locale === "en" ? "en-GB" : "ru-RU";
  try {
    return new Intl.DateTimeFormat(lang, opts).format(d);
  } catch {
    return new Intl.DateTimeFormat(lang, { ...opts, timeZone: "UTC" }).format(d);
  }
}
