import { useState } from "react";

import { addMaintenanceUpdate } from "@/entities/maintenance";
import { HttpError } from "@/shared/api";
import { Button, Field } from "@/shared/ui";

// Заметка к работам (без статуса — смена статуса идёт через редактирование).
export function AddMaintenanceUpdate({
  maintenanceId,
  onAdded,
}: {
  maintenanceId: string;
  onAdded: () => void;
}) {
  const [body, setBody] = useState("");
  const [notify, setNotify] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await addMaintenanceUpdate(maintenanceId, { body, notify });
      setBody("");
      onAdded();
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось добавить заметку");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit}>
      <Field label="Текст заметки">
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
        {busy ? "…" : "Добавить заметку"}
      </Button>
    </form>
  );
}
