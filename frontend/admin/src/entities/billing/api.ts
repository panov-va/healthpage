import { api } from "@/shared/api";
import type { BillingPeriod, BillingPlan, CheckoutResponse, Payment, Subscription } from "@/shared/api";

// Биллинг аккаунта (этап 6). Все эндпоинты — под операторским JWT (account-level).
export function getSubscription(): Promise<Subscription> {
  return api.get<Subscription>("/billing/subscription");
}

export function checkout(plan: BillingPlan, billingPeriod: BillingPeriod): Promise<CheckoutResponse> {
  return api.post<CheckoutResponse>("/billing/checkout", { plan, billing_period: billingPeriod });
}

export function cancelSubscription(): Promise<Subscription> {
  return api.post<Subscription>("/billing/cancel");
}

export function listPayments(page = 1, perPage = 50): Promise<Payment[]> {
  const q = new URLSearchParams({ page: String(page), per_page: String(perPage) });
  return api.get<Payment[]>(`/billing/payments?${q.toString()}`);
}
