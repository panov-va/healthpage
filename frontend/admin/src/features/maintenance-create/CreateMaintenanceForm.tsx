import { useState } from "react";

import type { Component } from "@/entities/component";
import { createMaintenance } from "@/entities/maintenance";
import type { Maintenance } from "@/entities/maintenance";
import { HttpError } from "@/shared/api";
import { inputToIso } from "@/shared/lib/incident";
import { Button, Field, Input } from "@/shared/ui";
import { ComponentChecklist } from "@/widgets/component-checklist";

// Создание плановых работ. Окно (начало/конец) обязательно; конец должен быть строго
// позже начала (иначе бэкенд вернёт 422).
export function CreateMaintenanceForm({
  pageId,
  components,
  onCreated,
}: {
  pageId: string;
  components: Component[];
  onCreated: (maintenance: Maintenance) => void;
}) {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [start, setStart] = useState("");
  const [end, setEnd] = useState("");
  const [componentIds, setComponentIds] = useState<string[]>([]);
  const [notify, setNotify] = useState(true);
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
      const maintenance = await createMaintenance({
        status_page_id: pageId,
        title,
        description: description || undefined,
        scheduled_start: startIso,
        scheduled_end: endIso,
        component_ids: componentIds,
        notify,
      });
      setTitle("");
      setDescription("");
      setStart("");
      setEnd("");
      setComponentIds([]);
      onCreated(maintenance);
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось создать работы");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit}>
      <h3 style={{ marginBottom: 12 }}>Новые плановые работы</h3>
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
      <label className="hp-row" style={{ gap: 8, marginBottom: 12 }}>
        <input type="checkbox" checked={notify} onChange={(e) => setNotify(e.target.checked)} />
        <span>Уведомить подписчиков</span>
      </label>
      {error && <div className="hp-error">{error}</div>}
      <Button type="submit" disabled={busy || !title}>
        {busy ? "…" : "Создать работы"}
      </Button>
    </form>
  );
}
