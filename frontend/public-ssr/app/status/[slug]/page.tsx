// Публичная страница статуса (SSR) — этап 1.8.
// Тянет GET /pages/{slug}/summary, рендерит общий статус + дерево компонентов и групп.
// Работает независимо от админки. Инциденты/работы — этап 2 (в сводке пока пусты).

import { notFound } from "next/navigation";

import { fetchPageSummary, PageNotFoundError } from "../../../lib/api";
import type { ApiComponent, ComponentStatus } from "../../../lib/api";
import { buildTree } from "../../../lib/tree";
import type { ComponentNode } from "../../../lib/tree";
import {
  dict,
  formatUpdatedAt,
  resolveLocale,
} from "../../../lib/i18n";
import type { Locale } from "../../../lib/i18n";

// SSR на каждый запрос: статус должен быть свежим (кэш сводки — отдельная задача, Redis на backend).
export const dynamic = "force-dynamic";

interface PageProps {
  params: { slug: string };
  searchParams: { lang?: string | string[] };
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

  return (
    <main className="page">
      <section className={`overall bg-${summary.overall_status}`}>
        <h1>{t.overall[summary.overall_status]}</h1>
        <div className="updated">
          {t.updatedAt}: {formatUpdatedAt(summary.updated_at, locale)} UTC
        </div>
      </section>

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

      <footer className="footer">{t.poweredBy}</footer>
    </main>
  );
}
