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

export interface PageSummary {
  overall_status: ComponentStatus;
  updated_at: string;
  groups: ApiGroup[];
  ungrouped_components: ApiComponent[];
  active_incidents: unknown[];
  active_maintenances: unknown[];
}

// Базовый URL backend-API. В docker — http://api:8080/api/v1; локально — http://localhost:8080/api/v1.
// Имя переменной задокументировано в .env.example / INFRASTRUCTURE.md.
function apiBaseURL(): string {
  return process.env.HEALTHPAGE_API_URL ?? "http://localhost:8080/api/v1";
}

// PageNotFoundError — страница не существует или приватна (backend отдаёт 404 в обоих случаях,
// чтобы не раскрывать существование приватных страниц).
export class PageNotFoundError extends Error {}

// fetchPageSummary запрашивает публичную сводку страницы по slug (SSR, без кэша Next:
// свежесть статуса важнее, кэширование сводки — отдельная задача через Redis на backend).
export async function fetchPageSummary(slug: string): Promise<PageSummary> {
  const url = `${apiBaseURL()}/pages/${encodeURIComponent(slug)}/summary`;
  const res = await fetch(url, { cache: "no-store" });
  if (res.status === 404) {
    throw new PageNotFoundError(slug);
  }
  if (!res.ok) {
    throw new Error(`summary request failed: ${res.status}`);
  }
  return (await res.json()) as PageSummary;
}
