import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { getPage } from "@/entities/page";
import type { StatusPage } from "@/entities/page";
import { SettingsForm } from "@/features/page-settings";
import { HttpError } from "@/shared/api";
import { Card } from "@/shared/ui";
import { PageNav } from "@/widgets/page-nav";

// Экран настроек страницы статуса: тема/брендинг/часовой пояс (этап 4.1).
export function SettingsPage() {
  const { id = "" } = useParams();
  const [page, setPage] = useState<StatusPage | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    getPage(id)
      .then(setPage)
      .catch((err) =>
        setError(err instanceof HttpError ? err.message : "Не удалось загрузить страницу"),
      )
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) return <div className="hp-container hp-muted">Загрузка…</div>;
  if (error && !page) return <div className="hp-container hp-error">{error}</div>;
  if (!page) return null;

  return (
    <div className="hp-container">
      <Link to="/" className="hp-muted">
        ← Все страницы
      </Link>
      <h1 style={{ marginTop: 8 }}>{page.name}</h1>
      <PageNav pageId={id} />

      <Card>
        <h2>Настройки</h2>
        <SettingsForm page={page} onSaved={setPage} />
      </Card>
    </div>
  );
}
