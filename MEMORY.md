# MEMORY.md — рабочая память HealthPage

> Агент читает этот файл **в начале** сессии и **обновляет в конце**. Цель — чтобы следующая
> сессия (или следующий запуск) знала, где мы остановились, без перечитывания всего кода.
> Держи кратко и актуально. Устаревшее — удаляй, не накапливай.
>
> Формат обновления: правь разделы ниже. Каждую сессию добавляй строку в «Журнал сессий».

---

## Текущий статус

**Ветка:** основная теперь **master** (main переименована, запушена). Дефолт на GitHub
переключить вручную в Settings→Branches, затем удалить main (`git push origin --delete main`).
**Фаза:** Этап 1 (Ядро домена) — **закрыт по коду**. Этап 0 закоммичен (+ возможно 1.1).
Задачи 1.1–1.10 написаны и проверены, ждут коммита человеком.
**Следующий шаг:** Этап 2.1 — миграции инцидентов/работ (Incident, IncidentComponent,
IncidentUpdate, Maintenance, ...; enum'ы incident_status/impact/maintenance_status).
Перед стартом этапа 2 — дать человеку закоммитить 1.1–1.10 и подтвердить acceptance этапа 1.

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

**Этап 1.10 — админка React+FSD (написано и собрано, ждёт коммита):**
- Стек: `frontend/admin` (Vite+React+TS), добавлен **react-router-dom@6**. Типы API берутся из
  сгенерированного `shared/api-types/ts/schema.ts` через alias `@api-types` (vite+tsconfig);
  руками не правятся. dev-proxy: `/api` → `http://localhost:8080` (vite.config) — обходит CORS,
  в проде admin и api за одним gateway. Базовый префикс запросов — `/api/v1`.
- **FSD-слои строго:**
  - `shared`: `config` (API_BASE, ключ токена), `api` (http-клиент с Bearer + **refresh-on-401**
    через httpOnly-cookie, `HttpError{status,code,message}`; псевдонимы типов из `@api-types`;
    `COMPONENT_STATUSES`), `ui` (Button/Field/Input/Select/Card + `styles.css`), `lib/status`
    (RU-подписи и цвета статусов).
  - `entities`: `session` (контекст `useSession`: user/loading/applyAuth/clear, проверка токена
    на старте через `/auth/me`; api register/login/logout/fetchMe), `page`, `component`
    (+`buildComponentTree` по parent_id), `componentGroup` — api-обёртки над эндпоинтами.
  - `features`: `auth` (AuthForm вход+регистрация, LogoutButton), `page-create`,
    `group-create`, `component-create` (выбор группы/родителя/статуса), `component-status`
    (инлайн StatusSelect — ручная смена, backend пишет историю периодов).
  - `widgets`: `component-tree` (рекурсивный рендер дерева: статус-селект + удаление, метка
    приватных).
  - `pages`: `login` (центр. форма), `pages-list` (список + создание/удаление страниц),
    `page-detail` (группы + дерево компонентов, добавление группы/компонента, смена статуса,
    удаление). `app`: SessionProvider + BrowserRouter, `RequireAuth` (гард), `AppLayout` (топбар).
- **Удаления:** компонент — каскад детей (FK parent_id CASCADE), группа — компоненты становятся
  без группы (FK group_id SET NULL); после удаления список компонентов перечитывается.
- Линтер для admin не настроен (как и у public-ssr) — CI гоняет только `npm run build`. **Собрано
  зелёным** (`tsc --noEmit && vite build`).
- **E2E проверен на живом стеке** (docker compose + vite-proxy `:5173/api` → api `:8080`): register →
  GET /pages([]) → create page/group/компонент-родитель/подкомпонент(parent_id) → ручная смена
  статуса (child→major_outage) → публичная сводка `overall=major_outage`, агрегат группы корректен →
  изоляция операторов (чужая страница 404) → удаление компонента/группы/страницы (204). Это ровно
  те эндпоинты и payload'ы, что шлёт админка.

⚠️ **НАЙДЕН И ИСПРАВЛЕН баг backend (этап 1.5, ждёт коммита):** `cmd/api/main.go` собирал
`api.Deps` **без `Store: st`** → `s.store` в управляющих хендлерах был nil → **panic (nil pointer)
в `store.AccountByOwner` на любом `/pages`,`/components`,`/component-groups`**. Интеграционный тест
1.5 wired сервер сам, поэтому баг не ловил; всплыл только на живом api. Фикс — одна строка
(`Store: st` в `api.Deps`). После фикса весь управляющий API работает.

**Этап 1.9 — лендинг SSR для SEO (написано и проверено, ждёт коммита):**
- `frontend/public-ssr/app/page.tsx` — главная: hero (заголовок/подзаголовок + CTA на `/status/demo`),
  блок «Возможности» (4 карточки), таблица тарифов Free/Premium (матрица фич DESIGN §10).
- SEO: `generateMetadata` рендерит title/description на сервере, зависят от локали.
- i18n: строки лендинга в `lib/i18n.ts` (`landing(locale)`), локаль через `?lang=en` (дефолт ru).
- Цена Premium НЕ указана (`pricePremium` = «Цена уточняется»/«Price TBD»): `[РЕШИТЬ]`/точная сумма
  берётся перед запуском (DESIGN §10). Премиум-фичи помечены в матрице, конкретной цифры нет.
- Стили лендинга в `app/globals.css`. Сборка зелёная; рендер и метатеги проверены (`next start`).

**Этап 1.8 — публичная SSR-страница статуса (написано и проверено, ждёт коммита):**
- `frontend/public-ssr/app/status/[slug]/page.tsx` — `force-dynamic` SSR (свежесть статуса важнее
  кэша; кэш сводки — отдельная задача через Redis на backend). Тянет `GET /pages/{slug}/summary`.
- Рендер: баннер общего статуса (цвет по статусу), карточки групп с агрегированным статусом,
  дерево компонентов (вложенность по `parent_id` строится на клиенте — `lib/tree.ts`), блок
  ungrouped, футер «Работает на HealthPage».
- `lib/api.ts` — типизированный клиент (типы повторяют JSON backend; `PageNotFoundError` → 404),
  `lib/i18n.ts` — минимальный RU/EN (локаль через `?lang=en`, дефолт ru), `lib/tree.ts` — дерево,
  `app/globals.css` — стили, `app/status/[slug]/not-found.tsx` — 404 (приватные тоже 404, текст
  нейтральный). Env `HEALTHPAGE_API_URL` (server-side, в .env.example).
- Проверено e2e с мок-API (`next start`): RU/EN рендер, вложенные подкомпоненты, 404. Сборка зелёная.
- ⚠️ **Флаг человеку:** `PageSummary` в контракте НЕ содержит имя/тему/лого страницы — заголовок
  страницы пока не выводится (acceptance 1.8 этого не требует). Реальной статус-странице нужно имя
  продукта и брендинг → предложить добавить в `PageSummary` (или отдельный публичный page-info)
  при работе над кастомизацией (этап 4). Контракт сам не менял.

**Этапы 1.6 + 1.7 — логика статусов и публичные эндпоинты (написано и проверено, ждёт коммита):**
- 1.6: `domain.BuildPublicSummary(groups, components)` — чистая сборка сводки: приватные скрыты,
  общий статус = `ComputeOverallStatus`, статус группы = `ComputeGroupStatus` (приоритет §6:
  maintenance выше деградации, но ниже сбоев). Юнит-тест `summary_test.go`.
- 1.7: публичные хендлеры (`public.go`, без авторизации) `GET /api/v1/pages/{slug}/summary` и
  `/components`. Приватные страницы → 404 (доступ по паролю/email — этап 4); приватные компоненты
  исключаются. `active_incidents`/`active_maintenances` пока `[]` (этап 2). `updated_at` = max
  из страницы и компонентов.
- **Маршрутизация:** сегмент страницы в роутах под `/pages/{...}` унифицирован как `{page}`
  (chi требует одно имя параметра на позицию); публичные хендлеры трактуют его как slug,
  управляющие — как uuid. `/components/{id}` и `/component-groups/{id}` — отдельный сегмент, `id`.
- Интеграционный тест расширен: публичная сводка отражает worst-статус (major_outage), приватный
  компонент скрыт из публичной выдачи. PASS на реальном PG16.
- **TODO:** Redis-кэш публичной сводки (DESIGN §8) пока не подключён — на каждый запрос идём в БД.
  Для MVP допустимо; добавить кэш при необходимости.

**Этап 1.5 — управляющий API страниц/групп/компонентов (написано и проверено, ждёт коммита):**
- **Контракт расширен** (решения человека: вложенные роуты, страница по uuid): тег `Pages`;
  `GET/POST /pages`, `GET/PATCH/DELETE /pages/{id}`; `GET/POST /pages/{id}/component-groups`,
  `PATCH/DELETE /component-groups/{id}`; в `ComponentCreate` добавлен `status_page_id`, в
  `GET /components` — query `status_page_id`. Схемы StatusPage/Create/Update,
  ComponentGroupCreate/Update. Типы перегенерированы (TS+Go).
- **Вынужденная деталь:** управление компонентами осталось на плоском `/components`
  (+`status_page_id`), т.к. вложенный `/pages/{id}/components` коллизирует с публичным
  `/pages/{slug}/components`. Страницы и группы — вложенные, как выбрано.
- Хендлеры (`pages.go`, `components.go`, `access.go`): все управляющие роуты под `requireAuth`.
  **Авторизация MVP — по владению аккаунтом**: оператор управляет страницей, если её
  `account_id` == его аккаунт (`AccountByOwner`). Чужая страница → 404 (не раскрываем).
  При создании страницы создаётся owner-`Membership` (для будущего multi-user; роли — задел).
- Ручная смена статуса: `PATCH /components/{id}` с `current_status` → `ChangeComponentStatus`
  (история периодов), прочие поля → `UpdateComponent`.
- store: `AccountByOwner`; `CreateStatusPage` теперь транзакционно создаёт страницу + owner-membership
  (сигнатура получила ownerUserID).
- Интеграционный тест `management_integration_test.go` (skip без `HEALTHPAGE_TEST_DB`): register →
  создать страницу/группу/компоненты(дерево) → список → смена статуса → patch страницы →
  удаление → изоляция операторов (404) → 401 без токена. PASS на реальном PG16.

**Этап 1.4 — store-слой для остальных сущностей (написано и проверено, ждёт коммита):**
- sqlc-запросы: `status_pages` (Create/GetByID/GetBySlug/ListByAccount/Update/SoftDelete),
  `component_groups` (CRUD), `components` (CRUD + SetStatus, дерево через parent_id),
  `component_status_history` (Close/Open period, List). Перегенерён `internal/store/db`.
- sqlc override добавлен: nullable text → `*string` (чтобы `status_pages` с nullable-полями
  чисто лёг на доменные `*string`).
- store-методы (`pages.go`, `components.go`) возвращают доменные сущности; `ErrSlugTaken`.
  `ChangeComponentStatus` — транзакция: закрыть открытый период истории → сменить current_status
  → открыть новый период (DESIGN §5/§6).
- Интеграционный тест `store_integration_test.go` (skip без `HEALTHPAGE_TEST_DB`): страница,
  группа, дерево компонентов, смена статуса с историей, обновление, soft-delete, дубль slug.
  Проверено на реальном PG16 (PASS). Обычный `go test ./...` интеграционный скипает (для CI).

**Этап 1.3 — аутентификация оператора (написано и проверено e2e, ждёт коммита человеком):**
- Решения человека: **JWT access+refresh** (refresh в httpOnly-cookie `hp_refresh`, ротация +
  отзыв), хэш **argon2id**, OAuth2-соцлогин — позже (сейчас email+пароль), управляющие
  эндпоинты §7.2 принимают **и JWT, и ApiToken**.
- **Контракт расширен** (с одобрения человека): тег `Auth`, схема `BearerAuth` (http bearer JWT),
  глобальный `security: [BearerAuth, ApiToken]` (OR), эндпоинты `/auth/register|login|refresh|logout|me`,
  схемы RegisterRequest/LoginRequest/RefreshRequest/AuthUser/AuthResult. Типы перегенерированы (TS+Go).
- Миграция `00005_refresh_tokens.sql` (вспомогательная таблица auth, не из §5; хранит ХЭШ токена).
- `internal/security`: argon2id (PHC), JWT HS256 (issue/parse, явный clock), refresh = random+SHA256-хэш.
- **sqlc введён** (`backend/sqlc.yaml`, схема из goose-миграций, pgx/v5, overrides uuid→google/uuid,
  timestamptz→time.Time): запросы для users/accounts/memberships/refresh_tokens →
  `internal/store/db` (генерится, не править). `internal/store`: пул pgxpool + маппинг в домен +
  транзакционная регистрация (user+account), ErrEmailTaken/ErrNotFound.
- `internal/auth.Service`: Register/Login/Refresh(ротация)/Logout/Authenticate. `internal/api`:
  хендлеры + middleware Bearer (`requireAuth`), формат ошибок по контракту, refresh-cookie,
  роуты под `/api/v1`. main.go поднимает пул БД + TokenManager + Service (api теперь требует БД и JWT_SECRET).
- Тесты: security (argon2/jwt/refresh), api auth-флоу через httptest с in-memory Repo
  (register→me→login→refresh→logout + негативы). Все зелёные.
- Env: `JWT_SECRET` (обязателен), `ACCESS_TTL`/`REFRESH_TTL` — в .env.example и docker-compose.
- e2e на реальном стеке: register/login/me/refresh(ротация отзывает старый)/logout; пароль в БД —
  argon2id; refresh_tokens отзываются корректно.

⚠️ **Решение к пересмотру (флагнуто человеку):** бэкенд НЕ импортирует `shared/api-types/go`
(Docker собирает api из контекста `./backend` без воркспейса — импорт сломал бы сборку). DTO в
api-слое написаны вручную, синхронны с контрактом; конформность закроют контрактные тесты (1.5).
Альтернатива на будущее — собрать api из воркспейса и подключить генерируемые типы.

**Этап 1.2 — доменный слой `internal/domain` (написано и проверено, ждёт коммита человеком):**
- `status.go` — `ComponentStatus` (нормативный enum) + `WorstStatus`/`displaySeverity`.
  Нормативный приоритет показа (DESIGN §6, [РЕШЕНО]):
  `operational(0) < degraded_performance(1) < under_maintenance(2) < partial_outage(3) < major_outage(4)`
  — т.е. плановые работы перекрывают деградацию, но не реальные сбои.
- `entities.go` — `User`, `Account`, `Membership` (+`Role` с `CanEdit`), `StatusPage` (+`Visibility`),
  `ComponentGroup`, `Component` (+`CountsTowardStatus`: приватные и `display_state=false` не влияют
  на статус), `ComponentStatusHistory` (+`HistorySource`), `BillingPlan`. ID — `uuid.UUID`.
- `tree.go` — `ComponentNode`, `BuildComponentTree` (лес по `ParentID`, сортировка
  Position→Name, безопасно к отсутствующим родителям/циклам), `EffectiveStatus` (худший в поддереве).
- `page_status.go` — `ComputeOverallStatus` (общий статус страницы) и `ComputeGroupStatus`.
- Тесты: `status_test.go`, `tree_test.go` — приоритет статусов, исключение приватных/скрытых,
  вложенность и порядок дерева, агрегация группы. Все зелёные.
- Решение: домен чист (без зависимостей от БД/HTTP/openapi); типы контракта (apitypes) мапятся
  на домен в api-слое (этап 1.5), не в домене (CLAUDE §7). Добавлена зависимость `google/uuid`.

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
- §7.2 в контракте принимает JWT и ApiToken, но **ApiToken-аутентификация ещё не реализована**
  (только Bearer). ApiToken (токены страницы со scope) — этап 5. До тех пор управляющие
  эндпоинты будут работать только по операторскому JWT.
- api теперь **требует БД и JWT_SECRET** для старта (auth). Без них процесс не поднимется —
  значит и `/healthz` недоступен. Для локального запуска без БД это ожидаемо (этап 1+).
- Интеграционный тест store гоняется только при заданном `HEALTHPAGE_TEST_DB`; в CI сейчас
  скипается. Возможное улучшение — добавить postgres-сервис в CI и прогонять интеграционные тесты.

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
- 2026-06-27 — Этап 1.2 (доменный слой `internal/domain`): сущности + enum'ы + нормативный
  приоритет статусов (§6) + дерево компонентов + агрегация общего/группового статуса; юнит-тесты.
  Зависимость google/uuid. Всё зелёное (build/test/vet/lint). Дальше — 1.3 (auth), там развилка
  для человека (сессии vs JWT, argon2 vs bcrypt).
- 2026-06-27 — Этап 1.3 (auth): JWT access+refresh + argon2id; расширен openapi (/auth/*, BearerAuth);
  введён sqlc + store + auth.Service + хендлеры/middleware; миграция refresh_tokens. Зависимости
  golang-jwt/v5, x/crypto/argon2. Проверено e2e на реальном стеке. Дальше — 1.4 (store для
  pages/components/groups/history).
- 2026-06-27 — Переименовал ветку main→master (запушена; дефолт на GitHub за человеком).
  Этап 1.4 (store): sqlc-запросы и store-методы для status_pages/component_groups/components/
  history + ChangeComponentStatus; интеграционный тест на реальном PG. Дальше — 1.5 (API CRUD).
- 2026-06-27 — Этап 1.5 (управляющий API): расширил контракт (Pages, группы, status_page_id),
  хендлеры CRUD страниц/групп/компонентов по JWT с авторизацией по владению аккаунтом, ручная
  смена статуса с историей. Интеграционный HTTP-тест на реальном PG. Дальше — 1.6 (логика
  общего/группового статуса в сводке) и 1.7 (публичные эндпоинты).
- 2026-06-27 — Этапы 1.6+1.7: `domain.BuildPublicSummary` (общий/групповой статус) + публичные
  `GET /pages/{slug}/summary` и `/components` (приватные скрыты, 404 для приватных страниц).
  Унифицировал chi-параметр `{page}`. Интеграционный тест расширен публичными проверками. PASS.
  Дальше — 1.8 (public-ssr страница статуса).
- 2026-06-27 — Этап 1.8 (public-ssr страница статуса): SSR `/status/[slug]` тянет summary, рендер
  общего статуса + дерева компонентов/групп, i18n RU/EN, 404 для приватных/несуществующих;
  lib/{api,i18n,tree}.ts + globals.css; env HEALTHPAGE_API_URL. Проверено e2e с мок-API.
  Флагнул человеку: PageSummary без имени/брендинга страницы. Дальше — 1.9 (лендинг SSR).
- 2026-06-27 — Этап 1.9 (лендинг SSR): главная (hero + возможности + таблица тарифов Free/Premium,
  матрица фич §10), SEO-метатеги per-locale, i18n RU/EN; цена Premium не указана ([РЕШИТЬ]).
  Сборка/рендер проверены. Дальше — 1.10 (админка React+FSD), закрывает Этап 1.
- 2026-06-28 — Этап 1.10 (админка React+FSD): FSD-слои shared/entities/features/widgets/pages/app,
  react-router, JWT-сессия с refresh-on-401, вход+регистрация, список/CRUD страниц, управление
  страницей (группы + дерево компонентов, инлайн-смена статуса, удаления), типы из `@api-types`,
  dev-proxy `/api`→:8080. `npm run build` зелёный. **Этап 1 закрыт по коду.** E2E на живом стеке
  пройден (register→CRUD страниц/групп/компонентов-дерева→смена статуса→публичная сводка→изоляция→
  удаления). **Попутно исправлен баг 1.5: `main.go` не прокидывал Store в `api.Deps`** (panic на
  всех управляющих эндпоинтах). Дальше — этап 2.1 (миграции инцидентов/работ); сперва человек
  коммитит 1.1–1.10 + фикс main.go и подтверждает acceptance этапа 1.
