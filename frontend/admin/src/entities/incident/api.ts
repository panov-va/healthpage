import { api } from "@/shared/api";
import type {
  Incident,
  IncidentCreate,
  IncidentList,
  IncidentPatch,
  IncidentUpdate,
  IncidentUpdateCreate,
} from "@/shared/api";

export interface IncidentFilter {
  status?: string;
  impact?: string;
  componentId?: string;
  page?: number;
  perPage?: number;
}

// Листинг и detail идут через ПУБЛИЧНЫЕ эндпоинты `/pages/{slug}/incidents` —
// отдельного админского list-эндпоинта в контракте нет (см. MEMORY.md, этап 2.9).
// ⚠️ Следствие: скрытые инциденты (is_visible=false) в этот список не попадают.
export function listIncidents(slug: string, filter: IncidentFilter = {}): Promise<IncidentList> {
  const q = new URLSearchParams();
  if (filter.status) q.set("status", filter.status);
  if (filter.impact) q.set("impact", filter.impact);
  if (filter.componentId) q.set("component_id", filter.componentId);
  q.set("page", String(filter.page ?? 1));
  q.set("per_page", String(filter.perPage ?? 20));
  return api.get<IncidentList>(`/pages/${encodeURIComponent(slug)}/incidents?${q.toString()}`);
}

export function getIncident(slug: string, id: string): Promise<Incident> {
  return api.get<Incident>(`/pages/${encodeURIComponent(slug)}/incidents/${id}`);
}

// Создание/изменение/удаление — управляющие (плоские) эндпоинты под JWT.
export function createIncident(body: IncidentCreate): Promise<Incident> {
  return api.post<Incident>("/incidents", body);
}

export function patchIncident(id: string, body: IncidentPatch): Promise<Incident> {
  return api.patch<Incident>(`/incidents/${id}`, body);
}

export function deleteIncident(id: string): Promise<void> {
  return api.delete<void>(`/incidents/${id}`);
}

export function addIncidentUpdate(
  id: string,
  body: IncidentUpdateCreate,
): Promise<IncidentUpdate> {
  return api.post<IncidentUpdate>(`/incidents/${id}/updates`, body);
}
