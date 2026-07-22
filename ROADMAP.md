# ROADMAP.md — декомпозиция разработки HealthPage

> Это рабочий план: этапы из `DESIGN.md §11`, разбитые на атомарные задачи. Агент берёт
> **одну задачу за раз**, сверху вниз, не перепрыгивая (зависимости важны). Каждая задача имеет
> критерий готовности (DoD). Этап закрыт, когда выполнены все его задачи и acceptance из DESIGN.
>
> **Легенда статусов:** `[ ]` не начато · `[~]` в работе · `[x]` готово (подтверждено человеком).
> Агент ставит `[~]`/`[x]` по ходу и дублирует прогресс в MEMORY.md.
>
> **Порядок этапов фиксирован.** Миграция (этап 7) — намеренно после запуска ядра: сначала
> продукт, потом переманивание клиентов. Не подтягивать раньше без решения человека.

---

## Этап 0 — Каркас

Цель: пустой, но запускающийся монорепо. Без бизнес-логики.

- [x] **0.1** Инициализировать монорепо по структуре CLAUDE.md §7 (каталоги, `go.work`, `go.mod`).
- [x] **0.2** `docker-compose.yml` для dev: postgres + redis + rabbitmq + сервис api. Healthcheck'и.
- [x] **0.3** Каркас Go-сервиса `cmd/api`: HTTP-сервер, эндпоинт `GET /healthz` → 200.
- [x] **0.4** Подключить миграции (goose/golang-migrate) с пустой начальной миграцией.
- [x] **0.5** Настроить линтеры/форматтеры (golangci-lint, gofmt) и базовый CI-скрипт сборки/тестов.
- [x] **0.6** Каркасы фронтов: `frontend/admin` (React+FSD пустой скелет), `frontend/public-ssr`
      (Next.js пустой скелет). Сборка проходит.
- [x] **0.7** Генерация типов из `openapi.yaml` в `shared/api-types` (TS + Go), скрипт генерации.

**Acceptance:** `docker compose up` поднимает стек; `GET /healthz` → 200; фронты собираются;
типы генерируются из openapi.yaml. — ✅ проверено (см. MEMORY.md), ждёт коммита человеком.

---

## Этап 1 — Ядро домена (manual-first MVP)

Зависит от: 0. Соответствует DESIGN §11 «Этап 1», §3.2, §5, §6.

- [x] **1.1** Миграции: `Account`, `User`, `Membership`, `StatusPage`, `ComponentGroup`,
      `Component` (с `parent_id` — дерево!), `ComponentStatusHistory`. Enum `component_status`.
      — ✅ проверено (up/status/reset/up, инварианты), ждёт коммита человеком.
- [x] **1.2** Доменный слой `internal/domain`: сущности выше + бизнес-правила (статусы, иерархия).
      — ✅ сущности, enum'ы, приоритет статусов (§6), дерево; юнит-тесты зелёные. Ждёт коммита.
