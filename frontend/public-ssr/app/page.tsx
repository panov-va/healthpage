// Лендинг (SSR) — заглушка этапа 0. Полноценные главная/тарифы — этап 1 (1.9).
// Публичные тексты RU/EN с первого дня (CLAUDE.md §6); полноценный i18n — этап 1.
export default function LandingPage() {
  return (
    <main>
      <h1>HealthPage</h1>
      <p lang="ru">Страницы статуса для вашего продукта. Каркас (этап 0).</p>
      <p lang="en">Status pages for your product. Skeleton (stage 0).</p>
    </main>
  );
}
