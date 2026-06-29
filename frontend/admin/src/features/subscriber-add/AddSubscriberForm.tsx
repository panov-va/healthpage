import { useState } from "react";

import type { Component } from "@/entities/component";
import {
  createSubscriber,
  MANUAL_SUBSCRIBER_CHANNELS,
  type Subscriber,
  type SubscriberChannel,
  type SubscriberScope,
} from "@/entities/subscriber";
import { HttpError } from "@/shared/api";
import { channelLabel } from "@/shared/lib/subscriber";
import { Button, Field, Input, Select } from "@/shared/ui";
import { ComponentChecklist } from "@/widgets/component-checklist";

// Ручное добавление подписчика оператором (этап 3.10). Подписчик создаётся сразу подтверждённым;
// оператор отвечает за наличие согласия на обработку ПД (152-ФЗ).
export function AddSubscriberForm({
  pageId,
  components,
  onCreated,
}: {
  pageId: string;
  components: Component[];
  onCreated: (sub: Subscriber) => void;
}) {
  const [channel, setChannel] = useState<SubscriberChannel>("email");
  const [address, setAddress] = useState("");
  const [scope, setScope] = useState<SubscriberScope>("page");
  const [componentIds, setComponentIds] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const sub = await createSubscriber({
        status_page_id: pageId,
        channel,
        address,
        scope,
        component_ids: scope === "components" ? componentIds : undefined,
      });
      setAddress("");
      setScope("page");
      setComponentIds([]);
      onCreated(sub);
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось добавить подписчика");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit}>
      <h3 style={{ marginBottom: 12 }}>Добавить подписчика</h3>
      <Field label="Канал">
        <Select
          value={channel}
          onChange={(e) => setChannel(e.target.value as SubscriberChannel)}
        >
          {MANUAL_SUBSCRIBER_CHANNELS.map((c) => (
            <option key={c} value={c}>
              {channelLabel(c)}
            </option>
          ))}
        </Select>
      </Field>
      <Field label="Адрес (email / chat_id / webhook URL)">
        <Input required value={address} onChange={(e) => setAddress(e.target.value)} />
      </Field>
      <Field label="Область">
        <Select value={scope} onChange={(e) => setScope(e.target.value as SubscriberScope)}>
          <option value="page">Вся страница</option>
          <option value="components">Выбранные компоненты</option>
        </Select>
      </Field>
      {scope === "components" && (
        <Field label="Компоненты">
          <ComponentChecklist
            components={components}
            value={componentIds}
            onChange={setComponentIds}
          />
        </Field>
      )}
      {error && <div className="hp-error">{error}</div>}
      <Button type="submit" disabled={busy}>
        {busy ? "Добавляем…" : "Добавить"}
      </Button>
    </form>
  );
}
