// Тема публичной страницы (этап 4.1). Хранится в StatusPage.theme — свободный jsonb
// (openapi PublicPage.theme: additionalProperties). Конкретную структуру задаёт соглашение
// фронт ↔ оператор (админка пишет те же ключи). Здесь — безопасная нормализация значения,
// пришедшего из публичного API: неизвестное/битьё игнорируем, подставляем дефолты.

import type { CSSProperties } from "react";

export type ThemeMode = "light" | "dark" | "auto";
export type TimeFormat = "24h" | "12h";

export interface PageTheme {
  // Акцентный цвет (ссылки, активная вкладка, плашка логотипа). null → дефолт темы.
  primaryColor: string | null;
  // Тёмная тема: light (по умолчанию), dark (всегда тёмная), auto (по prefers-color-scheme).
  mode: ThemeMode;
  // Формат времени: 24h (по умолчанию) или 12h (am/pm).
  timeFormat: TimeFormat;
}

const HEX_RE = /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;

export const DEFAULT_THEME: PageTheme = {
  primaryColor: null,
  mode: "light",
  timeFormat: "24h",
};

// parseTheme нормализует значение theme из публичного API (PublicPage.theme).
export function parseTheme(raw: unknown): PageTheme {
  const t = (raw && typeof raw === "object" ? raw : {}) as Record<string, unknown>;
  const mode: ThemeMode = t.mode === "dark" || t.mode === "auto" ? t.mode : "light";
  const timeFormat: TimeFormat = t.time_format === "12h" ? "12h" : "24h";
  const primaryColor =
    typeof t.primary_color === "string" && HEX_RE.test(t.primary_color)
      ? t.primary_color
      : null;
  return { primaryColor, mode, timeFormat };
}

// themeVars — inline CSS-переменные акцента для обёртки страницы.
export function themeVars(theme: PageTheme): CSSProperties {
  const vars: Record<string, string> = {};
  if (theme.primaryColor) {
    vars["--accent"] = theme.primaryColor;
  }
  return vars as CSSProperties;
}

export function is12h(theme: PageTheme): boolean {
  return theme.timeFormat === "12h";
}
