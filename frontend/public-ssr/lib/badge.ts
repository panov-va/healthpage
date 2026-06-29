// Цвета бейджей инцидентов/работ — поверх тех же CSS-переменных статусов (--st-*).

import type { IncidentImpact, IncidentStatus, MaintenanceStatus } from "./api";

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

export function incidentStatusColor(status: IncidentStatus): string {
  return status === "resolved"
    ? "var(--st-operational)"
    : "var(--st-degraded_performance)";
}

export function maintenanceStatusColor(status: MaintenanceStatus): string {
  switch (status) {
    case "completed":
      return "var(--st-operational)";
    case "in_progress":
    case "scheduled":
    default:
      return "var(--st-under_maintenance)";
  }
}
