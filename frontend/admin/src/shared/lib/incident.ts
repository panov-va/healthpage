import type { IncidentImpact, IncidentStatus, MaintenanceStatus } from "@/shared/api";

// Человекочитаемые подписи enum'ов инцидентов/работ (RU). Нормативные ключи — из
// openapi enum (incident_status / incident_impact / maintenance_status).
const INCIDENT_STATUS_LABELS: Record<IncidentStatus, string> = {
  investigating: "Расследуем",
  identified: "Причина найдена",
  monitoring: "Наблюдаем",
  resolved: "Решён",
};

const INCIDENT_IMPACT_LABELS: Record<IncidentImpact, string> = {
  none: "Без влияния",
  minor: "Незначительное",
  major: "Существенное",
  critical: "Критическое",
};

const MAINTENANCE_STATUS_LABELS: Record<MaintenanceStatus, string> = {
  scheduled: "Запланированы",
  in_progress: "Идут",
  completed: "Завершены",
};

export function incidentStatusLabel(status: IncidentStatus): string {
  return INCIDENT_STATUS_LABELS[status] ?? status;
}

export function incidentImpactLabel(impact: IncidentImpact): string {
  return INCIDENT_IMPACT_LABELS[impact] ?? impact;
}

export function maintenanceStatusLabel(status: MaintenanceStatus): string {
  return MAINTENANCE_STATUS_LABELS[status] ?? status;
}

// Цвет impact-бейджа: используем те же CSS-переменные статусов, что и компоненты.
export function impactColor(impact: IncidentImpact): string {
  switch (impact) {
    case "critical":
      return "var(--st-major_outage)";
    case "major":
      return "var(--st-partial_outage)";
    case "minor":
      return "var(--st-degraded_performance)";
    default:
      return "var(--muted)";
  }
}

// Локальный человекочитаемый формат даты/времени (берём локаль браузера).
export function formatDateTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}

// ── helpers для <input type="datetime-local"> ──
// Значение инпута — локальное "YYYY-MM-DDTHH:mm" без зоны. Конвертируем в/из ISO (UTC).
export function isoToInput(iso: string | null | undefined): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  // сдвигаем на локальную зону, чтобы datetime-local показал местное время
  const tzOffsetMs = d.getTimezoneOffset() * 60_000;
  return new Date(d.getTime() - tzOffsetMs).toISOString().slice(0, 16);
}

// "" → undefined (поле опционально); иначе локальное время → ISO.
export function inputToIso(value: string): string | undefined {
  if (!value) return undefined;
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return undefined;
  return d.toISOString();
}
