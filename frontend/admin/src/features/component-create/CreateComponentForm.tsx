import { useState } from "react";

import { createComponent } from "@/entities/component";
import type { Component } from "@/entities/component";
import type { ComponentGroup } from "@/entities/componentGroup";
import { COMPONENT_STATUSES, HttpError } from "@/shared/api";
import { statusLabel } from "@/shared/lib/status";
import { Button, Field, Input, Select } from "@/shared/ui";

export function CreateComponentForm({
  pageId,
  groups,
  components,
  onCreated,
}: {
  pageId: string;
  groups: ComponentGroup[];
  components: Component[];
  onCreated: (component: Component) => void;
}) {
  const [name, setName] = useState("");
  const [groupId, setGroupId] = useState("");
  const [parentId, setParentId] = useState("");
  const [status, setStatus] = useState<(typeof COMPONENT_STATUSES)[number]>("operational");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const component = await createComponent({
        name,
        status_page_id: pageId,
        group_id: groupId || undefined,
        parent_id: parentId || undefined,
        current_status: status,
        is_private: false,
        show_uptime: true,
        display_state: true,
      });
      setName("");
      setGroupId("");
      setParentId("");
      setStatus("operational");
      onCreated(component);
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось создать компонент");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit}>
      <h3 style={{ marginBottom: 12 }}>Новый компонент</h3>
      <Field label="Название">
        <Input required value={name} onChange={(e) => setName(e.target.value)} />
      </Field>
      <Field label="Группа">
        <Select value={groupId} onChange={(e) => setGroupId(e.target.value)}>
          <option value="">— без группы —</option>
          {groups.map((g) => (
            <option key={g.id} value={g.id}>
              {g.name}
            </option>
          ))}
        </Select>
      </Field>
      <Field label="Родительский компонент (подкомпонент)">
        <Select value={parentId} onChange={(e) => setParentId(e.target.value)}>
          <option value="">— верхний уровень —</option>
          {components.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </Select>
      </Field>
      <Field label="Начальный статус">
        <Select
          value={status}
          onChange={(e) => setStatus(e.target.value as (typeof COMPONENT_STATUSES)[number])}
        >
          {COMPONENT_STATUSES.map((s) => (
            <option key={s} value={s}>
              {statusLabel(s)}
            </option>
          ))}
        </Select>
      </Field>
      {error && <div className="hp-error">{error}</div>}
      <Button type="submit" disabled={busy || !name}>
        {busy ? "…" : "Создать компонент"}
      </Button>
    </form>
  );
}
