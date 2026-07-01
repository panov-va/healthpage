// Публичная вкладка «Релизы» / changelog (этап 7.2). Источник: GET /pages/{slug}/changelog
// (только опубликованные). body выводится как текст (React экранирует; переносы сохраняем).

import { notFound, redirect } from "next/navigation";

import { PageShell } from "../PageShell";
import { StatusTabs } from "../StatusTabs";
import {
  fetchChangelog,
  fetchPageMeta,
  PageAccessRequiredError,
  PageNotFoundError,
} from "../../../../lib/api";
import { buildStatusMetadata } from "../../../../lib/meta";
import { is12h, parseTheme } from "../../../../lib/theme";
import { dict, formatInZone, resolveLocale, withLang } from "../../../../lib/i18n";

export const dynamic = "force-dynamic";

interface PageProps {
  params: { slug: string };
  searchParams: { lang?: string | string[] };
}

export async function generateMetadata({ params }: PageProps) {
  return buildStatusMetadata(params.slug);
}

export default async function ChangelogPage({ params, searchParams }: PageProps) {
  const locale = resolveLocale(searchParams.lang);
  const t = dict(locale);
  const slug = params.slug;

  let entries;
  let meta;
  try {
    const [cl, m] = await Promise.all([fetchChangelog(slug, 1, 50), fetchPageMeta(slug)]);
    entries = cl;
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
        <StatusTabs slug={slug} locale={locale} active="changelog" />
        <h1 className="section-title">{t.changelogTitle}</h1>

        {entries.length === 0 ? (
          <p className="empty">{t.noChangelog}</p>
        ) : (
          entries.map((e) => (
            <section className="group" key={e.id}>
              <div className="group-header">
                <span>{e.title}</span>
                <span className="history-meta">
                  {formatInZone(e.published_at ?? e.created_at, locale, tz, hour12)}
                </span>
              </div>
              {e.body ? <p className="changelog-body">{e.body}</p> : null}
            </section>
          ))
        )}
      </main>
    </PageShell>
  );
}
