import { api } from "@/shared/api";
import type {
  IncidentTemplate,
  IncidentTemplateCreate,
  IncidentTemplatePatch,
} from "@/shared/api";

// Шаблоны — плоский CRUD (+ status_page_id), как компоненты (см. MEMORY.md, этап 2.7).
export function listTemplates(statusPageId: string): Promise<IncidentTemplate[]> {
  return api.get<IncidentTemplate[]>(
    `/incident-templates?status_page_id=${encodeURIComponent(statusPageId)}`,
  );
}

export function createTemplate(body: IncidentTemplateCreate): Promise<IncidentTemplate> {
  return api.post<IncidentTemplate>("/incident-templates", body);
}

export function patchTemplate(
  id: string,
  body: IncidentTemplatePatch,
): Promise<IncidentTemplate> {
  return api.patch<IncidentTemplate>(`/incident-templates/${id}`, body);
}

export function deleteTemplate(id: string): Promise<void> {
  return api.delete<void>(`/incident-templates/${id}`);
}