- [x] **1.3** Auth: регистрация/логин, сессии или JWT; хэш паролей (argon2/bcrypt). Membership с ролями.
      — ✅ JWT access+refresh (ротация/отзыв), argon2id, эндпоинты /auth/* (контракт расширен),
      middleware Bearer; sqlc введён для user/account/membership/refresh. Проверено e2e. Ждёт коммита.
- [x] **1.4** Store-слой (sqlc/pgx) для сущностей этапа.
      — ✅ status_pages/component_groups/components(дерево)/component_status_history: CRUD +
      ChangeComponentStatus (история). Интеграционный тест на реальном PG. Ждёт коммита.
- [x] **1.5** API: CRUD страниц; CRUD компонентов и групп, включая **вложенные подкомпоненты**;
      ручная смена `current_status`. Строго по `openapi.yaml`.
      — ✅ контракт расширен (Pages/группы, status_page_id у компонентов), хендлеры по JWT с
      авторизацией по владению аккаунтом, смена статуса с историей. Интеграционный тест. Ждёт коммита.
- [x] **1.6** Логика общего статуса страницы и статуса группы (DESIGN §6): «худший» статус,
      **плановые работы показываются выше деградации**.
      — ✅ `domain.BuildPublicSummary` + Compute*Status (приоритет статусов), юнит-тесты. Ждёт коммита.
- [x] **1.7** Публичный `GET /pages/{slug}/summary` и `GET /pages/{slug}/components`.
      — ✅ публичные хендлеры (без авторизации), приватные страницы/компоненты скрыты, сводка с
      overall/group-статусом. Интеграционный тест. Ждёт коммита.
- [x] **1.8** `public-ssr`: публичная страница статуса (SSR) — общий статус + дерево компонентов;
      работает независимо от админки.
      — ✅ `/status/[slug]` (force-dynamic SSR) тянет `GET /pages/{slug}/summary`: баннер общего
      статуса, группы с агрегированным статусом, дерево компонентов (parent_id), ungrouped, i18n
      RU/EN (`?lang=en`), 404 для несуществующих/приватных. Проверено e2e с мок-API. Ждёт коммита.
- [x] **1.9** `public-ssr`: базовый лендинг (SSR) для SEO — главная, тарифы (заглушки).
      — ✅ `/` (SSR): hero + возможности + таблица тарифов Free/Premium (матрица фич DESIGN §10),
      SEO-метатеги (title/description) per-locale, i18n RU/EN (`?lang=en`). Цена Premium не указана
      (`[РЕШИТЬ]`/уточняется перед запуском). Сборка зелёная, рендер проверен. Ждёт коммита.
- [x] **1.10** Админка (React+FSD): экран входа, список страниц, CRUD компонентов/групп, ручная
      смена статуса. `entities`: component/componentGroup; `features`: создать/сменить статус.
      — ✅ FSD-слои (shared/entities/features/widgets/pages/app), react-router, JWT-сессия с
      refresh-on-401, экран входа+регистрации, список/создание/удаление страниц, страница
      управления (группы + дерево компонентов по parent_id, инлайн-смена статуса с историей,
      создание/удаление). Типы из `@api-types` (генерятся из openapi). dev-proxy `/api`→:8080.
      `npm run build` зелёный. Ждёт коммита человеком.

**Acceptance:** можно создать страницу, добавить группы/компоненты/подкомпоненты, вручную
выставить статус, увидеть это на публичной SSR-странице; общий статус считается корректно
(с приоритетом плановых работ); лендинг отдаётся SSR и индексируется.

---

## Этап 2 — Инциденты и плановые работы

Зависит от: 1. DESIGN §3.3, §3.4, §6.

- [x] **2.1** Миграции: `Incident`, `IncidentComponent`, `IncidentUpdate`, `Maintenance`,
      `MaintenanceComponent`, `MaintenanceUpdate`. Enum'ы `incident_status`, `incident_impact`,
      `maintenance_status`.
      — ✅ `00006_incidents_maintenances.sql`: 3 pg-enum'а (нормативны), 6 таблиц (soft-delete у
      incidents/maintenances; FK CASCADE на страницу/компонент; unique компонент-в-инциденте/работе),
      триггеры updated_at. Проверено на PG16: up/status/down/up, FK-каскады, unique, дефолты,
      trigger. Ждёт коммита человеком.
- [x] **2.2** Домен инцидентов: жизненный цикл investigating→identified→monitoring→resolved;
      impact; постмортем; лента обновлений.
      — ✅ `internal/domain/incident.go`: enum'ы `IncidentStatus`/`IncidentImpact` (IsValid,
      IsTerminal, impactSeverity+WorstImpact), сущности `Incident`/`IncidentComponent`/
      `IncidentUpdate`, жизненный цикл `ApplyStatusChange` (фиксация/сброс ResolvedAt при
      resolve/повторном открытии), правило постмортема (только после resolved). Чистый домен,
      юнит-тесты зелёные. Ждёт коммита человеком.
- [x] **2.3** Домен плановых работ: scheduled→in_progress→completed; авто-перевод компонентов в
      `under_maintenance` на время работ и обратно.
      — ✅ `internal/domain/maintenance.go`: enum `MaintenanceStatus` (IsValid+All), сущности
      `Maintenance`/`MaintenanceUpdate`, предикаты стадий + `IsActive`, `ImposedComponentStatus`
      (under_maintenance во время in_progress — основа авто-перевода, применение в 2.4),
      `ApplyStatusChange` (фиксация/сброс StartedAt/CompletedAt), `ValidateSchedule` (end>start).
      Чистый домен, юнит-тесты зелёные. Ждёт коммита человеком.
- [x] **2.4** Авто-производный статус компонентов от активных инцидентов/работ (DESIGN §3.3, §6).
      — ✅ `internal/domain/derive.go`: `DerivedComponentStatus(componentID, incidents, maintenances)
      → (status, driven)` — худший по §6 среди статусов активных инцидентов и under_maintenance
      активных работ; нет активных → (operational, false). Soft-deleted/resolved/не-in_progress
      игнорируются. Чистая функция, применение (запись в историю) — в store/service на 2.5/2.6.
      Юнит-тесты зелёные. Ждёт коммита человеком.
- [x] **2.5** API инцидентов: create / patch / delete / updates (по openapi.yaml).
      — ✅ sqlc-запросы incidents/incident_components/incident_updates → store (`incidents.go`):
      CreateIncident (+стартовое обновление), IncidentByID (агрегат), UpdateIncident (replace
      components), AddIncidentUpdate (смена статуса инцидента), SoftDeleteIncident; всё транзакционно,
      с **авто-деривацией статуса компонентов** (`recomputeComponentStatusesTx` → `domain.DerivedComponentStatus`,
      запись в историю с source=incident). API (`api/incidents.go`): POST `/incidents`,
      PATCH/DELETE `/incidents/{id}`, POST `/incidents/{id}/updates` под requireAuth, авторизация по
      владению страницей, валидация enum'ов и принадлежности компонентов странице. **Контракт:**
      добавлен `status_page_id` в `IncidentCreate` (решение человека, как в 1.5), типы перегенерированы.
      Интеграционный тест на реальном PG16 (создание→деривация→обновления→resolve→возврат компонента→
      постмортем→изоляция→401). Ждёт коммита человеком.
- [x] **2.6** API работ: create / patch (смена статуса) / delete / updates.
      — ✅ sqlc-запросы maintenances/maintenance_components/maintenance_updates → store
      (`maintenances.go`): CreateMaintenance, MaintenanceByID (агрегат), UpdateMaintenance (replace
      components + смена статуса с авто under_maintenance), AddMaintenanceUpdate (заметка без статуса),
      SoftDeleteMaintenance; всё транзакционно. **recompute расширен**: общий `recomputeComponentStatusesTx`
      теперь читает и активные инциденты, и активные (in_progress) работы (`ListActiveMaintenanceComponentIDs`),
      пишет историю с source=maintenance для under_maintenance. API (`api/maintenances.go`): POST
      `/maintenances`, PATCH/DELETE `/maintenances/{id}`, POST `/maintenances/{id}/updates` под
      requireAuth, авторизация по владению, валидация окна (ValidateSchedule) и принадлежности
      компонентов. **Контракт:** добавлен `status_page_id` в `MaintenanceCreate` (решение человека,
      как в 2.5), типы перегенерированы. Интеграционный тест на реальном PG16 (scheduled→in_progress→
      under_maintenance→completed→operational→delete-возврат→изоляция→401). Ждёт коммита человеком.
- [x] **2.7** Шаблоны инцидентов (`IncidentTemplate`).
      — ✅ **Контракт расширен** (решения человека: плоские роуты + `status_page_id`;
      `default_components` как пары `{component_id, status}`; **только CRUD**, apply — на клиенте/этап 5):
      схемы `IncidentTemplate`/`Create`/`Patch`, эндпоинты GET(list)/POST `/incident-templates`,
      GET/PATCH/DELETE `/incident-templates/{id}` (тег Incidents). Типы перегенерированы (TS+Go).
      Миграция `00007_incident_templates.sql` (БД→7): `incident_templates` + `incident_template_components`
      (hard-delete — операторская конфигурация без истории, как в модели DESIGN §5; FK CASCADE; unique
      компонент-в-шаблоне; триггеры updated_at). Домен `incident_template.go` (+`Validate`: имя+impact;
      переиспользует `IncidentComponent`). Store `incident_templates.go` (Create/ByID/List/Update/Delete,
      транзакции, без рекомпьюта — шаблон не влияет на статус). API `incident_templates.go` под requireAuth,
      авторизация по владению, валидация impact/имени/принадлежности компонентов (переиспользует
      `parseIncidentComponents`). Юнит-тест домена + интеграционный на реальном PG16 (создание не меняет
      статус компонента → список/get → patch impact+замена компонентов → 422 → изоляция → delete/повторный
      404 → 401). Build/test/vet/gofmt/lint + admin build зелёные. Ждёт коммита человеком.
- [x] **2.8** Публичные: история инцидентов с фильтрами (компонент, impact) + пагинация;
      детальная страница инцидента; список работ.
      — ✅ Контракт уже описывал эндпоинты — НЕ менялся. sqlc: `ListPublicIncidents`/`Count*`
      (фильтры status/impact/component_id через sqlc.narg, пагинация), `ListActivePublicIncidents`,
      `ListPublicMaintenances`/`Count*` (фильтр status), `ListActivePublicMaintenances`. store:
      `ListPublicIncidents`/`ListActiveIncidents`, `ListPublicMaintenances`/`ListActiveMaintenances`
      (гидрация агрегатов вынесена в `hydrateIncident`/`hydrateMaintenance`). API (`public_history.go`,
      без авторизации): `GET /pages/{slug}/incidents` (фильтры+пагинация, IncidentList), `/incidents/{id}`
      (detail; скрытые/удалённые/чужие → 404), `/maintenances` (фильтр+пагинация, MaintenanceList);
      хелпер `parsePagination` (page≥1, per_page 1..100). **Публичная сводка наполнена**: `active_incidents`
      (не resolved, видимые), `active_maintenances` (не completed: scheduled+in_progress). Невалидный
      фильтр → 422. Интеграционный тест на PG16 (фильтры/пагинация/скрытые/detail/работы/сводка) PASS.
      Build/test/vet/gofmt/lint зелёные. Ждёт коммита человеком.
- [x] **2.9** Админка: создание/ведение инцидентов (лента обновлений) и работ; UI фильтров.
      — ✅ FSD-слои: `entities` incident/maintenance/incidentTemplate (api-обёртки); `features`
      incident-create (+UI «применить шаблон»: префилл из IncidentTemplate)/incident-update (смена
      статуса через ленту)/incident-edit (impact/видимость/постмортем-после-resolved/компоненты),
      maintenance-create/maintenance-edit (lifecycle-статус)/maintenance-update (заметка),
      template-form (CRUD); `widgets` affected-components (переиспользуется инцидентом+шаблоном),
      component-checklist (работы), page-nav (вкладки разделов); `pages` incidents (список+фильтры
      status/impact/компонент+пагинация / detail с лентой), maintenances (список+фильтр / detail),
      templates. Роуты `/pages/:id/{incidents,maintenances,templates}` + detail. Типы из `@api-types`.
      Листинг/detail — через **админские read-эндпоинты** `GET /incidents`(+фильтры,включая скрытые)/
      `/incidents/{id}`, `GET /maintenances`/`/maintenances/{id}` (добавлены в контракт с санкции человека;
      sqlc ListIncidents без is_visible, store/API-хендлеры под requireAuth, интеграционные тесты PASS на
      PG16). `npm run build` зелёный; e2e-смоук на живом стеке PASS. Ждёт коммита.
- [x] **2.10** Публичный SSR: вкладки «Инциденты» и «Плановые работы», детальные страницы.
      — ✅ `public-ssr`: вкладки Статус/Инциденты/Работы (`StatusTabs`, сохраняют `?lang`); страница
      `/status/[slug]/incidents` (история + пагинация `Pager`, ссылки на detail), `/status/[slug]/incidents/[id]`
      (detail: бейджи impact/статус, затронутые компоненты с именами, **хроника обновлений** новые-сверху,
      постмортем), `/status/[slug]/maintenances` (карточки работ + окно + описание + компоненты + лента
      inline, т.к. публичного GET одной работы нет). Обзорная страница дополнена активными инцидентами/
      работами из сводки. lib/api расширен (типы Incident/Maintenance + fetchIncidents/fetchIncident/
      fetchMaintenances/fetchComponents, componentNameMap), i18n RU/EN, lib/badge (цвета). Контракт НЕ
      менялся (эндпоинты из 2.8). `next build` зелёный; e2e на живом стеке PASS (RU/EN, лента, постмортем,
      404 для скрытого/несуществующего). Ждёт коммита.

**Acceptance:** полный жизненный цикл инцидента отражается на публичной странице и в истории;
работы переводят компоненты в `under_maintenance` и обратно; фильтры и пагинация работают.

---

## Этап 3 — Подписки и уведомления

Зависит от: 2. DESIGN §3.5, §4.1, §4.4, §8.1.

- [x] **3.1** Миграции: `Subscriber` (enum канала incl. `slack`), `Notification` (журнал/идемпотентность).
      — ✅ `00008_subscribers_notifications.sql` (БД→8): `subscribers` (channel/scope/notif-status —
      TEXT+CHECK, не pg-enum: их нет в нормативном списке §5, как role/visibility/source; нормативные
      значения = openapi SubscriberChannel/Scope; component_ids uuid[]; confirm_token/unsubscribe_token
      nullable; без soft-delete — отписка = удаление, как в §5) + `notifications` (event_type, payload
      jsonb, status pending|sent|failed, attempts, sent_at — журнал для идемпотентности/ретраев §8.1).
      FK CASCADE (страница→подписчики→уведомления); unique (page,channel,address); partial-unique по
      токенам; индексы page/subscriber/status; триггеры updated_at. Проверено на PG16: up→v8, схема,
      CHECK/unique/cascade/trigger (cross-tx), down (чисто) → up. Контракт не менялся. Ждёт коммита.
- [x] **3.2** RabbitMQ: exchange'и и очереди по DESIGN §8.1 (notifications topic, q.email,
      q.telegram, q.max, q.slack, DLQ, delayed exchange). Publisher confirms + manual ack.
      — ✅ Свой образ брокера (`docker/rabbitmq/Dockerfile`: base + плагин
      `rabbitmq_delayed_message_exchange` 3.13.0; compose `build:`). Пакет `internal/queue`:
      `DeclareTopology` (exchange'и notifications/webhooks.out/delayed.events/dlx; очереди
      `q.<channel>`+`q.dlq.<channel>`+`q.webhook.out` с DLX; привязки notify.<channel>.* к
      notifications и delayed.events), `Publisher` (publisher confirms; обычная + отложенная публикация
      через `x-delay`), `Consume` (manual ack, для 3.4+). Команда `cmd/queue-setup` (идемпотентно;
      в backend-образе). Dep amqp091-go. Проверено на живом брокере: топология по §8.1
      (list_exchanges/queues/bindings), интеграционный тест confirm/DLX/delayed PASS;
      build/vet/lint + оба образа зелёные. Ждёт коммита.
- [x] **3.3** Движок уведомлений: публикация событий (новый инцидент, обновление с notify,
      старт/конец работ) в очередь; идемпотентность по `Notification.id`; ретраи с backoff; DLQ.
      — ✅ Контракт НЕ менялся. Домен `subscriber.go` (enum'ы SubscriberChannel/Scope + IsPush +
      `WantsEvent`) и `notification.go` (NotificationStatus + EventType: incident_new/incident_update/
      maintenance_scheduled/started/completed). Store `subscribers.go`/`notifications.go` (sqlc):
      CreateSubscriber/ListConfirmedSubscribers, CreateNotification/NotificationByID/MarkSent/
      MarkFailed/IncrementAttempts. Пакет `internal/notify`: `Engine` (фан-аут по подтверждённым
      push-подписчикам с учётом scope/компонентов → запись журнала pending + публикация Message с
      notification_id), `RetryBackoff` (1м→5м→30м, MaxAttempts=3) + `Engine.Retry` (delayed-републикация
      или mark failed → DLQ). Врезка в API: create-инцидент (incident_new, только видимый+notify),
      add-update (incident_update), create-работа (maintenance_scheduled), patch-работа (started/
      completed по переходу статуса). Движок — мягкая зависимость api (RABBITMQ_URL пуст/недоступен →
      рассылка off, API работает). Юнит-тесты (домен+движок с фейками), интеграционные на PG16
      (подписчики/журнал) и живой e2e движок+брокер (фан-аут→q.email с notification_id). build/test/
      vet/gofmt/golangci-lint зелёные. Ждёт коммита.
- [x] **3.4** `worker-email`: SMTP (вкл. кастомный SMTP страницы); double opt-in; отписка по токену.
      — ✅ Контракт НЕ менялся. `internal/email`: `Sender` (SMTPSender — STARTTLS/неявный TLS + MIME
      multipart/alternative; LogSender — dev-заглушка), `Render` (RU/EN письма для incident_new/update,
      maintenance_scheduled/started/completed, subscriber_confirm; ссылки на страницу/отписку/подтверждение),
      `Worker.Process` (идемпотентность по Notification.id, выбор SMTP страница-vs-системный, ретрай через
      `notify.Engine.Retry` или DLQ при исчерпании; Disposition Ack/Reject/Requeue). `cmd/worker-email`
      consume q.email (manual ack, prefetch=16). `internal/subscription.UnsubscribeToken` — HMAC-stateless
      (см. флаг про колонку unsubscribe_token). Config: SMTP_*/SUBSCRIPTION_SECRET (дефолт=JWT_SECRET).
      Dockerfile+compose: сервис worker-email. **Эндпоинты subscribe/confirm/unsubscribe — это 3.5**;
      здесь готова доставка (вкл. рендер confirm/unsubscribe-ссылок). Юнит-тесты (token/render/worker с
      фейками) + живой e2e (engine→q.email→worker доставил+sent, повтор идемпотентен). build/test/vet/
      gofmt/golangci-lint зелёные. Ждёт коммита.
