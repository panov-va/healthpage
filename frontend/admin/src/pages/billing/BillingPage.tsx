import { useCallback, useEffect, useState } from "react";

import {
  cancelSubscription,
  checkout,
  getSubscription,
  listPayments,
  type Payment,
  type Subscription,
} from "@/entities/billing";
import { HttpError } from "@/shared/api";
import { formatDateTime } from "@/shared/lib/incident";
import { Button, Card } from "@/shared/ui";

const PLAN_LABEL: Record<string, string> = { free: "Free", premium: "Premium" };
const STATUS_LABEL: Record<string, string> = {
  active: "активна",
  pending: "ожидает оплаты",
  past_due: "просрочена (попытки списания)",
  canceled: "отменена",
};
const PAYMENT_STATUS_LABEL: Record<string, string> = {
  pending: "ожидает",
  succeeded: "оплачен",
  failed: "не прошёл",
  refunded: "возврат",
};

export function BillingPage() {
  const [sub, setSub] = useState<Subscription | null>(null);
  const [payments, setPayments] = useState<Payment[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(() => {
    return Promise.all([getSubscription(), listPayments()]).then(([s, p]) => {
      setSub(s);
      setPayments(p);
    });
  }, []);

  useEffect(() => {
    setLoading(true);
    reload()
      .catch((err) =>
        setError(err instanceof HttpError ? err.message : "Не удалось загрузить тариф"),
      )
      .finally(() => setLoading(false));
  }, [reload]);

  async function startCheckout(period: "monthly" | "yearly") {
    setError(null);
    setBusy(true);
    try {
      const res = await checkout("premium", period);
      if (res.confirmation_url) {
        // Редирект на страницу/виджет оплаты провайдера.
        window.location.href = res.confirmation_url;
        return;
      }
      await reload();
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось начать оплату");
    } finally {
      setBusy(false);
    }
  }

  async function cancel() {
    setError(null);
    setBusy(true);
    try {
      const updated = await cancelSubscription();
      setSub(updated);
    } catch (err) {
      setError(err instanceof HttpError ? err.message : "Не удалось отменить подписку");
    } finally {
      setBusy(false);
    }
  }

  if (loading) return <div className="hp-container hp-muted">Загрузка…</div>;

  const isPremium = sub?.plan === "premium";
  const canCancel = isPremium && sub?.status !== "canceled" && !sub?.cancel_at_period_end;

  return (
    <div className="hp-container">
      <h1>Тариф и оплата</h1>
      {error && <div className="hp-error">{error}</div>}

      <Card>
        <h2>Текущий тариф</h2>
        <p>
          <strong>{PLAN_LABEL[sub?.plan ?? "free"] ?? sub?.plan}</strong>
          {sub && ` — ${STATUS_LABEL[sub.status] ?? sub.status}`}
        </p>
        {sub?.current_period_end && (
          <p className="hp-muted">
            {sub.cancel_at_period_end
              ? `Доступ до ${formatDateTime(sub.current_period_end)} (автопродление отключено)`
              : `Текущий период до ${formatDateTime(sub.current_period_end)}`}
          </p>
        )}

        {!isPremium && (
          <div style={{ display: "flex", gap: "0.5rem", marginTop: "0.75rem" }}>
            <Button onClick={() => startCheckout("monthly")} disabled={busy}>
              Premium помесячно
            </Button>
            <Button onClick={() => startCheckout("yearly")} disabled={busy}>
              Premium на год
            </Button>
          </div>
        )}
        {canCancel && (
          <div style={{ marginTop: "0.75rem" }}>
            <Button variant="danger" onClick={cancel} disabled={busy}>
              Отменить автопродление
            </Button>
          </div>
        )}
        <p className="hp-muted" style={{ marginTop: "0.75rem" }}>
          Подключая Premium, вы соглашаетесь с условиями автоплатежа (см. оферту).
        </p>
      </Card>

      <Card>
        <h2>История платежей</h2>
        {payments.length === 0 ? (
          <p className="hp-muted">Платежей пока нет.</p>
        ) : (
          <table className="hp-table">
            <thead>
              <tr>
                <th>Дата</th>
                <th>Сумма</th>
                <th>Статус</th>
                <th>Чек</th>
              </tr>
            </thead>
            <tbody>
              {payments.map((p) => (
                <tr key={p.id}>
                  <td>{formatDateTime(p.created_at ?? "")}</td>
                  <td>
                    {p.amount} {p.currency}
                  </td>
                  <td>{PAYMENT_STATUS_LABEL[p.status] ?? p.status}</td>
                  <td>{p.receipt_id ?? "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>
    </div>
  );
}
