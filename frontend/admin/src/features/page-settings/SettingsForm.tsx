import { useEffect, useState } from "react";

import {
  addAllowedEmail,
  deleteAllowedEmail,
  listAllowedEmails,
  updatePage,
  verifyDomain,
} from "@/entities/page";
import type { AllowedEmail, DomainStatus, StatusPage } from "@/entities/page";
import { HttpError } from "@/shared/api";
import { Button, Field, Input, Select } from "@/shared/ui";

// Настройки страницы: тема (этап 4.1), приватность по паролю (4.2), собственный домен (4.3).
// Всё через PATCH /pages/{id}; проверка домена — POST /pages/{id}/domain/verify.

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
  const [visibility, setVisibility] = useState<"public" | "private">(
    page.visibility === "private" ? "private" : "public",
  );
  const [password, setPassword] = useState("");
  const [clearPassword, setClearPassword] = useState(false);
  const [allowedEmails, setAllowedEmails] = useState<AllowedEmail[]>([]);
  const [newEmail, setNewEmail] = useState("");
  const [timezone, setTimezone] = useState(page.timezone ?? "UTC");
  const [locale, setLocale] = useState(page.default_locale ?? "ru");
  const [color, setColor] = useState(initial.color);
  const [mode, setMode] = useState<ThemeMode>(initial.mode);
  const [timeFormat, setTimeFormat] = useState<TimeFormat>(initial.timeFormat);
  const [logoUrl, setLogoUrl] = useState(page.logo_url ?? "");
  const [faviconUrl, setFaviconUrl] = useState(page.favicon_url ?? "");
  const [hidePoweredBy, setHidePoweredBy] = useState(page.hide_powered_by ?? false);
  const [customDomain, setCustomDomain] = useState(page.custom_domain ?? "");
  // Письма (этап 4.5): from_email читается из ответа; smtp_config (с паролем) — нет, поэтому
  // отправляем его только если оператор тронул секцию SMTP (иначе не перетираем сохранённый).
  const [fromEmail, setFromEmail] = useState(page.from_email ?? "");
  const [useSMTP, setUseSMTP] = useState(page.smtp_configured ?? false);
  const [smtpTouched, setSmtpTouched] = useState(false);
  const [smtp, setSMTP] = useState({
    host: "",
    port: "587",
    username: "",
    password: "",
    from: "",
    tls: false,
  });
  const touchSMTP = (patch: Partial<typeof smtp>) => {
    setSmtpTouched(true);
    setSMTP((s) => ({ ...s, ...patch }));
  };
  const [domainStatus, setDomainStatus] = useState<DomainStatus | null>(null);
  const [verifying, setVerifying] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  // При наличии сохранённого домена подтягиваем статус + целевой хост (cname_target для инструкции).
  useEffect(() => {
    if (page.custom_domain) {
      verifyDomain(page.id)
        .then(setDomainStatus)
        .catch(() => setDomainStatus(null));
    }
  }, [page.id, page.custom_domain]);

  // Список email доступа (этап 4.2.1) — грузим для приватной страницы.
  useEffect(() => {
    if (page.visibility === "private") {
      listAllowedEmails(page.id)
        .then(setAllowedEmails)
        .catch(() => setAllowedEmails([]));
    }
  }, [page.id, page.visibility]);

  async function addEmail() {
    const email = newEmail.trim();
    if (!email) return;
    setError(null);
    try {
      const created = await addAllowedEmail(page.id, email);
      setAllowedEmails((prev) => [...prev, created]);
      setNewEmail("");
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось добавить email");
    }
  }

  async function removeEmail(id: string) {
    setError(null);
    try {
      await deleteAllowedEmail(id);
      setAllowedEmails((prev) => prev.filter((e) => e.id !== id));
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось удалить email");
    }
  }

  async function checkDomain() {
    setVerifying(true);
    setError(null);
    try {
      setDomainStatus(await verifyDomain(page.id));
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось проверить домен");
    } finally {
      setVerifying(false);
    }
  }

  const zones = TIMEZONES.includes(timezone) ? TIMEZONES : [timezone, ...TIMEZONES];

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setSaved(false);
    try {
      const body: Parameters<typeof updatePage>[1] = {
        name,
        description,
        visibility,
        timezone,
        default_locale: locale,
        theme: { primary_color: color, mode, time_format: timeFormat },
        logo_url: logoUrl.trim() || null,
        favicon_url: faviconUrl.trim() || null,
        hide_powered_by: hidePoweredBy,
        custom_domain: customDomain.trim() || null,
        from_email: fromEmail.trim() || null,
      };
      // Пароль: задаём только при непустом вводе; снимаем при отметке «снять» (null).
      // Пустой ввод без отметки — не трогаем (текущий пароль нельзя прочитать).
      if (clearPassword) {
        body.password = null;
      } else if (password.trim()) {
        body.password = password;
      }
      // SMTP: отправляем только если оператор тронул секцию (нельзя перечитать пароль).
      if (smtpTouched) {
        body.smtp_config = useSMTP
          ? {
              host: smtp.host.trim(),
              port: Number(smtp.port) || 587,
              username: smtp.username.trim(),
              password: smtp.password,
              from: smtp.from.trim(),
              tls: smtp.tls,
            }
          : null;
      }
      const updated = await updatePage(page.id, body);
      onSaved(updated);
      setPassword("");
      setClearPassword(false);
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

      <h3 style={{ margin: "16px 0 8px" }}>Доступ</h3>
      <Field label="Видимость">
        <Select
          value={visibility}
          onChange={(e) => setVisibility(e.target.value === "private" ? "private" : "public")}
        >
          <option value="public">Публичная</option>
          <option value="private">Приватная (по паролю)</option>
        </Select>
      </Field>
      {visibility === "private" ? (
        <>
          <Field label="Пароль страницы">
            <Input
              type="password"
              autoComplete="new-password"
              placeholder="Оставьте пустым, чтобы не менять"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={clearPassword}
            />
          </Field>
          <label className="hp-checkbox" style={{ display: "block", marginBottom: 8 }}>
            <input
              type="checkbox"
              checked={clearPassword}
              onChange={(e) => setClearPassword(e.target.checked)}
            />{" "}
            Снять пароль
          </label>
          <div className="hp-muted" style={{ fontSize: 13, marginBottom: 8 }}>
            Без заданного пароля приватная страница недоступна посетителям. Текущий пароль
            показать нельзя — введите новый, чтобы сменить.
          </div>

          <Field label="Доступ по списку email (magic-link)">
            <div className="hp-inline-form">
              <Input
                type="email"
                placeholder="guest@example.com"
                value={newEmail}
                onChange={(e) => setNewEmail(e.target.value)}
              />
              <Button type="button" onClick={addEmail} disabled={!newEmail.trim()}>
                Добавить
              </Button>
            </div>
          </Field>
          {allowedEmails.length > 0 ? (
            <div style={{ marginBottom: 8 }}>
              {allowedEmails.map((e) => (
                <div key={e.id} className="hp-list-item">
                  <span>{e.email}</span>
                  <Button type="button" onClick={() => removeEmail(e.id)}>
                    Удалить
                  </Button>
                </div>
              ))}
            </div>
          ) : (
            <div className="hp-muted" style={{ fontSize: 13, marginBottom: 8 }}>
              Список пуст. Добавьте адреса — они смогут получить ссылку доступа на почту.
            </div>
          )}
        </>
      ) : null}

      <h3 style={{ margin: "16px 0 8px" }}>Регион</h3>
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
      <label className="hp-checkbox" style={{ display: "block", marginBottom: 8 }}>
        <input
          type="checkbox"
          checked={hidePoweredBy}
          onChange={(e) => setHidePoweredBy(e.target.checked)}
        />{" "}
        Скрыть «Работает на HealthPage» (white-label)
      </label>

      <h3 style={{ margin: "16px 0 8px" }}>Письма</h3>
      <Field label="Адрес отправителя (From)">
        <Input
          type="email"
          placeholder="status@вашдомен.ru"
          value={fromEmail}
          onChange={(e) => setFromEmail(e.target.value)}
        />
      </Field>
      <label className="hp-checkbox" style={{ display: "block", marginBottom: 8 }}>
        <input
          type="checkbox"
          checked={useSMTP}
          onChange={(e) => {
            setSmtpTouched(true);
            setUseSMTP(e.target.checked);
          }}
        />{" "}
        Использовать собственный SMTP{page.smtp_configured ? " (настроен)" : ""}
      </label>
      {useSMTP ? (
        <>
          <div className="hp-inline-form">
            <Field label="Хост">
              <Input value={smtp.host} onChange={(e) => touchSMTP({ host: e.target.value })} />
            </Field>
            <Field label="Порт">
              <Input
                type="number"
                value={smtp.port}
                onChange={(e) => touchSMTP({ port: e.target.value })}
              />
            </Field>
          </div>
          <div className="hp-inline-form">
            <Field label="Пользователь">
              <Input
                value={smtp.username}
                onChange={(e) => touchSMTP({ username: e.target.value })}
              />
            </Field>
            <Field label="Пароль">
              <Input
                type="password"
                autoComplete="new-password"
                placeholder={page.smtp_configured ? "Введите заново для изменения" : ""}
                value={smtp.password}
                onChange={(e) => touchSMTP({ password: e.target.value })}
              />
            </Field>
          </div>
          <Field label="From внутри SMTP (необязательно)">
            <Input value={smtp.from} onChange={(e) => touchSMTP({ from: e.target.value })} />
          </Field>
          <label className="hp-checkbox" style={{ display: "block", marginBottom: 8 }}>
            <input
              type="checkbox"
              checked={smtp.tls}
              onChange={(e) => touchSMTP({ tls: e.target.checked })}
            />{" "}
            Неявный TLS (порт 465); иначе STARTTLS
          </label>
          <div className="hp-muted" style={{ fontSize: 13, marginBottom: 8 }}>
            Пароль SMTP не отображается в целях безопасности — при изменении введите заново.
          </div>
        </>
      ) : null}

      <h3 style={{ margin: "16px 0 8px" }}>Собственный домен</h3>
      <Field label="Домен">
        <Input
          type="text"
          placeholder="status.вашдомен.ru"
          value={customDomain}
          onChange={(e) => setCustomDomain(e.target.value)}
        />
      </Field>
      {domainStatus?.cname_target ? (
        <div className="hp-muted" style={{ fontSize: 13, marginBottom: 8 }}>
          Направьте CNAME вашего домена на <code>{domainStatus.cname_target}</code>, затем нажмите
          «Проверить домен». После проверки TLS-сертификат выпускается автоматически.
        </div>
      ) : (
        <div className="hp-muted" style={{ fontSize: 13, marginBottom: 8 }}>
          Укажите домен и сохраните, затем направьте на нас CNAME и проверьте привязку.
        </div>
      )}
      {page.custom_domain ? (
        <div style={{ marginBottom: 8 }}>
          <span className={`hp-badge ${domainStatus?.domain_verified ?? page.domain_verified ? "hp-badge--ok" : ""}`}>
            {(domainStatus?.domain_verified ?? page.domain_verified)
              ? "Домен подтверждён"
              : "Ожидает проверки CNAME"}
          </span>{" "}
          <Button type="button" onClick={checkDomain} disabled={verifying}>
            {verifying ? "Проверка…" : "Проверить домен"}
          </Button>
        </div>
      ) : null}

      <h3 style={{ margin: "16px 0 8px" }}>Виджет статуса</h3>
      <div style={{ marginBottom: 8 }}>
        <img
          src={`/api/v1/pages/${encodeURIComponent(page.slug)}/badge.svg`}
          alt="Превью бейджа статуса"
          style={{ verticalAlign: "middle" }}
        />
      </div>
      <div className="hp-muted" style={{ fontSize: 13, marginBottom: 4 }}>
        Встройте бейдж на свой сайт (замените хост на ваш публичный домен):
      </div>
      <textarea
        className="hp-input"
        readOnly
        rows={2}
        style={{ fontFamily: "monospace", fontSize: 12 }}
        value={`<a href="https://<ваш-домен>/status/${page.slug}"><img src="https://<ваш-домен>/api/v1/pages/${page.slug}/badge.svg" alt="${page.name} status" /></a>`}
        onFocus={(e) => e.currentTarget.select()}
      />

      {error && <div className="hp-error">{error}</div>}
      {saved && <div className="hp-muted">Сохранено</div>}
      <Button type="submit" disabled={busy || !name}>
        {busy ? "…" : "Сохранить"}
      </Button>
    </form>
  );
}
