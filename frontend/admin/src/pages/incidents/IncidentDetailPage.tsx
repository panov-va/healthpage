import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";

import { listComponents } from "@/entities/component";
import type { Component } from "@/entities/component";
import { deleteIncident, getIncident } from "@/entities/incident";
import type { Incident } from "@/entities/incident";
import { getPage } from "@/entities/page";
import type { StatusPage } from "@/entities/page";
import { EditIncidentForm } from "@/features/incident-edit";
import { AddIncidentUpdate } from "@/features/incident-update";
import { HttpError } from "@/shared/api";
import {
  formatDateTime,
  impactColor,
  incidentImpactLabel,
  incidentStatusLabel,
} from "@/shared/lib/incident";
import { Button, Card } from "@/shared/ui";
import { PageNav } from "@/widgets/page-nav";

export function IncidentDetailPage() {
  const { id = "", incidentId = "" } = useParams();
  const navigate = useNavigate();
  const [page, setPage] = useState<StatusPage | null>(null);
  const [components, setComponents] = useState<Component[]>([]);
  const [incident, setIncident] = useState<Incident | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const reloadIncident = useCallback(() => {
    return getIncident(incidentId).then(setIncident);
  }, [incidentId]);

  useEffect(() => {
    setLoading(true);
    Promise.all([getPage(id), listComponents(id), getIncident(incidentId)])
      .then(([p, c, inc]) => {
        setPage(p);
        setComponents(c);
        setIncident(inc);
      })
      .catch((err) =>
        setError(err instanceof HttpError ? err.message : "Не удалось загрузить инцидент"),
      )
      .finally(() => setLoading(false));
  }, [id, incidentId]);

  async function handleDelete() {
    if (!window.confirm("Удалить инцидент? Он исчезнет из публичной истории.")) return;
    try {
      await deleteIncident(incidentId);
      navigate(`/pages/${id}/incidents`);
    } catch {
      window.alert("Не удалось удалить инцидент");
    }
  }

  if (loading) return <div className="hp-container hp-muted">Загрузка…</div>;
  if (error) return <div className="hp-container hp-error">{error}</div>;
  if (!page || !incident) return null;

  // Лента: новые сверху.
  const updates = [...(incident.updates ?? [])].sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
  );

  return (
    <div className="hp-container">
      <Link to={`/pages/${id}/incidents`} className="hp-muted">
        ← К инцидентам
      </Link>
      <h1 style={{ marginTop: 8 }}>{page.name}</h1>
      <PageNav pageId={id} />

      <Card>
        <div className="hp-card__header" style={{ marginBottom: 8 }}>
          <h2>{incident.title}</h2>
          <Button variant="danger" size="sm" onClick={handleDelete}>
            Удалить
          </Button>
        </div>
        <div className="hp-row" style={{ gap: 6 }}>
          <span className="hp-badge" style={{ borderColor: impactColor(incident.impact) }}>
            {incidentImpactLabel(incident.impact)}
          </span>
          <span className="hp-badge">{incidentStatusLabel(incident.current_status)}</span>
          <span className="hp-muted" style={{ fontSize: 13 }}>
            начат {formatDateTime(incident.started_at)}
            {incident.resolved_at ? ` · решён ${formatDateTime(incident.resolved_at)}` : ""}
          </span>
        </div>
      </Card>

      <Card>
        <h3 style={{ marginBottom: 12 }}>Добавить обновление</h3>
        <AddIncidentUpdate
          incidentId={incident.id}
          currentStatus={incident.current_status}
          onAdded={reloadIncident}
        />
      </Card>

      <Card>
        <h3 style={{ marginBottom: 12 }}>Лента обновлений</h3>
        {updates.length === 0 ? (
          <div className="hp-empty">Обновлений нет</div>
        ) : (
          <div className="hp-timeline">
            {updates.map((u) => (
              <div key={u.id} className="hp-timeline-item">
                <div className="hp-row" style={{ gap: 6 }}>
                  <span className="hp-badge">{incidentStatusLabel(u.status)}</span>
                  <span className="hp-muted" style={{ fontSize: 13 }}>
                    {formatDateTime(u.created_at)}
                  </span>
                </div>
                <div style={{ whiteSpace: "pre-wrap", marginTop: 4 }}>{u.body}</div>
              </div>
            ))}
          </div>
        )}
      </Card>

      <Card>
        <h3 style={{ marginBottom: 12 }}>Редактировать инцидент</h3>
        <EditIncidentForm
          incident={incident}
          components={components}
          onSaved={setIncident}
        />
      </Card>
    </div>
  );
}
