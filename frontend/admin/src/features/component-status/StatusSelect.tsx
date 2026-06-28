import { useState } from "react";

import { changeComponentStatus } from "@/entities/component";
import type { Component, ComponentStatus } from "@/entities/component";
import { COMPONENT_STATUSES, HttpError } from "@/shared/api";
import { statusColor, statusLabel } from "@/shared/lib/status";

// Инлайновая ручная смена статуса компонента. Пишет историю периодов на backend
// (ChangeComponentStatus). Цвет рамки селекта отражает текущий статус.
export function StatusSelect({
  component,
  onChanged,
}: {
  component: Component;
  onChanged: (updated: Component) => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function change(next: ComponentStatus) {
    if (next === component.current_status) return;
    setBusy(true);
    setError(null);
    try {
      const updated = await changeComponentStatus(component.id, next);
      onChanged(updated);
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Ошибка");
    } finally {
      setBusy(false);
    }
  }

  return (
    <span className="hp-row" style={{ gap: 6 }}>
      <span
        className="hp-status-dot"
        style={{ background: statusColor(component.current_status) }}
      />
      <select
        className="hp-status-select"
        value={component.current_status}
        disabled={busy}
        onChange={(e) => change(e.target.value as ComponentStatus)}
        style={{ borderColor: statusColor(component.current_status) }}
        aria-label="Статус компонента"
      >
        {COMPONENT_STATUSES.map((s) => (
          <option key={s} value={s}>
            {statusLabel(s)}
          </option>
        ))}
      </select>
      {error && <span className="hp-error">{error}</span>}
    </span>
  );
}
