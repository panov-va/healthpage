// Публичная детальная страница инцидента: хроника обновлений + постмортем (этап 2.10).
// Источник: GET /pages/{slug}/incidents/{id} (скрытые/удалённые → 404). Имена компонентов —
// из публичного списка компонентов.

import Link from "next/link";
import { notFound } from "next/navigation";

import { Badge } from "../../Badge";
import { PageShell } from "../../PageShell";
import { StatusTabs } from "../../StatusTabs";
import {
  componentNameMap,
  fetchComponents,
  fetchIncident,
  fetchPageMeta,
  PageNotFoundError,
} from "../../../../../lib/api";
import { impactColor, incidentStatusColor } from "../../../../../lib/badge";
import { buildStatusMetadata } from "../../../../../lib/meta";
import { is12h, parseTheme } from "../../../../../lib/theme";
import { dict, formatInZone, resolveLocale, withLang } from "../../../../../lib/i18n";

export const dynamic = "force-dynamic";

interface PageProps {
  params: { slug: string; id: string };
  searchParams: { lang?: string | string[] };
}

export async function generateMetadata({ params }: PageProps) {
  return buildStatusMetadata(params.slug);
}

export default async function IncidentDetailPage({ params, searchParams }: PageProps) {
  const locale = resolveLocale(searchParams.lang);
  const t = dict(locale);
  const { slug, id } = params;

  let incident;
  let meta;
  let names: Record<string, string> = {};
  try {
    const [inc, comps, m] = await Promise.all([
      fetchIncident(slug, id),
      fetchComponents(slug),
      fetchPageMeta(slug),
    ]);
    incident = inc;
    names = componentNameMap(comps);
    meta = m;
  } catch (err) {
    if (err instanceof PageNotFoundError) {
      notFound();
    }
    throw err;
  }
  const tz = meta.timezone;
  const hour12 = is12h(parseTheme(meta.theme));

  // Хроника: новые сверху.
  const updates = [...incident.updates].sort(
    (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
  );

  return (
    <PageShell page={meta} locale={locale}>
    <main className="page">
      <StatusTabs slug={slug} locale={locale} active="incidents" />
      <Link
        href={withLang(`/status/${encodeURIComponent(slug)}/incidents`, locale)}
        className="back-link"
      >
        ← {t.incidentsTitle}
      </Link>

      <h1 className="section-title">{incident.title}</h1>
      <div className="badges">
        <Badge label={t.impact[incident.impact]} color={impactColor(incident.impact)} />
        <Badge
          label={t.incidentStatus[incident.current_status]}
          color={incidentStatusColor(incident.current_status)}
        />
      </div>
      <div className="history-meta" style={{ marginTop: 8 }}>
        {t.started}: {formatInZone(incident.started_at, locale, tz, hour12)}
        {incident.resolved_at
          ? ` · ${t.resolved}: ${formatInZone(incident.resolved_at, locale, tz, hour12)}`
          : ""}
      </div>

      {incident.components.length > 0 ? (
        <section className="group" style={{ marginTop: 16 }}>
          <div className="group-header">
            <span>{t.affectedComponents}</span>
          </div>
          <ul className="components">
            {incident.components.map((c) => (
              <li key={c.component_id}>
                <div className="component">
                  <span className="component-name">
                    <span>{names[c.component_id] ?? c.component_id}</span>
                  </span>
                  <span className={`status st-${c.component_status_in_incident}`}>
                    <span
                      className={`dot st-${c.component_status_in_incident}`}
                      aria-hidden="true"
                    />
                    {t.status[c.component_status_in_incident]}
                  </span>
                </div>
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      <section className="group" style={{ marginTop: 16 }}>
        <div className="group-header">
          <span>{t.updatesTitle}</span>
        </div>
        <ol className="timeline">
          {updates.map((u) => (
            <li key={u.id} className="timeline-item">
              <div className="timeline-head">
                <Badge
                  label={t.incidentStatus[u.status]}
                  color={incidentStatusColor(u.status)}
                />
                <span className="timeline-time">
                  {formatInZone(u.created_at, locale, tz, hour12)}
                </span>
              </div>
              <p className="timeline-body">{u.body}</p>
            </li>
          ))}
        </ol>
      </section>

      {incident.postmortem ? (
        <section className="group" style={{ marginTop: 16 }}>
          <div className="group-header">
            <span>{t.postmortem}</span>
          </div>
          <p className="postmortem">{incident.postmortem}</p>
        </section>
      ) : null}
    </main>
    </PageShell>
  );
}