- [x] **3.5** Email-подписка: `POST /pages/{slug}/subscribe`, confirm, unsubscribe.
      — ✅ Контракт НЕ менялся (эндпоинты уже в openapi). sqlc: GetSubscriberByPageChannelAddress/
      ByConfirmToken, SetSubscriberConfirmToken (перевыпуск), ConfirmSubscriber, DeleteSubscriber →
      store-методы. `subscription`: GenerateConfirmToken/HashConfirmToken (случайный токен, хэш в БД §9).
      `notify.Engine.SendConfirmation` (адресная публикация subscriber_confirm с plaintext-токеном).
      API `subscribe.go` (публичные, security[]): POST subscribe (только email; идемпотентно —
      создать/перевыпустить/не-трогать по (page,channel,address); приватная страница 404; не-email 422),
      GET confirm (хэш→поиск→confirmed=true, токен одноразовый; невалидный 400), GET unsubscribe
      (ParseUnsubscribeToken HMAC→DeleteSubscriber; битый 400). server.Deps.SubSecret. Интеграционный
      тест на PG16 (subscribe→202+письмо→pending; повтор→перевыпуск; старый токен 400; confirm→200+
      confirmed; повтор confirm 400; подтверждённый→202 без письма; unsubscribe→200+удалён; негативы).
      build/test/vet/gofmt/golangci-lint зелёные. Ждёт коммита.
