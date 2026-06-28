import type { ComponentStatus, OverallStatus } from "@/shared/api";

// Человекочитаемые подписи статусов (RU). Нормативные ключи — из openapi enum.
const LABELS: Record<ComponentStatus, string> = {
  operational: "Работает",
  degraded_performance: "Снижение производительности",
  under_maintenance: "Плановые работы",
  partial_outage: "Частичный сбой",
  major_outage: "Серьёзный сбой",
};

export function statusLabel(status: ComponentStatus | OverallStatus): string {
  return LABELS[status as ComponentStatus] ?? status;
}

// Цвет статуса берём из CSS-переменной --st-<status>.
export function statusColor(status: ComponentStatus | OverallStatus): string {
  return `var(--st-${status})`;
}
