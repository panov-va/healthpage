// Публичная вкладка «Плановые работы» — список работ с деталями и пагинацией (этап 2.10).
// Источник: GET /pages/{slug}/maintenances. Отдельного публичного GET одной работы в контракте
// нет, а список отдаёт работы полными (компоненты + хроника) — поэтому детали показываем прямо
// в карточке. Имена компонентов — из публичного списка компонентов.

import { notFound, redirect } from "next/navigation";

import { Badge } from "../Badge";
import { PageShell } from "../PageShell";
import { Pager } from "../Pager";
import { StatusTabs } from "../StatusTabs";
import {
  componentNameMap,
  fetchComponents,
  fetchMaintenances,
  fetchPageMeta,
  PageAccessRequiredError,
  PageNotFoundError,
} from "../../../../lib/api";
import { maintenanceStatusColor } from "../../../../lib/badge";
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

export default async function MaintenancesPage({ params, searchParams }: PageProps) {
  const locale = resolveLocale(searchParams.lang);
  const t = dict(locale);
  const slug = params.slug;
  const page = parsePage(searchParams.page);

  let list;
  let meta;
  let names: Record<string, string> = {};
  try {
    const [ml, comps, m] = await Promise.all([
      fetchMaintenances(slug, page, PER_PAGE),
      fetchComponents(slug),
      fetchPageMeta(slug),
    ]);
    list = ml;
    names = componentNameMap(comps);
    meta = m;
  } catch (err) {
    if (err instanceof PageAccessRequiredError) {
      redirect(withLang(`/status/${encodeURIComponent(slug)}`, locale));
    }
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
      <StatusTabs slug={slug} locale={locale} active="maintenances" />
      <h1 className="section-title">{t.maintenancesTitle}</h1>

      {list.items.length === 0 ? (
        <p className="empty">{t.noMaintenances}</p>
      ) : (
        list.items.map((m) => {
          const updates = [...m.updates].sort(
            (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
          );
          return (
            <section className="group" key={m.id}>
              <div className="group-header">
                <span>{m.title}</span>
                <Badge
                  label={t.maintenanceStatus[m.status]}
                  color={maintenanceStatusColor(m.status)}
                />
              </div>
              <div className="maint-body">
                <div className="history-meta">
                  {t.scheduledWindow}: {formatInZone(m.scheduled_start, locale, tz, hour12)} —{" "}
                  {formatInZone(m.scheduled_end, locale, tz, hour12)}
                </div>
                {m.description ? <p className="maint-desc">{m.description}</p> : null}
                {m.component_ids.length > 0 ? (
                  <div className="maint-components">
                    {t.affectedComponents}:{" "}
                    {m.component_ids.map((id) => names[id] ?? id).join(", ")}
                  </div>
                ) : null}
                {updates.length > 0 ? (
                  <ol className="timeline">
                    {updates.map((u) => (
                      <li key={u.id} className="timeline-item">
                        <div className="timeline-head">
                          <span className="timeline-time">
                            {formatInZone(u.created_at, locale, tz, hour12)}
                          </span>
                        </div>
                        <p className="timeline-body">{u.body}</p>
                      </li>
                    ))}
                  </ol>
                ) : null}
              </div>
            </section>
          );
        })
      )}

      <Pager
        basePath={`/status/${encodeURIComponent(slug)}/maintenances`}
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
