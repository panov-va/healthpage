import { api } from "@/shared/api";
import type { ImportJob, ImportRequest } from "@/shared/api";

// Импорт из внешних сервисов (этап 7.5). Только оператор.
export function startImport(body: ImportRequest): Promise<ImportJob> {
  return api.post<ImportJob>("/import", body);
}

export function getImportJob(jobId: string): Promise<ImportJob> {
  return api.get<ImportJob>(`/import/${jobId}`);
}
