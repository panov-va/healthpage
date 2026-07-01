-- name: EnsureSubscription :one
-- Создаёт дефолтную free-подписку аккаунта, если её ещё нет; возвращает текущую.
INSERT INTO subscriptions (account_id)
VALUES ($1)
ON CONFLICT (account_id) DO UPDATE SET account_id = subscriptions.account_id
RETURNING *;

-- name: GetSubscriptionByAccount :one
SELECT * FROM subscriptions WHERE account_id = $1;

-- name: UpdateSubscription :one
UPDATE subscriptions
SET plan = $2,
    status = $3,
    billing_period = $4,
    provider = $5,
    provider_customer_token = $6,
    trial_ends_at = $7,
    current_period_start = $8,
    current_period_end = $9,
    cancel_at_period_end = $10,
    dunning_attempts = $11
WHERE account_id = $1
RETURNING *;

-- name: ListDueSubscriptions :many
-- Подписки, требующие внимания worker-billing: активные/просроченные premium с истёкшим
-- текущим периодом (рекуррент или dunning/откат).
SELECT * FROM subscriptions
WHERE plan = 'premium'
  AND status IN ('active', 'past_due')
  AND current_period_end IS NOT NULL
  AND current_period_end <= $1
ORDER BY current_period_end ASC
LIMIT $2;

-- name: SetAccountPlan :exec
UPDATE accounts SET billing_plan = $2 WHERE id = $1;

-- name: CreatePayment :one
INSERT INTO payments (
    account_id, subscription_id, amount_minor, currency, status, provider,
    provider_payment_id, idempotency_key, billing_period
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetPayment :one
SELECT * FROM payments WHERE id = $1;

-- name: GetPaymentByProviderID :one
SELECT * FROM payments WHERE provider_payment_id = $1;

-- name: UpdatePaymentResult :one
UPDATE payments
SET status = $2,
    provider_payment_id = COALESCE($3, provider_payment_id),
    receipt_id = COALESCE($4, receipt_id),
    paid_at = $5
WHERE id = $1
RETURNING *;

-- name: ListPaymentsByAccount :many
SELECT * FROM payments
WHERE account_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
