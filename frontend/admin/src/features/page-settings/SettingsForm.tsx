import { useState } from "react";

import { updatePage } from "@/entities/page";
import type { StatusPage } from "@/entities/page";
import { HttpError } from "@/shared/api";
import { Button, Field, Input, Select } from "@/shared/ui";

// Настройки страницы: тема (акцент/тёмный режим/формат времени), логотип/favicon,
// часовой пояс, локаль, название/описание (этап 4.1). Всё через PATCH /pages/{id}
// (контракт не меняется). hide_powered_by/redirect/кастомный домен — отдельные задачи 4.x.

type ThemeMode = "light" | "dark" | "auto";
type TimeFormat = "24h" | "12h";

// Часовые пояса (IANA). Достаточный для MVP набор; текущее значение добавляется, если его нет.
const TIMEZONES = [
  "UTC",
  "Europe/Moscow",
  "Europe/Kaliningrad",
  "Europe/Samara",
  "Asia/Yekaterinburg",
  "Asia/Omsk",
  "Asia/Krasnoyarsk",
  "Asia/Irkutsk",
  "Asia/Yakutsk",
  "Asia/Vladivostok",
  "Asia/Magadan",
  "Asia/Kamchatka",
  "Europe/London",
  "Europe/Berlin",
  "America/New_York",
  "America/Los_Angeles",
];

function readTheme(theme: unknown): { color: string; mode: ThemeMode; timeFormat: TimeFormat } {
  const t = (theme && typeof theme === "object" ? theme : {}) as Record<string, unknown>;
  const mode: ThemeMode = t.mode === "dark" || t.mode === "auto" ? t.mode : "light";
  const timeFormat: TimeFormat = t.time_format === "12h" ? "12h" : "24h";
  const color = typeof t.primary_color === "string" ? t.primary_color : "#0969da";
  return { color, mode, timeFormat };
}

export function SettingsForm({
  page,
  onSaved,
}: {
  page: StatusPage;
  onSaved: (updated: StatusPage) => void;
}) {
  const initial = readTheme(page.theme);
  const [name, setName] = useState(page.name);
  const [description, setDescription] = useState(page.description ?? "");
  const [timezone, setTimezone] = useState(page.timezone ?? "UTC");
  const [locale, setLocale] = useState(page.default_locale ?? "ru");
  const [color, setColor] = useState(initial.color);
  const [mode, setMode] = useState<ThemeMode>(initial.mode);
  const [timeFormat, setTimeFormat] = useState<TimeFormat>(initial.timeFormat);
  const [logoUrl, setLogoUrl] = useState(page.logo_url ?? "");
  const [faviconUrl, setFaviconUrl] = useState(page.favicon_url ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  const zones = TIMEZONES.includes(timezone) ? TIMEZONES : [timezone, ...TIMEZONES];

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setSaved(false);
    try {
      const updated = await updatePage(page.id, {
        name,
        description,
        timezone,
        default_locale: locale,
        theme: { primary_color: color, mode, time_format: timeFormat },
        logo_url: logoUrl.trim() || null,
        favicon_url: faviconUrl.trim() || null,
      });
      onSaved(updated);
      setSaved(true);
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось сохранить настройки");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit}>
      <Field label="Название">
        <Input required value={name} onChange={(e) => setName(e.target.value)} />
      </Field>
      <Field label="Описание">
        <textarea
          className="hp-input"
          rows={2}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
      </Field>

      <div className="hp-inline-form">
        <Field label="Часовой пояс">
          <Select value={timezone} onChange={(e) => setTimezone(e.target.value)}>
            {zones.map((z) => (
              <option key={z} value={z}>
                {z}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Язык по умолчанию">
          <Select value={locale} onChange={(e) => setLocale(e.target.value)}>
            <option value="ru">Русский</option>
            <option value="en">English</option>
          </Select>
        </Field>
      </div>

      <h3 style={{ margin: "16px 0 8px" }}>Оформление</h3>
      <div className="hp-inline-form">
        <Field label="Акцентный цвет">
          <Input
            type="color"
            value={color}
            onChange={(e) => setColor(e.target.value)}
            style={{ height: 40, padding: 2 }}
          />
        </Field>
        <Field label="Тема">
          <Select value={mode} onChange={(e) => setMode(e.target.value as ThemeMode)}>
            <option value="light">Светлая</option>
            <option value="dark">Тёмная</option>
            <option value="auto">Как в системе</option>
          </Select>
        </Field>
        <Field label="Формат времени">
          <Select
            value={timeFormat}
            onChange={(e) => setTimeFormat(e.target.value as TimeFormat)}
          >
            <option value="24h">24 часа</option>
            <option value="12h">12 часов (AM/PM)</option>
          </Select>
        </Field>
      </div>

      <Field label="URL логотипа">
        <Input
          type="url"
          placeholder="https://…/logo.png"
          value={logoUrl}
          onChange={(e) => setLogoUrl(e.target.value)}
        />
      </Field>
      <Field label="URL favicon">
        <Input
          type="url"
          placeholder="https://…/favicon.ico"
          value={faviconUrl}
          onChange={(e) => setFaviconUrl(e.target.value)}
        />
      </Field>

      {error && <div className="hp-error">{error}</div>}
      {saved && <div className="hp-muted">Сохранено</div>}
      <Button type="submit" disabled={busy || !name}>
        {busy ? "…" : "Сохранить"}
      </Button>
    </form>
  );
}
