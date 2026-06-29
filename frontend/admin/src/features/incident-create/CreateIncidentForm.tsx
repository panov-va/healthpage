import { useState } from "react";

import type { Component } from "@/entities/component";
import { createIncident } from "@/entities/incident";
import type { Incident, IncidentComponent } from "@/entities/incident";
import type { IncidentTemplate } from "@/entities/incidentTemplate";
import { HttpError, INCIDENT_IMPACTS, INCIDENT_STATUSES } from "@/shared/api";
import type { IncidentImpact, IncidentStatus } from "@/shared/api";
import { incidentImpactLabel, incidentStatusLabel, inputToIso } from "@/shared/lib/incident";
import { Button, Field, Input, Select } from "@/shared/ui";
import { AffectedComponents } from "@/widgets/affected-components";

// Создание инцидента. Здесь же UI «применить шаблон»: выбор IncidentTemplate
// префиллит поля формы (title/body/impact/components) — POST /incidents на клиенте,
// серверного apply-эндпоинта нет (см. MEMORY.md, этап 2.7).
export function CreateIncidentForm({
  pageId,
  components,
  templates,
  onCreated,
}: {
  pageId: string;
  components: Component[];
  templates: IncidentTemplate[];
  onCreated: (incident: Incident) => void;
}) {
  const [title, setTitle] = useState("");
  const [status, setStatus] = useState<IncidentStatus>("investigating");
  const [impact, setImpact] = useState<IncidentImpact>("major");
  const [body, setBody] = useState("");
  const [startedAt, setStartedAt] = useState("");
  const [notify, setNotify] = useState(true);
  const [affected, setAffected] = useState<IncidentComponent[]>([]);
  const [templateId, setTemplateId] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function applyTemplate(id: string) {
    setTemplateId(id);
    if (!id) return;
    const tpl = templates.find((t) => t.id === id);
    if (!tpl) return;
    if (tpl.title_tmpl) setTitle(tpl.title_tmpl);
    if (tpl.body_tmpl) setBody(tpl.body_tmpl);
    if (tpl.default_impact) setImpact(tpl.default_impact);
    setAffected(tpl.default_components ?? []);
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const incident = await createIncident({
        status_page_id: pageId,
        title,
        status,
        impact,
        body,
        notify,
        started_at: inputToIso(startedAt),
        components: affected,
      });
      setTitle("");
      setBody("");
      setStartedAt("");
      setImpact("major");
      setStatus("investigating");
      setAffected([]);
      setTemplateId("");
      onCreated(incident);
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось создать инцидент");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit}>
      <h3 style={{ marginBottom: 12 }}>Новый инцидент</h3>
      {templates.length > 0 && (
        <Field label="Применить шаблон">
          <Select value={templateId} onChange={(e) => applyTemplate(e.target.value)}>
            <option value="">— без шаблона —</option>
            {templates.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </Select>
        </Field>
      )}
      <Field label="Заголовок">
        <Input required value={title} onChange={(e) => setTitle(e.target.value)} />
      </Field>
      <div className="hp-inline-form">
        <Field label="Статус">
          <Select
            value={status}
            onChange={(e) => setStatus(e.target.value as IncidentStatus)}
          >
            {INCIDENT_STATUSES.map((s) => (
              <option key={s} value={s}>
                {incidentStatusLabel(s)}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Влияние">
          <Select
            value={impact}
            onChange={(e) => setImpact(e.target.value as IncidentImpact)}
          >
            {INCIDENT_IMPACTS.map((i) => (
              <option key={i} value={i}>
                {incidentImpactLabel(i)}
              </option>
            ))}
          </Select>
        </Field>
      </div>
      <Field label="Первое обновление">
        <textarea
          className="hp-input"
          required
          rows={3}
          value={body}
          onChange={(e) => setBody(e.target.value)}
        />
      </Field>
      <Field label="Время начала (необязательно — по умолчанию сейчас)">
        <Input
          type="datetime-local"
          value={startedAt}
          onChange={(e) => setStartedAt(e.target.value)}
        />
      </Field>
      <Field label="Затронутые компоненты">
        <AffectedComponents components={components} value={affected} onChange={setAffected} />
      </Field>
      <label className="hp-row" style={{ gap: 8, marginBottom: 12 }}>
        <input type="checkbox" checked={notify} onChange={(e) => setNotify(e.target.checked)} />
        <span>Уведомить подписчиков</span>
      </label>
      {error && <div className="hp-error">{error}</div>}
      <Button type="submit" disabled={busy || !title || !body}>
        {busy ? "…" : "Создать инцидент"}
      </Button>
    </form>
  );
}
