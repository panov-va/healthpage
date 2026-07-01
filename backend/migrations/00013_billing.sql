-- Этап 6.1: биллинг и тарифы (DESIGN §4.2, §10).
-- subscriptions — одна на аккаунт: жизненный цикл подписки Premium (pending→active→past_due→
--   canceled), периодичность, триал, токен рекуррентных списаний провайдера (НЕ данные карты).
-- payments — журнал платежей (идемпотентность по provider_payment_id), ссылка на фискальный чек.
-- Эффективный флаг тарифа — accounts.billing_plan (его включает webhook успешной оплаты,
--   откат на free делает worker-billing). Подписка хранит детали жизненного цикла.
-- Наборы status/period/provider — TEXT+CHECK (нормативные значения задаёт openapi:
--   SubscriptionStatus/BillingPeriod/PaymentProvider), как channel/scope подписчиков и scopes
--   токенов; plan — существующий pg-enum billing_plan (как в accounts).
-- Деньги храним в минорных единицах (копейки) bigint — без ошибок плавающей точки; в API
--   amount отдаётся в рублях (amount_minor/100).

-- +goose Up
CREATE TABLE subscriptions (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id             uuid NOT NULL UNIQUE REFERENCES accounts (id) ON DELETE CASCADE,
    plan                   billing_plan NOT NULL DEFAULT 'free',
    status                 text NOT NULL DEFAULT 'active'
                               CHECK (status IN ('pending', 'active', 'past_due', 'canceled')),
    billing_period         text CHECK (billing_period IN ('monthly', 'yearly')),
    provider               text CHECK (provider IN ('yookassa', 'tinkoff', 'cloudpayments', 'robokassa')),
    -- Токен сохранённого способа оплаты у провайдера для рекуррентных списаний (НЕ данные карты).
    provider_customer_token text,
    trial_ends_at          timestamptz,
    current_period_start   timestamptz,
    current_period_end     timestamptz,
    cancel_at_period_end   boolean NOT NULL DEFAULT false,
    -- Счётчик неуспешных рекуррентных попыток (dunning); сбрасывается при успешном списании.
    dunning_attempts       int NOT NULL DEFAULT 0,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE payments (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id          uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    subscription_id     uuid REFERENCES subscriptions (id) ON DELETE SET NULL,
    amount_minor        bigint NOT NULL,
    currency            text NOT NULL DEFAULT 'RUB',
    status              text NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending', 'succeeded', 'failed', 'refunded')),
    provider            text CHECK (provider IN ('yookassa', 'tinkoff', 'cloudpayments', 'robokassa')),
    -- Идентификатор платежа у провайдера — ключ идемпотентности webhook'ов.
    provider_payment_id text,
    -- Ключ идемпотентности исходящего запроса к провайдеру (защита от двойной оплаты).
    idempotency_key     text,
    -- Ссылка на фискальный чек («Мой налог» / ОФД, DESIGN §4.2).
    receipt_id          text,
    -- Период, за который платёж (для рекуррентов и сверки).
    billing_period      text CHECK (billing_period IN ('monthly', 'yearly')),
    created_at          timestamptz NOT NULL DEFAULT now(),
    paid_at             timestamptz
);

-- Идемпотентность webhook'ов провайдера по provider_payment_id (только для непустых).
CREATE UNIQUE INDEX payments_provider_payment_id_key
    ON payments (provider_payment_id) WHERE provider_payment_id IS NOT NULL;
-- Защита от двойной оплаты по ключу идемпотентности исходящего запроса.
CREATE UNIQUE INDEX payments_idempotency_key
    ON payments (idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX payments_account_idx ON payments (account_id, created_at DESC);

CREATE TRIGGER subscriptions_set_updated_at BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS subscriptions;
