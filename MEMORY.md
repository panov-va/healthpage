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
**Фаза:** Этап 3 (Подписки и уведомления) — в работе. 3.1 (миграции) готово. Этапы 1–2 закоммичены
человеком (по его словам — коммит на каждом этапе; в т.ч. админские read-эндпоинты и 2.10 в git).
**Следующий шаг:** Этап 3.2 — RabbitMQ-топология (DESIGN §8.1): exchange `notifications` (topic),
очереди `q.email`/`q.telegram`/`q.max`/`q.slack`, DLQ `q.dlq.<channel>`, delayed exchange (плагин!),
publisher confirms + manual ack. ⚠️ delayed-exchange плагин в образ rabbitmq ещё не добавлен (TODO).
Затем 3.3 (движок уведомлений: публикация событий, идемпотентность по Notification.id, ретраи/backoff).

✅ **Закрыт флаг 2.9 (контракт расширен с санкции человека):** добавлены админские read-эндпоинты
`GET /incidents` (со status_page_id + фильтры + пагинация, **включая скрытые**), `GET /incidents/{id}`
(без скрытия), `GET /maintenances`, `GET /maintenances/{id}`. Админка теперь листит/читает инциденты и
работы через них (а не через публичные `/pages/{slug}/...`), скрытые инциденты доступны оператору.

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

**Этап 3.1 — миграции подписчиков/уведомлений (написано и проверено на PG16, ждёт коммита):**
- `backend/migrations/00008_subscribers_notifications.sql`. Версия БД → **8**.
- **Решение по типам (агентское, в рамках §5):** `channel` (email|telegram|rss|ical|webhook|max|slack),
  `scope` (page|components), `notifications.status` (pending|sent|failed) — **TEXT + CHECK**, НЕ pg-enum:
  их нет в нормативном списке enum'ов DESIGN §5 (как role/visibility/source в 00003/00004). Нормативные
  значения совпадают с openapi `SubscriberChannel`/`SubscriberScope`. Нормативные §5-enum'ы остаются
  pg-типами (component_status, incident_*, maintenance_status).
- `subscribers`: status_page_id (FK CASCADE), channel, address, confirmed (default false), confirm_token,
  unsubscribe_token (оба nullable; по §9 хранить ХЭШ — генерация/хэширование в сервисе на 3.4/3.5),
  scope (default 'page'), component_ids uuid[] (default '{}'), created_at/updated_at. **Без soft-delete**
  (в §5 у Subscriber нет deleted_at — отписка = физическое удаление). Индексы: unique
  (status_page_id, channel, address) — идемпотентность повторной подписки; partial-unique по
  confirm_token и unsubscribe_token; idx по status_page_id.
- `notifications`: subscriber_id (FK CASCADE), event_type, payload jsonb (default '{}'), status
  (default 'pending'), attempts int (default 0), sent_at nullable, created_at/updated_at. Журнал для
  идемпотентности/ретраев (§8.1). Индексы: subscriber_id, status (воркер выбирает pending). **dedup-ключ
  отдельной колонкой НЕ заводил** — §5 его не предусматривает; если 3.3 потребует, добавить тогда.
- Триггеры `set_updated_at` на обеих таблицах (общая функция из 00002).
- **Проверено на живом PG16:** up→v8; колонки/индексы/CHECK соответствуют; функционально — bad channel/
  scope/status → check_violation, дубль (page,channel,address) → unique_violation, дефолты
  pending/0, updated_at-триггер растёт между транзакциями, FK-каскад страница→подписчик→уведомление;
  down (таблицы исчезают без остатка) → up снова. Контракт/домен/store не трогал — это только миграция.

**Этап 2.10 — публичный SSR: вкладки Инциденты/Работы + детальные страницы (написано, `next build` зелёный, e2e на живом стеке PASS, ждёт коммита):**
- **Контракт НЕ менялся** — используются публичные эндпоинты 2.8 (`/pages/{slug}/incidents`,
  `/incidents/{id}`, `/maintenances`, `/components`). Только `frontend/public-ssr`.
- **lib/api.ts:** типы `ApiIncident`/`ApiIncidentUpdate`/`ApiIncidentComponent`/`ApiMaintenance`/
  `ApiMaintenanceUpdate`/`Pagination`/`IncidentList`/`MaintenanceList`; `PageSummary.active_*` теперь
  типизированы. Общий `getJSON` (404→PageNotFoundError). Функции `fetchIncidents`/`fetchIncident`/
  `fetchMaintenances`/`fetchComponents` + `componentNameMap` (id→имя; приватные компоненты API не отдаёт).
- **lib/i18n.ts:** расширен Dict (tabs, incidentStatus/impact/maintenanceStatus, заголовки, noIncidents/
  noMaintenances, started/resolved/scheduledWindow/affectedComponents/postmortem/updatesTitle/пагинация)
  RU+EN; хелпер `withLang` (сохраняет `?lang=en` в ссылках). **lib/badge.ts:** цвета impact/incident/
  maintenance поверх `--st-*`.