- [x] **3.6** RSS/Atom фид и iCal-фид (публичные эндпоинты).
      — ✅ Контракт НЕ менялся (`GET /pages/{slug}/rss`, `GET /pages/{slug}/calendar.ics` уже в openapi).
      Пакет `internal/feed` (чистые билдеры): `BuildRSS` (RSS 2.0 через encoding/xml — инциденты+работы,
      последние 50 по дате, descr инцидента из последнего апдейта, ссылки на /status/<slug>/...),
      `BuildICal` (RFC 5545: VEVENT на работу, DTSTART/DTEND в UTC basic, escapeText, фолдинг 75 октетов,
      CRLF, STATUS scheduled→TENTATIVE иначе CONFIRMED). API `feed.go` (публичные, без авторизации):
      `handleRSS`/`handleICal` через loadPublicPage (приватная→404) + ListPublicIncidents/
      ListPublicMaintenances; `writeRaw` с нужным Content-Type. `Deps.BaseURL`=cfg.BaseURL. Юнит-тесты
      (RSS parse/порядок/escaping, iCal поля/escaping/фолдинг) + интеграционный на PG16 (RSS+iCal
      content-type и содержимое, приватная→404). build/test/vet/gofmt/golangci-lint зелёные. Ждёт коммита.
- [x] **3.7** `worker-telegram`: бот, подписка на страницу/компоненты, доставка.
      — ✅ Контракт НЕ менялся. Пакет `internal/telegram`: `Client` (минимальный Bot API —
      sendMessage/getUpdates/getMe; классификация ошибок: 403/4xx → Permanent, 429 → RetryAfter,
      сеть/5xx → транзиентная), `Render` (RU/EN сообщения incident_new/update + maintenance,
      parse_mode=HTML с экранированием), `Worker.Process` (идемпотентность по Notification.id,
      ретрай через `notify.Engine.Retry`/DLQ; Permanent-ошибка → дроп(Ack)), `Bot.Run` (long-poll
      getUpdates: `/start <slug>` подписывает чат на страницу [confirmed сразу, scope=page],
      `/stop [slug]` отписывает). `cmd/worker-telegram` совмещает потребление q.telegram и бота.
      store: новый `SubscribersByChannelAddress` (+ sqlc-запрос) для `/stop` без аргумента.
      config: `TELEGRAM_BOT_TOKEN`. Dockerfile/compose/.env.example обновлены. Юнит-тесты
      (client HTTP/ошибки, render, worker happy/идемпотент/orphan/retry/permanent/bad-addr,
      bot parseCommand/start/stop) + интеграционный на PG16 (подписка/идемпотентность/отписка).
      build/test/vet/gofmt/golangci-lint зелёные. **Флаг: подписка только на страницу (scope=page);**
      компонентная подписка через бота отложена (deep-link payload ≤64 симв. не вмещает UUID
      компонентов — нужен интерактивный выбор inline-кнопками). Ждёт коммита.
- [~] **3.8** `worker-max`: MAX Bot API, подписка, доставка; троттлинг ~30 rps.
      ⏸ **ОТЛОЖЕНО (решение человека, 2026-06-29):** MAX организационно не готов (верификация
      самозанятого + модерация бота не пройдены). Разработка переносится **на после Этапа 7** —
      пока опциональна, не блокирует запуск. Реализовать симметрично `worker-telegram`, когда
      появится токен и пройдёт модерация. Из MVP-acceptance запуска MAX временно исключён.
- [x] **3.9** **Slack-канал** (DESIGN §4.4): Slack App, OAuth `/subscribe/slack/start` +
      `/subscribe/slack/callback`, сохранение `Subscriber{channel=slack}`, доставка через
      q.slack/worker-webhook в формате Block Kit. Прямая ссылка «Add to Slack» (без Directory).
      — ✅ Контракт НЕ менялся (оба эндпоинта + enum `slack` уже в openapi; `q.slack` уже в
      топологии 3.2, движок 3.3 уже фанит на slack). Пакет `internal/slack`: `OAuth` (AuthorizeURL +
      Exchange code→incoming-webhook URL), `Client.PostMessage` (POST в webhook; 4xx→Permanent,
      429→RetryAfter, 5xx/сеть→транзиент), `Render` (Block Kit attachment с цветом по impact,
      RU/EN, экранирование mrkdwn), `Worker.Process` (зеркало telegram: идемпотентность по
      Notification.id, retry/DLQ, Permanent→дроп). API `slack.go`: `handleSlackStart` (302 на
      Slack authorize с подписанным state), `handleSlackCallback` (ParseSlackState→Exchange→
      создать confirmed slack-подписчика). `subscription.SignSlackState`/`ParseSlackState`
      (HMAC + TTL 1ч, привязка к page). `cmd/worker-webhook` потребляет q.slack. config:
      `SLACK_CLIENT_ID/SECRET` (пусто → эндпоинты 404). Dockerfile/compose/.env обновлены.
      Юнит (oauth/client/render/worker) + интеграционные на PG16 (OAuth-флоу со стабом:
      start 302+state → callback создаёт подписчика, идемпотентность, негативы, фича-off→404).
      build/test/vet/gofmt/golangci-lint зелёные. **Флаг: scope=page** (как telegram; компонентная
      подписка не выставлена в openapi-контракте /start — добавить с санкции человека).
      **Флаг:** повторный «Add to Slack» выдаёт НОВЫЙ webhook URL (особенность Slack) → возможен
      второй подписчик на тот же канал; дедуп по URL ловит только тот же URL. Ждёт коммита.
- [x] **3.10** Управление подписчиками в админке; страница управления подпиской для клиента.
      — ✅ **Контракт расширен с санкции человека:** в `GET /subscribers` добавлен query
      `status_page_id` (обязателен при JWT, вкл. неподтверждённых), в `SubscriberCreate` —
      `status_page_id`, в ответ `Subscriber` — `status_page_id`. Типы перегенерированы (TS+Go).
      Backend: sqlc `ListSubscribersByPage` + store-метод; хендлеры `handleListSubscribers`
      (status_page_id+пагинация, plain-array как в контракте), `handleCreateSubscriber`
      (**ручное добавление, confirmed=true** — оператор отвечает за согласие 152-ФЗ; только push-
      каналы email/telegram/max/slack; дубль/невалидный канал → 422), `handleDeleteSubscriber`
      (авторизация по владению страницей); роуты в server.go; `toSubscriberResponse`. Интеграционный
      тест на PG16 (список вкл. pending, создание, дубль/rss→422, удаление/повтор-404, изоляция, 401).
      Admin (FSD): `entities/subscriber`, `features/subscriber-add`, `pages/subscribers`
      (список + удаление + форма добавления + scope/компоненты), вкладка «Подписчики» в page-nav,
      роут. public-ssr: страница `/unsubscribe?token=` (вызывает backend, дружелюбное RU/EN
      подтверждение); ссылка отписки в письмах теперь ведёт на неё (public-ssr `/unsubscribe`,
      а не на API). Решение по клиентской странице — **минимум (отписка)**; просмотр/смена scope
      по токену отложены (требуют новых эндпоинтов). build (go/admin/next) + test/vet/lint зелёные.
      Ждёт коммита. **Этап 3 закрыт по коду** (MVP-каналы: email/telegram/slack).

