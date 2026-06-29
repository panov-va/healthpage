import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { listComponents } from "@/entities/component";
import type { Component } from "@/entities/component";
import { deleteTemplate, listTemplates } from "@/entities/incidentTemplate";
import type { IncidentTemplate } from "@/entities/incidentTemplate";
import { getPage } from "@/entities/page";
import type { StatusPage } from "@/entities/page";
import { TemplateForm } from "@/features/template-form";
import { HttpError } from "@/shared/api";
import { incidentImpactLabel } from "@/shared/lib/incident";
import { Button, Card } from "@/shared/ui";
import { PageNav } from "@/widgets/page-nav";

export function TemplatesPage() {
  const { id = "" } = useParams();
  const [page, setPage] = useState<StatusPage | null>(null);
  const [components, setComponents] = useState<Component[]>([]);
  const [templates, setTemplates] = useState<IncidentTemplate[]>([]);
  const [editing, setEditing] = useState<IncidentTemplate | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

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

  function upsert(tpl: IncidentTemplate) {
    setTemplates((prev) => {
      const exists = prev.some((t) => t.id === tpl.id);
      return exists ? prev.map((t) => (t.id === tpl.id ? tpl : t)) : [...prev, tpl];
    });
    setEditing(null);
  }

  async function handleDelete(tpl: IncidentTemplate) {
    if (!window.confirm(`Удалить шаблон «${tpl.name}»?`)) return;
    try {
      await deleteTemplate(tpl.id);
      setTemplates((prev) => prev.filter((t) => t.id !== tpl.id));
      if (editing?.id === tpl.id) setEditing(null);
    } catch {
      window.alert("Не удалось удалить шаблон");
    }
  }

  if (loading) return <div className="hp-container hp-muted">Загрузка…</div>;
  if (error && !page) return <div className="hp-container hp-error">{error}</div>;
  if (!page) return null;

  return (
    <div className="hp-container">
      <Link to="/" className="hp-muted">
        ← Все страницы
      </Link>
      <h1 style={{ marginTop: 8 }}>{page.name}</h1>
      <PageNav pageId={id} />

      <Card>
        <h2>Шаблоны инцидентов</h2>
        {templates.length === 0 ? (
          <div className="hp-empty">Шаблонов нет</div>
        ) : (
          templates.map((t) => (
            <div key={t.id} className="hp-list-item">
              <span>
                {t.name}
                <div className="hp-muted" style={{ fontSize: 13 }}>
                  влияние: {incidentImpactLabel(t.default_impact)}
                  {t.default_components?.length
                    ? ` · компонентов: ${t.default_components.length}`
                    : ""}
                </div>
              </span>
              <span className="hp-row" style={{ gap: 6 }}>
                <Button size="sm" variant="secondary" onClick={() => setEditing(t)}>
                  Изменить
                </Button>
                <Button size="sm" variant="danger" onClick={() => handleDelete(t)}>
                  Удалить
                </Button>
              </span>
            </div>
          ))
        )}
      </Card>

      <Card>
        {editing ? (
          <TemplateForm
            key={editing.id}
            pageId={id}
            template={editing}
            components={components}
            onSaved={upsert}
            onCancel={() => setEditing(null)}
          />
        ) : (
          <TemplateForm pageId={id} components={components} onSaved={upsert} />
        )}
      </Card>
    </div>
  );
}
