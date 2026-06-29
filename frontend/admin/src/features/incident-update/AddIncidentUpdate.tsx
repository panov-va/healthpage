import { useState } from "react";

import { addIncidentUpdate } from "@/entities/incident";
import type { IncidentStatus } from "@/entities/incident";
import { HttpError, INCIDENT_STATUSES } from "@/shared/api";
import { incidentStatusLabel } from "@/shared/lib/incident";
import { Button, Field, Select } from "@/shared/ui";

// Добавить обновление в ленту инцидента. Смена статуса инцидента идёт ИМЕННО через
// обновление (POST /incidents/{id}/updates со status), не через PATCH.
export function AddIncidentUpdate({
  incidentId,
  currentStatus,
  onAdded,
}: {
  incidentId: string;
  currentStatus: IncidentStatus;
  onAdded: () => void;
}) {
  const [status, setStatus] = useState<IncidentStatus>(currentStatus);
  const [body, setBody] = useState("");
  const [notify, setNotify] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await addIncidentUpdate(incidentId, { status, body, notify });
      setBody("");
      onAdded();
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось добавить обновление");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit}>
      <Field label="Новый статус">
        <Select value={status} onChange={(e) => setStatus(e.target.value as IncidentStatus)}>
          {INCIDENT_STATUSES.map((s) => (
            <option key={s} value={s}>
              {incidentStatusLabel(s)}
            </option>
          ))}
        </Select>
      </Field>
      <Field label="Текст обновления">
        <textarea
          className="hp-input"
          required
          rows={3}
          value={body}
          onChange={(e) => setBody(e.target.value)}
        />
      </Field>
      <label className="hp-row" style={{ gap: 8, marginBottom: 12 }}>
        <input type="checkbox" checked={notify} onChange={(e) => setNotify(e.target.checked)} />
        <span>Уведомить подписчиков</span>
      </label>
      {error && <div className="hp-error">{error}</div>}
      <Button type="submit" disabled={busy || !body}>
        {busy ? "…" : "Добавить обновление"}
      </Button>
    </form>
  );
}
