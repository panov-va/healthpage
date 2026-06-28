import { api } from "@/shared/api";
import type {
  Component,
  ComponentCreate,
  ComponentStatus,
  ComponentUpdate,
} from "@/shared/api";

// Компоненты управляются плоско через /components (+ status_page_id), т.к. вложенный
// /pages/{id}/components коллизирует с публичным роутом (см. MEMORY.md, этап 1.5).
export function listComponents(statusPageId: string): Promise<Component[]> {
  return api.get<Component[]>(
    `/components?status_page_id=${encodeURIComponent(statusPageId)}`,
  );
}

export function createComponent(body: ComponentCreate): Promise<Component> {
  return api.post<Component>("/components", body);
}

export function updateComponent(id: string, body: ComponentUpdate): Promise<Component> {
  return api.patch<Component>(`/components/${id}`, body);
}

// Ручная смена статуса — частный случай PATCH (backend пишет историю периодов).
export function changeComponentStatus(
  id: string,
  status: ComponentStatus,
): Promise<Component> {
  return api.patch<Component>(`/components/${id}`, { current_status: status });
}

export function deleteComponent(id: string): Promise<void> {
  return api.delete<void>(`/components/${id}`);
}
