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

export type Component = Schemas["Component"];
export type ComponentCreate = Schemas["ComponentCreate"];
export type ComponentUpdate = Schemas["ComponentUpdate"];

export type ComponentGroup = Schemas["ComponentGroup"];
export type ComponentGroupCreate = Schemas["ComponentGroupCreate"];
export type ComponentGroupUpdate = Schemas["ComponentGroupUpdate"];

export type ApiError = Schemas["Error"];

// Нормативный список статусов компонента (для селектов в UI).
export const COMPONENT_STATUSES: ComponentStatus[] = [
  "operational",
  "degraded_performance",
  "under_maintenance",
  "partial_outage",
  "major_outage",
];
