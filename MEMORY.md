# MEMORY.md — рабочая память HealthPage

> Агент читает этот файл **в начале** сессии и **обновляет в конце**. Цель — чтобы следующая
> сессия (или следующий запуск) знала, где мы остановились, без перечитывания всего кода.
> Держи кратко и актуально. Устаревшее — удаляй, не накапливай.
>
> Формат обновления: правь разделы ниже. Каждую сессию добавляй строку в «Журнал сессий».

---

## Текущий статус

**Фаза:** Этап 1 (Ядро домена). Этап 0 закоммичен. Задача 1.1 (миграции) — написана и
проверена, ждёт коммита человеком.
**Следующий шаг:** Этап 1.2 — доменный слой `internal/domain` (сущности этапа + бизнес-правила
статусов и иерархии). Затем 1.3 (auth) и 1.4 (store-слой, ввод sqlc).

Готовые артефакты:
- `DESIGN.md` — дизайн-документ (нормативный, финальный для MVP).
- `openapi.yaml` — API-контракт (OpenAPI 3.1, провалиден).
- `CLAUDE.md`, `ROADMAP.md`, `MEMORY.md`, `INFRASTRUCTURE.md` — рабочие файлы процесса.
- **Каркас монорепо** (этап 0): backend (Go/chi), миграции (goose), docker-compose,
  фронты (admin Vite+React+FSD, public-ssr Next.js), генерация типов из openapi.yaml.

---

## Принятые ключевые решения (зафиксировано)

> Это решения владельца. Агент их НЕ пересматривает, опирается на них.

- **Продукт:** manual-first статус-страница, MVP для проверки спроса.
- **Имя/домен:** HealthPage / healthpage.ru.
- **Репозиторий:** монорепо (структура — CLAUDE.md §7).
- **Стек:** Go (backend), React+FSD (admin), Next.js SSR (public + лендинг вместе),
  RabbitMQ (очереди), Redis (кэш), PostgreSQL, ЮKassa (платежи).
- **Git:** коммитит и мержит ТОЛЬКО человек. Агент — готовит код и предлагает коммит.
- **Решения:** мелкие — агент сам; архитектурные/продуктовые развилки и все `[РЕШИТЬ]` — человек.
- **Режим работы:** один агент, последовательные сессии.
- **Платежи:** самозанятость + ЮKassa, приём только от физлиц, лимиты НПД как рамки. Фискализация
  через «Мой налог». B2B/ОФД/ИП — не в MVP.
- **Каналы подписки:** email, RSS, iCal, Telegram, MAX, Slack. MAX и Slack — обязательны для
  запуска. Slack — через OAuth «Add to Slack», без публикации в App Directory.
- **under_maintenance:** плановые работы показываются выше деградации; не считаются downtime.
- **Подкомпоненты:** дерево через `parent` (как у Статусмейта).
- **Rate-limit:** на уровне инфраструктуры (ingress/gateway), не в коде MVP.
- **Цена Premium:** цена Статусмейта −10% (точную цифру уточнить перед запуском).

---

## Открытые вопросы / стоп-маркеры (ждут человека)

- **[ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ МИГРАЦИИ]** 152-ФЗ: согласие импортированных подписчиков. Дефолт —
  повторный opt-in (импортированные email НЕ подтверждены автоматически). DESIGN §4.3.
- **MAX-бот:** организационно — верификация профиля самозанятого через Госуслуги + модерация
  бота. Нужно для этапа 3, запускать заранее.
- **Детали миграции** (второй этап, после запуска): таблицы маппинга кастомных типов StatusPal;
  размер годовой скидки; длительность триала (дефолт 14 дней); нужен ли адаптер Atlassian Statuspage.
- Точная сумма Premium у Статусмейта (для расчёта −10%).

---

## Что сделано

**Этап 0 — Каркас (всё проверено локально, ждёт коммита человеком):**
- 0.1 Монорепо по CLAUDE.md §7: `go.work` (модули `backend` + `shared/api-types/go`),
  каталоги `backend/{cmd,internal,migrations}`, `frontend/{admin,public-ssr}`, `scripts/`.
- 0.2 `docker-compose.yml`: postgres:16, redis:7, rabbitmq:3-management, api — с healthcheck'ами
  и `depends_on: service_healthy`. `.env.example` (только имена переменных), `.env` в .gitignore.
- 0.3 `cmd/api`: chi-сервер, `GET /healthz` → 200 `{"status":"ok"}`, graceful shutdown,
  флаг `-health` для docker HEALTHCHECK (образ distroless без shell). Тест `router_test.go`.
- 0.4 Миграции goose: `cmd/migrate` (up/down/status), начальная заглушка `00001_init.sql`.
- 0.5 `.golangci.yml`, `Makefile` (build/test/lint/fmt/migrate/gen/up), CI `.github/workflows/ci.yml`
  (backend build+test+lint, оба фронта build).
- 0.6 `frontend/admin` (Vite+React+TS, слои FSD), `frontend/public-ssr` (Next.js App Router,
  заглушки лендинга `/` и статус-страницы `/status/[slug]` — SSR). Обе сборки зелёные.
- 0.7 Генерация типов из `openapi.yaml`: `scripts/gen-ts-types.sh` (openapi-typescript → TS),
  `scripts/gen-go-types.sh` (oapi-codegen → Go). Сгенерированы `shared/api-types/ts/schema.ts`
  и `shared/api-types/go/apitypes.gen.go`.

