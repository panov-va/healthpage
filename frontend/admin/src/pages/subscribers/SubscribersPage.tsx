import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { listComponents } from "@/entities/component";
import type { Component } from "@/entities/component";
import { getPage } from "@/entities/page";
import type { StatusPage } from "@/entities/page";
import {
  deleteSubscriber,
  listSubscribers,
  type Subscriber,
} from "@/entities/subscriber";
import { AddSubscriberForm } from "@/features/subscriber-add";
import { HttpError } from "@/shared/api";
import { formatDateTime } from "@/shared/lib/incident";
import { channelLabel, scopeLabel } from "@/shared/lib/subscriber";
import { Button, Card } from "@/shared/ui";
import { PageNav } from "@/widgets/page-nav";

const PER_PAGE = 50;

export function SubscribersPage() {
  const { id = "" } = useParams();
  const [page, setPage] = useState<StatusPage | null>(null);
  const [components, setComponents] = useState<Component[]>([]);
  const [items, setItems] = useState<Subscriber[]>([]);
  const [pageNum, setPageNum] = useState(1);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(() => {
    return listSubscribers(id, pageNum, PER_PAGE).then(setItems);
  }, [id, pageNum]);

  useEffect(() => {
    setLoading(true);
    Promise.all([getPage(id), listComponents(id)])
      .then(([p, c]) => {
        setPage(p);
        setComponents(c);
      })
      .catch((err) =>
        setError(err instanceof HttpError ? err.message : "Не удалось загрузить страницу"),
      )
      .finally(() => setLoading(false));
  }, [id]);

  useEffect(() => {
    reload().catch((err) =>
      setError(err instanceof HttpError ? err.message : "Не удалось загрузить подписчиков"),
    );
  }, [reload]);

  async function remove(sub: Subscriber) {
    setError(null);
    try {
      await deleteSubscriber(sub.id);
      setItems((prev) => prev.filter((s) => s.id !== sub.id));
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось удалить подписчика");
    }
  }

  if (loading) return <div className="hp-container hp-muted">Загрузка…</div>;
  if (error && !page) return <div className="hp-container hp-error">{error}</div>;
  if (!page) return null;

  const hasNextPage = items.length === PER_PAGE;

  return (
    <div className="hp-container">
      <Link to="/" className="hp-muted">
        ← Все страницы
      </Link>
      <h1 style={{ marginTop: 8 }}>{page.name}</h1>
      <PageNav pageId={id} />

      <Card>
        <h2>Подписчики</h2>
        {error && <div className="hp-error">{error}</div>}
        {items.length === 0 ? (
          <div className="hp-empty">Подписчиков нет</div>
        ) : (
          items.map((s) => (
            <div key={s.id} className="hp-list-item">
              <span>
                <strong>{s.address}</strong>
                <div className="hp-muted" style={{ fontSize: 13 }}>
                  {channelLabel(s.channel)} · {scopeLabel(s.scope)} ·{" "}
                  {formatDateTime(s.created_at ?? "")}
                </div>
              </span>
              <span className="hp-row" style={{ gap: 8 }}>
                <span className="hp-badge">
                  {s.confirmed ? "подтверждён" : "ожидает"}
                </span>
                <Button variant="secondary" size="sm" onClick={() => remove(s)}>
                  Удалить
                </Button>
              </span>
            </div>
          ))
        )}

        {(pageNum > 1 || hasNextPage) && (
          <div className="hp-row" style={{ gap: 8, marginTop: 12 }}>
            <Button
              variant="secondary"
              size="sm"
              disabled={pageNum <= 1}
              onClick={() => setPageNum((p) => p - 1)}
            >
              ← Назад
            </Button>
            <span className="hp-muted" style={{ fontSize: 13 }}>
              Стр. {pageNum}
            </span>
            <Button
              variant="secondary"
              size="sm"
              disabled={!hasNextPage}
              onClick={() => setPageNum((p) => p + 1)}
            >
              Вперёд →
            </Button>
          </div>
        )}
      </Card>

      <Card>
        <AddSubscriberForm
          pageId={id}
          components={components}
          onCreated={(sub) => {
            // Новый подписчик — наверх списка (новые сверху, как на бэке).
            setItems((prev) => [sub, ...prev]);
          }}
        />
      </Card>
    </div>
  );
}
