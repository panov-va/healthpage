import type { Component } from "@/entities/component";

// Простой множественный выбор компонентов (массив id). Для плановых работ, где
// статус навязывается единый (under_maintenance), без выбора статуса на компонент.
export function ComponentChecklist({
  components,
  value,
  onChange,
}: {
  components: Component[];
  value: string[];
  onChange: (next: string[]) => void;
}) {
  function toggle(id: string) {
    if (value.includes(id)) onChange(value.filter((v) => v !== id));
    else onChange([...value, id]);
  }

  if (components.length === 0) {
    return <div className="hp-muted" style={{ fontSize: 13 }}>Нет компонентов</div>;
  }

  return (
    <div>
      {components.map((c) => (
        <label
          key={c.id}
          className="hp-row"
          style={{ gap: 8, padding: "4px 0", cursor: "pointer" }}
        >
          <input
            type="checkbox"
            checked={value.includes(c.id)}
            onChange={() => toggle(c.id)}
          />
          <span>{c.name}</span>
        </label>
      ))}
    </div>
  );
}