**Acceptance:** подписчик получает уведомление по каждому помеченному обновлению через email,
Telegram и Slack (MAX отложен — см. 3.8); отписка работает; повторная доставка не дублирует;
лимита подписчиков нет. ✅ Выполнено по коду.

---

## Этап 4 — Кастомизация и white-label

Зависит от: 1 (3 желательно). DESIGN §3.6.

- [x] **4.1** Тема страницы (цвета/макет), тёмная тема, логотип/favicon, формат времени/часовой пояс.
      — ✅ **Контракт расширен с санкции человека:** схема `PublicPage` (публично-безопасное
      подмножество страницы) + поле `page` в `PageSummary` (name/description/slug/timezone/
      default_locale/theme/logo_url/favicon_url/hide_powered_by). Типы перегенерированы (TS+Go).
      БД-поля темы уже были (миграция не нужна), PATCH `/pages/{id}` уже их персистил.
      Backend: `publicPageResponse`+наполнение в сводке. public-ssr: тема (акцент через `--accent`,
      тёмный режим `data-theme` + prefers-color-scheme, формат времени 24h/12h), логотип в шапке,
      favicon+title через `generateMetadata`, время в часовом поясе страницы (`formatInZone`,
      метка пояса вместо хардкода UTC), футер honor `hide_powered_by`; общий `PageShell` на всех
      вкладках. Admin: feature `page-settings` + экран настроек + вкладка в page-nav (через PATCH,
      контракт не меняется). Структура `theme` (jsonb): `{primary_color, mode, time_format}` —
      соглашение фронт↔оператор. Build (go/admin/next) + go test (вкл. интеграционный с проверкой
      брендинга в сводке) + vet/gofmt/golangci-lint зелёные; живой e2e (api :8081 + next start):
      тёмная тема, логотип, акцент, title, favicon, время GMT+3/12h, white-label скрывает футер. Ждёт коммита.
      **Флаг:** макет (layout) пока единственный — мульти-layout не закладывал (MVP). Тумблер
      `hide_powered_by` в админке — в 4.4 (white-label premium); публичный рендер уже его учитывает.
- [x] **4.2** Приватные страницы (пароль / список email) + приватные компоненты + noindex.
      — ✅ **Объём по решению человека: только пароль** (список email отложен в 4.2.x — нужна
      таблица allowed_emails + magic-link). Механизм — **подписанная HttpOnly-cookie**: backend
      минтит HMAC-токен (привязка к page + TTL 7д), public-ssr хранит cookie `hp_access_<slug>` и
      форвардит в API как `X-Page-Access`. **Контракт расширен с санкции человека:**
      `POST /pages/{slug}/access` (пароль→токен), `StatusPageUpdate.password` (set/clear, write-only),
      `PublicPage.visibility` (для noindex), заголовок `X-Page-Access` + ответ 401 `password_required`
      на публичных read. Типы перегенерированы. Backend: `subscription.PageAccessToken/Parse`,
      `SetStatusPagePassword` (argon2id, хэш в БД §9), `handlePageAccess`, `loadPublicPage` гейтит
      приватные (401 без токена). **Приватные компоненты уже корректно скрыты** из вывода и статуса
      (`CountsTowardStatus`) — правок не потребовалось. public-ssr: парольный гейт (`AccessGate` +
      route handler `/status/[slug]/access` ставит cookie), форвард cookie, redirect подстраниц на
      гейт, `noindex` для приватных через `generateMetadata`. Admin: видимость + пароль (set/снять)
      в настройках. Юнит (token) + интеграционный (флоу пароль→токен→гейт на PG16) + живой e2e
      (гейт→неверный→верный→cookie→доступ; публичная не гейтится/индексируется) PASS. Build/test/
      vet/gofmt/golangci-lint + admin/next build зелёные. Ждёт коммита.
      **Флаг:** список разрешённых email (4.2.x) — отдельная задача (таблица + magic-link, расширение
      контракта). На гейте локаль по умолчанию ru (layout без searchParams не критично).
- [x] **4.2.1** Приватные страницы по **списку email** + magic-link.
      — ✅ Миграция `00009_page_allowed_emails`. **Контракт расширен с санкции человека:** CRUD
      `GET/POST /pages/{id}/allowed-emails`, `DELETE /allowed-emails/{id}`; публичные
      `POST /pages/{slug}/access/request-link` (всегда 202, анти-энумерация) и
      `GET /pages/{slug}/access/verify?token=`; схемы AllowedEmail/Create/AccessLinkRequest. Backend:
      store allowed-emails (+IsEmailAllowed, ErrEmailAlreadyAllowed), domain `AllowedEmail`,
      `subscription.AccessLinkToken/Parse` (HMAC page+email+TTL 1ч), транзакционное письмо magic-link
      (`EventAccessLink`, `notify.Engine.SendAccessLink` без журнала, worker-email обрабатывает
      пустой NotificationID + рендер RU/EN), `handleRequestAccessLink`/`handleVerifyAccessLink`
      (повторная сверка email со списком). public-ssr: гейт расширен email-формой (request-link
      route handler) + route handler `access/verify` (cookie). Admin: управление списком email в
      настройках (для приватной). Юнит (render) + интеграционный (CRUD+request-link 202+verify→токен+
      отзыв доступа, capturePublisher) PASS на PG16. **Флаг:** гейт показывает оба метода (пароль и
      email) — оператор настраивает нужный.
- [x] **4.3** Собственный домен: верификация CNAME + автонастройка TLS (ACME). — ✅ по коду
      (4.3.1–4.3.3). Реальный выпуск сертификатов/HTTPS-доступ по домену проверяется на прод-деплое.
      Разбито на подзадачи (решения человека: **собственный ACME-сервис на Go** + **CNAME-верификация**).
  - [x] **4.3.1** Управление доменом + верификация CNAME (контракт + backend + админка).
        — ✅ **Контракт с санкции человека:** `StatusPageUpdate.custom_domain` (set/clear, сбрасывает
        verified), `POST /pages/{id}/domain/verify` → `DomainStatus{custom_domain,domain_verified,
        cname_target}`, ответ `Conflict` (409, домен занят). Типы перегенерированы. Backend: store
        `SetCustomDomain`(ErrDomainTaken)/`SetDomainVerified`, sqlc-запросы; PATCH принимает
        custom_domain (RawMessage: absent/null/строка, нормализация lower+trim, 409 при дубле);
        `handleVerifyDomain` (резолв CNAME через инъектируемый `CNAMEResolver`, сравнение с
        `CNAMETarget`=env CNAME_TARGET по нормализованному хосту). config `CNAMETarget`. Admin:
        секция «Собственный домен» в настройках (ввод + статус verified/pending + «Проверить» +
        инструкция с cname_target). Юнит + интеграционный (фейк-резолвер: match/mismatch/409/clear)
        PASS на PG16; живой смоук эндпоинтов. build/test/vet/lint + admin build зелёные. Ждёт коммита.
  - [x] **4.3.2** ACME-сервис `cmd/tls-manager` (lego). — ✅ Миграция `00010_domain_certificates`
        (domain_certificates/acme_accounts/acme_challenges). Пакет `internal/acme` (lego v4): аккаунт
        load/create+register (ключ+registration в БД), HTTP-01 через `dbChallengeProvider` (challenge
        в БД), `Obtain` (выпуск+парс срока+сохранение), `RenewDue` (verified-домены без серта/с
        истечением <renew_before). `cmd/tls-manager` (renewal-loop + первый прогон). config ACME_*.
        Юнит-тесты (needsIssue/challenge-provider/certExpiry/key round-trip). **Реальный выпуск — на
        прод-деплое** (нужен публичный домен на :80).
  - [x] **4.3.3** Edge-прокси `cmd/edge`. — ✅ Пакет `internal/edge`: TLS по SNI (серты из БД, кэш),
        :80 HTTP-01 (challenge из БД) + redirect на https, :443 роутинг `/api/*`→API, корень
        кастомного домена→`/status/{slug}` (резолв через `SlugByCustomDomain`), прочее→public-ssr.
        config EDGE_*. Dockerfile (+tls-manager,+edge), compose (профиль `edge`). Юнит-тесты
        (isAPIPath/HTTP-01/redirect/hostOnly). **Отладка — на прод-деплое** (TLS+DNS+бэкенды).
