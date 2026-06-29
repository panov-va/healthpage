import { useCallback, useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { listComponents } from "@/entities/component";
import type { Component } from "@/entities/component";
import { listIncidents } from "@/entities/incident";
import type { Incident } from "@/entities/incident";
import { listTemplates } from "@/entities/incidentTemplate";
import type { IncidentTemplate } from "@/entities/incidentTemplate";
import { getPage } from "@/entities/page";
import type { StatusPage } from "@/entities/page";
import { CreateIncidentForm } from "@/features/incident-create";
import {
  HttpError,
  INCIDENT_IMPACTS,
  INCIDENT_STATUSES,
} from "@/shared/api";
import {
  formatDateTime,
  impactColor,
  incidentImpactLabel,
  incidentStatusLabel,
} from "@/shared/lib/incident";
import { Button, Card, Field, Select } from "@/shared/ui";
import { PageNav } from "@/widgets/page-nav";

const PER_PAGE = 20;

export function IncidentsListPage() {
  const { id = "" } = useParams();
  const [page, setPage] = useState<StatusPage | null>(null);
  const [components, setComponents] = useState<Component[]>([]);
  const [templates, setTemplates] = useState<IncidentTemplate[]>([]);
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [total, setTotal] = useState(0);
  const [pageNum, setPageNum] = useState(1);
  const [status, setStatus] = useState("");
  const [impact, setImpact] = useState("");
  const [componentId, setComponentId] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const reloadIncidents = useCallback(() => {
    return listIncidents(id, {
      status: status || undefined,
      impact: impact || undefined,
      componentId: componentId || undefined,
      page: pageNum,
      perPage: PER_PAGE,
    }).then((res) => {
      setIncidents(res.items ?? []);
      setTotal(res.pagination?.total ?? 0);
    });
  }, [id, status, impact, componentId, pageNum]);

  useEffect(() => {
    setLoading(true);
    Promise.all([getPage(id), listComponents(id), listTemplates(id)])
      .then(([p, c, t]) => {
        setPage(p);
        setComponents(c);
        setTemplates(t);
      })
      .catch((err) =>
        setError(err instanceof HttpError ? err.message : "Не удалось загрузить страницу"),
      )
      .finally(() => setLoading(false));
  }, [id]);

  useEffect(() => {
    reloadIncidents().catch((err) =>
      setError(err instanceof HttpError ? err.message : "Не удалось загрузить инциденты"),
    );
  }, [reloadIncidents]);

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
        <h2>Инциденты</h2>
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
              {INCIDENT_STATUSES.map((s) => (
                <option key={s} value={s}>
                  {incidentStatusLabel(s)}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Влияние">
            <Select
              value={impact}
              onChange={(e) => {
                setImpact(e.target.value);
                setPageNum(1);
              }}
            >
              <option value="">— любое —</option>
              {INCIDENT_IMPACTS.map((i) => (
                <option key={i} value={i}>
                  {incidentImpactLabel(i)}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Компонент">
            <Select
              value={componentId}
              onChange={(e) => {
                setComponentId(e.target.value);
                setPageNum(1);
              }}
            >
              <option value="">— любой —</option>
              {components.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </Select>
          </Field>
        </div>

        {incidents.length === 0 ? (
          <div className="hp-empty">Инцидентов нет</div>
        ) : (
          incidents.map((inc) => (
            <div key={inc.id} className="hp-list-item">
              <span>
                <Link to={`/pages/${id}/incidents/${inc.id}`}>{inc.title}</Link>
                <div className="hp-muted" style={{ fontSize: 13 }}>
                  {formatDateTime(inc.started_at)}
                  {inc.is_visible === false ? " · скрыт" : ""}
                </div>
              </span>
              <span className="hp-row" style={{ gap: 6 }}>
                <span className="hp-badge" style={{ borderColor: impactColor(inc.impact) }}>
                  {incidentImpactLabel(inc.impact)}
                </span>
                <span className="hp-badge">{incidentStatusLabel(inc.current_status)}</span>
              </span>
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
        <CreateIncidentForm
          pageId={id}
          components={components}
          templates={templates}
          onCreated={() => {
            setPageNum(1);
            reloadIncidents();
          }}
        />
      </Card>
    </div>
  );
}
