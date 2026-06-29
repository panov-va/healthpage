// Публичная страница статуса (SSR) — этап 1.8.
// Тянет GET /pages/{slug}/summary, рендерит общий статус + дерево компонентов и групп.
// Работает независимо от админки. Инциденты/работы — этап 2 (в сводке пока пусты).

import Link from "next/link";
import { notFound } from "next/navigation";

import { Badge } from "./Badge";
import { PageShell } from "./PageShell";
import { StatusTabs } from "./StatusTabs";
import { fetchPageSummary, PageNotFoundError } from "../../../lib/api";
import type { ApiComponent, ComponentStatus } from "../../../lib/api";
import { impactColor, maintenanceStatusColor } from "../../../lib/badge";
import { buildStatusMetadata } from "../../../lib/meta";
import { is12h, parseTheme } from "../../../lib/theme";
import { buildTree } from "../../../lib/tree";
import type { ComponentNode } from "../../../lib/tree";
import {
  dict,
  formatInZone,
  resolveLocale,
  withLang,
} from "../../../lib/i18n";
import type { Locale } from "../../../lib/i18n";

// SSR на каждый запрос: статус должен быть свежим (кэш сводки — отдельная задача, Redis на backend).
export const dynamic = "force-dynamic";

interface PageProps {
  params: { slug: string };
  searchParams: { lang?: string | string[] };
}

export async function generateMetadata({ params }: PageProps) {
  return buildStatusMetadata(params.slug);
}

function StatusBadge({ status, label }: { status: ComponentStatus; label: string }) {
  return (
    <span className={`status st-${status}`}>
      <span className={`dot st-${status}`} aria-hidden="true" />
      {label}
    </span>
  );
}

function ComponentRow({
  node,
  locale,
}: {
  node: ComponentNode;
  locale: Locale;
}) {
  const t = dict(locale);
  const c = node.component;
  return (
    <li>
      <div className="component">
        <span className="component-name">
          <span>{c.name}</span>
          {c.description ? <span className="desc">{c.description}</span> : null}
        </span>
        <StatusBadge status={c.current_status} label={t.status[c.current_status]} />
      </div>
      {node.children.length > 0 ? (
        <ul className="subtree">
          {node.children.map((child) => (
            <ComponentRow key={child.component.id} node={child} locale={locale} />
          ))}
        </ul>
      ) : null}
    </li>
  );
}

function ComponentList({
  components,
  locale,
}: {
  components: ApiComponent[];
  locale: Locale;
}) {
  const roots = buildTree(components);
  return (
    <ul className="components">
      {roots.map((node) => (
        <ComponentRow key={node.component.id} node={node} locale={locale} />
      ))}
    </ul>
  );
}

export default async function StatusPage({ params, searchParams }: PageProps) {
  const locale = resolveLocale(searchParams.lang);
  const t = dict(locale);

  let summary;
  try {
    summary = await fetchPageSummary(params.slug);
  } catch (err) {
    if (err instanceof PageNotFoundError) {
      notFound();
    }
    throw err;
  }

  const hasComponents =
    summary.groups.length > 0 || summary.ungrouped_components.length > 0;
  const slug = params.slug;
  const tz = summary.page.timezone;
  const hour12 = is12h(parseTheme(summary.page.theme));

  return (
    <PageShell page={summary.page} locale={locale}>
    <main className="page">
      <section className={`overall bg-${summary.overall_status}`}>
        <h1>{t.overall[summary.overall_status]}</h1>
        <div className="updated">
          {t.updatedAt}: {formatInZone(summary.updated_at, locale, tz, hour12)}
        </div>
      </section>

      <StatusTabs slug={slug} locale={locale} active="overview" />

      {summary.active_incidents.length > 0 ? (
        <section className="group">
          <div className="group-header">
            <span>{t.incidentsTitle}</span>
          </div>
          <ul className="components">
            {summary.active_incidents.map((inc) => (
              <li key={inc.id}>
                <div className="component">
                  <Link
                    className="component-name"
                    href={withLang(`/status/${encodeURIComponent(slug)}/incidents/${inc.id}`, locale)}
                  >
                    <span>{inc.title}</span>
                    <span className="desc">
                      {t.started}: {formatInZone(inc.started_at, locale, tz, hour12)}
                    </span>
                  </Link>
                  <span className="badges">
                    <Badge label={t.impact[inc.impact]} color={impactColor(inc.impact)} />
                    <span className={`status st-${"degraded_performance"}`}>
                      {t.incidentStatus[inc.current_status]}
                    </span>
                  </span>
                </div>
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      {summary.active_maintenances.length > 0 ? (
        <section className="group">
          <div className="group-header">
            <span>{t.maintenancesTitle}</span>
          </div>
          <ul className="components">
            {summary.active_maintenances.map((m) => (
              <li key={m.id}>
                <div className="component">
                  <span className="component-name">
                    <span>{m.title}</span>
                    <span className="desc">
                      {t.scheduledWindow}: {formatInZone(m.scheduled_start, locale, tz, hour12)} —{" "}
                      {formatInZone(m.scheduled_end, locale, tz, hour12)}
                    </span>
                  </span>
                  <Badge
                    label={t.maintenanceStatus[m.status]}
                    color={maintenanceStatusColor(m.status)}
                  />
                </div>
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      {hasComponents ? (
        <>
          {summary.groups.map((g) => (
            <section className="group" key={g.id}>
              <div className="group-header">
                <span>{g.name}</span>
                <StatusBadge
                  status={g.aggregated_status}
                  label={t.status[g.aggregated_status]}
                />
              </div>
              <ComponentList components={g.components} locale={locale} />
            </section>
          ))}

          {summary.ungrouped_components.length > 0 ? (
            <section className="group">
              <ComponentList
                components={summary.ungrouped_components}
                locale={locale}
              />
            </section>
          ) : null}
        </>
      ) : (
        <p className="empty">{t.noComponents}</p>
      )}
    </main>
    </PageShell>
  );
}
