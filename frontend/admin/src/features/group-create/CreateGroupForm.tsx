import { useState } from "react";

import { createGroup } from "@/entities/componentGroup";
import type { ComponentGroup } from "@/entities/componentGroup";
import { HttpError } from "@/shared/api";
import { Button, Input } from "@/shared/ui";

export function CreateGroupForm({
  pageId,
  onCreated,
}: {
  pageId: string;
  onCreated: (group: ComponentGroup) => void;
}) {
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const group = await createGroup(pageId, { name });
      setName("");
      onCreated(group);
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось создать группу");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="hp-inline-form" onSubmit={submit}>
      <Input
        placeholder="Название группы"
        value={name}
        onChange={(e) => setName(e.target.value)}
        style={{ maxWidth: 280 }}
      />
      <Button type="submit" size="sm" disabled={busy || !name}>
        Добавить группу
      </Button>
      {error && <span className="hp-error">{error}</span>}
    </form>
  );
}
