// Удобные псевдонимы типов API. Источник — сгенерированный из openapi.yaml файл
// (@api-types), руками НЕ правится (CLAUDE.md §7). Здесь только пере-экспорт под
// короткими именами, чтобы слои выше не лезли в components["schemas"][...] напрямую.
import type { components } from "@api-types";

type Schemas = components["schemas"];

export type ComponentStatus = Schemas["ComponentStatus"];
export type OverallStatus = Schemas["OverallStatus"];

export type AuthUser = Schemas["AuthUser"];
export type AuthResult = Schemas["AuthResult"];
export type RegisterRequest = Schemas["RegisterRequest"];
export type LoginRequest = Schemas["LoginRequest"];

export type StatusPage = Schemas["StatusPage"];
export type StatusPageCreate = Schemas["StatusPageCreate"];
export type StatusPageUpdate = Schemas["StatusPageUpdate"];
export type DomainStatus = Schemas["DomainStatus"];
export type AllowedEmail = Schemas["AllowedEmail"];

export type Component = Schemas["Component"];
export type ComponentCreate = Schemas["ComponentCreate"];
export type ComponentUpdate = Schemas["ComponentUpdate"];

export type ComponentGroup = Schemas["ComponentGroup"];
export type ComponentGroupCreate = Schemas["ComponentGroupCreate"];
export type ComponentGroupUpdate = Schemas["ComponentGroupUpdate"];

export type Pagination = Schemas["Pagination"];

// ── инциденты ──
export type IncidentStatus = Schemas["IncidentStatus"];
export type IncidentImpact = Schemas["IncidentImpact"];
export type IncidentComponent = Schemas["IncidentComponent"];
export type Incident = Schemas["Incident"];
export type IncidentCreate = Schemas["IncidentCreate"];
export type IncidentPatch = Schemas["IncidentPatch"];
export type IncidentUpdate = Schemas["IncidentUpdate"];
export type IncidentUpdateCreate = Schemas["IncidentUpdateCreate"];
export type IncidentList = Schemas["IncidentList"];

// ── шаблоны инцидентов ──
export type IncidentTemplate = Schemas["IncidentTemplate"];
export type IncidentTemplateCreate = Schemas["IncidentTemplateCreate"];
export type IncidentTemplatePatch = Schemas["IncidentTemplatePatch"];

// ── плановые работы ──
export type MaintenanceStatus = Schemas["MaintenanceStatus"];
export type Maintenance = Schemas["Maintenance"];
export type MaintenanceCreate = Schemas["MaintenanceCreate"];
export type MaintenancePatch = Schemas["MaintenancePatch"];
export type MaintenanceUpdate = Schemas["MaintenanceUpdate"];
export type MaintenanceUpdateCreate = Schemas["MaintenanceUpdateCreate"];
export type MaintenanceList = Schemas["MaintenanceList"];

// ── подписчики ──
export type Subscriber = Schemas["Subscriber"];
export type SubscriberCreate = Schemas["SubscriberCreate"];
export type SubscriberChannel = Schemas["SubscriberChannel"];
export type SubscriberScope = Schemas["SubscriberScope"];

export type ApiError = Schemas["Error"];

// Нормативный список статусов компонента (для селектов в UI).
export const COMPONENT_STATUSES: ComponentStatus[] = [
  "operational",
  "degraded_performance",
  "under_maintenance",
  "partial_outage",
  "major_outage",
];

// Нормативные enum'ы инцидентов/работ (порядок — как в openapi.yaml).
export const INCIDENT_STATUSES: IncidentStatus[] = [
  "investigating",
  "identified",
  "monitoring",
  "resolved",
];

export const INCIDENT_IMPACTS: IncidentImpact[] = ["none", "minor", "major", "critical"];

export const MAINTENANCE_STATUSES: MaintenanceStatus[] = [
  "scheduled",
  "in_progress",
  "completed",
];

// Каналы, доступные оператору для ручного добавления подписчика (push-каналы).
// rss/ical — pull-фиды, webhook — отдельный поток; через ручное добавление не заводятся.
export const MANUAL_SUBSCRIBER_CHANNELS: SubscriberChannel[] = [
  "email",
  "telegram",
  "max",
  "slack",
];
