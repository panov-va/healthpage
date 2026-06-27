// Публичная страница статуса (SSR) — заглушка этапа 0.
// Реальная сводка (общий статус + дерево компонентов) подключается на этапе 1 (1.8)
// через GET /pages/{slug}/summary.
export default function StatusPage({ params }: { params: { slug: string } }) {
  return (
    <main>
      <h1>Статус: {params.slug}</h1>
      <p lang="ru">Каркас публичной страницы статуса (этап 0).</p>
      <p lang="en">Public status page skeleton (stage 0).</p>
    </main>
  );
}