**Проверено:** `docker compose up` → стек healthy; `curl /healthz` → 200; goose up/status/down
работают на чистой БД; `go build/test/vet` и `golangci-lint` зелёные; обе фронт-сборки проходят.

---

## Что в работе

**Этап 1.1 — миграции домена (написано и проверено, ждёт коммита человеком):**
- `00002_enums_and_helpers.sql` — enum `component_status` (нормативный), enum `billing_plan`
  (нужен accounts уже сейчас), trigger-функция `set_updated_at()`.
- `00003_accounts_users_pages.sql` — `users` (email уникален по `lower(email)`), `accounts`
  (`owner_user_id`→users, `billing_plan`), `status_pages` (slug/custom_domain — partial-unique
  среди не-удалённых; `theme` jsonb; soft-delete), `memberships` (роль через CHECK
  owner/admin/editor/viewer; unique по user+page). Триггеры updated_at на все.
- `00004_components.sql` — `component_groups`, `components` (дерево `parent_id` self-FK,
  ON DELETE CASCADE; `current_status` component_status), `component_status_history`
  (partial-unique `csh_open_period_key` — не более одного открытого периода на компонент;
  `source` через CHECK manual/incident/maintenance/api). Триггеры updated_at.

Решения по схеме (в рамках DESIGN §5, контракт не менялся):
- UUID — `gen_random_uuid()` (встроен в PG16, без расширений).
- `updated_at` — общий trigger `set_updated_at()`; проверено, что растёт между транзакциями.
- soft-delete (`deleted_at`) — у пользовательского контента: status_pages, component_groups,
  components. У users/accounts/memberships — нет (управление через is_active / каскад).
- role/visibility/source/csh — TEXT + CHECK (не входят в нормативный список enum'ов §5),
  чтобы не плодить pg-типы; component_status и billing_plan — pg enum (нормативны).

Проверено на живом PG16: up→version 4; схема (8 таблиц + 2 enum) соответствует DESIGN §5;
reset откатывает всё без остатка (enum'ы тоже), повторный up чистый; FK-цепочка, дерево
parent_id, partial-unique slug после soft-delete и open-period unique — работают.

_Этап 0 — завершён и закоммичен._

---

## Принятые решения этой сессии

- **Библиотеки (выбор человека):** роутер — **chi**; доступ к БД — **sqlc поверх pgx**
  (на этапе 0 подключён только драйвер `pgx/stdlib` для goose; sqlc вводится на этапе 1);
  миграции — **goose**.
- **Генерация типов (агентское решение, контракт не меняли):** TS — `openapi-typescript`
  (поддерживает 3.1); Go — `oapi-codegen`, но он НЕ поддерживает OpenAPI 3.1, поэтому
  `gen-go-types.sh` на лету конвертирует спеку 3.1→3.0 во временный файл
  (`@apiture/openapi-down-convert`) — **сам `openapi.yaml` не меняется**.
- **Образ api** — distroless; healthcheck выполняет сам бинарь (`api -health`).
- Убрал `chi middleware.RealIP` (уязвим к IP-спуфингу; реальный IP/rate-limit — на ingress, §3).

---

## Активные TODO / технический долг

- **sqlc** ещё не введён (нет SQL-запросов на этапе 0) — подключить на этапе 1 вместе с первыми
  store-методами (config sqlc.yaml, генерация в `internal/store`).
- **RabbitMQ delayed-exchange плагин** (`rabbitmq_delayed_message_exchange`, DESIGN §8.1) пока
  НЕ установлен в образе — добавить на этапе 3, когда строится топология очередей.
- **go.mod директива go 1.25.7** (поднята транзитивно из-за goose v3.27.1; Go тянет toolchain
  автоматически). Если захотим остаться на 1.24 — придётся понизить goose. Пока оставлено.
- npm-сборки сообщают про уязвимости в dev-зависимостях (vite/esbuild) — для каркаса некритично,
  пересмотреть при стабилизации фронта.
- Сгенерированные типы (`apitypes.gen.go`, `schema.ts`) пока никем не импортируются — потребители
  появятся на этапе 1.
- Миграции «запекаются» в api-образ на сборке. При изменении миграций для запуска через
  `docker compose exec api /app/migrate` нужно пересобрать образ (`docker compose up -d --build`).
  В dev проще гонять миграции с хоста: `make migrate-up` (берёт свежие из `backend/migrations`).

---

## Журнал сессий

> Одна строка на сессию: дата — что сделано — где остановились.

- 2026-06-27 — Этап 0 (Каркас) полностью: монорепо, docker-compose, api `/healthz`, goose,
  линтеры/CI, каркасы фронтов, генерация типов. Всё проверено локально. Остановились перед
  коммитом человеком; дальше — Этап 1.1.
- 2026-06-27 — Этап 0 закоммичен. Этап 1.1 (миграции домена): 00002–00004, enum
  `component_status`/`billing_plan`, 8 таблиц, триггеры updated_at, дерево компонентов,
  history с open-period unique. Проверено на PG16 (up/status/reset/up + инварианты).
  Остановились перед коммитом; дальше — Этап 1.2 (доменный слой).
