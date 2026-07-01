import { api } from "@/shared/api";
import type { ChangelogEntry, ChangelogEntryCreate, ChangelogEntryPatch } from "@/shared/api";

// Changelog / страница релизов (этап 7.2). Управление — под операторским JWT.
export function listChangelog(statusPageId: string): Promise<ChangelogEntry[]> {
  return api.get<ChangelogEntry[]>(`/changelog?status_page_id=${encodeURIComponent(statusPageId)}`);
}

export function createChangelog(body: ChangelogEntryCreate): Promise<ChangelogEntry> {
  return api.post<ChangelogEntry>("/changelog", body);
}

export function patchChangelog(id: string, body: ChangelogEntryPatch): Promise<ChangelogEntry> {
  return api.patch<ChangelogEntry>(`/changelog/${id}`, body);
}

export function deleteChangelog(id: string): Promise<void> {
  return api.delete<void>(`/changelog/${id}`);
}
