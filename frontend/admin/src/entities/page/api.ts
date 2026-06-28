import { api } from "@/shared/api";
import type { StatusPage, StatusPageCreate, StatusPageUpdate } from "@/shared/api";

export function listPages(): Promise<StatusPage[]> {
  return api.get<StatusPage[]>("/pages");
}

export function getPage(id: string): Promise<StatusPage> {
  return api.get<StatusPage>(`/pages/${id}`);
}

export function createPage(body: StatusPageCreate): Promise<StatusPage> {
  return api.post<StatusPage>("/pages", body);
}

export function updatePage(id: string, body: StatusPageUpdate): Promise<StatusPage> {
  return api.patch<StatusPage>(`/pages/${id}`, body);
}

export function deletePage(id: string): Promise<void> {
  return api.delete<void>(`/pages/${id}`);
}
