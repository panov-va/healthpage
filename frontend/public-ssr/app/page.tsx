// Лендинг (SSR) для SEO — этап 1.9.
// Главная (hero + возможности) и тарифы (Free/Premium — матрица фич DESIGN §10).
// Конкретная цена Premium не указана: точная сумма берётся перед запуском ([ТРЕБОВАНИЕ] DESIGN §10).
// Публичные тексты RU/EN с первого дня (CLAUDE.md §6); локаль — через ?lang=en (дефолт ru).

import type { Metadata } from "next";

import { landing, resolveLocale } from "../lib/i18n";

interface LandingProps {
  searchParams: { lang?: string | string[] };
}

// SEO-метаданные рендерятся на сервере (SSR), зависят от локали.
export function generateMetadata({ searchParams }: LandingProps): Metadata {
  const t = landing(resolveLocale(searchParams.lang));
  return {
    title: t.metaTitle,
    description: t.metaDescription,
  };
}

function Check({ on }: { on: boolean }) {
  return (
    <span className={on ? "feat-yes" : "feat-no"} aria-label={on ? "yes" : "no"}>
      {on ? "✓" : "—"}
    </span>
  );
}

export default function LandingPage({ searchParams }: LandingProps) {
  const locale = resolveLocale(searchParams.lang);
  const t = landing(locale);
  const langQuery = locale === "en" ? "?lang=en" : "";

  return (
    <main className="landing">
      <section className="hero">
        <h1>{t.heroTitle}</h1>
        <p>{t.heroSubtitle}</p>
        <div className="cta">
          <a className="btn btn-primary" href={`/status/demo${langQuery}`}>
            {t.ctaPrimary}
          </a>
          <a className="btn btn-secondary" href={`/status/demo${langQuery}`}>
            {t.ctaSecondary}
          </a>
        </div>
      </section>

      <section className="features">
        <h2>{t.featuresTitle}</h2>
        <div className="feature-grid">
          {t.features.map((f) => (
            <article className="feature-card" key={f.title}>
              <h3>{f.title}</h3>
              <p>{f.body}</p>
            </article>
          ))}
        </div>
      </section>

      <section className="pricing">
        <h2>{t.pricingTitle}</h2>
        <div className="pricing-table-wrap">
          <table className="pricing-table">
            <thead>
              <tr>
                <th scope="col">{t.featureColumn}</th>
                <th scope="col">
                  {t.planFree}
                  <span className="price">{t.priceFree}</span>
                </th>
                <th scope="col">
                  {t.planPremium}
                  <span className="price">{t.pricePremium}</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {t.planFeatures.map((pf) => (
                <tr key={pf.label}>
                  <th scope="row">{pf.label}</th>
                  <td>
                    <Check on={pf.free} />
                  </td>
                  <td>
                    <Check on={pf.premium} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </main>
  );
}
