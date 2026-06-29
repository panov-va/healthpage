// Клиент публичного API HealthPage для SSR.
//
// Типы повторяют JSON, который реально отдаёт backend (internal/api/public.go,
// internal/api/components.go) — он соответствует openapi.yaml (PageSummary, Component),
// но публичная сводка отдаёт у группы агрегированный статус (aggregated_status).
// Контракт — единственный источник истины; типы фронта генерируются из openapi.yaml,
// здесь — узкая прослойка под конкретный SSR-запрос.

export type ComponentStatus =
  | "operational"
  | "degraded_performance"
  | "under_maintenance"
  | "partial_outage"
  | "major_outage";

export interface ApiComponent {
  id: string;
  group_id: string | null;
  parent_id: string | null;
  name: string;
  description: string;
  position: number;
  current_status: ComponentStatus;
  is_private: boolean;
  show_uptime: boolean;
  display_state: boolean;
  created_at: string;
  updated_at: string;
}

export interface ApiGroup {
  id: string;
  name: string;
  position: number;
  aggregated_status: ComponentStatus;
  components: ApiComponent[];
}

export type IncidentStatus = "investigating" | "identified" | "monitoring" | "resolved";
export type IncidentImpact = "none" | "minor" | "major" | "critical";
export type MaintenanceStatus = "scheduled" | "in_progress" | "completed";

export interface ApiIncidentComponent {
  component_id: string;
  component_status_in_incident: ComponentStatus;
}

export interface ApiIncidentUpdate {
  id: string;
  status: IncidentStatus;
  body: string;
  notify_subscribers: boolean;
  created_at: string;
}

export interface ApiIncident {
  id: string;
  title: string;
  current_status: IncidentStatus;
  impact: IncidentImpact;
  started_at: string;
  resolved_at: string | null;
  postmortem: string | null;
  is_visible: boolean;
  components: ApiIncidentComponent[];
  updates: ApiIncidentUpdate[];
}

export interface ApiMaintenanceUpdate {
  id: string;
  body: string;
  notify_subscribers: boolean;
  created_at: string;
}

export interface ApiMaintenance {
  id: string;
  title: string;
  description: string | null;
  status: MaintenanceStatus;
  scheduled_start: string;
  scheduled_end: string;
  started_at: string | null;
  completed_at: string | null;
  component_ids: string[];
  updates: ApiMaintenanceUpdate[];
}

export interface Pagination {
  page: number;
  per_page: number;
  total: number;
}

export interface IncidentList {
  items: ApiIncident[];
  pagination: Pagination;
}

export interface MaintenanceList {
  items: ApiMaintenance[];
  pagination: Pagination;
}

export interface PageSummary {
  overall_status: ComponentStatus;
  updated_at: string;
  groups: ApiGroup[];
  ungrouped_components: ApiComponent[];
  active_incidents: ApiIncident[];
  active_maintenances: ApiMaintenance[];
}

// Базовый URL backend-API. В docker — http://api:8080/api/v1; локально — http://localhost:8080/api/v1.
// Имя переменной задокументировано в .env.example / INFRASTRUCTURE.md.
function apiBaseURL(): string {
  return process.env.HEALTHPAGE_API_URL ?? "http://localhost:8080/api/v1";
}

// PageNotFoundError — страница не существует или приватна (backend отдаёт 404 в обоих случаях,
// чтобы не раскрывать существование приватных страниц).
export class PageNotFoundError extends Error {}

// getJSON — общий SSR-GET к публичному API (без кэша Next: свежесть важнее; кэш сводки —
// отдельная задача через Redis на backend). 404 → PageNotFoundError (страница/ресурс не найден
// или приватный/скрытый — backend не раскрывает разницу).
async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${apiBaseURL()}${path}`, { cache: "no-store" });
  if (res.status === 404) {
    throw new PageNotFoundError(path);
  }
  if (!res.ok) {
    throw new Error(`request failed: ${res.status} ${path}`);
  }
  return (await res.json()) as T;
}

export async function fetchPageSummary(slug: string): Promise<PageSummary> {
  return getJSON<PageSummary>(`/pages/${encodeURIComponent(slug)}/summary`);
}

// fetchComponents — публичный список компонентов (для маппинга id → имя в инцидентах/работах;
// приватные компоненты backend не отдаёт).
export async function fetchComponents(slug: string): Promise<ApiComponent[]> {
  return getJSON<ApiComponent[]>(`/pages/${encodeURIComponent(slug)}/components`);
}

export async function fetchIncidents(
  slug: string,
  page: number,
  perPage: number,
): Promise<IncidentList> {
  const q = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  return getJSON<IncidentList>(`/pages/${encodeURIComponent(slug)}/incidents?${q.toString()}`);
}

export async function fetchIncident(slug: string, id: string): Promise<ApiIncident> {
  return getJSON<ApiIncident>(
    `/pages/${encodeURIComponent(slug)}/incidents/${encodeURIComponent(id)}`,
  );
}

export async function fetchMaintenances(
  slug: string,
  page: number,
  perPage: number,
): Promise<MaintenanceList> {
  const q = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  return getJSON<MaintenanceList>(
    `/pages/${encodeURIComponent(slug)}/maintenances?${q.toString()}`,
  );
}

// unsubscribeByToken выполняет отписку по HMAC-токену из письма (GET /unsubscribe). Идемпотентно
// на стороне backend (валидный токен всегда 200, даже если строки уже нет). Возвращает true при
// успехе, false при недействительном токене (400).
export async function unsubscribeByToken(token: string): Promise<boolean> {
  const res = await fetch(
    `${apiBaseURL()}/unsubscribe?token=${encodeURIComponent(token)}`,
    { cache: "no-store" },
  );
  return res.ok;
}

// componentNameMap строит id → имя из публичного списка компонентов.
export function componentNameMap(components: ApiComponent[]): Record<string, string> {
  const map: Record<string, string> = {};
  for (const c of components) {
    map[c.id] = c.name;
  }
  return map;
}
