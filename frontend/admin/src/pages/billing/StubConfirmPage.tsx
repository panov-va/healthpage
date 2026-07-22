import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";

import { confirmStubPayment } from "@/entities/billing";
import { HttpError } from "@/shared/api";
import { Card } from "@/shared/ui";

// Dev-имитация страницы подтверждения оплаты платёжного провайдера (см. backend/internal/
// billing/stub.go — используется, пока не заданы реальные ключи ЮKassa). В боевом провайдере
// эта страница не участвует: подтверждение приходит с самого ЮKassa на /billing/webhook/yookassa.
export function StubConfirmPage() {
  const [searchParams] = useSearchParams();
  const payment = searchParams.get("payment");
  const [status, setStatus] = useState<"pending" | "done" | "error">("pending");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!payment) {
      setStatus("error");
      setError("В ссылке нет параметра payment");
      return;
    }
    confirmStubPayment(payment)
      .then(() => setStatus("done"))
      .catch((err) => {
        setStatus("error");
        setError(err instanceof HttpError ? err.message : "Не удалось подтвердить оплату");
      });
  }, [payment]);

  return (
    <div className="hp-center">
      <Card>
        <h1>Dev-оплата (stub)</h1>
        {status === "pending" && <p className="hp-muted">Подтверждаем оплату…</p>}
        {status === "done" && (
          <>
            <p>Оплата подтверждена — тариф Premium активирован.</p>
            <Link to="/billing">К тарифу</Link>
          </>
        )}
        {status === "error" && (
          <>
            <div className="hp-error">{error}</div>
            <Link to="/billing">К тарифу</Link>
          </>
        )}
      </Card>
    </div>
  );
}
