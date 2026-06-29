import { useState } from "react";

import type { Component } from "@/entities/component";
import { patchIncident } from "@/entities/incident";
import type { Incident, IncidentComponent } from "@/entities/incident";
import { HttpError, INCIDENT_IMPACTS } from "@/shared/api";
import type { IncidentImpact } from "@/shared/api";
import { incidentImpactLabel } from "@/shared/lib/incident";
import { Button, Field, Input, Select } from "@/shared/ui";
import { AffectedComponents } from "@/widgets/affected-components";

// Редактирование инцидента: заголовок/влияние/видимость/постмортем/компоненты.
// Постмортем — только для resolved (бэкенд вернёт 422 иначе).
export function EditIncidentForm({
  incident,
  components,
  onSaved,
}: {
  incident: Incident;
  components: Component[];
  onSaved: (updated: Incident) => void;
}) {
  const [title, setTitle] = useState(incident.title);
  const [impact, setImpact] = useState<IncidentImpact>(incident.impact);
  const [isVisible, setIsVisible] = useState(incident.is_visible ?? true);
  const [postmortem, setPostmortem] = useState(incident.postmortem ?? "");
  const [affected, setAffected] = useState<IncidentComponent[]>(incident.components ?? []);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const resolved = incident.current_status === "resolved";

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const updated = await patchIncident(incident.id, {
        title,
        impact,
        is_visible: isVisible,
        postmortem: resolved ? postmortem : undefined,
        components: affected,
      });
      onSaved(updated);
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось сохранить инцидент");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit}>
      <Field label="Заголовок">
        <Input required value={title} onChange={(e) => setTitle(e.target.value)} />
      </Field>
      <Field label="Влияние">
        <Select value={impact} onChange={(e) => setImpact(e.target.value as IncidentImpact)}>
          {INCIDENT_IMPACTS.map((i) => (
            <option key={i} value={i}>
              {incidentImpactLabel(i)}
            </option>
          ))}
        </Select>
      </Field>
      <Field label="Затронутые компоненты">
        <AffectedComponents components={components} value={affected} onChange={setAffected} />
      </Field>
      <Field label={resolved ? "Постмортем" : "Постмортем (доступен после решения инцидента)"}>
        <textarea
          className="hp-input"
          rows={4}
          disabled={!resolved}
          value={postmortem}
          onChange={(e) => setPostmortem(e.target.value)}
        />
      </Field>
      <label className="hp-row" style={{ gap: 8, marginBottom: 12 }}>
        <input
          type="checkbox"
          checked={isVisible}
          onChange={(e) => setIsVisible(e.target.checked)}
        />
        <span>Виден на публичной странице</span>
      </label>
      {error && <div className="hp-error">{error}</div>}
      <Button type="submit" disabled={busy || !title}>
        {busy ? "…" : "Сохранить"}
      </Button>
    </form>
  );
}
