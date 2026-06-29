import { api } from "@/shared/api";
import type { Subscriber, SubscriberCreate } from "@/shared/api";

// Админ-управление подписчиками (этап 3.10). Список — по status_page_id (вкл. неподтверждённых),
// ручное добавление и удаление — управляющие эндпоинты под JWT.
export function listSubscribers(
  statusPageId: string,
  page = 1,
  perPage = 50,
): Promise<Subscriber[]> {
  const q = new URLSearchParams();
  q.set("status_page_id", statusPageId);
  q.set("page", String(page));
  q.set("per_page", String(perPage));
  return api.get<Subscriber[]>(`/subscribers?${q.toString()}`);
}

export function createSubscriber(body: SubscriberCreate): Promise<Subscriber> {
  return api.post<Subscriber>("/subscribers", body);
}

export function deleteSubscriber(id: string): Promise<void> {
  return api.delete<void>(`/subscribers/${id}`);
}