- [x] **4.4** Скрытие «Работает на …» (white-label). — ✅ Тумблер `hide_powered_by` в SettingsForm
      (контракт уже имел поле; публичный рендер уже учитывал). Премиум-гейтинг — этап 6.
- [x] **4.5** Custom SMTP / собственный From (premium). — ✅ Контракт: `StatusPageUpdate.smtp_config`
      (write-only, объект/null) + `from_email`; ответ `StatusPage` +`from_email`+`smtp_configured`
      (без секрета). store `SetStatusPageSMTP`; PATCH merge (не перетирает при отсутствии). Admin:
      секция «Письма» (From + собственный SMTP, пароль вводится заново). Доставка (effectiveSMTP) —
      уже была в worker-email. Интеграционный тест расширен.
- [x] **4.6** Виджет статуса (встраиваемый бейдж/сниппет). — ✅ Контракт: `GET /pages/{slug}/badge.svg`
      (public, гейтится для приватных). Пакет `internal/widget` (SVG shields-стиль, цвет по статусу,
      i18n ru/en). Хендлер (ComputeOverallStatus, Cache-Control 60s). Admin: превью + embed-сниппет.
      Юнит-тест + живой смоук (operational/major_outage, en, приватная→401).

**Acceptance:** страница доступна на кастомном домене по HTTPS; приватная закрыта от анонимов;
white-label убирает брендинг; виджет встраивается на сторонний сайт. — ✅ Закрыто по коду
(кастомный домен/HTTPS — реальная проверка на прод-деплое, см. 4.3.2/4.3.3).

---

## Этап 5 — API и интеграции (слой автоматизации)

Зависит от: 2. DESIGN §4, §7.2, §7.3.

- [x] **5.1** `ApiToken` со scope'ами; аутентификация управляющих запросов (`Authorization`).
      — ✅ **Контракт расширен с санкции человека:** в `TokenCreate` добавлен required `status_page_id`
      (токен page-scoped, как модель §5); `TokenCreated` +`status_page_id`+`scopes` (required); новая
      схема `Token` (метаданные без значения) + `TokenScope` enum `[read, write]`; добавлен `GET /tokens`
      (query status_page_id → список). Типы перегенерированы (TS+Go). Миграция `00011_api_tokens.sql`
      (БД→11): `api_tokens` (token_hash unique, scopes text[], last_used_at; FK CASCADE). Домен
      `apitoken.go` (TokenScope read/write, `HasScope` — write⊇read, `NormalizeScopes`). security
      `GenerateAPIToken`/`HashAPIToken` (префикс `hp_`, SHA-256, в БД только хэш §9). store
      `api_tokens.go`. **Middleware:** `requireAuth` теперь принимает ЛИБО операторский JWT
      (`Authorization: Bearer`), ЛИБО page-токен (`Authorization: <token>` без Bearer) → principal в
      контексте; scope-энфорсинг по HTTP-методу (мутации→write, GET→read; write⊇read; нехватка→403);
      `authorizePage` работает для обоих субъектов (токен — только своя страница). API `tokens.go`:
      POST/GET/DELETE `/tokens` — **только оператор** (page-токен не управляет токенами→403; пустой
      scopes→полный доступ read+write). Юнит (домен scopes, security gen/hash) + интеграционный на PG16
      (создание/список/отзыв; аутентификация токеном; scope 403/200; привязка к странице→404; токен не
      создаёт токен→403; невалидный/отозванный→401; изоляция операторов; last_used_at). build/test/vet/
      gofmt/golangci-lint + admin build зелёные; миграция up/down/up обратима. Ждёт коммита.
- [x] **5.2** Полный write-API (компоненты/инциденты/работы/подписчики/токены) по openapi.yaml §7.2.
      — ✅ Все эндпоинты §7.2 уже существовали (этапы 1–3 + токены 5.1); 5.2 сделал их полноценно
      рабочими под page-токеном. **Контракт расширен с санкции человека:** `status_page_id` убран из
      `required` в `IncidentCreate`/`MaintenanceCreate`/`IncidentTemplateCreate`/`SubscriberCreate`
      (+комментарий «обязателен при JWT; при ApiToken из токена» — симметрично с уже опциональным
      `ComponentCreate` и GET-эндпоинтами, query которых уже `required:false`). Типы перегенерированы.
      Backend: общий резолвер `resolveManagedPage(w, r, raw)` (access.go) — оператор: status_page_id
      обязателен (422); page-токен: страница из токена, переданный raw должен совпадать (иначе 404).
      Все 5 list + 5 create хендлеров (components/incidents/incident-templates/maintenances/subscribers)
      переведены на него. PATCH/DELETE/updates уже авторизовались по ресурсу через `authorizePage` —
      под токеном работают без правок. Интеграционный тест на PG16 (`write_api_token_integration_test`):
      под write-токеном БЕЗ status_page_id — CRUD компонента, **жизненный цикл инцидента
      open→update→resolve→delete**, работы create→in_progress→delete, подписчики create→list→delete,
      шаблоны create→list; чужой status_page_id под токеном→404; оператор без status_page_id→422.
      **PASS.** build/test/vet/gofmt/golangci-lint + admin build зелёные. Ждёт коммита.
- [x] **5.3** Входящие webhook'и: Grafana, Prometheus, PagerDuty, generic; маппинг на компоненты;
      HMAC-подпись; идемпотентность по dedup-ключу. (Форматы payload — позже, см. DESIGN.)
      — ✅ **Объём по решению человека: только grafana + prometheus** (generic/pagerduty отложены —
      роуты возвращают 501). **Контракт расширен с санкции человека:** добавлены CRUD
      `/webhook-integrations` (GET list?status_page_id / POST / GET / PATCH / DELETE) + схемы
      `WebhookIntegration`/`Create`/`Patch`/`Created` + enum `WebhookIntegrationSource`; у 4 Create-схем
      status_page_id из required убран ранее (5.2) — у WebhookIntegrationCreate тоже опционален при
      ApiToken; secret генерится сервером, показывается единожды (POST + PATCH regenerate_secret);
      generic/pagerduty inbound +501. Типы перегенерированы. Миграция `00012_webhook_integrations.sql`
      (БД→12): таблица `webhook_integrations` (source CHECK, secret **plaintext** — нужен для HMAC,
      не §9-токен; component_mapping jsonb) + в `incidents` добавлены `integration_id` (FK SET NULL) и
      `external_dedup_key` + **partial-unique** (status_page_id, external_dedup_key) WHERE открыт →
      один открытый инцидент на dedup-ключ. Домен `webhook_integration.go` (WebhookSource +Implemented).
      Пакет `internal/webhook` (чистый): `VerifySignature` (HMAC-SHA256 X-Signature, sha256=-префикс,
      constant-time), `ParseGrafana`/`ParsePrometheus` (Alertmanager-совместимый payload → []Alert;
      dedup=fingerprint|хэш меток), `Mapping` (match_label→map / default_component_ids + default_impact).
      store: webhook_integrations CRUD + `OpenIncidentByDedup` + CreateIncident расширен (dedup/integration,
      ErrDedupConflict на гонке). API `integrations.go` (inbound, HMAC не JWT): firing→создать инцидент
      (если открытого с ключом нет; маппинг на компоненты страницы, impact→статус компонента),
      resolved→закрыть; повторные доставки идемпотентны. `webhook_integrations.go` (CRUD, operator-only).
      Юнит (webhook: подпись/парсинг/маппинг) + интеграционный на PG16 (CRUD+секрет единожды; firing→
      инцидент→идемпотентный повтор→resolved→закрытие→рецидив; битая подпись/чужой источник/неизвестная
      интеграция→401; generic→501; prometheus; ротация секрета; изоляция операторов). **PASS.**
      build/test/vet/gofmt/golangci-lint + admin build зелёные; миграция up/down/up обратима. Ждёт коммита.