- **Компоненты** (server): `StatusTabs` (Статус/Инциденты/Работы, подсветка активной, сохранение локали),
  `Badge` (пилюля с точкой), `Pager` (новее/старее, сохраняет page+lang).
- **Страницы:** `/status/[slug]` дополнена секциями активных инцидентов/работ из сводки + вкладки;
  `/status/[slug]/incidents` (список + пагинация, ссылки на detail); `/status/[slug]/incidents/[id]`
  (detail: бейджи impact/статус, затронутые компоненты с именами + их статус-в-инциденте, **хроника
  обновлений новые-сверху**, постмортем; скрытый/несуществующий → 404); `/status/[slug]/maintenances`
  (карточки: окно, описание, компоненты по именам, лента обновлений inline — публичного GET одной работы
  нет, список отдаёт работы полными). Все force-dynamic. CSS в globals.css (tabs/badge/history/timeline/
  pager/maint).
- **Проверка:** `next build` зелёный (4 динамических роута). e2e на живом api+PG (seed: resolved-инцидент с
  3 обновлениями+постмортем, активный инцидент, in_progress-работа с заметкой): обзор показывает активные;
  список инцидентов RU; detail — лента+постмортем+компоненты; работы — окно+описание+лента; EN-локаль;
  404 для неизвестного slug и скрытого/несуществующего инцидента. **Этап 2 закрыт по коду.**

**Админские read-эндпоинты инцидентов/работ (контракт расширен с санкции человека; написано, тесты PASS, ждёт коммита):**
- **Контракт (openapi.yaml):** добавлены `GET /incidents` (tag Incidents; query `status_page_id`+`status`+
  `impact`+`component_id`+Page/PerPage → IncidentList; **включает скрытые** is_visible=false),
  `GET /incidents/{id}` (Incident, без скрытия), `GET /maintenances` (query status_page_id+status+
  пагинация → MaintenanceList), `GET /maintenances/{id}` (Maintenance). Под global security (JWT/ApiToken).
  Типы перегенерированы (TS+Go), openapi провалиден (оба gen прошли).
- **sqlc** (incidents.sql): новые `ListIncidents`+`CountIncidents` — копия публичных, но **без**
  `is_visible = true`. Работы признака видимости не имеют → админский список переиспользует
  `ListPublicMaintenances`, а GET одной — существующий `MaintenanceByID`. db перегенерирован (sqlc 1.31.1).
- **store** (`incidents.go`): `ListIncidents(pageID,filter,limit,offset)` (та же гидрация/фильтры, что
  ListPublicIncidents, но включает скрытые).
- **API:** `handleListIncidents`/`handleGetIncident` (incidents.go), `handleListMaintenances`/
  `handleGetMaintenance` (maintenances.go) под requireAuth, авторизация `authorizePage`. Извлечены общие
  хелперы `parseIncidentFilter`/`parseMaintenanceStatusFilter` (public_history.go) — публичные хендлеры
  отрефакторены на них. Роуты `GET /incidents`, `GET /incidents/{id}`, `GET /maintenances`,
  `GET /maintenances/{id}` в server.go (рядом с POST/PATCH). Список переиспользует
  `incidentListResponse`/`maintenanceListResponse`/`parsePagination`.
- **Фронт (admin):** `entities/incident` listIncidents(statusPageId,...) + getIncident(id) — теперь по
  id, без slug; `entities/maintenance` listMaintenances(statusPageId,...) + новый getMaintenance(id).
  Страницы incidents/maintenances (list+detail) переведены на эти эндпоинты — **detail работы больше не
  ищет в списке** (прямой GET), скрытые инциденты видны в админке. `npm run build` зелёный.
- **Тесты:** интеграционные расширены — incidents (скрытый: в админском списке/фильтре/GET есть, в
  публичном нет → 404; изоляция оператора 404; 401), maintenances (админский список+фильтр+GET; изоляция;
  401). **PASS на PG16** (api+store). go build/vet/gofmt/golangci-lint зелёные. e2e-смоук на живом
  api+PG PASS (скрытый в админ-списке, public 404, изоляция, 401).

**Этап 2.9 — админка инцидентов/работ/шаблонов + «применить шаблон» (написано, build зелёный, e2e-смоук на живом стеке PASS, ждёт коммита):**
- **Контракт НЕ менялся.** Только `frontend/admin` (FSD), типы из `@api-types` (уже сгенерированы в 2.5–2.7).
- **shared:** `api/types.ts` — псевдонимы Incident*/Maintenance*/IncidentTemplate*/Pagination + enum-массивы
  `INCIDENT_STATUSES`/`INCIDENT_IMPACTS`/`MAINTENANCE_STATUSES`. `lib/incident.ts` — RU-подписи статусов/impact,
  `impactColor`, `formatDateTime`, хелперы `isoToInput`/`inputToIso` для `<input datetime-local>`.
