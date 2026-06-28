import { api } from "@/shared/api";
import type {
  ComponentGroup,
  ComponentGroupCreate,
  ComponentGroupUpdate,
} from "@/shared/api";

export function listGroups(pageId: string): Promise<ComponentGroup[]> {
  return api.get<ComponentGroup[]>(`/pages/${pageId}/component-groups`);
}

export function createGroup(
  pageId: string,
  body: ComponentGroupCreate,
): Promise<ComponentGroup> {
  return api.post<ComponentGroup>(`/pages/${pageId}/component-groups`, body);
}

export function updateGroup(
  id: string,
  body: ComponentGroupUpdate,
): Promise<ComponentGroup> {
  return api.patch<ComponentGroup>(`/component-groups/${id}`, body);
}

export function deleteGroup(id: string): Promise<void> {
  return api.delete<void>(`/component-groups/${id}`);
}
