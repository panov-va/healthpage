import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { deletePage, listPages } from "@/entities/page";
import type { StatusPage } from "@/entities/page";
import { CreatePageForm } from "@/features/page-create";
import { HttpError } from "@/shared/api";
import { Button, Card } from "@/shared/ui";

export function PagesListPage() {
  const [pages, setPages] = useState<StatusPage[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);

  useEffect(() => {
    listPages()
      .then(setPages)
      .catch((err) => setError(err instanceof HttpError ? err.message : "Ошибка загрузки"));
  }, []);

  async function handleDelete(page: StatusPage) {
    if (!window.confirm(`Удалить страницу «${page.name}»?`)) return;
    try {
      await deletePage(page.id);
      setPages((prev) => prev?.filter((p) => p.id !== page.id) ?? null);
    } catch {
      window.alert("Не удалось удалить страницу");
    }
  }

  return (
    <div className="hp-container">
      <div className="hp-card__header" style={{ marginBottom: 16 }}>
        <h1>Страницы статуса</h1>
        <Button onClick={() => setShowForm((v) => !v)}>
          {showForm ? "Скрыть" : "Создать страницу"}
        </Button>
      </div>

      {showForm && (
        <Card>
          <CreatePageForm
            onCreated={(page) => {
              setPages((prev) => [page, ...(prev ?? [])]);
              setShowForm(false);
            }}
          />
        </Card>
      )}

      {error && <div className="hp-error">{error}</div>}

      {pages === null && !error && <p className="hp-muted">Загрузка…</p>}

      {pages && pages.length === 0 && (
        <Card>
          <div className="hp-empty">
            Пока нет ни одной страницы. Создайте первую.
          </div>
        </Card>
      )}

      {pages &&
        pages.map((page) => (
          <Card key={page.id}>
            <div className="hp-card__header">
              <div>
                <h3>
                  <Link to={`/pages/${page.id}`}>{page.name}</Link>
                </h3>
                <div className="hp-muted" style={{ fontSize: 13 }}>
                  /{page.slug}
                  {" · "}
                  {page.visibility === "private" ? "приватная" : "публичная"}
                </div>
              </div>
              <div className="hp-row">
                <Link to={`/pages/${page.id}`}>
                  <Button variant="secondary" size="sm">
                    Управление
                  </Button>
                </Link>
                <Button variant="danger" size="sm" onClick={() => handleDelete(page)}>
                  Удалить
                </Button>
              </div>
            </div>
          </Card>
        ))}
    </div>
  );
}