- **entities:** `incident` (listIncidents(slug,filter)→IncidentList; getIncident(slug,id); create/patch/delete/
  addUpdate — плоские роуты), `maintenance` (listMaintenances(slug,filter); create/patch/delete/addUpdate;
  **GET одной работы в контракте нет** — detail ищет в списке), `incidentTemplate` (CRUD, `?status_page_id=`).
- **features:** `incident-create` (форма + **UI «применить шаблон»**: Select шаблонов префиллит title/body/
  impact/components, POST /incidents — apply на клиенте, как решено в 2.7); `incident-update`
  (**смена статуса инцидента идёт через ленту** POST /updates, не PATCH); `incident-edit` (title/impact/
  is_visible/постмортем[только resolved → иначе disabled]/components); `maintenance-create`;
  `maintenance-edit` (включая lifecycle-статус scheduled→in_progress→completed); `maintenance-update`
  (заметка без статуса); `template-form` (create+edit одной формой).
- **widgets:** `affected-components` (выбор компонентов + статус-в-инциденте → IncidentComponent[];
  переиспользуется инцидентом и шаблоном; дефолт статус major_outage), `component-checklist` (плоский
  выбор component_ids для работ), `page-nav` (вкладки Компоненты/Инциденты/Работы/Шаблоны).
- **pages:** `incidents` (IncidentsListPage: фильтры status/impact/component + пагинация + create;
  IncidentDetailPage: лента-таймлайн новые-сверху + add-update + edit + delete), `maintenances`
  (List: фильтр status + пагинация + create; Detail: таймлайн + заметка + edit + delete),
  `templates` (список + create/edit/delete inline).
- **app/App.tsx:** роуты `/pages/:id/{incidents,incidents/:incidentId,maintenances,maintenances/:maintenanceId,
  templates}`. `PageNav` добавлен и в PageDetailPage. CSS: `.hp-tabs/.hp-tab`, `.hp-badge`, `.hp-filters`,
  `.hp-timeline`.
