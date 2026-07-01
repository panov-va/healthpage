import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useParams } from "react-router-dom";

import {
  createChangelog,
  deleteChangelog,
  listChangelog,
  patchChangelog,
  type ChangelogEntry,
} from "@/entities/changelog";
import { getPage } from "@/entities/page";
import type { StatusPage } from "@/entities/page";
import { HttpError } from "@/shared/api";
import { formatDateTime } from "@/shared/lib/incident";
import { Button, Card } from "@/shared/ui";
import { PageNav } from "@/widgets/page-nav";

export function ChangelogPage() {
  const { id = "" } = useParams();
  const [page, setPage] = useState<StatusPage | null>(null);
  const [items, setItems] = useState<ChangelogEntry[]>([]);
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [publishNow, setPublishNow] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  const reload = useCallback(() => listChangelog(id).then(setItems), [id]);

  useEffect(() => {
    setLoading(true);
    Promise.all([getPage(id), listChangelog(id)])
      .then(([p, list]) => {
        setPage(p);
        setItems(list);
      })
      .catch((err) =>
        setError(err instanceof HttpError ? err.message : "Не удалось загрузить changelog"),
      )
      .finally(() => setLoading(false));
  }, [id]);

  async function create(e: FormEvent) {
    e.preventDefault();
    if (!title.trim()) return;
    setBusy(true);
    setError(null);
    try {
      await createChangelog({ status_page_id: id, title: title.trim(), body, published: publishNow });
      setTitle("");
      setBody("");
      setPublishNow(false);
      await reload();
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось создать запись");
    } finally {
      setBusy(false);
    }
  }

  async function togglePublish(entry: ChangelogEntry) {
    setError(null);
    try {
      await patchChangelog(entry.id, { published: !entry.published });
      await reload();
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось изменить запись");
    }
  }

  async function remove(entry: ChangelogEntry) {
    setError(null);
    try {
      await deleteChangelog(entry.id);
      setItems((prev) => prev.filter((x) => x.id !== entry.id));
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось удалить запись");
    }
  }

  if (loading) return <div className="hp-container hp-muted">Загрузка…</div>;
  if (error && !page) return <div className="hp-container hp-error">{error}</div>;
  if (!page) return null;

  return (
    <div className="hp-container">
      <PageNav pageId={id} />
      <h1>Релизы (changelog) — {page.name}</h1>
      {error && <div className="hp-error">{error}</div>}

      <Card>
        <h2>Новая запись</h2>
        <form onSubmit={create} className="hp-form">
          <label>
            Заголовок
            <input value={title} onChange={(e) => setTitle(e.target.value)} required />
          </label>
          <label>
            Текст
            <textarea value={body} onChange={(e) => setBody(e.target.value)} rows={4} />
          </label>
          <label className="hp-checkbox">
            <input
              type="checkbox"
              checked={publishNow}
              onChange={(e) => setPublishNow(e.target.checked)}
            />
            Опубликовать сразу
          </label>
          <Button type="submit" disabled={busy}>
            Добавить
          </Button>
        </form>
      </Card>

      <Card>
        <h2>Записи</h2>
        {items.length === 0 ? (
          <p className="hp-muted">Записей пока нет.</p>
        ) : (
          <ul className="hp-list">
            {items.map((e) => (
              <li key={e.id} className="hp-list-item">
                <div>
                  <strong>{e.title}</strong>{" "}
                  <span className={e.published ? "hp-badge hp-badge--ok" : "hp-badge"}>
                    {e.published ? "опубликовано" : "черновик"}
                  </span>
                  <div className="hp-muted">
                    {formatDateTime((e.published_at ?? e.created_at) || "")}
                  </div>
                  {e.body ? <div style={{ whiteSpace: "pre-wrap" }}>{e.body}</div> : null}
                </div>
                <div style={{ display: "flex", gap: "0.5rem" }}>
                  <Button variant="secondary" onClick={() => togglePublish(e)}>
                    {e.published ? "Снять" : "Опубликовать"}
                  </Button>
                  <Button variant="danger" onClick={() => remove(e)}>
                    Удалить
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  );
}
