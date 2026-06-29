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

// Админский листинг и detail — по status_page_id/id (эндпоинты `GET /incidents`,
// `GET /incidents/{id}`). В отличие от публичной истории, видны и скрытые инциденты.
export function listIncidents(
  statusPageId: string,
  filter: IncidentFilter = {},
): Promise<IncidentList> {
  const q = new URLSearchParams();
  q.set("status_page_id", statusPageId);
  if (filter.status) q.set("status", filter.status);
  if (filter.impact) q.set("impact", filter.impact);
  if (filter.componentId) q.set("component_id", filter.componentId);
  q.set("page", String(filter.page ?? 1));
  q.set("per_page", String(filter.perPage ?? 20));
  return api.get<IncidentList>(`/incidents?${q.toString()}`);
}

export function getIncident(id: string): Promise<Incident> {
  return api.get<Incident>(`/incidents/${id}`);
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
