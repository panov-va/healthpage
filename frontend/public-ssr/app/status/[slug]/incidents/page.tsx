// Публичная вкладка «Инциденты» — история инцидентов с пагинацией (этап 2.10).
// Источник: GET /pages/{slug}/incidents (видимые, не удалённые; фильтры/пагинация — 2.8).

import Link from "next/link";
import { notFound } from "next/navigation";

import { Badge } from "../Badge";
import { PageShell } from "../PageShell";
import { Pager } from "../Pager";
import { StatusTabs } from "../StatusTabs";
import { fetchIncidents, fetchPageMeta, PageNotFoundError } from "../../../../lib/api";
import { impactColor, incidentStatusColor } from "../../../../lib/badge";
import { buildStatusMetadata } from "../../../../lib/meta";
import { is12h, parseTheme } from "../../../../lib/theme";
import { dict, formatInZone, resolveLocale, withLang } from "../../../../lib/i18n";

export const dynamic = "force-dynamic";

const PER_PAGE = 20;

interface PageProps {
  params: { slug: string };
  searchParams: { lang?: string | string[]; page?: string | string[] };
}

export async function generateMetadata({ params }: PageProps) {
  return buildStatusMetadata(params.slug);
}

function parsePage(value: string | string[] | undefined): number {
  const raw = Array.isArray(value) ? value[0] : value;
  const n = parseInt(raw ?? "1", 10);
  return Number.isFinite(n) && n > 0 ? n : 1;
}

export default async function IncidentsPage({ params, searchParams }: PageProps) {
  const locale = resolveLocale(searchParams.lang);
  const t = dict(locale);
  const slug = params.slug;
  const page = parsePage(searchParams.page);

  let list;
  let meta;
  try {
    [list, meta] = await Promise.all([
      fetchIncidents(slug, page, PER_PAGE),
      fetchPageMeta(slug),
    ]);
  } catch (err) {
    if (err instanceof PageNotFoundError) {
      notFound();
    }
    throw err;
  }
  const tz = meta.timezone;
  const hour12 = is12h(parseTheme(meta.theme));

  return (
    <PageShell page={meta} locale={locale}>
    <main className="page">
      <StatusTabs slug={slug} locale={locale} active="incidents" />
      <h1 className="section-title">{t.incidentsTitle}</h1>

      {list.items.length === 0 ? (
        <p className="empty">{t.noIncidents}</p>
      ) : (
        <ul className="history">
          {list.items.map((inc) => (
            <li key={inc.id} className="history-item">
              <div className="history-row">
                <Link
                  href={withLang(`/status/${encodeURIComponent(slug)}/incidents/${inc.id}`, locale)}
                  className="history-title"
                >
                  {inc.title}
                </Link>
                <span className="badges">
                  <Badge label={t.impact[inc.impact]} color={impactColor(inc.impact)} />
                  <Badge
                    label={t.incidentStatus[inc.current_status]}
                    color={incidentStatusColor(inc.current_status)}
                  />
                </span>
              </div>
              <div className="history-meta">
                {t.started}: {formatInZone(inc.started_at, locale, tz, hour12)}
                {inc.resolved_at
                  ? ` · ${t.resolved}: ${formatInZone(inc.resolved_at, locale, tz, hour12)}`
                  : ""}
              </div>
            </li>
          ))}
        </ul>
      )}

      <Pager
        basePath={`/status/${encodeURIComponent(slug)}/incidents`}
        locale={locale}
        page={page}
        perPage={PER_PAGE}
        total={list.pagination.total}
        prevLabel={t.prevPage}
        nextLabel={t.nextPage}
      />
    </main>
    </PageShell>
  );
}
