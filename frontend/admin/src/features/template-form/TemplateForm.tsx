import { useState } from "react";

import type { Component } from "@/entities/component";
import {
  createTemplate,
  patchTemplate,
} from "@/entities/incidentTemplate";
import type { IncidentTemplate } from "@/entities/incidentTemplate";
import { HttpError, INCIDENT_IMPACTS } from "@/shared/api";
import type { IncidentComponent, IncidentImpact } from "@/shared/api";
import { incidentImpactLabel } from "@/shared/lib/incident";
import { Button, Field, Input, Select } from "@/shared/ui";
import { AffectedComponents } from "@/widgets/affected-components";

// Форма шаблона инцидента: создание (template не задан) или редактирование.
export function TemplateForm({
  pageId,
  template,
  components,
  onSaved,
  onCancel,
}: {
  pageId: string;
  template?: IncidentTemplate;
  components: Component[];
  onSaved: (tpl: IncidentTemplate) => void;
  onCancel?: () => void;
}) {
  const [name, setName] = useState(template?.name ?? "");
  const [titleTmpl, setTitleTmpl] = useState(template?.title_tmpl ?? "");
  const [bodyTmpl, setBodyTmpl] = useState(template?.body_tmpl ?? "");
  const [impact, setImpact] = useState<IncidentImpact>(template?.default_impact ?? "none");
  const [affected, setAffected] = useState<IncidentComponent[]>(
    template?.default_components ?? [],
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const payload = {
        name,
        title_tmpl: titleTmpl,
        body_tmpl: bodyTmpl,
        default_impact: impact,
        default_components: affected,
      };
      const saved = template
        ? await patchTemplate(template.id, payload)
        : await createTemplate({ ...payload, status_page_id: pageId });
      if (!template) {
        setName("");
        setTitleTmpl("");
        setBodyTmpl("");
        setImpact("none");
        setAffected([]);
      }
      onSaved(saved);
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось сохранить шаблон");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit}>
      <h3 style={{ marginBottom: 12 }}>{template ? "Редактировать шаблон" : "Новый шаблон"}</h3>
      <Field label="Имя шаблона (для оператора)">
        <Input required value={name} onChange={(e) => setName(e.target.value)} />
      </Field>
      <Field label="Заготовка заголовка">
        <Input value={titleTmpl} onChange={(e) => setTitleTmpl(e.target.value)} />
      </Field>
      <Field label="Заготовка первого обновления">
        <textarea
          className="hp-input"
          rows={3}
          value={bodyTmpl}
          onChange={(e) => setBodyTmpl(e.target.value)}
        />
      </Field>
      <Field label="Влияние по умолчанию">
        <Select value={impact} onChange={(e) => setImpact(e.target.value as IncidentImpact)}>
          {INCIDENT_IMPACTS.map((i) => (
            <option key={i} value={i}>
              {incidentImpactLabel(i)}
            </option>
          ))}
        </Select>
      </Field>
      <Field label="Преднастроенные компоненты">
        <AffectedComponents components={components} value={affected} onChange={setAffected} />
      </Field>
      {error && <div className="hp-error">{error}</div>}
      <div className="hp-row" style={{ gap: 8 }}>
        <Button type="submit" disabled={busy || !name}>
          {busy ? "…" : template ? "Сохранить" : "Создать шаблон"}
        </Button>
        {onCancel && (
          <Button type="button" variant="secondary" onClick={onCancel}>
            Отмена
          </Button>
        )}
      </div>
    </form>
  );
}
