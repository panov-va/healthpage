import { useState } from "react";

import type { Component } from "@/entities/component";
import { patchMaintenance } from "@/entities/maintenance";
import type { Maintenance, MaintenanceStatus } from "@/entities/maintenance";
import { HttpError, MAINTENANCE_STATUSES } from "@/shared/api";
import { inputToIso, isoToInput, maintenanceStatusLabel } from "@/shared/lib/incident";
import { Button, Field, Input, Select } from "@/shared/ui";
import { ComponentChecklist } from "@/widgets/component-checklist";

// Редактирование работ, в т.ч. смена статуса жизненного цикла
// (scheduled→in_progress→completed). in_progress переводит компоненты в
// under_maintenance, completed — возвращает (логика на бэкенде).
export function EditMaintenanceForm({
  maintenance,
  components,
  onSaved,
}: {
  maintenance: Maintenance;
  components: Component[];
  onSaved: (updated: Maintenance) => void;
}) {
  const [title, setTitle] = useState(maintenance.title);
  const [description, setDescription] = useState(maintenance.description ?? "");
  const [status, setStatus] = useState<MaintenanceStatus>(maintenance.status);
  const [start, setStart] = useState(isoToInput(maintenance.scheduled_start));
  const [end, setEnd] = useState(isoToInput(maintenance.scheduled_end));
  const [componentIds, setComponentIds] = useState<string[]>(maintenance.component_ids ?? []);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const startIso = inputToIso(start);
    const endIso = inputToIso(end);
    if (!startIso || !endIso) {
      setError("Укажите начало и конец работ");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const updated = await patchMaintenance(maintenance.id, {
        title,
        description,
        status,
        scheduled_start: startIso,
        scheduled_end: endIso,
        component_ids: componentIds,
      });
      onSaved(updated);
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось сохранить работы");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit}>
      <Field label="Заголовок">
        <Input required value={title} onChange={(e) => setTitle(e.target.value)} />
      </Field>
      <Field label="Описание">
        <textarea
          className="hp-input"
          rows={3}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
      </Field>
      <Field label="Статус">
        <Select
          value={status}
          onChange={(e) => setStatus(e.target.value as MaintenanceStatus)}
        >
          {MAINTENANCE_STATUSES.map((s) => (
            <option key={s} value={s}>
              {maintenanceStatusLabel(s)}
            </option>
          ))}
        </Select>
      </Field>
      <div className="hp-inline-form">
        <Field label="Начало">
          <Input
            type="datetime-local"
            required
            value={start}
            onChange={(e) => setStart(e.target.value)}
          />
        </Field>
        <Field label="Конец">
          <Input
            type="datetime-local"
            required
            value={end}
            onChange={(e) => setEnd(e.target.value)}
          />
        </Field>
      </div>
      <Field label="Затронутые компоненты">
        <ComponentChecklist
          components={components}
          value={componentIds}
          onChange={setComponentIds}
        />
      </Field>
      {error && <div className="hp-error">{error}</div>}
      <Button type="submit" disabled={busy || !title}>
        {busy ? "…" : "Сохранить"}
      </Button>
    </form>
  );
}
