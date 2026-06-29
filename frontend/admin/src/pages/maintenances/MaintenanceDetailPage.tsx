import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";

import { listComponents } from "@/entities/component";
import type { Component } from "@/entities/component";
import { deleteMaintenance, getMaintenance } from "@/entities/maintenance";
import type { Maintenance } from "@/entities/maintenance";
import { getPage } from "@/entities/page";
import type { StatusPage } from "@/entities/page";
import { EditMaintenanceForm } from "@/features/maintenance-edit";
import { AddMaintenanceUpdate } from "@/features/maintenance-update";
import { HttpError } from "@/shared/api";
import { formatDateTime, maintenanceStatusLabel } from "@/shared/lib/incident";
import { Button, Card } from "@/shared/ui";
import { PageNav } from "@/widgets/page-nav";

export function MaintenanceDetailPage() {
  const { id = "", maintenanceId = "" } = useParams();
  const navigate = useNavigate();
  const [page, setPage] = useState<StatusPage | null>(null);
  const [components, setComponents] = useState<Component[]>([]);
  const [maintenance, setMaintenance] = useState<Maintenance | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(() => {
    return getMaintenance(maintenanceId).then(setMaintenance);
  }, [maintenanceId]);

  useEffect(() => {
    setLoading(true);
    Promise.all([getPage(id), listComponents(id), getMaintenance(maintenanceId)])
      .then(([p, c, m]) => {
        setPage(p);
        setComponents(c);
        setMaintenance(m);
      })
      .catch((err) =>
        setError(err instanceof HttpError ? err.message : "Не удалось загрузить работы"),
      )
      .finally(() => setLoading(false));
  }, [id, maintenanceId]);

  async function handleDelete() {
    if (!window.confirm("Удалить плановые работы?")) return;
    try {
      await deleteMaintenance(maintenanceId);
      navigate(`/pages/${id}/maintenances`);
    } catch {
      window.alert("Не удалось удалить работы");
    }
  }

  if (loading) return <div className="hp-container hp-muted">Загрузка…</div>;
  if (error && !maintenance)
    return <div className="hp-container hp-error">{error}</div>;
  if (!page || !maintenance) return null;

  const updates = [...(maintenance.updates ?? [])].sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
  );

  return (
    <div className="hp-container">
      <Link to={`/pages/${id}/maintenances`} className="hp-muted">
        ← К работам
      </Link>
      <h1 style={{ marginTop: 8 }}>{page.name}</h1>
      <PageNav pageId={id} />

      <Card>
        <div className="hp-card__header" style={{ marginBottom: 8 }}>
          <h2>{maintenance.title}</h2>
          <Button variant="danger" size="sm" onClick={handleDelete}>
            Удалить
          </Button>
        </div>
        <div className="hp-row" style={{ gap: 6 }}>
          <span className="hp-badge">{maintenanceStatusLabel(maintenance.status)}</span>
          <span className="hp-muted" style={{ fontSize: 13 }}>
            {formatDateTime(maintenance.scheduled_start)} —{" "}
            {formatDateTime(maintenance.scheduled_end)}
          </span>
        </div>
      </Card>

      <Card>
        <h3 style={{ marginBottom: 12 }}>Добавить заметку</h3>
        <AddMaintenanceUpdate maintenanceId={maintenance.id} onAdded={reload} />
      </Card>

      <Card>
        <h3 style={{ marginBottom: 12 }}>Лента обновлений</h3>
        {updates.length === 0 ? (
          <div className="hp-empty">Обновлений нет</div>
        ) : (
          <div className="hp-timeline">
            {updates.map((u) => (
              <div key={u.id} className="hp-timeline-item">
                <span className="hp-muted" style={{ fontSize: 13 }}>
                  {formatDateTime(u.created_at)}
                </span>
                <div style={{ whiteSpace: "pre-wrap", marginTop: 4 }}>{u.body}</div>
              </div>
            ))}
          </div>
        )}
      </Card>

      <Card>
        <h3 style={{ marginBottom: 12 }}>Редактировать работы</h3>
        <EditMaintenanceForm
          maintenance={maintenance}
          components={components}
          onSaved={setMaintenance}
        />
      </Card>
    </div>
  );
}