- [x] **5.4** Исходящие webhook'и: Mattermost / произвольный URL (`worker-webhook`).
      (Slack как подписка уже сделан в этапе 3.)
      — ✅ **Контракт НЕ менялся** (канал `webhook` уже в enum `SubscriberChannel`; очередь
      `q.webhook.out` и exchange `webhooks.out` — в топологии с 3.2). Исходящий webhook = подписчик
      `Subscriber{channel=webhook, address=URL}`. Регистрация — через ручное добавление оператором
      (`POST /subscribers`, теперь принимает `webhook` + валидирует http(s)-URL; rss/ical по-прежнему
      422). Домен: `SubscriberChannel.Deliverable()` (push + webhook). Движок (`notify`): фан-аут теперь
      включает webhook-подписчиков; `Publisher.PublishWebhookOut` — webhook-сообщения идут в
      `webhooks.out` (а push — в `notifications`); ретраи webhook'а — через `delayed.events` (добавлена
      привязка `q.webhook.out ← delayed.events` по `notify.webhook.*`). Пакет `internal/webhookout`
      (зеркало `slack`): `Client.Post` (классификация 4xx/429/5xx), `Render` (payload
      **Mattermost/Slack-совместимый `{text}` + структурированные поля** event/status_page/url/
      incident|maintenance, RU/EN), `Worker` (идемпотентность по Notification.id, retry/DLQ).
      `cmd/worker-webhook` теперь потребляет ОБЕ очереди — `q.slack` (3.9) и `q.webhook.out` (5.4).
      Admin: `webhook` добавлен в `MANUAL_SUBSCRIBER_CHANNELS` (форма уже принимает URL-адрес).
      Юнит (webhookout render/client/worker; notify webhook-routing) + интеграционные на PG16
      (регистрация webhook-подписчика) + **живой e2e на PG+RabbitMQ** (engine→webhooks.out→q.webhook.out,
      channel=webhook+URL). build/test/vet/gofmt/golangci-lint + admin build зелёные. Ждёт коммита.

**Acceptance:** алерт из Grafana создаёт инцидент идемпотентно; внешний скрипт по токену
открывает/обновляет/закрывает инцидент; повторные webhook'и не плодят дубли.
✅ **Этап 5 закрыт по коду** (5.1–5.4; generic/pagerduty inbound отложены — 501, по решению человека).

---

## Этап 6 — Биллинг и тарифы

Зависит от: 1. DESIGN §4.2, §10.
> Рамки MVP: самозанятость + ЮKassa, приём от физлиц, фискализация через «Мой налог».
> Не реализовывать B2B-приём, ОФД, обвязку «на вырост».

> **Контракт `/billing/*` уже был описан в openapi (эндпоинты + схемы Subscription/Payment/
> CheckoutRequest/CheckoutResponse + enum'ы) — Этап 6 чисто реализационный, контракт НЕ менялся.**
> **Решения человека (2026-07-01):** объём — полный 6.1–6.9; реальные ключи ЮKassa и боевые
> списания — стоп-маркер `[ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ]`, код против sandbox/стаба; цены —
> плейсхолдер конфигом (990 ₽/мес, годовая −20%, триал 14 дней).

- [x] **6.1** Миграции: `Subscription`, `Payment`. Enum'ы биллинга.
      — ✅ `00013_billing.sql` (БД→13): `subscriptions` (одна на аккаунт, UNIQUE; plan=billing_plan
      pg-enum; status/period/provider — TEXT+CHECK как контракт; provider_customer_token — токен
      рекуррента, НЕ данные карты; trial/period/cancel/dunning_attempts) + `payments` (amount_minor
      bigint=копейки; partial-unique provider_payment_id для идемпотентности + idempotency_key;
      receipt_id; FK CASCADE/SET NULL). Домен `billing.go` (enum'ы+IsValid, Subscription.GrantsPremium/
      InTrial, BillingPeriod.Advance, Payment) + `features.go` (Feature, PlanAllows). Юнит-тесты. up/down/up.
- [x] **6.2** Интерфейс `PaymentProvider` (Go) + реализация под ЮKassa.
      — ✅ `internal/billing`: `Provider` (CreatePayment/ChargeRecurring/ParseWebhook), `StubProvider`
      (dev, без денег), `YooKassaProvider` (боевой HTTP-адаптер, save_payment_method; **реальные
      вызовы [ВЕРНУТЬСЯ]**), `SelectProvider` (ключи есть→ЮKassa, иначе stub). `Service` (оркестратор:
      Checkout/HandleWebhook/Cancel/ProcessDue+dunning) с инъектируемыми часами. Юнит-тесты (fake repo +
      stub): checkout→webhook→premium, идемпотентность, рекуррент, исчерпание dunning→Free, cancel.
- [x] **6.3** Checkout Premium (`POST /billing/checkout`), сохранение платёжного токена.
      — ✅ `api/billing.go` handleCheckout: pending-платёж + ссылка подтверждения провайдера; подписка
      pending; токен рекуррента сохраняется по webhook'у успеха. GET /billing/subscription, POST cancel,
      GET payments. Под операторским JWT (account-level). config: цены/триал/dunning/ЮKassa-ключи.
- [x] **6.4** `worker-billing`: рекуррентные списания по токену; dunning при неуспехе; откат на Free.
      — ✅ `cmd/worker-billing`: периодический цикл (BILLING_SCAN_INTERVAL) `Service.ProcessDue` —
      списание по сохранённому токену → продление; неуспех→dunning (past_due, grace); исчерпание/отмена→
      откат на Free. Dockerfile/compose/.env.
- [x] **6.5** Webhook ЮKassa (`POST /billing/webhook/{provider}`): проверка подписи,
      идемпотентность по `provider_payment_id`, включение Premium feature-flags.
      — ✅ Публичный роут (подлинность — на провайдере: ЮKassa по IP-allowlist; stub — нормализованный
      JSON). HandleWebhook идемпотентен по provider_payment_id (повтор успеха не активирует дважды);
      succeeded→активирует подписку+account.billing_plan=premium. Неизвестный провайдер→400.
- [x] **6.6** Фискализация через «Мой налог» (ЮKassa передаёт в ФНС); `Payment.receipt_id`.
      — ✅ receipt_id сохраняется из webhook'а/charge в `payments`. Боевая фискализация (реальный чек
      ЮKassa→ФНС в режиме НПД) — часть `[ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ БИЛЛИНГА]`; в dev stub проставляет плейсхолдер.
