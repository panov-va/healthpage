import type { Component } from "@/entities/component";
import type { IncidentComponent } from "@/shared/api";
import { COMPONENT_STATUSES } from "@/shared/api";
import { statusColor, statusLabel } from "@/shared/lib/status";
import { Select } from "@/shared/ui";

// Выбор затронутых компонентов и их статуса «в инциденте» (IncidentComponent[]).
// Переиспользуется формами инцидента и шаблона. Статус по умолчанию для нового —
// major_outage (инцидент обычно означает сбой; оператор может изменить).
const DEFAULT_STATUS = "major_outage";

export function AffectedComponents({
  components,
  value,
  onChange,
}: {
  components: Component[];
  value: IncidentComponent[];
  onChange: (next: IncidentComponent[]) => void;
}) {
  const byId = new Map(components.map((c) => [c.id, c]));
  const selectedIds = new Set(value.map((v) => v.component_id));
  const available = components.filter((c) => !selectedIds.has(c.id));

  function add(componentId: string) {
    if (!componentId) return;
    onChange([
      ...value,
      { component_id: componentId, component_status_in_incident: DEFAULT_STATUS },
    ]);
  }

  function setStatus(componentId: string, status: string) {
    onChange(
      value.map((v) =>
        v.component_id === componentId
          ? { ...v, component_status_in_incident: status as IncidentComponent["component_status_in_incident"] }
          : v,
      ),
    );
  }

  function remove(componentId: string) {
    onChange(value.filter((v) => v.component_id !== componentId));
  }

  return (
    <div>
      {value.length === 0 && (
        <div className="hp-muted" style={{ fontSize: 13, marginBottom: 8 }}>
          Компоненты не выбраны
        </div>
      )}
      {value.map((v) => {
        const comp = byId.get(v.component_id);
        return (
          <div key={v.component_id} className="hp-list-item">
            <span className="hp-row" style={{ gap: 6 }}>
              <span
                className="hp-status-dot"
                style={{ background: statusColor(v.component_status_in_incident) }}
              />
              {comp ? comp.name : v.component_id}
            </span>
            <span className="hp-row" style={{ gap: 6 }}>
              <select
                className="hp-status-select"
                value={v.component_status_in_incident}
                onChange={(e) => setStatus(v.component_id, e.target.value)}
                aria-label="Статус компонента в инциденте"
              >
                {COMPONENT_STATUSES.map((s) => (
                  <option key={s} value={s}>
                    {statusLabel(s)}
                  </option>
                ))}
              </select>
              <button
                type="button"
                className="hp-btn hp-btn--danger hp-btn--sm"
                onClick={() => remove(v.component_id)}
              >
                ✕
              </button>
            </span>
          </div>
        );
      })}
      {available.length > 0 && (
        <Select
          value=""
          onChange={(e) => add(e.target.value)}
          aria-label="Добавить компонент"
          style={{ marginTop: 8 }}
        >
          <option value="">+ добавить компонент…</option>
          {available.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </Select>
      )}
    </div>
  );
}
