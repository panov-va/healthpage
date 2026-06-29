import { api } from "@/shared/api";
import type {
  Maintenance,
  MaintenanceCreate,
  MaintenanceList,
  MaintenancePatch,
  MaintenanceUpdate,
  MaintenanceUpdateCreate,
} from "@/shared/api";

export interface MaintenanceFilter {
  status?: string;
  page?: number;
  perPage?: number;
}

// Админский листинг/detail — по status_page_id/id (`GET /maintenances`, `GET /maintenances/{id}`).
export function listMaintenances(
  statusPageId: string,
  filter: MaintenanceFilter = {},
): Promise<MaintenanceList> {
  const q = new URLSearchParams();
  q.set("status_page_id", statusPageId);
  if (filter.status) q.set("status", filter.status);
  q.set("page", String(filter.page ?? 1));
  q.set("per_page", String(filter.perPage ?? 20));
  return api.get<MaintenanceList>(`/maintenances?${q.toString()}`);
}

export function getMaintenance(id: string): Promise<Maintenance> {
  return api.get<Maintenance>(`/maintenances/${id}`);
}

export function createMaintenance(body: MaintenanceCreate): Promise<Maintenance> {
  return api.post<Maintenance>("/maintenances", body);
}

export function patchMaintenance(id: string, body: MaintenancePatch): Promise<Maintenance> {
  return api.patch<Maintenance>(`/maintenances/${id}`, body);
}

export function deleteMaintenance(id: string): Promise<void> {
  return api.delete<void>(`/maintenances/${id}`);
}

export function addMaintenanceUpdate(
  id: string,
  body: MaintenanceUpdateCreate,
): Promise<MaintenanceUpdate> {
  return api.post<MaintenanceUpdate>(`/maintenances/${id}/updates`, body);
}