- [x] **6.7** Единый слой **feature-flags**: все ограничения тарифов (DESIGN §10) через флаги.
      — ✅ `domain.PlanAllows(plan, Feature)`; гейтинг ВКЛЮЧЕНИЯ premium-фич в `handlePatchPage`
      (custom_domain/private/SMTP/white-label) и `handleAddAllowedEmail` → 403 `feature_required`
      (выключение/очистка разрешены всегда). Эффективный флаг — `accounts.billing_plan`. 4 теста этапа 4
      поднимают аккаунт до premium.
- [x] **6.8** Публичная оферта (условия подключения/отключения автоплатежа).
      — ✅ public-ssr `/offer` (SSR, RU/EN): предмет, цена/период, автоплатёж и его отключение, возвраты,
      фискализация (НПД). **Черновик — финализировать текст/реквизиты с юристом перед запуском.**
- [x] **6.9** Админка: страница тарифа, история платежей/чеков.
      — ✅ FSD `entities/billing` + `pages/billing` (текущий план/статус/период; checkout monthly/yearly
      с редиректом на confirmation_url; отмена автопродления; история платежей с чеками); ссылка «Тариф»
      в шапке, роут `/billing`.

**Acceptance:** оплата Premium включает фичи; рекуррентное списание проходит автоматически;
неуспех запускает dunning и откат на Free; по каждому платежу формируется чек; повторный
webhook не создаёт дублей. ✅ **Этап 6 закрыт по коду** (реальные ЮKassa-вызовы/чеки — на прод-деплое).

---

## Этап 7 — Зрелость и миграция

Зависит от: 1–3. DESIGN §4.3, §6, §9.
> Миграция — ПОСЛЕ запуска ядра (второй этап вывода на рынок).

> **Решения человека (2026-07-01):** импорт — **только StatusPal** сейчас (Instatus/Статусмейт
> 7.7/7.8 отложены); наблюдаемость — только `/metrics` кодом, Grafana/вывод статуса/бэкапы — прод-решение.

- [x] **7.1** Расчёт uptime (90 дней) и график на компонент; `under_maintenance` не как downtime.
      — ✅ Контракт уже был (`GET /pages/{slug}/uptime` + `UptimeReport`). Домен `uptime.go`
      (`ComputeUptime`: **under_maintenance ИСКЛЮЧЁН** из окна (Statuspage-семантика), downtime =
      partial/major_outage, degraded/operational = доступен; окно клиппируется датой создания
      компонента; daily по календарным суткам UTC). store `StatusHistorySince` (оконный запрос
      истории). API `handleUptime` (публичный, гейт приватных через X-Page-Access; приватные/чужие
      компоненты→404). public-ssr: полоса uptime (90 ячеек по дням) на компонентах с show_uptime.
      Юнит (домен) + интеграционный на PG16 (100%/бэкдейт-простой/приватный→404). Ждёт коммита.
- [x] **7.2** Страница релизов (changelog, `ChangelogEntry`).
      — ✅ **Контракт расширен с санкции человека:** `ChangelogEntry`/`Create`/`Patch` + `GET/POST
      /changelog`, `GET/PATCH/DELETE /changelog/{id}` (admin, плоские роуты + status_page_id как
      incident-templates) + публичный `GET /pages/{slug}/changelog` (только опубликованные). Миграция
      `00014_changelog.sql`. Домен/store/API; публикация — флаг `published` (+published_at). Admin
      (entities/changelog + страница + вкладка «Релизы») + public-ssr вкладка «Релизы» (body — текст,
      экранируется). Интеграционный на PG16 (черновик скрыт→публикация→виден→снятие→изоляция→delete).
- [x] **7.3** Наблюдаемость: метрики (Prometheus). — ✅ **По решению человека — только `/metrics`.**
      Пакет `internal/metrics` (promhttp + middleware `healthpage_http_requests_total`/
      `_request_duration_seconds`, метка route=шаблон chi); `GET /metrics` на api. Юнит-тест.
      Grafana/трейсинг/логи/собственная статус-страница сервиса — прод-решение (INFRASTRUCTURE §7).
- [ ] **7.4** Бэкапы: PITR PostgreSQL, снапшоты, проверка восстановления.
      — ⏸ **Прод-задача (решение человека):** средствами прод-БД/облака, не код MVP. См. INFRASTRUCTURE §7.
- [x] **7.5** Миграция — каркас: `ImportJob`, `ExternalIdMap`, интерфейс `Importer`, `worker-import`,
      очередь `q.import`, эндпоинты `POST /import` + `GET /import/{job_id}`. Идемпотентность; без
      рассылки уведомлений по истории.
      — ✅ Миграция `00015_import.sql` (import_jobs + external_id_map). Домен `import.go` (enum'ы +
      ImportJob + Imported* + `Importer`). Очередь `q.import` (topology + `PublishImport`).
      Движок `internal/importer` (идемпотентность через external_id_map, дерево компонентов 2-м
      проходом, без notify — пишет через store). `cmd/worker-import` (consume q.import). API:
      **контракт расширен** (`ImportRequest.status_page_id` опционален — цель или создать новую
      страницу); POST /import (только statuspal→иначе 422; api_key в сообщении, НЕ в БД) + GET
      /import/{job_id}. Интеграционный на PG16 (fake Importer: создание+дерево+инцидент+работа+
      подписчик confirmed=false + идемпотентный повтор). Dockerfile/compose/.env.
      ⚠️ **[ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ]** — 152-ФЗ: импортированные email `confirmed=false` (opt-in).
- [x] **7.6** Адаптер **StatusPal** (API v2; маппинг — DESIGN §4.3.1).
      — ✅ `internal/importer/statuspal.go`: HTTP-адаптер (base по региону us/eu, auth-заголовок),
      парсинг services/incidents/maintenances/subscriptions → Imported*, маппинг статусов/impact.
      **Сверено на живом read-only ключе клиента (2026-07-22)** — обёртки ответа, дерево сервисов
      через children_ids, /maintenances→incidents?type=scheduled, UUID-подписчики; юнит-тесты на
      httptest. **[ВЕРНУТЬСЯ] частично:** плановые работы не встретились на тестовых данных, схема
      по аналогии — сверить при первом реальном импорте с maintenance-записями.
- [ ] **7.7** Адаптер **Instatus** — ⏸ отложен (решение человека: пока только StatusPal).
- [ ] **7.8** Адаптер **Статусмейт** — ⏸ отложен (решение человека: пока только StatusPal).
- [x] **7.9** UI импорта: форма (источник/регион/ключ/идентификатор), прогресс, отчёт.
      — ✅ Admin `entities/import` + `pages/import` (форма StatusPal + опрос статуса задачи + отчёт JSON),
      ссылка «Импорт» в шапке, роут `/import`.

**Acceptance:** uptime совпадает с историей ✅; импорт со StatusPal переносит структуру сервисов
(incl. parent/child, private), историю инцидентов с обновлениями, работы и email-подписчиков;
идемпотентно; без рассылки по истории ✅ (реальная сверка StatusPal API — на прод-деплое).
Instatus/Статусмейт — отложены; бэкапы — прод.

---

## Принципы декомпозиции (для агента, если задачу нужно дробить дальше)

- Одна задача = один связный кусок, который собирается и тестируется отдельно.
- Сначала миграция/модель → домен → store → API → фронт. Не начинать с UI.
- Каждая задача с изменением API сверяется с `openapi.yaml`; расхождение → стоп, к человеку.
- Доменную логику покрывать тестами до перехода к следующей задаче.
- Если задача оказалась крупнее ожидаемого — разбить на подзадачи прямо здесь (7.x.1, 7.x.2),
  отметить в MEMORY.md, и двигаться по ним.
