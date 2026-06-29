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
- [ ] **3.7** `worker-telegram`: бот, подписка на страницу/компоненты, доставка.
- [ ] **3.8** `worker-max`: MAX Bot API, подписка, доставка; троттлинг ~30 rps.
      ⚠️ Зависит от организационной готовности (верификация самозанятого + модерация бота).
- [ ] **3.9** **Slack-канал** (DESIGN §4.4): Slack App, OAuth `/subscribe/slack/start` +
      `/subscribe/slack/callback`, сохранение `Subscriber{channel=slack}`, доставка через
      q.slack/worker-webhook в формате Block Kit. Прямая ссылка «Add to Slack» (без Directory).
- [ ] **3.10** Управление подписчиками в админке; страница управления подпиской для клиента.

**Acceptance:** подписчик получает уведомление по каждому помеченному обновлению через email,
Telegram, MAX и Slack; отписка работает; повторная доставка не дублирует; лимита подписчиков нет.

---

## Этап 4 — Кастомизация и white-label

Зависит от: 1 (3 желательно). DESIGN §3.6.

- [ ] **4.1** Тема страницы (цвета/макет), тёмная тема, логотип/favicon, формат времени/часовой пояс.
- [ ] **4.2** Приватные страницы (пароль / список email) + приватные компоненты + noindex.
- [ ] **4.3** Собственный домен: верификация CNAME + автонастройка TLS (ACME).
- [ ] **4.4** Скрытие «Работает на …» (white-label).
- [ ] **4.5** Custom SMTP / собственный From (premium).
- [ ] **4.6** Виджет статуса (встраиваемый бейдж/сниппет).

**Acceptance:** страница доступна на кастомном домене по HTTPS; приватная закрыта от анонимов;
white-label убирает брендинг; виджет встраивается на сторонний сайт.

---

## Этап 5 — API и интеграции (слой автоматизации)

Зависит от: 2. DESIGN §4, §7.2, §7.3.

- [ ] **5.1** `ApiToken` со scope'ами; аутентификация управляющих запросов (`Authorization`).
- [ ] **5.2** Полный write-API (компоненты/инциденты/работы/подписчики/токены) по openapi.yaml §7.2.
- [ ] **5.3** Входящие webhook'и: Grafana, Prometheus, PagerDuty, generic; маппинг на компоненты;
      HMAC-подпись; идемпотентность по dedup-ключу. (Форматы payload — позже, см. DESIGN.)
- [ ] **5.4** Исходящие webhook'и: Mattermost / произвольный URL (`worker-webhook`).
      (Slack как подписка уже сделан в этапе 3.)

**Acceptance:** алерт из Grafana создаёт инцидент идемпотентно; внешний скрипт по токену
открывает/обновляет/закрывает инцидент; повторные webhook'и не плодят дубли.

---

## Этап 6 — Биллинг и тарифы

Зависит от: 1. DESIGN §4.2, §10.
> Рамки MVP: самозанятость + ЮKassa, приём от физлиц, фискализация через «Мой налог».
> Не реализовывать B2B-приём, ОФД, обвязку «на вырост».

- [ ] **6.1** Миграции: `Subscription`, `Payment`. Enum'ы биллинга.
- [ ] **6.2** Интерфейс `PaymentProvider` (Go) + реализация под ЮKassa.
- [ ] **6.3** Checkout Premium (`POST /billing/checkout`), сохранение платёжного токена.
- [ ] **6.4** `worker-billing`: рекуррентные списания по токену; dunning при неуспехе; откат на Free.
- [ ] **6.5** Webhook ЮKassa (`POST /billing/webhook/{provider}`): проверка подписи,
      идемпотентность по `provider_payment_id`, включение Premium feature-flags.
- [ ] **6.6** Фискализация через «Мой налог» (ЮKassa передаёт в ФНС); `Payment.receipt_id`.
- [ ] **6.7** Единый слой **feature-flags**: все ограничения тарифов (DESIGN §10) через флаги.
- [ ] **6.8** Публичная оферта (условия подключения/отключения автоплатежа).
- [ ] **6.9** Админка: страница тарифа, история платежей/чеков.

**Acceptance:** оплата Premium включает фичи; рекуррентное списание проходит автоматически;
неуспех запускает dunning и откат на Free; по каждому платежу формируется чек; повторный
webhook не создаёт дублей.

---

## Этап 7 — Зрелость и миграция

Зависит от: 1–3. DESIGN §4.3, §6, §9.
> Миграция — ПОСЛЕ запуска ядра (второй этап вывода на рынок).

- [ ] **7.1** Расчёт uptime (90 дней) и график на компонент; `under_maintenance` не как downtime.
- [ ] **7.2** Страница релизов (changelog, `ChangelogEntry`).
- [ ] **7.3** Наблюдаемость: метрики (Prometheus), трейсинг, логи; собственная статус-страница сервиса.
- [ ] **7.4** Бэкапы: PITR PostgreSQL, снапшоты, проверка восстановления.
- [ ] **7.5** Миграция — каркас: `ImportJob`, `ExternalIdMap`, интерфейс `Importer`, `worker-import`,
      очередь `q.import`, эндпоинты `POST /import` + `GET /import/{job_id}`. Идемпотентность; без
      рассылки уведомлений по истории.
      ⚠️ **[ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ]** — 152-ФЗ по подписчикам (импортированные email НЕ
      подтверждены автоматически до решения человека).
- [ ] **7.6** Адаптер **StatusPal** (API v2; маппинг — DESIGN §4.3.1).
- [ ] **7.7** Адаптер **Instatus** (REST API; DESIGN §4.3.2).
- [ ] **7.8** Адаптер **Статусмейт** (best-effort, нестабильный внешний API; DESIGN §4.3.3).
- [ ] **7.9** UI импорта: форма (источник/регион/ключ/идентификатор), прогресс, отчёт.

**Acceptance:** uptime совпадает с историей; импорт со StatusPal и Instatus переносит структуру
сервисов (incl. parent/child, private), историю инцидентов с обновлениями, работы и
email-подписчиков; идемпотентно; без рассылки по истории; адаптер Статусмейта переносит хотя бы
компоненты.

---

## Принципы декомпозиции (для агента, если задачу нужно дробить дальше)

- Одна задача = один связный кусок, который собирается и тестируется отдельно.
- Сначала миграция/модель → домен → store → API → фронт. Не начинать с UI.
- Каждая задача с изменением API сверяется с `openapi.yaml`; расхождение → стоп, к человеку.
- Доменную логику покрывать тестами до перехода к следующей задаче.
- Если задача оказалась крупнее ожидаемого — разбить на подзадачи прямо здесь (7.x.1, 7.x.2),
  отметить в MEMORY.md, и двигаться по ним.
