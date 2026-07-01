import { useEffect, useRef, useState, type FormEvent } from "react";

import { getImportJob, startImport, type ImportJob } from "@/entities/import";
import { HttpError } from "@/shared/api";
import { Button, Card } from "@/shared/ui";

export function ImportPage() {
  const [apiKey, setApiKey] = useState("");
  const [subdomain, setSubdomain] = useState("");
  const [region, setRegion] = useState("");
  const [mode, setMode] = useState("skip");
  const [statusPageId, setStatusPageId] = useState("");
  const [job, setJob] = useState<ImportJob | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const timer = useRef<ReturnType<typeof setInterval> | null>(null);

  // Опрос статуса задачи, пока она pending/running.
  useEffect(() => {
    if (!job || job.status === "completed" || job.status === "failed") {
      if (timer.current) clearInterval(timer.current);
      return;
    }
    timer.current = setInterval(async () => {
      try {
        const updated = await getImportJob(job.id);
        setJob(updated);
      } catch {
        /* транзиентная ошибка опроса — игнорируем до следующего тика */
      }
    }, 2000);
    return () => {
      if (timer.current) clearInterval(timer.current);
    };
  }, [job]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      const created = await startImport({
        source: "statuspal",
        api_key: apiKey,
        subdomain,
        region: region ? (region as "us" | "eu") : undefined,
        mode: mode as "skip" | "update",
        status_page_id: statusPageId || undefined,
      });
      setJob(created);
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось запустить импорт");
    } finally {
      setBusy(false);
    }
  }

  const running = job && (job.status === "pending" || job.status === "running");

  return (
    <div className="hp-container">
      <h1>Импорт из внешнего сервиса</h1>
      <p className="hp-muted">
        Пока поддерживается <strong>StatusPal</strong>. Переносятся компоненты, инциденты, работы и
        email-подписчики. Импортированные подписчики <strong>не подтверждены</strong> (потребуют
        повторного согласия — 152-ФЗ). Уведомления по истории не рассылаются.
      </p>
      {error && <div className="hp-error">{error}</div>}

      <Card>
        <h2>Параметры</h2>
        <form onSubmit={submit} className="hp-form">
          <label>
            Источник
            <select value="statuspal" disabled>
              <option value="statuspal">StatusPal</option>
            </select>
          </label>
          <label>
            API-ключ источника
            <input value={apiKey} onChange={(e) => setApiKey(e.target.value)} required />
          </label>
          <label>
            Subdomain (идентификатор страницы в источнике)
            <input value={subdomain} onChange={(e) => setSubdomain(e.target.value)} required />
          </label>
          <label>
            Регион
            <select value={region} onChange={(e) => setRegion(e.target.value)}>
              <option value="">—</option>
              <option value="us">US</option>
              <option value="eu">EU</option>
            </select>
          </label>
          <label>
            Режим
            <select value={mode} onChange={(e) => setMode(e.target.value)}>
              <option value="skip">skip (пропускать существующие)</option>
              <option value="update">update (обновлять существующие)</option>
            </select>
          </label>
          <label>
            ID целевой страницы (необязательно — пусто = создать новую)
            <input value={statusPageId} onChange={(e) => setStatusPageId(e.target.value)} />
          </label>
          <Button type="submit" disabled={busy || Boolean(running)}>
            Запустить импорт
          </Button>
        </form>
      </Card>

      {job && (
        <Card>
          <h2>Задача {job.id}</h2>
          <p>
            Статус: <strong>{job.status}</strong>
            {running ? " (выполняется…)" : ""}
          </p>
          {job.report && (
            <pre style={{ whiteSpace: "pre-wrap" }}>{JSON.stringify(job.report, null, 2)}</pre>
          )}
        </Card>
      )}
    </div>
  );
}
