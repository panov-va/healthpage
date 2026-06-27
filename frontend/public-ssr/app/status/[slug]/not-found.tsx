// Страница 404 для несуществующих/недоступных страниц статуса.
// Приватные страницы backend также отдаёт как 404 (не раскрывая их существование),
// поэтому текст нейтральный. searchParams в not-found недоступны — показываем RU и EN сразу.
import { dict } from "../../../lib/i18n";

export default function StatusNotFound() {
  const ru = dict("ru");
  const en = dict("en");
  return (
    <main className="page">
      <section className="group">
        <div style={{ padding: "32px 24px", textAlign: "center" }}>
          <h1 style={{ marginTop: 0 }}>{ru.notFoundTitle}</h1>
          <p>{ru.notFoundBody}</p>
          <hr style={{ border: "none", borderTop: "1px solid var(--border)", margin: "24px 0" }} />
          <h2 style={{ fontSize: "1.1rem" }}>{en.notFoundTitle}</h2>
          <p>{en.notFoundBody}</p>
        </div>
      </section>
    </main>
  );
}
