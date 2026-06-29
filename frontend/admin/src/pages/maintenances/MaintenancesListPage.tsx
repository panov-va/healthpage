import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { listComponents } from "@/entities/component";
import type { Component } from "@/entities/component";
import { listMaintenances } from "@/entities/maintenance";
import type { Maintenance } from "@/entities/maintenance";
import { getPage } from "@/entities/page";
import type { StatusPage } from "@/entities/page";
import { CreateMaintenanceForm } from "@/features/maintenance-create";
import { HttpError, MAINTENANCE_STATUSES } from "@/shared/api";
import { formatDateTime, maintenanceStatusLabel } from "@/shared/lib/incident";
import { Button, Card, Field, Select } from "@/shared/ui";
import { PageNav } from "@/widgets/page-nav";

const PER_PAGE = 20;

export function MaintenancesListPage() {
  const { id = "" } = useParams();
  const [page, setPage] = useState<StatusPage | null>(null);
  const [components, setComponents] = useState<Component[]>([]);
  const [items, setItems] = useState<Maintenance[]>([]);
  const [total, setTotal] = useState(0);
  const [pageNum, setPageNum] = useState(1);
  const [status, setStatus] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(() => {
    return listMaintenances(id, {
      status: status || undefined,
      page: pageNum,
      perPage: PER_PAGE,
    }).then((res) => {
      setItems(res.items ?? []);
      setTotal(res.pagination?.total ?? 0);
    });
  }, [id, status, pageNum]);

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
      setError(err instanceof HttpError ? err.message : "Не удалось загрузить работы"),
    );
  }, [reload]);

  if (loading) return <div className="hp-container hp-muted">Загрузка…</div>;
  if (error && !page) return <div className="hp-container hp-error">{error}</div>;
  if (!page) return null;

  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));

  return (
    <div className="hp-container">
      <Link to="/" className="hp-muted">
        ← Все страницы
      </Link>
      <h1 style={{ marginTop: 8 }}>{page.name}</h1>
      <PageNav pageId={id} />

      <Card>
        <h2>Плановые работы</h2>
        <div className="hp-filters">
          <Field label="Статус">
            <Select
              value={status}
              onChange={(e) => {
                setStatus(e.target.value);
                setPageNum(1);
              }}
            >
              <option value="">— любой —</option>
              {MAINTENANCE_STATUSES.map((s) => (
                <option key={s} value={s}>
                  {maintenanceStatusLabel(s)}
                </option>
              ))}
            </Select>
          </Field>
        </div>

        {items.length === 0 ? (
          <div className="hp-empty">Работ нет</div>
        ) : (
          items.map((m) => (
            <div key={m.id} className="hp-list-item">
              <span>
                <Link to={`/pages/${id}/maintenances/${m.id}`}>{m.title}</Link>
                <div className="hp-muted" style={{ fontSize: 13 }}>
                  {formatDateTime(m.scheduled_start)} — {formatDateTime(m.scheduled_end)}
                </div>
              </span>
              <span className="hp-badge">{maintenanceStatusLabel(m.status)}</span>
            </div>
          ))
        )}

        {totalPages > 1 && (
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
              {pageNum} / {totalPages}
            </span>
            <Button
              variant="secondary"
              size="sm"
              disabled={pageNum >= totalPages}
              onClick={() => setPageNum((p) => p + 1)}
            >
              Вперёд →
            </Button>
          </div>
        )}
      </Card>

      <Card>
        <CreateMaintenanceForm
          pageId={id}
          components={components}
          onCreated={() => {
            setPageNum(1);
            reload();
          }}
        />
      </Card>
    </div>
  );
}
