import { useState } from "react";

import { createPage } from "@/entities/page";
import type { StatusPage } from "@/entities/page";
import { HttpError } from "@/shared/api";
import { Button, Field, Input, Select } from "@/shared/ui";

// Простой slug из произвольной строки (латиница/цифры/дефис).
function slugify(s: string): string {
  return s
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

export function CreatePageForm({ onCreated }: { onCreated: (page: StatusPage) => void }) {
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [slugTouched, setSlugTouched] = useState(false);
  const [visibility, setVisibility] = useState<"public" | "private">("public");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const page = await createPage({ name, slug, visibility });
      setName("");
      setSlug("");
      setSlugTouched(false);
      onCreated(page);
    } catch (err) {
      setError(
        err instanceof HttpError
          ? err.code === "slug_taken"
            ? "Такой slug уже занят"
            : err.message
          : "Не удалось создать страницу",
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit}>
      <h3 style={{ marginBottom: 12 }}>Новая страница статуса</h3>
      <Field label="Название">
        <Input
          required
          value={name}
          onChange={(e) => {
            setName(e.target.value);
            if (!slugTouched) setSlug(slugify(e.target.value));
          }}
        />
      </Field>
      <Field label="Slug (часть публичного адреса)">
        <Input
          required
          value={slug}
          onChange={(e) => {
            setSlugTouched(true);
            setSlug(slugify(e.target.value));
          }}
          placeholder="my-product"
        />
      </Field>
      <Field label="Видимость">
        <Select
          value={visibility}
          onChange={(e) => setVisibility(e.target.value as "public" | "private")}
        >
          <option value="public">Публичная</option>
          <option value="private">Приватная</option>
        </Select>
      </Field>
      {error && <div className="hp-error">{error}</div>}
      <Button type="submit" disabled={busy || !name || !slug}>
        {busy ? "…" : "Создать"}
      </Button>
    </form>
  );
}