- **Листинг через публичные эндпоинты** (админских нет) → флаг выше про скрытые инциденты.
- **e2e-смоук** (live api+PG, теми же payload'ами, что шлёт админка): register→page→2 компонента→шаблон(+list)→
  инцидент(+авто major_outage компонента)→update→patch→resolve+постмортем→list+пагинация+detail→работы→
  in_progress(+авто under_maintenance)→заметка→list+find-by-id. Всё PASS. `npm run build` зелёный.

**Этап 2.8 — публичная история инцидентов/работ + наполнение сводки (написано и проверено на PG, ждёт коммита):**
- **Контракт НЕ менялся** — эндпоинты `/pages/{slug}/incidents`, `/incidents/{id}`, `/maintenances`
  (фильтры, пагинация, IncidentList/MaintenanceList, Pagination) уже были в openapi. Регенерация типов
  не нужна.
- **sqlc** (incidents.sql/maintenances.sql): `ListPublicIncidents`+`CountPublicIncidents` (фильтры
  status/impact/component_id через `sqlc.narg(...)::type IS NULL OR ...`, пагинация LIMIT/OFFSET; nullable
  uuid → `*uuid.UUID`, nullable enum → `db.Null*`), `ListActivePublicIncidents` (не resolved, видимые),
  `ListPublicMaintenances`+`Count*` (фильтр status), `ListActivePublicMaintenances` (не completed).
- **store:** `IncidentFilter{Status,Impact,ComponentID}`; `ListPublicIncidents(pageID,filter,limit,offset)
  → ([]Incident, total, err)`, `ListActiveIncidents`; `ListPublicMaintenances(pageID,*status,limit,offset)`,
  `ListActiveMaintenances`. Гидрация агрегата вынесена в `hydrateIncident`/`hydrateMaintenance`
  (IncidentByID/MaintenanceByID теперь зовут их). ⚠️ N+1 на гидрации списков (компоненты+лента на каждую
  запись) — допустимо для MVP-объёмов (per_page≤100), как и в ListIncidentTemplates; при росте — батч-запрос
  `WHERE id = ANY(...)`.
- **API** (`public_history.go`, без авторизации): `handlePublicIncidents` (валидирует enum-фильтры → 422,
  пагинация), `handlePublicIncidentDetail` (скрытый/удалённый/с чужой страницы/битый uuid → 404),
  `handlePublicMaintenances`. Хелпер `parsePagination` (page≥1 деф.1; per_page 1..100 деф.20) +
  `atoiDefault`. DTO списков `incidentListResponse`/`maintenanceListResponse` ({items, pagination}).
  Переиспользуют `toIncidentResponse`/`toMaintenanceResponse` из 2.5/2.6.
- **Публичная сводка** (`public.go`): `active_incidents`/`active_maintenances` больше НЕ `[]` — наполнены
  через `ListActiveIncidents`/`ListActiveMaintenances`; поля стали типизированными
  (`[]incidentResponse`/`[]maintenanceResponse`). **Решение (агентское, не контракт):** active_maintenances
  = scheduled + in_progress (не completed) — на странице показываются и идущие, и предстоящие работы.
- **Роуты** в server.go рядом с summary/components (публичная группа). **Интеграционный тест**
  `public_history_integration_test.go`: история (скрытый исключён) → фильтры impact/status/component_id →
  пагинация → 422 → detail (видимый ок, скрытый/битый 404) → работы (список+фильтр) → сводка
  (active_incidents=1, active_maintenances=2). **PASS на PG16.** Build/test/vet/gofmt/lint зелёные.

**Этап 2.7 — шаблоны инцидентов (написано и проверено на живом PG, ждёт коммита):**
- **Решения человека (контракт):** (1) роуты **плоские** + `status_page_id` (как 2.5/2.6), не вложенные
  в /pages; (2) `default_components` — **пары `{component_id, component_status_in_incident}`** (переиспользуем
  схему `IncidentComponent`); (3) объём — **только CRUD**, применение шаблона (префилл формы → POST
  /incidents) на клиенте (админка 2.9); серверный apply-эндпоинт — возможно на этапе 5 при спросе.
- **Контракт расширен:** схемы `IncidentTemplate`/`IncidentTemplateCreate`/`IncidentTemplatePatch`,
  пути GET(list)/POST `/incident-templates`, GET/PATCH/DELETE `/incident-templates/{id}` (тег Incidents,
  под global security). Типы перегенерированы (TS+Go). openapi провалиден (gen прошёл).
- **Миграция** `00007_incident_templates.sql` (БД→**7**): `incident_templates` (name, title_tmpl,
  body_tmpl, default_impact `incident_impact` DEFAULT none) + `incident_template_components`
  (зеркало incident_components). **Hard-delete** (без deleted_at) — шаблон это операторская
  конфигурация без публичной истории (в модели DESIGN §5 у IncidentTemplate нет deleted_at). FK
  CASCADE на страницу/компонент, unique компонент-в-шаблоне, триггеры updated_at. Проверено up/status/
  down/up на PG16.
- **Домен** `internal/domain/incident_template.go`: сущность `IncidentTemplate` (+`DefaultComponents
  []IncidentComponent` — переиспользование), метод `Validate()` (непустое имя + валидный impact).
  Добавлена ошибка `ErrInvalidIncidentImpact` в incident.go. Юнит-тест `incident_template_test.go`.
- **sqlc** `queries/incident_templates.sql` → регенерация db (sqlc 1.31.1 brew).
- **store** `internal/store/incident_templates.go`: Create/ByID(агрегат)/List(по странице)/Update
  (replaceComponents)/Delete — транзакции, **без рекомпьюта** (шаблон не навязывает статус). Delete
  грузит строку для ErrNotFound (hard delete).
- **API** `internal/api/incident_templates.go`: POST/GET-list/GET-one/PATCH/DELETE под `requireAuth`,
  авторизация `authorizePage`, валидация default_impact/имени, **переиспользует `parseIncidentComponents`**
  (принадлежность компонентов странице). Хелпер `loadAuthorizedTemplate`. Список требует
  `?status_page_id` (как /components). Роуты в server.go.
- **Интеграционный тест** `incident_templates_integration_test.go` (skip без HEALTHPAGE_TEST_DB):
  создание (компонент НЕ меняет статус) → list → get → patch (impact + замена компонентов на пустой) →
  422 (impact/имя) → изоляция операторов (404) → delete/повторный 404 → 401. **PASS на PG16.**
  Build/test/vet/gofmt/golangci-lint + admin build зелёные.

**Этап 2.6 — store + API плановых работ (написано и проверено на живом PG, ждёт коммита):**
- **Контракт:** в `MaintenanceCreate` добавлен обязательный `status_page_id` (uuid) — решение
  человека (зеркалит 2.5). Типы перегенерированы (TS+Go). Изменение аддитивно; админка работ (2.9)
  ещё не существует, `npm run build` admin зелёный.
- **sqlc:** `internal/store/queries/maintenances.sql` → перегенерирован `internal/store/db`
  (sqlc 1.31.1 brew, как в 2.5).
- **store** (`internal/store/maintenances.go`): `CreateMaintenance` (работы + maintenance_components +
  рекомпьют), `MaintenanceByID` (агрегат: строка + component_ids + лента), `UpdateMaintenance(m,
  replaceComponents)` (affected = старые∪новые; смена статуса через `ApplyStatusChange` уже применена
  вызывающим — store просто персистит StartedAt/CompletedAt и рекомпьютит), `AddMaintenanceUpdate`
  (заметка БЕЗ статуса — смена статуса идёт через PATCH, не через updates; рекомпьют не нужен),
  `SoftDeleteMaintenance` (рекомпьют — работы перестают навязывать under_maintenance). Всё в транзакциях.
- **recompute расширен (закрыт TODO 2.5):** `recomputeComponentStatusesTx` (в `store/incidents.go`) —
  теперь общий хелпер: читает И активные инциденты (`ListActiveIncidentComponentStatuses`), И активные
  in_progress работы (`ListActiveMaintenanceComponentIDs`), собирает в агрегаты и зовёт
  `domain.DerivedComponentStatus(cid, []Incident{...}, []Maintenance{...})`. Источник истории:
  under_maintenance → `source=maintenance`, прочее (вкл. возврат в operational) → `source=incident`.
- **API** (`internal/api/maintenances.go`): `POST /maintenances`, `PATCH /maintenances/{id}` (title/
  description/status[lifecycle]/scheduled_*/component_ids), `DELETE /maintenances/{id}`,
  `POST /maintenances/{id}/updates`. Все под `requireAuth`, авторизация `authorizePage`. Валидация:
  `ValidateSchedule` (end>start) на create и при смене окна в patch, enum status, принадлежность
  компонентов странице, RFC3339 (хелпер `parseRFC3339`). DTO синхронны с openapi (description как
  `*string`: пустая строка БД → null в ответе).
- **Интеграционный тест** `maintenances_integration_test.go` (skip без HEALTHPAGE_TEST_DB): создание
  scheduled (компонент остаётся operational) → in_progress (компонент under_maintenance, публичная
  сводка under_maintenance) → заметка-update (201, без статуса) → completed (возврат в operational) →
  невалидный статус 422 → окно end≤start 422 → удаление in_progress (возврат компонента) → повторный
  delete 404 → изоляция операторов (404) → 401. **PASS на реальном PG16.** Build/test/vet/gofmt/lint зелёные.

**Этап 2.5 — store + API инцидентов (написано и проверено на живом PG, ждёт коммита):**
- **Контракт:** в `IncidentCreate` добавлен обязательный `status_page_id` (uuid) — решение человека
  (зеркалит 1.5 для компонентов; флэт-роуты создания, авторизация по владению страницей). Типы
  перегенерированы (TS+Go). MaintenanceCreate пока НЕ трогал — это 2.6.
- **sqlc:** новый `internal/store/queries/incidents.sql` → перегенерирован `internal/store/db`.
  ⚠️ sqlc локально поставлен через brew — **версия 1.31.1** (в заголовках сгенерированных файлов
  было 1.27.0; CI-скрипт `gen-sqlc.sh` пинит `go run ...@v1.27.0`, который на macOS не собирается из-за
  cgo/pg_query). Функциональный код идентичен, изменилась только строка версии в заголовках. Если
  важна консистентность — перегенерировать на CI (linux) вернёт 1.27.0.
- **store** (`internal/store/incidents.go`): `CreateIncident` (incident + стартовое обновление +
  incident_components + рекомпьют), `IncidentByID` (агрегат: строка+компоненты+лента),
  `UpdateIncident(inc, replaceComponents)` (affected = старые∪новые компоненты), `AddIncidentUpdate`
  (грузит инцидент → `domain.ApplyStatusChange` → пишет update + новый current_status/resolved_at →
  рекомпьют), `SoftDeleteIncident` (рекомпьют, инцидент перестаёт быть активным). Всё в транзакциях.
- **Авто-деривация** `recomputeComponentStatusesTx(tx, pageID, componentIDs)`: грузит активные
  incident_components страницы (`ListActiveIncidentComponentStatuses`), зовёт
  `domain.DerivedComponentStatus`, и при изменении пишет статус через `changeComponentStatusTx`
  (рефакторинг: `ChangeComponentStatus` теперь обёртка над tx-хелпером) с source=incident. Если
  активных инцидентов на компонент нет → возврат в operational.
  ✅ **Закрыто в 2.6:** recompute стал общим хелпером и учитывает также активные in_progress работы
  (авто-under_maintenance). См. секцию 2.6 выше.
- **Известное упрощение (флаг человеку):** при возврате компонента из инцидента ставится
  `operational`, ручной «базовый» статус не сохраняется (нет колонки baseline). DESIGN §3.3
  «если оператор не указал иное» в полном виде требовал бы baseline — отложено (возможная схема позже).
- **API** (`internal/api/incidents.go`): `POST /incidents`, `PATCH /incidents/{id}` (title/impact/
  postmortem/is_visible/components; постмортем только для resolved → иначе 422), `DELETE /incidents/{id}`,
  `POST /incidents/{id}/updates` (смена статуса + запись в ленту). Все под `requireAuth`, авторизация
  `authorizePage`. Валидация: enum'ы status/impact/component_status, принадлежность каждого компонента
  странице (изоляция), started_at RFC3339. DTO синхронны с openapi.
- **Интеграционный тест** `incidents_integration_test.go` (skip без HEALTHPAGE_TEST_DB): создание →
  авто-major_outage компонента → публичная сводка → update identified → resolve → возврат компонента в
  operational → постмортем (ок после resolved; 422 до) → невалидный статус 422 → изоляция операторов
  (404) → повторный delete 404 → 401. **PASS на реальном PG16.** Build/test/vet/gofmt/golangci-lint зелёные.

**Этап 2.4 — авто-производный статус компонентов (написано и проверено, ждёт коммита):**
- `backend/internal/domain/derive.go` — чистая функция деривации, переиспользует `WorstStatus`
  (§6) и `Maintenance.ImposedComponentStatus()`.
- `DerivedComponentStatus(componentID, incidents []Incident, maintenances []Maintenance)
  → (status ComponentStatus, driven bool)`:
  - худший по приоритету показа (§6) среди: статусов активных инцидентов на этом компоненте
    (`IncidentComponent.ComponentStatusInIncident`, только `Incident.IsActive`) и
    `under_maintenance` от активных (in_progress) работ, затрагивающих компонент;
  - soft-deleted инциденты/работы, resolved-инциденты и не-in_progress работы — игнорируются;
  - нет активных → `(operational, false)`. **Флаг `driven`**: true → авто-статус навязан
    (store пишет историю с source=incident/maintenance); false → авто-статус снят, компонент
    возвращается в operational / остаётся под ручным управлением («если оператор не указал иное», §3.3).
- Юнит-тесты `derive_test.go`: нет активных; активный инцидент; resolved/deleted игнор; активная/
  scheduled работа; приоритет major>maintenance>degraded между источниками; worst среди инцидентов;
  незатронутый компонент. Build/vet/test/gofmt/lint — зелёные. Контракт не трогал.
- **Применение функции** (запись в component_status_history, авто-апдейт current_status при
  create/update/resolve) — задача 2.5/2.6 (store/service), здесь только чистая логика.

**Этап 2.3 — доменный слой плановых работ (написано и проверено, ждёт коммита):**
- `backend/internal/domain/maintenance.go` — чистый домен, по образцу incident.go.
- Enum `MaintenanceStatus` (scheduled/in_progress/completed) + `IsValid`/`AllMaintenanceStatuses`.
- Сущности: `Maintenance` (+`ComponentIDs []uuid.UUID` и `Updates []MaintenanceUpdate` —
  заполняются store; component_ids 1:1 с openapi), `MaintenanceUpdate` (без статуса, в отличие от
  инцидента — как в openapi/миграции).
- Предикаты `IsScheduled`/`IsInProgress`/`IsCompleted`, `IsActive`(==in_progress).
- `ImposedComponentStatus()` → (under_maintenance, true) только во время in_progress, иначе
  ("",false). Это доменная основа авто-перевода компонентов; фактическое применение (запись в
  историю, объединение с инцидентами) — задача 2.4.
- `ApplyStatusChange(status, at)`: in_progress фиксирует StartedAt (если nil) и сбрасывает
  CompletedAt; completed фиксирует CompletedAt (если nil), StartedAt не трогает; scheduled
  сбрасывает обе метки. Переходы между валидными статусами не ограничены; невалидный →
  `ErrInvalidMaintenanceStatus` без мутации.
- `ValidateSchedule()` → `ErrInvalidSchedule`, если scheduled_end не строго позже scheduled_start
  (для api на create/patch).
- Юнит-тесты `maintenance_test.go` (enum, предикаты, imposed-статус, валидация окна, полный
  жизненный цикл, повторный запуск/сброс, невалидный переход). Build/vet/test/gofmt/lint — зелёные.
- Контракт/openapi не трогал.

**Этап 2.2 — доменный слой инцидентов (написано и проверено, ждёт коммита):**
- `backend/internal/domain/incident.go` — чистый домен (без БД/HTTP), по образцу status.go/entities.go.
- Enum'ы (нормативны, как в openapi): `IncidentStatus`
  (investigating/identified/monitoring/resolved) + `IsValid`/`IsTerminal`+`AllIncidentStatuses`;
  `IncidentImpact` (none/minor/major/critical) + `IsValid`+`AllIncidentImpacts`, порядок тяжести
  `impactSeverity` (none0<minor1<major2<critical3) и `WorstImpact` (для агрегации, пустой→none).
- Сущности: `Incident` (+связи `Components []IncidentComponent`, `Updates []IncidentUpdate`,
  заполняются store при чтении агрегата), `IncidentComponent`, `IncidentUpdate` — поля 1:1 с
  таблицами 2.1 и схемами openapi. `IsResolved`/`IsActive`.
- Жизненный цикл `(*Incident).ApplyStatusChange(status, at)`: переходы между валидными статусами
  НЕ ограничены (оператор может вернуться назад / повторно открыть); инвариант ResolvedAt —
  resolve фиксирует время (повторный resolve не сдвигает), переход из resolved в активный сбрасывает.
  Невалидный статус → `ErrInvalidIncidentStatus`, без мутации.
- Постмортем: `CanSetPostmortem`/`SetPostmortem` — только после resolved (DESIGN §3.3), иначе
  `ErrPostmortemBeforeResolved`; пустая строка снимает постмортем.
- Юнит-тесты `incident_test.go` (валидность enum'ов, WorstImpact, resolve/повторный resolve/
  reopen, невалидный переход, постмортем до/после resolved). Build/vet/test/gofmt/golangci-lint — зелёные.
- Контракт/openapi не трогал. [ТРЕБОВАНИЕ §3.3] про авто-возврат компонентов в operational при
  resolve и авто-производный статус — это задача 2.4 (отдельный слой деривации), здесь не реализуется.

**Этап 2.1 — миграции инцидентов/работ (написано и проверено, ждёт коммита):**
- `backend/migrations/00006_incidents_maintenances.sql`. Версия БД → 6.
- 3 pg-enum'а (нормативны, как `component_status`): `incident_status`
  (investigating/identified/monitoring/resolved), `incident_impact` (none/minor/major/critical),
  `maintenance_status` (scheduled/in_progress/completed).
- 6 таблиц (имена plural): `incidents`, `incident_components`, `incident_updates`,
  `maintenances`, `maintenance_components`, `maintenance_updates`.
  - `incidents`/`maintenances` — soft-delete (`deleted_at`), как пользовательский контент.
  - FK на `status_pages`/`components` — `ON DELETE CASCADE`. Удаление страницы/компонента чистит
    связанные инциденты/работы и их связи.
  - unique компонент-в-инциденте (`incident_components_unique`) и компонент-в-работе
    (`maintenance_components_unique`).
  - `maintenances.status` DEFAULT `scheduled`; `incidents.is_visible` DEFAULT true;
    `current_status`/`impact` инцидента — NOT NULL без дефолта (всегда задаются при создании).
  - created_at/updated_at + триггер `set_updated_at()` на всех 6 таблицах (DESIGN §5).
- Проверено на живом PG16: up→v6, status, down (таблицы и enum'ы исчезают без остатка), up снова
  чисто; FK-каскады (компонент→incident/maintenance_components; инцидент→updates; страница→всё),
  unique, дефолты, срабатывание updated_at-триггера.
- Контракт/openapi/доменный слой не трогал — это только миграция. Домен — задача 2.2/2.3.

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
- 2026-06-28 — Этап 2.1 (миграции инцидентов/работ): `00006_incidents_maintenances.sql` —
  3 нормативных enum'а + 6 таблиц (soft-delete у incidents/maintenances, FK CASCADE, unique
  компонент-в-инциденте/работе, триггеры updated_at). Проверено на PG16 (up/status/down/up,
  каскады, unique, дефолты, trigger). Дальше — 2.2 (доменный слой инцидентов).
- 2026-06-28 — Этап 2.2 (доменный слой инцидентов): `internal/domain/incident.go` — enum'ы
  IncidentStatus/IncidentImpact (IsValid/IsTerminal/WorstImpact), сущности Incident/
  IncidentComponent/IncidentUpdate, жизненный цикл ApplyStatusChange (ResolvedAt при resolve/
  reopen) + постмортем только после resolved; юнит-тесты. Build/test/lint зелёные. Контракт не
  трогал. Дальше — 2.3 (домен плановых работ), затем 2.4 (авто-производный статус компонентов).
- 2026-06-28 — Этап 2.3 (доменный слой плановых работ): `internal/domain/maintenance.go` — enum
  MaintenanceStatus, сущности Maintenance/MaintenanceUpdate, предикаты стадий, ImposedComponentStatus
  (under_maintenance в in_progress), ApplyStatusChange (StartedAt/CompletedAt), ValidateSchedule;
  юнит-тесты. Build/test/lint зелёные. Контракт не трогал. Дальше — 2.4 (деривация статуса).
- 2026-06-28 — Этап 2.4 (авто-производный статус компонентов): `internal/domain/derive.go` —
  `DerivedComponentStatus` (худший §6 среди активных инцидентов + under_maintenance активных работ;
  driven-флаг для возврата в operational/ручное управление); юнит-тесты. Build/test/lint зелёные.
  Доменный слой этапа 2 (2.2–2.4) завершён. Дальше — 2.5 (store + API инцидентов).
- 2026-06-28 — Этап 2.5 (store + API инцидентов): sqlc-запросы incidents/*, store с транзакциями и
  авто-деривацией статуса компонентов (recomputeComponentStatusesTx → domain.DerivedComponentStatus,
  source=incident), API POST/PATCH/DELETE /incidents + /incidents/{id}/updates под requireAuth.
  Контракт: +status_page_id в IncidentCreate (решение человека), типы перегенерированы. sqlc bump
  1.27.0→1.31.1 (brew). Интеграционный тест на реальном PG16 PASS. Дальше — 2.6 (store + API работ;
  обязательно расширить recompute активными работами).
- 2026-06-28 — Этап 2.6 (store + API плановых работ): sqlc-запросы maintenances/*, store
  (`maintenances.go`) с транзакциями; **recompute расширен** до общего хелпера (активные инциденты +
  активные in_progress работы → авто-under_maintenance, source=maintenance). API POST/PATCH/DELETE
  /maintenances + /maintenances/{id}/updates под requireAuth (смена статуса через PATCH, заметки через
  updates без статуса). Контракт: +status_page_id в MaintenanceCreate (решение человека), типы
  перегенерированы. Интеграционный тест на реальном PG16 PASS (scheduled→in_progress→under_maintenance→
  completed→operational→delete-возврат→изоляция→401). Дальше — 2.7 (шаблоны инцидентов; возможна
  развилка по openapi) или 2.8 (публичная история + наполнение active_maintenances).
- 2026-06-29 — Этап 2.7 (шаблоны инцидентов): развилка контракта решена человеком (плоские роуты +
  status_page_id; default_components как пары {component_id,status}; только CRUD, apply на клиенте).
  Контракт расширен (IncidentTemplate/Create/Patch + /incident-templates*), типы перегенерированы.
  Миграция 00007 (БД→7, hard-delete), домен+Validate (+ErrInvalidIncidentImpact), sqlc/store/API,
  переиспользование IncidentComponent и parseIncidentComponents. Юнит + интеграционный тест на PG16
  PASS. Дальше — 2.8 (публичная история + наполнение active_incidents/active_maintenances).
- 2026-06-29 — Этап 2.8 (публичная история + сводка): контракт не менялся; sqlc-запросы публичных
  списков (фильтры через narg, пагинация) + активных для сводки; store ListPublic*/ListActive*
  (+ гидрация вынесена в hydrateIncident/hydrateMaintenance); API public_history.go (история инцидентов
  с фильтрами status/impact/component_id + пагинация, detail со скрытием невидимых→404, список работ);
  публичная сводка наполнена active_incidents (не resolved)/active_maintenances (не completed).
  Интеграционный тест на PG16 PASS. Дальше — 2.9 (админка инцидентов/работ/шаблонов + «применить шаблон»).
- 2026-06-29 — Этап 2.9 (админка инцидентов/работ/шаблонов): FSD entities incident/maintenance/
  incidentTemplate, features (incident create[+«применить шаблон»]/update[смена статуса через ленту]/edit,
  maintenance create/edit[lifecycle]/update, template-form), widgets affected-components/component-checklist/
  page-nav, pages incidents(list+фильтры+detail-таймлайн)/maintenances/templates, роуты. Контракт НЕ менялся;
  листинг через публичные эндпоинты. ⚠️ Флаг: скрытые инциденты недоступны оператору (нужен админский
  read-API — контракт/человек). build зелёный, e2e-смоук на живом стеке PASS. Дальше — 2.10 (публичный SSR
  вкладки Инциденты/Работы + detail).
- 2026-06-29 — Админские read-эндпоинты инцидентов/работ (по запросу человека, контракт расширен):
  `GET /incidents`(+фильтры/пагинация, включая скрытые)/`/incidents/{id}`, `GET /maintenances`/
  `/maintenances/{id}`. sqlc ListIncidents/CountIncidents (без is_visible), store.ListIncidents, API-хендлеры
  + общие хелперы фильтров (рефактор публичных), роуты. Фронт-админка переведена на новые эндпоинты (по id,
  без slug; detail работы — прямой GET). Закрыт флаг 2.9 (скрытые инциденты теперь доступны оператору).
  Интеграционные тесты расширены, PASS на PG16; build/vet/lint/admin-build зелёные; e2e-смоук PASS.
  Дальше — 2.10 (публичный SSR вкладки Инциденты/Работы + detail).
- 2026-06-29 — Этап 2.10 (публичный SSR): вкладки Статус/Инциденты/Работы (`StatusTabs`), страницы
  `/status/[slug]/incidents` (список+пагинация), `/incidents/[id]` (detail: лента новые-сверху, постмортем,
  затронутые компоненты по именам), `/maintenances` (карточки с лентой inline); обзор дополнен активными
  инцидентами/работами. lib/api (типы+fetch'еры+componentNameMap), i18n RU/EN+withLang, lib/badge, CSS.
  Контракт не менялся (эндпоинты 2.8). `next build` зелёный, e2e на живом стеке PASS (RU/EN, постмортем,
  404). **Этап 2 закрыт по коду.** Дальше — этап 3 (подписки/уведомления), начать с 3.1 (миграции
  Subscriber+Notification).
- 2026-06-29 — Этап 3.1 (миграции подписок): `00008_subscribers_notifications.sql` (БД→8) — `subscribers`
  (channel/scope TEXT+CHECK, component_ids uuid[], confirm/unsubscribe-токены, без soft-delete) +
  `notifications` (event_type/payload/status/attempts — журнал идемпотентности §8.1). FK CASCADE, unique
  (page,channel,address), partial-unique по токенам, индексы, триггеры updated_at. Проверено на PG16
  (up/схема/CHECK/unique/cascade/trigger/down→up). Контракт не менялся. Дальше — 3.2 (RabbitMQ-топология;
  ⚠️ нужен delayed-exchange плагин в образе).
