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
**Фаза:** Этапы 0–2 закоммичены человеком; Этап 1 закрыт по коду. **Этап 3 (Подписки и уведомления)
ЗАКРЫТ ПО КОДУ** (3.1–3.7, 3.9, 3.10 готовы, ждут коммита). **3.8 (MAX) ОТЛОЖЕН** (после Этапа 7).
**Этап 4 (Кастомизация и white-label) — ЗАКРЫТ ПО КОДУ** (4.1–4.6 + 4.2.1, все ждут коммита; детали
ниже в «Что в работе»). 4.1 тема/тёмный режим/логотип/favicon/таймзона; 4.2 приватные по паролю +
noindex; 4.2.1 приватные по списку email + magic-link; 4.3.1 управление доменом + CNAME; 4.3.2
ACME-сервис `cmd/tls-manager` (lego); 4.3.3 edge-прокси `cmd/edge`; 4.4 white-label; 4.5 custom SMTP;
4.6 виджет-бейдж. **Этап 5 (API и интеграции) — ЗАКРЫТ ПО КОДУ** (5.1–5.4, ждут коммита; детали в
«Что в работе»). 5.1 ApiToken+аутентификация; 5.2 полный write-API под page-токеном (status_page_id
опционален при ApiToken); 5.3 входящие webhook'и **grafana+prometheus** (generic/pagerduty отложены →
501; CRUD `/webhook-integrations` + HMAC + идемпотентность по dedup-ключу); 5.4 исходящие webhook'и
(channel=webhook → q.webhook.out → worker-webhook POST на URL, Mattermost-совместимый payload).
**Этап 6 (Биллинг и тарифы) — ЗАКРЫТ ПО КОДУ** (6.1–6.9, ждут коммита; детали в «Что в работе»).
Контракт `/billing/*` уже был в openapi — НЕ менялся. Решения человека (2026-07-01): полный объём;
реальные ЮKassa-ключи/боевые списания/чеки — `[ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ]` (код против sandbox/стаба);
цены — плейсхолдер конфигом (990 ₽/мес, годовая −20%, триал 14 дней). Добавлен `worker-billing`.
**Этап 7 (Зрелость и миграция) — ПОЧТИ ЗАКРЫТ ПО КОДУ** (7.1/7.2/7.3/7.5/7.6/7.9 готовы, ждут коммита;
детали в «Что в работе»). 7.1 uptime (домен+API+public-ssr полоса); 7.2 changelog (контракт расширен;
admin+public-ssr «Релизы»); 7.3 **только `/metrics`** (Prometheus, решение человека); 7.5 каркас импорта
(ImportJob/external_id_map/Importer/q.import/worker-import); 7.6 адаптер **StatusPal** (только он —
решение человека); 7.9 UI импорта. Добавлен `worker-import`, зависимость `prometheus/client_golang`.
**Отложено (решения человека):** 7.4 бэкапы + Grafana/вывод статуса/собственная статус-страница — прод;
7.7 Instatus / 7.8 Статусмейт — позже; inbound generic/pagerduty webhook'и (501, этап 5); боевая ЮKassa.
**Деплой/CI-CD (решения человека, 2026-07-01):** self-hosted **Dokploy** на РФ-VPS (`201.51.12.191`);
отдельные приложения + managed Postgres/Redis Dokploy (бэкапы → 7.4); RabbitMQ — приложение; образы
CI→GHCR, Dokploy тянет по вебхуку; ingress/TLS — Traefik. Полный runbook — **`DEPLOY.md`** (compose+
Caddy+SSH — Приложение B, ручная альтернатива). CD: `.github/workflows/deploy.yml` (build+push GHCR +
вебхуки). Провайдер РФ — [РЕШИТЬ]; сервер у человека есть.
**Прод развёрнут и работает (2026-07-21/22):** все сервисы, кроме `worker-telegram` (нет
`TELEGRAM_BOT_TOKEN`, оставлен недеплоенным), задеплоены в Dokploy и подтверждены живыми логами:
`migrate` (схема на версии 15), `queue-setup` (топология RabbitMQ объявлена), `api`, `worker-email`
(dev-режим — SMTP не задан, письма логируются), `worker-webhook`, `worker-billing` (dev-режим — ключи
ЮKassa не заданы, stub-провайдер), `worker-import`, `public-ssr`, `admin`. Домены с HTTPS/Let's Encrypt
живые: `healthpage.ru`+`www` (public-ssr), `app.healthpage.ru` (admin + `/api`→api), `api.healthpage.ru`
(api). **Демо-страница:** лендинг ссылается на `/status/demo` — создана вручную через API (аккаунт
`demo@healthpage.ru` + status page slug `demo` + 3 компонента); готового seed-скрипта в репозитории
нет, при пересоздании БД демо-страницу нужно создавать заново тем же способом (`POST /auth/register`
→ `POST /pages` со slug `demo` → `POST /components` ×N). Пароль демо-аккаунта не хранится в репо —
только у человека.
**Следующий шаг (код):** коммит человеком; затем прод-подготовка (стоп-маркеры ниже) ИЛИ 7.7/7.8/бэкапы по приоритету.
**2026-07-22 — `worker-telegram` задеплоен и живой** (человек дал `TELEGRAM_BOT_TOKEN`, задеплоили через
Dokploy): бот `@healtpage_ru_bot` авторизован, `q.telegram` потребляется, long-poll `getUpdates` запущен.
Webhook-URL добавлен в секрет `DOKPLOY_WEBHOOKS` на GitHub (человек внёс сам) — теперь редеплоится
вместе с остальными 7 по пушу. **⚠️ Найдено при проверке (2026-07-22): на публичной статус-странице
(`frontend/public-ssr`) НЕТ НИКАКОГО self-serve виджета подписки** — ни email-формы (double opt-in
backend `POST /pages/{slug}/subscribe` есть и работает, но нигде не вызывается из UI), ни кнопки
Telegram (deep-link `https://t.me/<bot>?start=<slug>` — бот его умеет обрабатывать, `internal/telegram/
bot.go:111`, но ссылки нигде нет), ни «Add to Slack» (`GET /pages/{slug}/subscribe/slack/start` тоже
без UI-входа). Единственный способ подписать кого-то сейчас — вручную через админку (этап 3.10).
Проверено grep'ом по всему `frontend/public-ssr/app` — ни одного упоминания subscribe/telegram/slack/
rss/ical в JSX. **Это отдельная задача (виджет подписки на публичной странице), не багфикс** — решение
человека: делать после SMTP-инфраструктуры (см. ниже), приоритет подтверждён 2026-07-22.
- 2026-07-22 — **Настроен SMTP через UniSender Go, найден и исправлен реальный прод-баг: письма
  зависали навечно.** Человек дал SMTP-реквизиты UniSender Go (`smtp.go2.unisender.ru:587`, логин
  `8285366`, `SMTP_FROM=status@healthpage.ru`); прописали в `worker-email` через Dokploy, задеплоили —
  подхватил (пропала строка "SMTP_HOST не задан"). **Тестовая отправка зависла**: `rabbitmqctl
  list_queues` на проде показал `q.email messages_unacknowledged=1` — сообщение доставлено
  консьюмеру, но не ack/nack. Диагностика через Docker Terminal (`rabbitmq` в Dokploy): `nc -zv
  smtp.go2.unisender.ru 587/465` и даже `smtp.gmail.com:587` — **все таймаут**. Вывод: у
  VPS-провайдера прод-сервера исходящие SMTP-порты (25/465/587) заблокированы на уровне сети (типовая
  антиспам-политика), не проблема кода/DKIM/UniSender. `net/smtp.SendMail` при этом не имеет таймаута
  на подключение — воркер молча висит навсегда и блокирует обработку ВСЕХ следующих писем в очереди
  (единственный consumer застревает на одном сообщении).
  - **Исправлено системно:** `internal/email/sender.go` — `dialTimeout=15s`, явный
    `net.DialTimeout`/`tls.DialWithDialer` вместо `smtp.SendMail`/`tls.Dial` без таймаута (свой
    `sendStartTLS`, `sendImplicitTLS` тоже поправлен). Недоступность SMTP теперь даёт быструю ошибку
    вместо зависания навечно — актуально и для кастомного SMTP клиентских страниц (4.5).
  - **Решение человека:** вместо борьбы с провайдером за открытие портов — системный (платформенный)
    отправитель переведён на **HTTP Web API UniSender Go**
    (`POST https://goapi.unisender.ru/ru/transactional/api/v1/email/send.json`, `X-API-KEY`,
    HTTPS:443 не блокируется). Новый `internal/email/unisender.go`: `UniSenderGoSender` (реализует
    `Sender`; `Endpoint` — инъекция для тестов). `internal/config` +`UniSenderGoAPIKey` (env
    `UNISENDER_GO_API_KEY`).
  - **Рефакторинг `internal/email/worker.go`:** `Worker` держит `systemSender`+`customSender` вместо
    одного `sender` — кастомный SMTP страницы (4.5, произвольный провайдер клиента) **всегда** идёт
    через настоящий `SMTPSender`, даже если системный — `UniSenderGoSender` (нельзя провести чужой
    почтовый сервер через свой аккаунт UniSender Go). `effectiveSMTP`/`build` возвращают доп. флаг
    `custom`; `senderFor(custom)` выбирает отправителя. `NewWorker` сигнатура расширена (systemSender,
    customSender). `cmd/worker-email`: приоритет — `UNISENDER_GO_API_KEY` → UniSender Go API; иначе
    `SMTP_HOST` → SMTPSender; иначе LogSender (dev).
  - **Проверено:** юнит `unisender_test.go` (httptest: happy path/api-error/rejected-recipient/
    no-api-key) + `worker_test.go` (`TestEffectiveSMTPCustomVsSystem`, `TestSenderForDispatchesByCustom`
    — ловит перепутанный выбор системный/кастомный отправитель) + весь существующий `internal/email`
    зелёный. build/vet/gofmt/golangci-lint + полный `go build ./...` зелёные. Контракт openapi.yaml
    НЕ менялся (внутренняя реализация доставки).
  - **Дальше:** прописать `UNISENDER_GO_API_KEY` в `worker-email` на Dokploy (ключ из UniSender Go →
    раздел Web API, отдельный от SMTP-логина/пароля) и передеплоить; повторить тестовую отправку.
- 2026-07-22 — **UniSender Go API задеплоен, поймана и исправлена вторая ошибка: неверный хост
  Web API.** Человек прописал `UNISENDER_GO_API_KEY`, закоммитил/запушил/задеплоил — таймаут-фикс
  сработал сразу (оба зависших с прошлой попытки письма быстро переобработались вместо зависания),
  но обе отправки упали с `unisender error: user not found`. Разобрался: у UniSender Go **нет одного
  общего хоста API для всех аккаунтов** — их несколько дата-центров (go1/go2/...), и дефолтный
  `goapi.unisender.ru` (как в общей документации) отвечает "user not found" (код 114) для аккаунтов
  на другом дата-центре. У этого аккаунта дата-центр **go2** (виден в его же SMTP-хосте
  `smtp.go2.unisender.ru`). **Исправлено:** дефолтный `uniSenderGoEndpoint` →
  `https://go2.unisender.ru/ru/transactional/api/v1/email/send.json`; плюс добавлен опциональный
  override `UNISENDER_GO_API_URL` (config+`NewUniSenderGoSender(apiKey, apiURL)`) на случай переезда
  аккаунта на другой дата-центр в будущем. build/test/lint зелёные.
- 2026-07-22 — **Третья и последняя находка: правильный ключ = пароль SMTP, плюс tracking-домен.**
  После фикса хоста (go2) API вернул подробную ошибку с готовым ответом: "Api key mismatch ...
  Users api key: '67m...'" — этот "правильный" ключ совпал с уже сохранённым `SMTP_PASSWORD`
  (у аккаунта единый секрет на SMTP и Web API). Человек прописал его в `UNISENDER_GO_API_KEY` —
  аутентификация прошла. Следующая ошибка: **"Custom backend domain or tracking domain required for
  sending"** — не про SPF/DKIM (человек подтвердил: домен уже верифицирован для отправки), а про
  ОТДЕЛЬНУЮ фичу — tracking-домен для отслеживания открытий/кликов, которую API включает по
  умолчанию (`track_links`/`track_read` дефолт 1) и требует настроенного CNAME для него, даже если
  сам домен отправителя уже ок. **Исправлено:** `uniSenderGoMessage` явно передаёт `track_links: 0,
  track_read: 0` — нам open/click-трекинг на служебных письмах не нужен, и это снимает зависимость
  от ещё одной DNS-настройки. Юнит-тест `TestUniSenderGoSenderHappyPath` расширен проверкой этих
  полей. build/test/vet/gofmt/golangci-lint зелёные.
- 2026-07-23 — **Четвёртая находка: нужен `custom_backend_id` явно.** После деплоя track_links-фикса
  ошибка "Custom backend domain or tracking domain required for sending" осталась той же — дело было
  не в трекинге. По документации `email/send`: `custom_backend_id` — id backend-домена, через
  который шлётся письмо; если не передан, используется дефолтный backend-домен проекта — а у этого
  проекта дефолтный backend-домен, судя по всему, не назначен (отдельная настройка от SPF/DKIM,
  которую человек уже подтвердил). Человек нашёл id в личном кабинете UniSender Go: **35417**.
  **Исправлено:** `uniSenderGoMessage.CustomBackendID` (`custom_backend_id,omitempty`);
  `UniSenderGoSender.BackendID` + `NewUniSenderGoSender(apiKey, apiURL, backendID)`; config
  `UniSenderGoBackendID` (env `UNISENDER_GO_BACKEND_ID`). Юнит-тест
  `TestUniSenderGoSenderSendsCustomBackendID`. build/test/vet/gofmt/golangci-lint зелёные.
  **Ждёт коммита + `UNISENDER_GO_BACKEND_ID=35417` в Dokploy + деплоя + финальной проверки доставки.**
**[ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ ИМПОРТА] — частично снят (2026-07-22):** схема StatusPal API v2
(`internal/importer/statuspal.go`) сверена на живом read-only ключе клиента (services/incidents/
subscribers — реальные обёртки, дерево через children_ids, UUID-подписчики, отсутствие /maintenances
исправлены). **Остаётся:** плановые работы (type=scheduled) не встретились в тестовых данных — не
подтверждены вживую, только по аналогии. 152-ФЗ — импортированные email `confirmed=false` (opt-in),
не менялось.
**[ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ БИЛЛИНГА]:** реальные ключи ЮKassa, согласование рекуррентов с менеджером,
боевые списания и фискальные чеки локально НЕ проверены; цены — плейсхолдер; оферта — черновик.
**Кастомные домены (4.3) — стоп-маркер снят (2026-07-22), решение человека:** свой edge/tls-manager
в проде на Dokploy не используется (конфликт :80/:443 с Traefik). Вместо этого `POST /pages/{id}/
domain/verify` при успешной CNAME-проверке сам вызывает Dokploy API (`domain.create`/`domain.delete`
на приложение `public-ssr`) — Traefik+Let's Encrypt обслуживает домен клиента сам Dokploy. Новый
пакет `backend/internal/dokploy` + миграция 00016 (`status_pages.dokploy_domain_id`). Требует env
`api`: `DOKPLOY_API_URL`, `DOKPLOY_API_TOKEN` (ключ из Dokploy Profile→API/CLI Keys, вписывается
человеком), `DOKPLOY_PUBLIC_SSR_APP_ID`. Без токена — фича молча выключена (домен остаётся только
verified). **Не проверено на реальном клиентском домене** (только юнит+интеграционные тесты с
фейковым Dokploy API) — проверить на тестовом поддомене перед тем, как отдавать фичу платным
клиентам. `cmd/edge`/`cmd/tls-manager` оставлены в репозитории, но не деплоятся.
**⚠️ Найдено 2026-07-22, не устранено:** `CNAME_TARGET` (дефолт `cname.healthpage.ru`) **не
резолвится** (`dig` — пусто) — нужна A-запись `cname.healthpage.ru → 201.51.12.191` у регистратора,
иначе verify отметит `domain_verified=true`, но трафик клиента физически не дойдёт до сервера.
Ручной прогон проверки — `DEPLOY.md` §8.1.
Организационно осталось: для 3.7 — `TELEGRAM_BOT_TOKEN` у @BotFather; для 3.9 — Slack App +
`SLACK_CLIENT_ID/SECRET`; для 4.3 — `DOKPLOY_API_TOKEN`/`DOKPLOY_PUBLIC_SSR_APP_ID` (выше).
**Прод:** `SUBSCRIPTION_SECRET` api ↔ воркеры должны совпадать (им же подписываются токены доступа к
приватным страницам 4.2 и magic-link 4.2.1).

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

**Этап 7 — Зрелость и миграция (написано, build+test(вкл. PG16)+lint+оба фронта зелёные, ждёт коммита):**
- **Решения человека (2026-07-01):** импорт — только StatusPal (7.7/7.8 отложены); наблюдаемость —
  только `/metrics` кодом (Grafana/вывод статуса/бэкапы — прод).
- **7.1 uptime:** контракт уже был. Домен `uptime.go` `ComputeUptime` — under_maintenance ИСКЛЮЧЁН из
  окна (Statuspage-семантика), downtime=partial/major_outage, degraded/operational=доступен; окно
  клиппируется `component.created_at`; daily по суткам UTC; round2. store `StatusHistorySince`
  (+запрос ListStatusHistorySince: ended_at IS NULL OR >= since). API `handleUptime` (public, гейт
  приватных, приватный/чужой компонент→404, days 1..365). public-ssr: `fetchUptime` + полоса
  `UptimeBar` (90 ячеек, цвет по %) на компонентах с show_uptime; i18n `uptimeOver`. Юнит+интеграция PG16.
- **7.2 changelog:** **контракт расширен с санкции человека** (ChangelogEntry/Create/Patch +
  `/changelog` CRUD + публичный `/pages/{slug}/changelog`). Миграция `00014_changelog.sql`
  (published+published_at, индекс по published_at). Домен/store/API (`publishedAtFor` — set now при
  публикации, null при снятии). Admin `entities/changelog`+`pages/changelog`+вкладка «Релизы»;
  public-ssr вкладка «Релизы» (body — текст, экранируется React; `.changelog-body` pre-wrap). Флаг:
  body без markdown-движка (безопаснее). Интеграция PG16 (черновик скрыт→публикация→snятие→изоляция→delete).
- **7.3 метрики:** `internal/metrics` (promhttp дефолтный реестр + middleware
  `healthpage_http_requests_total{method,route,code}` / `_request_duration_seconds{method,route}`,
  route=шаблон chi через RouteContext.RoutePattern после ServeHTTP). `GET /metrics` + `metrics.Middleware`
  в NewRouter. Dep `github.com/prometheus/client_golang`. Юнит-тест (счётчик+go_* коллекторы+шаблон route).
- **7.5 каркас импорта:** миграция `00015_import.sql` (`import_jobs` — api_key НЕ хранит; `external_id_map`
  unique(page,source,entity,external_id)). Домен `import.go` (ImportSource/Region/Mode/Status + IsValid;
  ImportJob; ImportReport; **Imported\*** структуры — Source\* заняты HistorySource-константами!; Importer
  интерфейс). Очередь `q.import` (topology durable, `Publisher.PublishImport` → default exchange).
  Движок `internal/importer` (`Engine.Run`: компоненты 2 прохода (parent), группы по имени, инциденты
  с хроникой, работы, подписчики **confirmed=false**; идемпотентность через external_id_map, skip/update;
  БЕЗ notify — пишет через store напрямую). `cmd/worker-import` (consume q.import, ручной ack, job→
  running→completed/failed+report). store `import.go` (CreateImportJob/UpdateImportJob/ExternalMapping/
  SetExternalMapping). API `import.go`: **контракт расширен** — `ImportRequest.status_page_id` опционален
  (цель или создать новую страницу из subdomain); POST /import (только statuspal иначе 422; api_key в
  сообщении очереди, НЕ в БД; publisher nil→503) + GET /import/{job_id} (изоляция по аккаунту). Deps
  `ImportPublisher` (интерфейс, nil→503; api main `setupImportPublisher` — своё соединение RabbitMQ).
- **7.6 StatusPal:** `statuspal.go` (base us/eu, auth-заголовок, services/incidents/maintenances/
  subscriptions→Imported*, маппинг статусов/impact). **Флаг [ВЕРНУТЬСЯ]:** схема JSON/пути API v2 по
  докам — сверить на живом ключе (прод).
- **7.9 UI импорта:** admin `entities/import`+`pages/import` (форма StatusPal + опрос статуса задачи
  каждые 2с + отчёт JSON), ссылка «Импорт» в шапке, роут `/import`.
- **Проверено:** юнит (uptime домен; metrics) + интеграционные на PG16 (uptime; changelog; движок импорта
  с fake Importer — создание/дерево/инцидент+хроника/работа/подписчик confirmed=false/идемпотентный
  повтор skip). Полный go test (вкл. PG16) + vet/gofmt/golangci-lint + admin `npm run build` + public-ssr
  `next build` зелёные. Миграции 00013–00015 применены; up обратимы. Ждёт коммита.

**Этап 6 — Биллинг и тарифы (написано, build+test(вкл. PG16)+lint+оба фронта зелёные, ждёт коммита):**
- **Контракт НЕ менялся** — `/billing/*` (subscription/checkout/cancel/payments/webhook/{provider}) +
  схемы Subscription/Payment/CheckoutRequest/CheckoutResponse + enum'ы уже были в openapi. Реализация.
- **Решения человека (2026-07-01):** полный объём 6.1–6.9; реальные ЮKassa-ключи и боевые списания —
  `[ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ]` (код против sandbox/стаба); цены — плейсхолдер конфигом.
- **Миграция** `00013_billing.sql` (БД→13): `subscriptions` (одна на аккаунт UNIQUE; plan=billing_plan
  pg-enum; status/billing_period/provider — TEXT+CHECK; provider_customer_token — токен рекуррента, НЕ
  карта; trial_ends_at/current_period_*; cancel_at_period_end; dunning_attempts; триггер updated_at) +
  `payments` (amount_minor bigint=копейки; status/provider TEXT+CHECK; **partial-unique provider_payment_id**
  для идемпотентности webhook'а + partial-unique idempotency_key; receipt_id; FK CASCADE на аккаунт,
  SET NULL на подписку). up/down/up обратима.
- **Домен** `billing.go` (SubscriptionStatus/PaymentStatus/PaymentProvider/BillingPeriod + IsValid;
  `Subscription.GrantsPremium()` = premium && (active||past_due — grace); `InTrial`; `BillingPeriod.Advance`;
  `Payment`) + `features.go` (`Feature`, `PlanAllows(plan, feature)` — premium-only набор: custom_domain/
  private_pages/custom_smtp/white_label/priority_support; прочее доступно всегда).
- **Пакет `internal/billing`:** `Provider` интерфейс (CreatePayment/ChargeRecurring/ParseWebhook);
  `StubProvider` (dev — фейк-confirmation-URL, рекуррент всегда ок, парсит нормализованный JSON);
  `YooKassaProvider` (HTTP-адаптер api.yookassa.ru v3, Basic-auth+Idempotence-Key, save_payment_method;
  **реальные вызовы [ВЕРНУТЬСЯ]; верификация webhook'а ЮKassa — по IP-allowlist, не HMAC**);
  `SelectProvider` (ключи есть→ЮKassa, иначе stub); `DefaultPricing(monthlyMinor, yearlyDiscountPct,
  trialDays, currency)`. `Service` (репо-интерфейс): `Checkout` (pending-платёж+ссылка, подписка pending),
  `HandleWebhook` (идемпотент по provider_payment_id; succeeded→активирует premium+account.billing_plan,
  сохраняет токен/receipt), `Cancel` (cancel_at_period_end), `ProcessDue` (рекуррент→продление;
  неуспех→dunning past_due grace; исчерпание/отмена→Free). Юнит-тесты (fake repo+stub).
- **store** `billing.go` (+queries/billing.sql, sqlc): EnsureSubscription (lazy free), SubscriptionByAccount,
  UpdateSubscription, ListDueSubscriptions, SetAccountPlan, CreatePayment, PaymentByID/ByProviderID,
  UpdatePaymentResult, ListPaymentsByAccount; +`AccountByID`. Конвертеры *enum↔*string.
- **API** `billing.go`: handleGetSubscription/Checkout/CancelSubscription/ListPayments (operator-only,
  account из AccountByOwner) + handleBillingWebhook (**публичный**, провайдер из пути, idempotent; nil-
  billing→503/202). DTO amount=amount_minor/100 (₽). Роуты в server.go (webhook — в публичной группе).
  `cmd/api` `setupBilling`. config: PREMIUM_MONTHLY_MINOR/PREMIUM_YEARLY_DISCOUNT_PCT/BILLING_CURRENCY/
  TRIAL_DAYS/BILLING_SCAN_INTERVAL/BILLING_MAX_DUNNING/BILLING_RETRY_INTERVAL/YOOKASSA_SHOP_ID/SECRET_KEY.
- **6.7 гейтинг:** `handlePatchPage` (`gatePagePremiumFeatures`) и `handleAddAllowedEmail` блокируют
  ВКЛЮЧЕНИЕ premium-фич на Free → 403 `feature_required` (выключение/null/false — всегда можно). 4 теста
  этапа 4 (page_domain/management/page_access/page_email_access) поднимают аккаунт до premium в setup.
- **6.4 `cmd/worker-billing`:** периодический цикл `Service.ProcessDue` (Dockerfile+compose+.env).
- **6.8 оферта:** public-ssr `/offer` (SSR RU/EN; **черновик — финализировать с юристом**).
- **6.9 админка:** FSD `entities/billing` + `pages/billing` (план/статус/период; checkout monthly/yearly→
  redirect confirmation_url; отмена; история платежей+чеки); ссылка «Тариф» в шапке, роут `/billing`.
- **Флаги/решения:** (1) free-trial-флоу с картой-на-файле НЕ реализован (требует реального провайдера) —
  MVP: pay-to-activate; TrialDays/InTrial заложены на будущее. (2) past_due — grace (premium сохраняется
  до исчерпания dunning). (3) гейтинг — на ВКЛЮЧЕНИИ (PATCH), рантайм-фичи у уже-настроенных премиум-полей
  при откате не сносятся (MVP). (4) webhook ЮKassa без HMAC (IP-allowlist на ingress).
- **Проверено:** юнит (домен billing/features; billing service: checkout→webhook→premium, идемпотентность,
  рекуррент-продление, dunning-исчерпание→Free, cancel→Free) + интеграционный `billing_integration_test`
  на PG16 (free→checkout pending→403 на premium-фиче→webhook успеха→premium active→403 снят (PATCH домена
  ок)→платёж succeeded+receipt→идемпотентный повтор webhook'а→cancel→неизвестный провайдер 400). Все 4
  затронутых теста этапа 4 PASS. Полный go test (вкл. интеграционные PG16) + vet/gofmt/golangci-lint +
  admin `npm run build` + public-ssr `next build` зелёные. Миграция up/down/up обратима. Ждёт коммита.

**Этап 5.4 — исходящие webhook'и (написано, build+test(вкл. PG16+RabbitMQ e2e)+lint+admin build зелёные, ждёт коммита):**
- **Контракт НЕ менялся.** Канал `webhook` уже в enum `SubscriberChannel`; `q.webhook.out`/exchange
  `webhooks.out` — в топологии с 3.2; движок 3.3 уже фанил (но webhook отсекался как не-push).
  Исходящий webhook = `Subscriber{channel=webhook, address=URL}`.
- **Решение агента (payload-формат, в рамках §5):** Mattermost/Slack-совместимый — top-level `{text}`
  (incoming webhook рендерит как сообщение) + структурированные поля `event`/`status_page`/`url`/
  `incident`|`maintenance` (для произвольных консьюмеров). RU/EN по локали страницы.
- **Домен** `subscriber.go`: `SubscriberChannel.Deliverable()` = IsPush || webhook (каналы, на которые
  движок активно доставляет через очередь; rss/ical — pull, не доставляются).
- **Движок** (`notify`): в `Publisher` добавлен `PublishWebhookOut(ctx, body)`; `dispatch` фильтрует по
  `Deliverable()` (не IsPush); `publish` роутит — `channel==webhook` → `PublishWebhookOut` (webhooks.out),
  иначе `PublishNotification` (notifications). Ретраи webhook'а — штатно через `Retry`→
  `PublishNotificationDelayed` (delayed.events, key `notify.webhook.<event>`).
- **Топология** (`queue/topology.go`): добавлена привязка `q.webhook.out ← delayed.events` по
  `notify.webhook.*` — чтобы отложенные ретраи исходящих webhook'ов доходили (первичная публикация —
  через webhooks.out direct по `webhook.out`). Идемпотентно; объявляется на старте воркеров/queue-setup.
- **Пакет `internal/webhookout`** (зеркало `internal/slack`, тот же транспорт HTTP POST): `client.go`
  (`Client.Post` → `PostError{Permanent,RetryAfter}`: 4xx кроме 429→Permanent, 429→RetryAfter, 5xx/сеть→
  транзиент, битый URL→Permanent), `render.go` (`Render` → payload text+структура, RU/EN dict),
  `worker.go` (`Worker.Process`→Disposition: идемпотентность по Notification.id, orphan/sent→Ack,
  Permanent→Ack(дроп), транзиент→Retry/DLQ, mark-sent-fail→Requeue).
- **`cmd/worker-webhook`:** теперь потребляет ОБЕ очереди — `q.slack` (slack.Worker, 3.9) и
  `q.webhook.out` (webhookout.Worker, 5.4); общий Publisher+Engine(Retrier), graceful stop обеих.
- **API** (`subscribers.go`): `handleCreateSubscriber` принимает `webhook` (через `Deliverable()`),
  валидирует http(s)-URL для webhook; confirmed=true (как прочие ручные). rss/ical → 422.
- **Admin:** `webhook` добавлен в `MANUAL_SUBSCRIBER_CHANNELS` (форма уже с generic-адресом + channelLabel
  «Webhook»).
- **Флаги:** (1) исходящий webhook регистрируется ТОЛЬКО ручным добавлением оператором (нет публичного
  self-serve — это технический канал). (2) scope/components работает как у прочих каналов. (3) дедуп по
  URL — повторное добавление того же URL ловится unique(page,channel,address).
- **Проверено:** юнит — `webhookout` (render incident RU + maintenance EN + ошибки; client 200/204/400/
  404/429/500/503 классификация + битый URL; worker happy/идемпотент/orphan/permanent-drop/транзиент-
  retry/исчерпание/malformed), `notify` (webhook фан-аут → PublishWebhookOut, rss отсекается).
  Интеграционный на PG16 — регистрация webhook-подписчика (URL ок / не-URL 422 / rss 422). **Живой e2e
  на PG+RabbitMQ** (`notify.TestEngineIntegrationWebhookOut`): webhook-подписчик → IncidentCreated →
  сообщение в `q.webhook.out` с channel=webhook + address=URL + notification_id. build/test/vet/gofmt/
  golangci-lint + admin build зелёные. (capturePublisher/fakePublisher в тестах дополнены PublishWebhookOut.)

**Этап 5.3 — входящие webhook'и grafana/prometheus (написано, build+test(вкл. PG16)+lint+admin build зелёные, ждёт коммита):**
- **Решения человека:** (1) объём — **только grafana + prometheus** (generic/pagerduty отложены →
  роуты возвращают 501); (2) CRUD `/webhook-integrations` — **добавить** (secret генерится сервером,
  единожды); (3) идемпотентность — **колонки в incidents** (external_dedup_key + integration_id,
  partial-unique по открытому); (4) маппинг — **простой key→component_id + default**.
- **Контракт расширен:** enum `WebhookIntegrationSource` [grafana,prometheus,pagerduty,generic];
  схемы `WebhookIntegration` (без секрета) / `WebhookIntegrationCreate` (status_page_id опционален при
  ApiToken) / `WebhookIntegrationPatch` (name/component_mapping/regenerate_secret) / `Created` (с secret);
  пути `/webhook-integrations` (GET list?status_page_id / POST) и `/webhook-integrations/{id}`
  (GET/PATCH/DELETE); inbound generic/pagerduty +501. Типы перегенерированы (TS+Go), openapi провалиден.
- **Миграция** `00012_webhook_integrations.sql` (БД→**12**): `webhook_integrations` (status_page_id FK
  CASCADE, source TEXT+CHECK, name, **secret plaintext**, component_mapping jsonb DEFAULT '{}', триггер
  updated_at). `incidents` +`integration_id` (FK SET NULL) +`external_dedup_key`; **partial-unique**
  `incidents_open_dedup_key (status_page_id, external_dedup_key) WHERE key NOT NULL AND resolved_at IS
  NULL AND deleted_at IS NULL` → один открытый инцидент на dedup-ключ. up/down/up обратима на PG16.
  **Флаг:** secret хранится В ОТКРЫТОМ ВИДЕ (НЕ хэш §9) — нужен для проверки HMAC; как пароль SMTP в
  smtp_config или webhook URL slack-подписчика. API не возвращает его в list/get (только create/rotate).
- **Домен** `webhook_integration.go`: `WebhookSource` (+IsValid/All/Implemented — grafana|prometheus),
  `WebhookIntegration` (Secret в домене, в API DTO опускается). `Incident` +`IntegrationID`/
  `ExternalDedupKey` (*; nil для ручных).
- **Пакет `internal/webhook`** (чистый, как feed/widget): `signature.go` (`VerifySignature` HMAC-SHA256
  X-Signature, поддержка `sha256=`-префикса, constant-time; `Sign` для тестов), `alert.go`
  (`Alert{DedupKey,Firing,Title,Body,Labels}`; `ParseGrafana`=`ParsePrometheus`=Alertmanager-совместимый
  парс; dedup=fingerprint, иначе sha256 по сортированным меткам; title=summary|alertname; body=
  description|message|summary), `mapping.go` (`Mapping{Map,DefaultComponentIDs,MatchLabel,DefaultImpact}`,
  `ParseMapping`, `Impact()` деф. major, `Resolve(alert)` — метка match_label→Map, иначе default).
- **store:** `webhook_integrations.go` (Create/ByID/ListByPage/Update/Delete + mapWebhookIntegration;
  mappingBytes нормализует nil→'{}'). `incidents.go`: `OpenIncidentByDedup` (открытый по ключу),
  CreateIncident расширен (integration_id/external_dedup_key; **isUniqueViolation→ErrDedupConflict**
  на гонке firing). `ErrDedupConflict` в store.go. sqlc: webhook_integrations.sql + CreateIncident(+2
  колонки) + GetOpenIncidentByDedup → регенерация db.
- **API:** `integrations.go` (inbound, **HMAC а не JWT**, в публичной группе роутов): `handleGrafanaWebhook`/
  `handlePrometheusWebhook` → `handleInboundWebhook(source, parser)`: загрузить интеграцию по id
  (любая проблема аутентификации→401, не раскрываем существование), сверить source с роутом,
  прочитать тело (MaxBytesReader 1MiB), VerifySignature, распарсить, для каждого алерта `ingestAlert`:
  firing+нет открытого→`createWebhookIncident` (impact из маппинга, статус компонента по impact:
  critical→major_outage/major→partial/иначе degraded; компоненты фильтруются по принадлежности
  странице; status=investigating; emit IncidentCreated), firing+есть открытый→no-op, resolved+есть→
  AddIncidentUpdate(resolved)+emit IncidentUpdated, resolved+нет→no-op; ErrDedupConflict (гонка)→no-op;
  → 202. generic/pagerduty→501. `webhook_integrations.go` (management, **operator-only** как токены —
  минтит секрет): create (resolveManagedPage, валидация source/mapping, GenerateWebhookSecret),
  list (без секрета), get/patch (name/mapping/regenerate_secret→новый секрет единожды)/delete;
  `loadAuthorizedIntegration`. security `GenerateWebhookSecret` (префикс whsec_, plaintext, без хэша).
- **Флаги/решения:** (1) управление интеграциями — operator-only (как токены). (2) component_mapping
  валидируется только на парсимость (ParseMapping); принадлежность компонентов странице проверяется
  при ingest (фильтр по pageComponentSet) — чужие/удалённые id молча отбрасываются. (3) dedup-ключ
  пустой → алерт игнорируется (идемпотентность невозможна). (4) рецидив после resolve (тот же ключ)
  создаёт новый инцидент — корректно (partial-unique только по открытым). (5) inbound не уведомляет по
  скрытым? — webhook-инцидент всегда is_visible=true (видимый). (6) маршруты inbound в **публичной**
  группе (security override), не под requireAuth.
- **Проверено:** юнит `internal/webhook` (подпись happy/префикс/неверная/чужой секрет/пустые; парсинг
  grafana+prometheus, fallback title, dedup по меткам стабилен; маппинг по метке/default/пустой;
  ParseMapping). Интеграционный `integrations_integration_test` на PG16: создать grafana-интеграцию
  (секрет единожды) → список без секрета → firing webhook (HMAC) → инцидент (impact major, title из
  summary, компонент partial_outage) → повторный firing идемпотентен (1 инцидент) → resolved → инцидент
  закрыт (resolved_at) → повторный resolved no-op → рецидив firing → новый инцидент (всего 2) → битая
  подпись/чужой источник/неизвестная интеграция → 401 → generic → 501 → prometheus-интеграция firing →
  инцидент → get/patch(ротация секрета→новый; без ротации→без секрета)/delete→204→get 404 → изоляция
  оператора B (404). **PASS.** Полный go test + vet/gofmt/golangci-lint + admin build зелёные.
  (Хелпер теста `signedWebhook` подписывает X-Signature.)

**Этап 5.2 — полный write-API под page-токеном (написано, build+test(вкл. PG16)+lint+admin build зелёные, ждёт коммита):**
- **Решение человека:** убрать `status_page_id` из `required` в `IncidentCreate`/`MaintenanceCreate`/
  `IncidentTemplateCreate`/`SubscriberCreate` — чтобы page-токен мог опускать (берётся из токена),
  симметрично с уже опциональным `ComponentCreate` и GET-эндпоинтами (их query `status_page_id` уже
  `required:false`). Типы перегенерированы (TS+Go), openapi провалиден.
- **Все эндпоинты §7.2 уже существовали** (components/incidents/maintenances/subscribers — этапы 1–3;
  tokens — 5.1). 5.2 — НЕ новые эндпоинты, а сделать их полноценно рабочими под page-токеном.
- **Backend:** общий резолвер `resolveManagedPage(w, r, raw)` (access.go): оператор → status_page_id
  обязателен (422 при отсутствии/невалидности); page-токен → страница из токена, переданный raw должен
  совпадать с ней (иначе 404, как чужая страница). Переведены 10 хендлеров — list+create для
  components/incidents/incident-templates/maintenances/subscribers (раньше каждый сам парсил
  status_page_id + authorizePage; теперь единый резолвер; у components list код ошибки сменился 400→422).
  **PATCH/DELETE/updates НЕ трогал** — они уже авторизуются по StatusPageID загруженного ресурса через
  `authorizePage`, который с 5.1 поддерживает оба субъекта → под токеном работают без правок.
  `/tokens` остаются operator-only (5.1), резолвер не применяется.
- **Флаги:** (1) под токеном чужой status_page_id (в теле или query) → 404 (а не молчаливая подмена на
  свою страницу) — защита от записи не на ту страницу. (2) у `handleListComponents` код «нет
  status_page_id» сменился 400→422 (унификация; ни один тест не пинил 400).
- **Проверено:** интеграционный `write_api_token_integration_test` на PG16 — под write-токеном БЕЗ
  status_page_id: CRUD компонента (create+list), **жизненный цикл инцидента investigating→identified
  (update)→PATCH impact→resolved→delete**, работы create→in_progress→delete, подписчики
  create→list→delete, шаблоны create→list; чужой status_page_id под токеном→404 (тело и query); свой→ок;
  оператор без status_page_id→422. **PASS.** Полный go test (вкл. все интеграционные) + vet/gofmt/
  golangci-lint + admin `npm run build` зелёные. (Хелперы теста: `rawTokenReq` из 5.1, `decodeBody`/
  `wantStatus` добавлены в write_api_token_integration_test.go.)

**Этап 5.1 — ApiToken со scope'ами + аутентификация управляющих запросов (написано, build+test(вкл. PG16)+lint зелёные, ждёт коммита):**
- **Решения человека (развилки 5.1):** (1) токен **page-scoped** → добавить `status_page_id` в `TokenCreate`
  (как 2.5/2.6/3.10); (2) scope'ы — **грубо read/write** (write⊇read); (3) **добавить GET /tokens** сейчас.
- **Контракт расширен:** `TokenScope` enum `[read, write]`; `TokenCreate` +required `status_page_id`,
  `scopes: [TokenScope]`; `TokenCreated` +`status_page_id`+required `scopes`; новая `Token` (id/
  status_page_id/name/scopes/last_used_at/created_at — без значения/хэша); `GET /tokens?status_page_id`
  → `[Token]`. Глобальный `security` уже допускал BearerAuth|ApiToken (не менялся). Типы перегенерированы
  (TS+Go), openapi провалиден (оба gen прошли).
- **Миграция** `00011_api_tokens.sql` (БД→**11**): `api_tokens` (id, status_page_id FK CASCADE, token_hash
  text, name, scopes text[] DEFAULT '{}', last_used_at, created_at/updated_at). unique-индекс по token_hash,
  idx по status_page_id, триггер updated_at. scopes — TEXT[] (не pg-enum: набор задаёт контракт, как
  channel/scope подписчиков). Проверено up/down/up на PG16.
- **Домен** `apitoken.go`: `TokenScope` (ScopeRead/ScopeWrite, IsValid, AllTokenScopes), `APIToken`
  (без token_hash — он не покидает store/security), `HasScope` (**write подразумевает read**), `CanWrite`,
  `NormalizeScopes` (валидация+дедуп; пустой набор разрешён, дефолт решает API). Юнит-тест.
- **security** `token.go`: `GenerateAPIToken` (32 байта rand, префикс `hp_`, base64url) → (plaintext, hex
  SHA-256-хэш); `HashAPIToken`. В БД только хэш (§9), plaintext отдаётся единожды. Юнит-тест.
- **store** `api_tokens.go`: CreateAPIToken/APITokenByHash/APITokenByID/ListAPITokensByPage/TouchAPIToken
  (last_used_at=now)/DeleteAPIToken. sqlc `queries/api_tokens.sql` → регенерация db (sqlc 1.31.1 brew).
- **Middleware** (`middleware.go`, ПЕРЕРАБОТАН): вместо `userCtxKey domain.User` теперь `principalCtxKey`
  с `principal{operator *User | token *APIToken}` (ровно одно ненулевое). `requireAuth`: префикс `Bearer `
  → операторский JWT (как раньше); сырое значение `Authorization` → page-токен (`HashAPIToken`→
  `APITokenByHash`; не найден→401); **scope-энфорсинг** `tokenScopeAllows` по HTTP-методу (GET/HEAD/
  OPTIONS→read, прочее→write; нехватка→403); `TouchAPIToken` best-effort. Хелперы `userFromContext`
  (оператор или ok=false для токена), `requireOperator` (403 для токена — account-level операции),
  `principalFromContext`. `bearerToken` без изменений.
- **authorizePage** (`access.go`): теперь через `principalOwnsPage(ctx, p, page)` — оператор: владеет
  аккаунтом страницы; токен: `token.StatusPageID == page.ID`. Все существующие управляющие хендлеры
  (компоненты/инциденты/работы/подписчики/группы/шаблоны), вызывающие authorizePage, **уже работают под
  page-токеном** при явном status_page_id (токен валидируется по владению). `handleListPages`/
  `handleCreatePage` обёрнуты `requireOperator` (создание/список страниц — только оператор).
- **API** `tokens.go`: `handleCreateToken` (POST /tokens — **только оператор**; name+status_page_id+
  authorizePage; NormalizeScopes; **пустой scopes→[read,write] полный доступ**, как у Статусмейт; генерит
  токен, отдаёт TokenCreated с plaintext единожды), `handleListTokens` (GET /tokens?status_page_id,
  оператор, authorizePage, без значений), `handleDeleteToken` (DELETE /tokens/{id}, оператор,
  APITokenByID→authorizePage(page)→delete; несуществующий→404). Роуты в admin-группе server.go.
- **Флаги/решения:**
  - (1) **Управление токенами — только оператор (JWT)**, не сам page-токен (предотвращает эскалацию;
    `requireOperator`→403). То же для POST/GET /pages и /auth/me (токен→401/403).
  - (2) **status_page_id в управляющих эндпоинтах всё ещё обязателен** даже под page-токеном (хендлеры
    парсят req.StatusPageID/?status_page_id). Сделать его **опциональным при ApiToken** (брать из токена,
    как в комментариях openapi) — **это 5.2** (полный write-API). Сейчас токен функционален, но должен
    слать свою же страницу явно.
  - (3) **Дефолт scopes = полный доступ** при пустом массиве (создаёт владелец страницы; гранулярность —
    только при явном сужении до read).
  - (4) Префикс токена `hp_` — для распознавания в логах/секрет-сканерах и отделения от Bearer-JWT.
- **Проверено:** юнит (domain scopes/HasScope/NormalizeScopes; security gen/hash/uniqueness) +
  интеграционный `tokens_integration_test` на PG16 (create write/read токенов→list без значений→
  аутентификация токеном POST/GET компонентов→last_used_at проставлен→read-токен write→403, read→200→
  чужая страница→404→токен создаёт токен→403→невалидный scope 422→невалидный токен 401→изоляция
  оператора B 404→отзыв 204→отозванный 401→повтор delete 404→пустой scopes даёт полный доступ).
  **PASS.** Полный api+store интеграционный прогон зелёный (рефактор middleware не сломал прочие).
  build/test/vet/gofmt/golangci-lint + admin `npm run build` зелёные. public-ssr не трогал (не импортит
  api-types; изменения аддитивны). Миграция 00011 up/down/up обратима.

**Этап 4 добивка: 4.4/4.5/4.6/4.2.1/4.3.2/4.3.3 (написано, build+test+lint зелёные, ждёт коммита):**
- **4.4 white-label:** тумблер `hide_powered_by` в `SettingsForm` (поле уже было в StatusPageUpdate;
  публичный футер уже учитывал). Премиум-гейтинг — этап 6.
- **4.5 custom SMTP:** контракт — `StatusPageUpdate.smtp_config` (write-only объект/null) + `from_email`;
  ответ `StatusPage` +`from_email`+`smtp_configured` (булев, без секрета). store `SetStatusPageSMTP`;
  PATCH merge (smtp из админки шлётся только если тронут — нельзя перечитать пароль). Admin — секция
  «Письма». Доставка (`effectiveSMTP`) уже была в worker-email. management-тест расширен.
- **4.6 виджет:** контракт `GET /pages/{slug}/badge.svg` (public, гейтится приватной). Пакет
  `internal/widget` (SVG shields-стиль, цвет/i18n по статусу), хендлер (ComputeOverallStatus,
  Cache-Control 60s). Admin — превью `<img>` + копируемый embed-сниппет. Юнит+смоук.
- **4.2.1 приватность по email + magic-link:** миграция `00009_page_allowed_emails`. Контракт — CRUD
  allowed-emails (admin) + публичные `access/request-link` (всегда 202) и `access/verify?token`.
  `subscription.AccessLinkToken/Parse` (HMAC page+email, TTL 1ч). Транзакционное письмо: `EventAccessLink`,
  `notify.Engine.SendAccessLink` (NotificationID пустой → без журнала), worker-email обрабатывает
  транзакционную ветку + рендер RU/EN. API: `handleRequestAccessLink` (сверка email, анти-энумерация),
  `handleVerifyAccessLink` (повторная сверка → access-токен 4.2). public-ssr: гейт + email-форма +
  route handlers `access/request-link` и `access/verify` (ставит cookie). Admin: список email в настройках.
  Интеграционный (capturePublisher) + render-тест. **Флаг:** гейт показывает оба метода (пароль+email).
- **4.3.2 ACME (`cmd/tls-manager`, lego v4):** миграция `00010_domain_certificates`
  (domain_certificates/acme_accounts/acme_challenges). `internal/acme`: аккаунт в БД, HTTP-01 через
  `dbChallengeProvider` (challenge в БД → отдаёт edge), `Obtain`, `RenewDue` (verified-домены, <renew_before).
  config ACME_*. Юнит: needsIssue/challenge-provider/certExpiry/key-roundtrip. **Реальный выпуск — на проде.**
- **4.3.3 edge (`cmd/edge`):** `internal/edge` — TLS по SNI (серты из БД, кэш с проверкой срока),
  :80 HTTP-01+redirect, :443 `/api/*`→API / корень кастом-домена→`/status/{slug}` (store.SlugByCustomDomain)
  / прочее→public-ssr. config EDGE_*. Dockerfile (+2 бинаря), compose (профиль `edge`, порты 80/443).
  Юнит: isAPIPath/HTTP-01/redirect/hostOnly. **Отладка — на проде.**
- **Зависимость:** добавлен `github.com/go-acme/lego/v4` v4.17.4. Миграции up/down обратимы (до v10).
  Все backend-тесты (вкл. интеграционные PG16) + vet + golangci-lint + admin/next build зелёные.

**Этап 4.3.1 — собственный домен: управление + верификация CNAME (написано, build+test+смоук PASS, ждёт коммита):**
- **Решения человека (4.3):** TLS — собственный ACME-сервис на Go (lego, задача 4.3.2); верификация — CNAME.
  4.3 разбит на 4.3.1 (это) / 4.3.2 (tls-manager) / 4.3.3 (edge-прокси). Acceptance 4.3 закроется на 4.3.3.
- **Контракт расширен с санкции человека:** `StatusPageUpdate.custom_domain` (RawMessage-семантика:
  отсутствие/null/строка; задание сбрасывает domain_verified); `POST /pages/{id}/domain/verify` →
  `DomainStatus{custom_domain, domain_verified, cname_target}`; ответ `Conflict` (409). БД-поля
  custom_domain/domain_verified уже были (00003, unique-индекс по custom_domain среди живых).
- **Backend:** sqlc `SetCustomDomain` (сбрасывает verified) + `SetDomainVerified`; store-методы
  (`ErrDomainTaken` при unique-конфликте). `handlePatchPage` парсит custom_domain (`parseCustomDomain`:
  lower+trim, null/""→снять, дубль→409, перечитывает страницу для ответа). `handleVerifyDomain`
  (page_domain.go): домен не задан→422; резолв CNAME через `cnameResolver` (Deps.CNAMEResolver,
  дефолт net.DefaultResolver.LookupCNAME — **тесты инъектируют фейк**), сравнение `normalizeHost`
  (lower, без завершающей точки) с `cnameTarget`; пишет SetDomainVerified при изменении. config
  `CNAMETarget` (env CNAME_TARGET, дефолт cname.healthpage.ru) → Deps. Маршрут
  `POST /pages/{page}/domain/verify` в admin-группе.
- **Admin:** entities/page `verifyDomain(id)` + тип DomainStatus. SettingsForm: секция «Собственный
  домен» — ввод custom_domain (в общий PATCH; пусто→null), бейдж verified/pending, кнопка «Проверить
  домен» (POST verify), инструкция с cname_target. На mount при заданном домене подтягивает статус+target.
  CSS `.hp-badge--ok`.
- **Флаги:** (1) verify сравнивает каноническое имя из LookupCNAME с target; цепочки CNAME за target
  не разворачиваются (MVP). (2) custom_domain уникален среди живых страниц (partial unique index);
  дубль→409. (3) TLS/выпуск сертов и реальная доставка по домену — 4.3.2/4.3.3 (локально не
  e2e-тестируется без публичного DNS+LE). cname_target отдаётся только из verify-ответа (отдельного
  GET статуса домена не делал — админка тянет через verify).
- **Проверено:** юнит (нет; логика резолва покрыта интеграционным с фейком) + интеграционный
  `page_domain_integration_test` на PG16 (verify без домена→422; PATCH задаёт домен (lower)→verified
  false; verify match→true+cname_target; смена домена→verify mismatch→false; дубль на 2-й странице→409;
  снятие null→очищен). build/test/vet/gofmt/golangci-lint + admin build зелёные. Живой смоук эндпоинтов
  на :8081 (PATCH→verified false; verify несуществующего домена→false+cname_target).

**Этап 4.2 — приватные страницы по паролю + noindex (написано, build+test+живой e2e PASS, ждёт коммита):**
- **Решения человека:** (1) объём — **только пароль**; список email отложен в 4.2.1 (нужна таблица
  allowed_emails + magic-link флоу). (2) Механизм — **подписанная HttpOnly-cookie**.
- **Контракт расширен с санкции человека:** `POST /pages/{slug}/access` (PageAccessRequest{password}→
  PageAccessResult{access_token,expires_in}); `StatusPageUpdate.password` (write-only: строка задаёт,
  null/"" снимает); `PublicPage.visibility` (для noindex); параметр-заголовок `X-Page-Access` +
  ответ `PasswordRequired` (401) добавлены к публичным read (summary/components/incidents/incidents/{id}/
  maintenances/rss/calendar.ics/uptime). Типы перегенерированы (TS+Go), openapi провалиден.
- **Backend:** `subscription.PageAccessToken/ParsePageAccessToken` (`<pageID>.<expUnix>.<HMAC>`, TTL
  `PageAccessTTL`=7д, секрет = `subSecret`/SUBSCRIPTION_SECRET). sqlc `SetStatusPagePassword` +
  store-метод (хранит хэш, §9). API: `handlePageAccess` (page_access.go — verify argon2id
  `security.VerifyPassword`→ токен; не приватная → 404; неверный/без пароля → 401), `handlePatchPage`
  принимает `password` (RawMessage: отсутствие≠null; `hashPagePassword` хэширует/снимает),
  `loadPublicPage` для приватной требует валидный `X-Page-Access` (иначе 401 password_required),
  `publicPageResponse.visibility`. Маршрут `POST /pages/{page}/access` в публичной группе.
  **Приватные компоненты НЕ трогал** — уже исключаются из вывода (summary.go) и из общего/группового
  статуса (`Component.CountsTowardStatus` = !private && display_state). Юнит `token_test`
  (PageAccessToken round-trip/секрет/TTL/формат) + интеграционный `page_access_integration_test`
  на PG16 (public→/access 404; private без токена→401; неверный пароль→401; верный→токен;
  X-Page-Access→200+visibility=private; битый токен→401; снять пароль→/access снова 401).
- **public-ssr:** `lib/api` — `pageAccessHeaders(slug)` читает cookie `hp_access_<slug>` (next/headers
  `cookies()`) и форвардит как `X-Page-Access` во все slug-запросы; `getJSON` 401→`PageAccessRequiredError`;
  `PublicPage.visibility`. Гейт: `AccessGate` (нативная форма POST, без клиентского JS) + route handler
  `app/status/[slug]/access/route.ts` (POST→backend /access; успех→Set-Cookie HttpOnly hp_access_<slug>
  + 303 на страницу; неверный→303 `?access_error=1`). Главная статус-страница при 401 рендерит
  `AccessGate` (показывает ошибку по `access_error`); подстраницы (incidents/detail/maintenances) при
  401 `redirect` на главную (где гейт). `noindex` для приватных (visibility=private или 401) через
  `buildStatusMetadata` (lib/meta). CSS `.gate*`.
- **Admin:** `SettingsForm` — Select видимости (public/private) + поле пароля (показывается для private:
  ввод задаёт новый, чекбокс «снять пароль»→null, пусто→не менять) через PATCH `/pages/{id}`.
- **Решения/флаги:** (1) приватная страница отдаёт 401 (а не 404) — раскрывает существование, но это
  нужно для гейта (DESIGN хочет ввод пароля, не скрытие). (2) cookie ставит **public-ssr** (origin
  посетителя), а backend минтит/проверяет токен — корректно для кросс-origin SSR (вариант
  «Set-Cookie из API» не работает cross-origin). (3) RSS/iCal приватной страницы тоже гейтятся
  (фид-ридер без токена → 401) — для приватных фид через токен в URL не делал (out of scope MVP).
  (4) На гейте локаль ru по умолчанию (форма несёт lang). (5) список email — 4.2.1.
- **Проверено:** go build/test(+интеграц.)/vet/gofmt/golangci-lint; admin `npm run build`; public-ssr
  `next build` — зелёные. **Живой e2e** (api :8081 + next start): без cookie→гейт(200)+noindex;
  неверный пароль→303 access_error=1 (текст ошибки на гейте); верный→303+Set-Cookie hp_access_<slug>;
  с cookie→статус-страница (brand «Secret Co», overall, noindex сохраняется); публичная страница не
  гейтится и индексируется.

**Этап 4.1 — тема/брендинг/таймзона (написано, build+test+живой e2e PASS, ждёт коммита):**
- **Контракт расширен с санкции человека:** новая схема `PublicPage` (публично-безопасное
  подмножество страницы) + поле `page` (required) в `PageSummary`. Поля: name, description, slug,
  timezone, default_locale, theme, logo_url, favicon_url, hide_powered_by. БЕЗ приватных
  (account_id/password_hash/smtp_config/custom_domain/redirect_url). Типы перегенерированы (TS+Go),
  openapi провалиден. **Развилка решена человеком:** встроить page в PageSummary (не отдельный
  публичный GET) — SSR уже тянет сводку, без второго round-trip.
- **БД/PATCH уже готовы:** все поля темы (theme jsonb, logo_url, favicon_url, hide_powered_by,
  redirect_url, timezone, default_locale) есть в `status_pages` с миграции 00003 и персистятся
  хендлером `PATCH /pages/{id}` — миграция и изменения store НЕ потребовались.
- **Backend** (`internal/api/public.go`): `publicPageResponse` DTO + `toPublicPageResponse`,
  наполнение `Page` в `pageSummaryResponse`. Интеграционный тест (`management_integration_test`)
  расширен: PATCH темы/таймзоны/логотипа → проверка брендинга в публичной сводке. PASS на PG16.
- **public-ssr:** `lib/theme.ts` (PageTheme: primaryColor/mode/timeFormat; `parseTheme` нормализует
  публичный jsonb, `themeVars`, `is12h`), `lib/meta.ts` (`buildStatusMetadata` → title=имя,
  favicon=favicon_url через `generateMetadata`), `lib/api.ts` (+PublicPage, +`fetchPageMeta` —
  тянет сводку, Next дедуплицирует одинаковый fetch в одном рендере), `formatInZone` в i18n
  (время в часовом поясе страницы + метка пояса timeZoneName:short; убран хардкод " UTC";
  невалидный tz → фолбэк UTC). Общий серверный компонент `app/status/[slug]/PageShell.tsx`
  (обёртка `data-theme` + inline `--accent` + шапка logo/name + футер honor hide_powered_by) —
  оборачивает ВСЕ вкладки (overview/incidents/incident-detail/maintenances). CSS: `.page-shell`,
  `.brand*`, тёмная тема через `[data-theme="dark"]` и `@media prefers-color-scheme` для "auto",
  `.tab-active` → `--accent`.
- **Структура theme jsonb** (соглашение фронт↔оператор, не нормативный enum): `{primary_color: "#hex",
  mode: "light"|"dark"|"auto", time_format: "24h"|"12h"}`. layout (макет) — пока единственный,
  мульти-layout не закладывал (MVP).
- **Admin (FSD):** `features/page-settings/SettingsForm` (name/description/timezone[Select IANA]/
  default_locale/акцент[color]/тема[mode]/формат времени/logo_url/favicon_url → PATCH `/pages/{id}`,
  контракт не меняется; пустой url → null), `pages/page-settings/SettingsPage`, вкладка «Настройки»
  в `widgets/page-nav`, роут `/pages/:id/settings`. `npm run build` зелёный.
- **Решения/флаги:** (1) тумблер `hide_powered_by` в админке — задача 4.4 (white-label premium);
  публичный рендер уже его учитывает (футер скрывается). (2) redirect_url — не в 4.1, в page-мету
  не выводил. (3) часовой пояс — Select из ~16 IANA-зон + текущее значение; свободный ввод не делал.
- **Проверено:** go build/test(+интеграц.)/vet/gofmt/golangci-lint зелёные; admin `npm run build` +
  public-ssr `next build` зелёные; **живой e2e** (свежий api :8081 + `next start`): сводка отдаёт
  page; рендер — data-theme="dark", brand-name, logo src, `--accent:#e11d48`, `<title>`, favicon
  rel=icon, время GMT+3 + 12h(PM), футер «Работает на HealthPage»; брендинг на вкладке инцидентов;
  white-label (hide_powered_by=true) скрывает футер, mode=light → data-theme="light".

**Этап 3.2 — топология RabbitMQ (написано и проверено на живом брокере, ждёт коммита):**
- **Образ брокера** `docker/rabbitmq/Dockerfile`: `rabbitmq:3.13-management-alpine` + community-плагин
  `rabbitmq_delayed_message_exchange` 3.13.0 (ADD .ez → enable --offline). docker-compose: rabbitmq
  теперь `build:` (не `image:`). Плагин в работающем брокере подтверждён (`rabbitmq-plugins list -e`).
- **Зависимость:** `github.com/rabbitmq/amqp091-go` v1.10.0 (прямая).
- **Пакет `internal/queue`** (нормативная топология §8.1):
  - `queue.go` — `Conn` (Dial/Channel/Close).
  - `topology.go` — константы (exchange'и `notifications`/`webhooks.out`/`delayed.events`/`dlx`;
    `WorkQueue`/`DLQQueue`/`NotifyKey`), `DeclareTopology(ch)` — идемпотентно объявляет 4 exchange'а,
    рабочие очереди `q.<channel>` (email/telegram/max/slack) с DLX-аргументами, DLQ `q.dlq.<channel>`,
    `q.webhook.out`(+DLQ); привязки notifications→q.<channel> (`notify.<channel>.*`), delayed.events→
    те же (retry/анонсы), dlx→q.dlq.<channel> (`dlq.<channel>`), webhooks.out→q.webhook.out.
  - `publisher.go` — `Publisher` с **publisher confirms** (PublishWithDeferredConfirm + ожидание ack):
    `PublishNotification`, `PublishNotificationDelayed` (header `x-delay`), `PublishWebhookOut`.
  - `consumer.go` — `Consume(queue, prefetch, handler)` с **ручным ack** (autoAck=false) — для воркеров 3.4+.
- **Команда `cmd/queue-setup`** — объявляет топологию и выходит (идемпотентно). Добавлена в backend
  Dockerfile (`/app/queue-setup`). `config.MustRabbitMQURL()`.
- **Проверено на живом брокере:** queue-setup объявляет топологию (дважды — идемпотентно);
  `rabbitmqctl list_exchanges/list_queues/list_bindings` — всё точно по §8.1. Интеграционный тест
  `queue_integration_test.go` (skip без `HEALTHPAGE_TEST_AMQP`): publisher confirm + маршрутизация →
  q.email; Nack(requeue=false) → dead-letter в q.dlq.email; delayed-публикация приходит через ~1s.
  **PASS.** go build/vet/gofmt/golangci-lint зелёные; backend и rabbitmq образы собираются.

**Этап 3.10 — управление подписчиками в админке + клиентская отписка (написано, тесты/сборки PASS, ждёт коммита):**
- **Контракт расширен с санкции человека** (как 2.5/2.6/2.9): `GET /subscribers` +query `status_page_id`
  (обязателен при JWT, включает неподтверждённых); `SubscriberCreate` +`status_page_id` (required);
  ответ `Subscriber` +`status_page_id`. Типы перегенерированы (TS+Go), openapi провалиден.
- **Backend:** sqlc `ListSubscribersByPage` (вкл. pending, пагинация, новые сверху) → store-метод
  `ListSubscribersByPage`. API `subscribers.go`: `handleListSubscribers` (status_page_id+authorizePage+
  parsePagination, ответ — **plain array** как в контракте, без envelope), `handleCreateSubscriber`
  (**ручное добавление оператором, confirmed=true**; только push-каналы `IsPush` email/telegram/max/slack
  иначе 422; дубль (page,channel,address) → 422; scope=components валидирует компоненты через
  parseMaintenanceComponents), `handleDeleteSubscriber` (SubscriberByID→authorizePage(sub.StatusPageID)→
  delete→204; чужой/несуществующий→404). `toSubscriberResponse` DTO. Роуты в admin-группе server.go.
- **Решения/флаги:** (1) ручное добавление **минует double opt-in** (confirmed=true) — оператор
  отвечает за согласие на обработку ПД (152-ФЗ, DESIGN §4.3/§9); rss/ical/webhook через ручное
  добавление не заводятся. (2) Список — plain array (контракт `GET /subscribers` так задан), пагинация
  page/per_page без total; UI листает «вперёд», пока приходит полная страница.
- **Admin (FSD):** `entities/subscriber` (list/create/delete + типы), `features/subscriber-add`
  (форма: канал/адрес/scope/компоненты через ComponentChecklist), `pages/subscribers/SubscribersPage`
  (список с бейджем confirmed/pending + удаление + форма), `shared/lib/subscriber` (channelLabel/
  scopeLabel), `MANUAL_SUBSCRIBER_CHANNELS` в shared/api/types, вкладка «Подписчики» в `widgets/page-nav`,
  роут `/pages/:id/subscribers` в App.tsx. `npm run build` зелёный.
- **public-ssr (клиентская страница, минимум):** `app/unsubscribe/page.tsx` (force-dynamic) — читает
  `?token`+`?lang`, вызывает backend `GET /unsubscribe` через `lib/api.unsubscribeByToken`, показывает
  дружелюбное RU/EN подтверждение (успех/недействителен/нет токена). `next build` зелёный.
- **worker-email:** ссылка отписки в письме теперь ведёт на public-ssr `/unsubscribe?token=` (а не на
  `/api/v1/unsubscribe`) — единообразно с ссылкой на `/status/<slug>` (обе от `baseURL`=публичный origin).
  Сам API-эндпоинт `/unsubscribe` сохранён (его и дёргает страница). **Флаг (прежний):** confirm-ссылка
  всё ещё на `/api/v1/...`; единый PUBLIC_BASE_URL для прод-разделения origin'ов — отдельная задача (этап 4).
- **Клиентская самоуправление — минимум (решение человека):** только отписка; просмотр/смена scope по
  токену отложены (нужны новые эндпоинты get/patch subscription by token — расширение контракта).
- **Проверено:** интеграционный `subscribers_admin_integration_test` на PG16 (пустой список → создание
  email confirmed → pending через store виден в списке → дубль 422 → rss 422 → удаление 204 → повтор 404 →
  изоляция операторов 404 → 401). build (go/admin/next) + go test (вкл. интеграционные) + vet/gofmt/
  golangci-lint зелёные.

**Этап 3.9 — Slack-канал (написано, юнит + интеграционный на PG16 PASS, ждёт коммита):**
- **Контракт НЕ менялся.** Эндпоинты `GET /pages/{slug}/subscribe/slack/start` и `GET /subscribe/slack/callback`
  уже в openapi (security[]); enum `slack` уже в SubscriberChannel; `q.slack`+привязки — с 3.2;
  движок 3.3 уже фанит на slack (IsPush). Подписка — через OAuth, не через POST /subscribe.
- **Пакет `internal/slack`:**
  - `oauth.go` — `OAuth`: `AuthorizeURL(state)` (slack.com/oauth/v2/authorize, scope incoming-webhook),
    `Exchange(ctx, code)→WebhookGrant{WebhookURL,Channel,TeamName}` (POST oauth.v2.access). Опция
    `WithAccessURL` — тестовый seam (стаб-сервер).
  - `client.go` — `Client.PostMessage(ctx, webhookURL, payload)`: POST JSON; `PostError` с
    классификацией (4xx кроме 429 → Permanent; 429 → RetryAfter из Retry-After; 5xx/сеть → транзиент).
  - `render.go` — `Render(RenderInput)→[]byte`: Slack Block Kit через attachment (color по impact /
    синий для работ), header+section fields+context-ссылка, i18n RU/EN, `escMrkdwn` (& < >). Свой dict.
  - `worker.go` — `Worker.Process` (Disposition Ack/Reject/Requeue локальные). Зеркало telegram:
    идемпотентность по Notification.id (orphan/sent→Ack), пустой address→Reject, рендер, POST.
    Permanent-ошибка → Ack (дроп — webhook отозван/канал удалён); транзиент → `Retrier.Retry` →
    delayed-ретрай/DLQ; ошибка MarkSent → Requeue.
- **API** (`internal/api/slack.go`, публичные): `handleSlackStart` — loadPublicPage (приватная→404),
  фича-off (slackOAuth=nil)→404, SignSlackState→302 на AuthorizeURL. `handleSlackCallback` —
  фича-off→404, нет code/state→400, ParseSlackState (битый/истёкший→400)→StatusPageByID→Exchange
  (ошибка→400)→идемпотентный upsert Subscriber{slack, address=webhook_url, confirmed=true, scope=page}→
  200 JSON {status,channel}. `Deps.SlackOAuth *slack.OAuth` (nil → выключено), routes в server.go.
  api main `setupSlackOAuth` (redirect_uri = BASE_URL + /api/v1/subscribe/slack/callback).
- **subscription** (`token.go`): `SignSlackState(secret, pageID, issuedUnix)` / `ParseSlackState
  (secret, state, now)` — `<pageID>.<unix>.<HMAC>`, TTL `SlackStateTTL`=1ч (CSRF + привязка к странице,
  т.к. callback страницы не знает).
- **cmd/worker-webhook:** потребляет `q.slack` (manual ack, prefetch=16, Process→Ack/Nack), Publisher+
  Engine как Retrier, `slack.NewClient`. Секреты Slack OAuth НЕ нужны (адрес = сохранённый webhook URL).
  На 5.4 добавит `q.webhook.out` (исходящие webhook'и). Dockerfile (/app/worker-webhook), compose
  (сервис worker-webhook; api получил SLACK_CLIENT_ID/SECRET+SUBSCRIPTION_SECRET), .env.example.
- **config:** `SlackClientID`/`SlackClientSecret` (SLACK_CLIENT_ID/SECRET); пусто → эндпоинты Slack 404.
- **Флаги (решения агента в рамках §5):** (1) **scope=page** — компонентная подписка Slack не выставлена
  в openapi-контракте `/start` (только Slug), добавлять с санкции человека (как у telegram). (2) **Дубли:**
  повторный «Add to Slack» выдаёт НОВЫЙ webhook URL (особенность Slack) → возможен второй подписчик на тот
  же канал; идемпотентность ловит только тот же URL. (3) callback отвечает JSON (как confirm 3.5);
  «красивая» HTML-страница/редирект на public-ssr — этап 4 (UX).
- **Проверено:** юнит — `oauth_test` (AuthorizeURL, Exchange happy/ok:false/без webhook), `client_test`
  (200 ok, 404→Permanent, 429→RetryAfter, 5xx→транзиент), `render_test` (incident RU+цвет, maintenance
  EN+цвет, escaping декодированного payload, ошибки), `worker_test` (happy/идемпотент/orphan/empty-addr/
  транзиент-retry/исчерпание/Permanent-дроп/malformed/unrenderable/mark-sent→requeue),
  `subscription/token_test` (SlackState round-trip/секрет/TTL/формат). Интеграционные на PG16
  (`slack_integration_test`, стаб oauth.v2.access): start→302+state → callback создаёт confirmed
  page-подписчика → повтор идемпотентен → негативы (нет code/битый state→400, неизвестная страница→404) →
  фича-off → оба эндпоинта 404. **PASS.** build/test/vet/gofmt/golangci-lint зелёные.

**Этап 3.7 — worker-telegram (написано, юнит + интеграционный на PG16 PASS, ждёт коммита):**
- **Контракт НЕ менялся.** Подписка через бота (DESIGN §3.4), не через `POST /subscribe` (тот email-only).
- **Пакет `internal/telegram`:**
  - `client.go` — минимальный Bot API клиент: `SendMessage`/`GetUpdates`/`GetMe`; общий `call` (JSON POST).
    `APIError` с классификацией: `error_code` 403/прочие 4xx → `Permanent=true` (повтор бесполезен),
    429 → `RetryAfter` (из parameters.retry_after, не перманентна), сеть/5xx → транзиентная.
  - `render.go` — `Render(RenderInput)→string` (parse_mode=HTML): incident_new/update + maintenance
    scheduled/started/completed, i18n RU/EN (симметрично email), экранирование через html.EscapeString,
    подвал — ссылка на страницу + подсказка `/stop`. **subscriber_confirm НЕ рендерится** (в Telegram
    нет double opt-in). Свой минимальный dict (не тянем зависимость от пакета email).
  - `worker.go` — `Worker.Process(ctx, body)→Disposition` (Ack/Reject/Requeue, локальные — пакеты
    каналов независимы). Зеркало email-воркера: идемпотентность по Notification.id (orphan→Ack,
    sent→Ack), парс chat_id из Address (нечисловой→Reject), рендер, отправка. **Permanent-ошибка →
    Ack (дроп)** — заблокировавшего/несуществующий чат ретраить/DLQ-ить бессмысленно; транзиентная →
    `Retrier.Retry` (=notify.Engine) → delayed-ретрай или DLQ при исчерпании; ошибка MarkSent → Requeue.
  - `bot.go` — `Bot.Run(ctx)` long-poll getUpdates (offset, timeout 30с, allowed_updates=[message];
    ошибка → лог + пауза `retryDelay` с учётом 429). `/start <slug>` → StatusPageBySlug → upsert
    `Subscriber{channel=telegram, address=chat_id, confirmed=true, scope=page}` (старт бота = согласие);
    идемпотентно (есть → «уже подписаны»). `/start` без арга / неизвестный slug → подсказка.
    `/stop <slug>` → снять одну; `/stop` → снять все подписки чата (через `SubscribersByChannelAddress`).
    Ответы на локали страницы (или языке клиента). `parseCommand` (нижний регистр, срез `@botname`).
- **store:** новый sqlc-запрос `ListSubscribersByChannelAddress` (channel+address) → метод
  `SubscribersByChannelAddress` (для `/stop` без арга — chat_id может быть подписан на много страниц).
- **cmd/worker-telegram:** один процесс совмещает (1) consume `q.telegram` (manual ack, prefetch=16,
  Process→Ack/Nack) и (2) `Bot.Run` в горутине. `GetMe` при старте — проверка токена + лог @username.
  `config.MustTelegramBotToken()` (без токена воркер не стартует). DeclareTopology идемпотентно.
- **config:** `TELEGRAM_BOT_TOKEN` (env). Dockerfile (бинарь /app/worker-telegram), docker-compose
  (сервис worker-telegram), .env.example обновлены.
- **Флаг (решение агента в рамках §5):** подписка через бота — **только на всю страницу (scope=page)**.
  Компонентная подписка отложена: deep-link start-payload Telegram ≤64 симв. и [A-Za-z0-9_-] не вмещает
  UUID компонентов; для неё нужен интерактивный выбор inline-кнопками (отдельная задача). Email-подписка
  компонентами (scope=components) при этом работает (3.5).
- **Проверено:** юнит — `client_test` (sendMessage path/body, 403→Permanent, 429→RetryAfter, getUpdates
  парс), `render_test` (incident RU, maintenance EN, escaping, ошибки), `worker_test` (happy/идемпотент/
  orphan/bad-addr/транзиент-retry/исчерпание/Permanent-дроп/malformed/unrenderable/mark-sent→requeue),
  `bot_test` (parseCommand, start подписка/идемпотент/no-arg/unknown, stop one/all/none с фейками).
  Интеграционный `bot_integration_test` на PG16 (HEALTHPAGE_TEST_DB): /start→confirmed page-подписчик,
  повтор без дубля, вторая страница → SubscribersByChannelAddress видит обе, /stop slug снимает одну,
  /stop снимает остальное. **PASS.** build/test/vet/gofmt/golangci-lint зелёные.

**Этап 3.6 — публичные фиды RSS/iCal (написано, проверено на PG16, ждёт коммита):**
- **Контракт НЕ менялся** — `GET /pages/{slug}/rss` (application/rss+xml), `GET /pages/{slug}/calendar.ics`
  (text/calendar) уже в openapi (tag Public, security[]).
- **Пакет `internal/feed`** (чистые билдеры, без БД/HTTP): `BuildRSS(page, incidents, maintenances,
  baseURL)` — RSS 2.0 через encoding/xml (экранирование штатное); инциденты+работы единым фидом,
  сортировка по дате desc, лимит 50; описание инцидента — из последнего по CreatedAt апдейта; ссылки
  baseURL+/status/<slug>/incidents/<id> и /maintenances; guid `incident:`/`maintenance:`+id.
  `BuildICal(page, maintenances, baseURL, now)` — RFC 5545: VCALENDAR + VEVENT на работу, DTSTART/DTEND/
  DTSTAMP в UTC basic (`20060102T150405Z`), `escapeText` (\\ ; , \n), фолдинг строк >75 октетов, CRLF,
  STATUS scheduled→TENTATIVE иначе CONFIRMED, UID `maintenance-<id>@healthpage`.
- **API `feed.go`** (публичные): `handleRSS`/`handleICal` через `loadPublicPage` (приватная/нет → 404) +
  `ListPublicIncidents(filter{},50,0)`/`ListPublicMaintenances(nil,50,0)`; хелпер `writeRaw`
  (Content-Type+body). Роуты в публичной группе server.go. `Deps.BaseURL`=cfg.BaseURL (main.go).
- **Флаги:** (1) фид берёт последние 50 записей (без пагинации — для фида достаточно). (2) ссылки на
  /status/<slug>/... используют BASE_URL (как и письма; в проде нужен публичный URL public-ssr —
  тот же флаг, что в 3.4). (3) iCal включает работы всех статусов (вкл. completed) из последних 50.
- **Проверено:** юнит `feed_test` (RSS parse+порядок+escaping round-trip; iCal поля/DTSTART-формат/
  escaping ;,\n/фолдинг 75/CRLF/STATUS) + интеграционный `feed_integration_test` на PG16 (RSS:
  content-type+обе записи+ссылка; iCal: content-type+VEVENT+SUMMARY; приватная страница→404).
  build/test/vet/gofmt/golangci-lint зелёные.

**Этап 3.5 — эндпоинты подписки (написано, проверено на PG16, ждёт коммита):**
- **Контракт НЕ менялся** — `POST /pages/{slug}/subscribe`, `GET /subscribe/confirm`, `GET /unsubscribe`
  уже были в openapi (security[]). Регенерация типов не нужна.
- **sqlc** (subscribers.sql): GetSubscriberByPageChannelAddress, GetSubscriberByConfirmToken (по хэшу),
  SetSubscriberConfirmToken (перевыпуск токена+scope), ConfirmSubscriber (confirmed=true, токен→NULL),
  DeleteSubscriber (hard delete) → перегенерирован db. store-методы + `ReissueConfirmToken`/
  `SubscriberByConfirmTokenHash`/`ConfirmSubscriber`/`DeleteSubscriber`/`SubscriberByPageChannelAddress`.
- **subscription** (token.go): `GenerateConfirmToken`→(plaintext, hex-SHA256-хэш), `HashConfirmToken`
  (поиск/сравнение). В БД — только хэш (§9), plaintext уходит в письмо.
- **notify** (`Engine.SendConfirmation(sub, confirmToken)`): адресная (без фан-аута) публикация письма
  `subscriber_confirm` — запись журнала + Message с `ConfirmPayload{confirm_token}` в очередь канала.
- **API** (`subscribe.go`, публичные): `handleSubscribe` — **только channel=email** (telegram/MAX/Slack —
  свои флоу 3.7–3.9, иначе 422); приватная страница → 404 (loadPublicPage); идемпотентность по
  (page,channel,address): нет → create(pending+хэш)+письмо; есть и не подтверждён → ReissueConfirmToken+
  письмо; есть и подтверждён → 202 без письма; scope=components валидирует принадлежность компонентов
  (переиспользует parseMaintenanceComponents). `handleConfirmSubscribe` — хэш токена → поиск → confirmed
  (одноразовый); невалидный/использованный → 400. `handleUnsubscribe` — ParseUnsubscribeToken(HMAC) →
  DeleteSubscriber (идемпотентно); битый → 400. `Deps.SubSecret`=cfg.SubscriptionSecret в main.go.
- **Флаги:** (1) **только email** на этом этапе (фокус 3.5 = email-подписка). (2) confirm/unsubscribe
  отвечают JSON {status:...} 200 — это API-эндпоинты, кликаются из письма в браузере; «красивую»
  HTML-страницу подтверждения/отписки (или редирект на public-ssr) можно добавить в этапе 4 (UX).
  (3) Если notifier=nil (RabbitMQ off) — подписчик создаётся pending, письмо не уходит (лог), ответ 202.
- **Проверено:** интеграционный `subscribe_integration_test` (PG16, fake-publisher ловит confirm-токен):
  subscribe→202+письмо+pending → повтор→перевыпуск+новое письмо → старый токен 400 → confirm свежим→200+
  confirmed → повтор confirm 400 → подтверждённый subscribe→202 без письма → unsubscribe(HMAC)→200+удалён →
  битый токен 400, чужой slug 404, telegram 422. build/test/vet/gofmt/golangci-lint зелёные.

**Этап 3.4 — worker-email (написано, проверено на живых PG16+RabbitMQ, ждёт коммита):**
- **Контракт НЕ менялся.** Новый пакет `internal/email` + команда `cmd/worker-email` + пакет
  `internal/subscription` (токены) + расширение config.
- **`internal/email/sender.go`:** `SMTP` (host/port/user/pass/from/tls — json-теги для page.smtp_config),
  `Sender` интерфейс; `SMTPSender` (STARTTLS через net/smtp.SendMail; неявный TLS 465 через tls.Dial+
  smtp.NewClient; PLAIN-auth) + `buildMIME` (multipart/alternative text+html, QEncode. Subject);
  `LogSender` (dev/fallback — логирует, не шлёт).
- **`internal/email/render.go`:** `Render(RenderInput)→Content{Subject,Text,HTML}` для событий
  incident_new/update, maintenance_scheduled/started/completed, **subscriber_confirm**. i18n RU/EN
  (статусы/impact/заголовки/подписи), ссылки: «открыть страницу», «отписаться», «подтвердить». HTML
  экранируется (html.EscapeString).
- **`internal/email/worker.go`:** `Worker.Process(ctx, body)→Disposition` (Ack/Reject→DLQ/Requeue).
  Идемпотентность: NotificationByID — orphan(ErrNotFound)→Ack, уже sent→Ack. Сборка: грузит страницу
  (locale/name/slug/smtp), парсит payload по событию, строит ссылки. Отправка через Sender; успех →
  MarkNotificationSent+Ack; ошибка → `Retrier.Retry` (=notify.Engine): scheduled→Ack (отложенная
  копия), исчерпано→Reject(DLQ); ошибка ретрая→Requeue. `effectiveSMTP`: page.smtp_config (+from_email)
  иначе системный.
- **`cmd/worker-email`:** store+queue, DeclareTopology (идемпотентно), Publisher+Engine(Retrier),
  Sender (SMTPSender если SMTP_HOST задан, иначе LogSender), `queue.Consume(q.email, prefetch=16,
  manual ack)` → Process→Ack/Nack. Graceful stop по сигналу.
- **`internal/subscription/token.go`:** `UnsubscribeToken`/`ParseUnsubscribeToken` — HMAC-SHA256 от
  subscriber_id (`<id>.<base64url(hmac)>`, constant-time сверка).
- **Config:** SMTP_HOST/PORT(587)/USERNAME/PASSWORD/FROM/TLS, SUBSCRIPTION_SECRET (дефолт=JWT_SECRET).
  .env.example, docker-compose (сервис worker-email), Dockerfile (бинарь /app/worker-email) обновлены.
- **Решения/флаги:** (1) **Отписка — HMAC-stateless**: колонка `subscribers.unsubscribe_token`
  НЕ используется (вестигиальна) — plaintext нельзя восстановить из хэша для каждого письма, поэтому
  токен вычисляется из subscriber_id+секрет. Возможна будущая миграция-дроп колонки (с санкции человека).
  (2) **PageURL = BASE_URL + /status/<slug>** — placeholder; в проде публичная страница на public-ssr
  (нужен отдельный PUBLIC_BASE_URL или кастом-домен). Ссылки confirm/unsubscribe → API (BASE_URL +
  /api/v1/...). (3) **SUBSCRIPTION_SECRET в api и worker должны совпадать** (для проверки токена отписки
  эндпоинтом 3.5). (4) Кастомный SMTP страницы работает только при реальном SMTPSender (в dev LogSender
  игнорирует cfg). (5) double opt-in: воркер умеет рендерить+слать confirm-письмо; **триггер (subscribe)
  и confirm/unsubscribe-эндпоинты — этап 3.5**.
- **Проверено:** юнит — `subscription/token_test` (round-trip+tamper), `email/render_test`
  (incident RU, maintenance EN, confirm, ошибки), `email/worker_test` (happy/идемпотентный-skip/orphan/
  retry-scheduled/retry-exhausted/malformed/unrenderable с фейками). Живой e2e `email/worker_integration_test`
  (PG+AMQP: engine→q.email→worker доставил, NotificationByID→sent, повтор не дублирует). build/test/vet/
  gofmt/golangci-lint зелёные.

**Этап 3.3 — движок уведомлений (написано, проверено на живых PG16+RabbitMQ, ждёт коммита):**
- **Контракт НЕ менялся** (subscribers/notifications уже в схеме с 3.1; enum-значения = openapi
  SubscriberChannel/Scope). Решение агентское в рамках §5 — стоп к человеку не требовалось.
- **Домен** `subscriber.go`: `SubscriberChannel` (+`IsValid`/`IsPush` — push только email/telegram/max/
  slack; rss/ical=pull(3.6), webhook=исходящий(5.4)), `SubscriberScope`, сущность `Subscriber` +
  `WantsEvent(affected)` (scope=page → всё; scope=components → пересечение; пустой affected при
  components → нет). `notification.go`: `NotificationStatus` (pending/sent/failed), `EventType`
  (incident_new/incident_update/maintenance_scheduled/started/completed), сущность `Notification`.
- **Store** (sqlc `subscribers.sql`/`notifications.sql` → регенерация db): `CreateSubscriber`
  (нормализует nil component_ids → '{}' — колонка NOT NULL), `SubscriberByID`, `ListConfirmedSubscribers`
  (confirmed=true; фильтрация scope — в движке); `CreateNotification`, `NotificationByID`,
  `MarkNotificationSent`(+sent_at), `MarkNotificationFailed`, `IncrementNotificationAttempts`(→новое значение).
- **Пакет `internal/notify`**: `Message` (формат тела очереди: notification_id[ключ идемпотентности §8.1],
  subscriber_id, channel, event, address, attempt, payload, status_page_id) + payload-структуры
  `IncidentPayload`/`MaintenancePayload`. `backoff.go`: `RetryBackoff(attempt)`→(1м/5м/30м, ok),
  `MaxAttempts=3`. `engine.go`: `Engine` (интерфейсы Store+Publisher; Publisher не потокобезопасен →
  публикации под mutex). `IncidentCreated/IncidentUpdated/MaintenanceEvent` → `dispatch` (фан-аут:
  список подтверждённых → фильтр IsPush+WantsEvent → CreateNotification(pending) → публикация Message
  с n.ID; ошибки публикации не прерывают, запись остаётся pending=восстановима). `Engine.Retry(msg)`
  (для воркеров 3.4+): инкремент attempts → republish через delayed.events с backoff, либо при
  исчерпании MarkFailed + (false,nil) — сигнал воркеру слать в DLQ.
- **Врезка в API** (`server.go`: `Deps.Notifier *notify.Engine` опц., хелпер `emitNotify` nil-safe,
  логирует, не валит запрос): create-инцидент → IncidentCreated (только `IsVisible && notify`),
  add-update → IncidentUpdated (захвачен полный агрегат `updated` вместо `_`), create-работа →
  MaintenanceEvent(scheduled, gated by notify), patch-работа → started/completed по фактическому
  переходу статуса (`maintenanceTransitionEvent`, PATCH без notify-флага → уведомляем всегда при переходе).
- **main.go** `setupNotifier`: движок — **мягкая зависимость** (RABBITMQ_URL пуст или брокер недоступен
  → лог + nil-движок, API работает без рассылки). Закрывает publisher+conn при shutdown.
- **Решения/упрощения (флаги):** (1) фан-аут **синхронный** в хендлере (для объёмов MVP публикация
  дёшева; при росте — фоновый sweeper по pending или outbox). (2) Скрытые инциденты не уведомляют.
  (3) Воркеры-consumer'ы (consume-loop, реальная доставка) — это 3.4+; здесь только producer + механизм
  ретраев. (4) **Recovery pending-уведомлений** (sweeper, который дочитывает не опубликованные при сбое
  publish записи) — НЕ реализован; пока ручной разбор. Добавить при необходимости.
- **Проверено:** юнит-тесты домена (`subscriber_test`: каналы/scope/WantsEvent) и движка
  (`engine_test`: фан-аут push-only+scope, routing key, retry-delayed, retry-exhausted→failed, продолжение
  при ошибке publish; `backoff_test`). Интеграционные на PG16 (`subscribers_integration_test`: confirmed-
  фильтр, component_ids round-trip, дубль(page,channel,address), журнал create/get/attempts/sent/failed,
  каскад страница→подписчик). Живой e2e (`engine_integration_test`, гейт PG+AMQP): IncidentCreated →
  сообщение в q.email с notification_id, указывающим на pending-запись, payload корректен.
  build/test/vet/gofmt/golangci-lint зелёные. Frontend не трогал.

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
- ✅ **RabbitMQ delayed-exchange плагин** — добавлен в образ (`docker/rabbitmq/Dockerfile`,
  `rabbitmq_delayed_message_exchange` 3.13.0) на этапе 3.2; топология объявляется `queue-setup`.
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
- 2026-06-29 — Этап 3.2 (RabbitMQ-топология §8.1): свой образ rabbitmq с плагином
  delayed_message_exchange (docker/rabbitmq/Dockerfile, compose build), пакет `internal/queue`
  (Conn, DeclareTopology — 4 exchange'а + q.<channel>/q.dlq.<channel>/q.webhook.out + привязки + DLX,
  Publisher с confirms, Consume с manual ack), `cmd/queue-setup` (идемпотентно). Dep amqp091-go.
  Проверено на живом брокере: топология по §8.1 (list_exchanges/queues/bindings), интеграционный тест
  (confirm/DLX/delayed) PASS. Закрыт TODO про плагин. Дальше — 3.3 (движок уведомлений).
- 2026-06-29 — Этап 3.7 (worker-telegram): пакет `internal/telegram` — Bot API клиент (sendMessage/
  getUpdates/getMe + классификация ошибок: 403/4xx Permanent, 429 RetryAfter, сеть/5xx транзиент),
  Render (RU/EN HTML-сообщения incident/maintenance), Worker.Process (зеркало email: идемпотентность по
  Notification.id, retry через notify.Engine/DLQ, Permanent→дроп), Bot.Run (long-poll: /start <slug>
  подписывает [confirmed, scope=page], /stop [slug] отписывает). cmd/worker-telegram совмещает
  consume q.telegram + бота. store: +SubscribersByChannelAddress (sqlc) для /stop. config:
  TELEGRAM_BOT_TOKEN. Dockerfile/compose/.env обновлены. Контракт НЕ менялся. Юнит + интеграционный на
  PG16 PASS; build/test/vet/gofmt/lint зелёные. **Флаг:** подписка через бота только на страницу
  (scope=page); компонентная отложена (deep-link ≤64 симв. не вмещает UUID). Дальше — 3.8 (worker-max,
  симметрично; ⚠️ ждёт оргготовности MAX) → 3.9 (slack) → 3.10 (управление подписчиками в админке).
- 2026-06-29 — **3.8 (MAX) ОТЛОЖЕН** решением человека (нет оргготовности) → перенесён на после
  Этапа 7, опционален, из acceptance запуска исключён. Этап 3.9 (Slack-канал): пакет `internal/slack`
  (OAuth Exchange code→webhook URL; Client.PostMessage с классификацией ошибок; Render Block Kit RU/EN;
  Worker.Process — зеркало telegram). API slack/start (302+signed state) + slack/callback (Exchange→
  создать confirmed slack-подписчика). `subscription.Sign/ParseSlackState` (HMAC+TTL). cmd/worker-webhook
  потребляет q.slack. config SLACK_CLIENT_ID/SECRET (пусто→404). Контракт НЕ менялся (эндпоинты+enum уже
  в openapi). Юнит + интеграционный на PG16 PASS; build/test/vet/gofmt/lint зелёные. Флаги: scope=page,
  дубли при повторном Add-to-Slack, callback=JSON. Дальше — 3.10 (управление подписчиками в админке).
- 2026-06-29 — Этап 3.10 (управление подписчиками + клиентская отписка): контракт расширен с санкции
  человека (status_page_id в GET /subscribers query + SubscriberCreate + ответ Subscriber; типы
  перегенерированы). Backend: sqlc ListSubscribersByPage, store, хендлеры list/create(confirmed=true,
  только push-каналы)/delete, роуты, DTO; интеграционный тест на PG16. Admin (FSD): entities/subscriber,
  features/subscriber-add, pages/subscribers, вкладка page-nav, роут. public-ssr: /unsubscribe (дружелюбная
  RU/EN страница, дёргает backend); ссылка отписки в письмах → public-ssr. Решения: ручное добавление
  минует double opt-in (152-ФЗ на операторе); клиентская страница — минимум (отписка). build (go/admin/
  next) + test/vet/gofmt/lint зелёные. **Этап 3 закрыт по коду** (email/telegram/slack; MAX отложен).
  Дальше — Этап 4 (кастомизация/white-label).
- 2026-06-30 — Этап 5.1 (ApiToken + аутентификация управляющих запросов): миграция 00011_api_tokens
  (БД→11); домен apitoken (scope read/write, write⊇read); security GenerateAPIToken/HashAPIToken
  (префикс hp_, хэш в БД); store api_tokens; **middleware переработан** — requireAuth принимает JWT
  (Bearer) ИЛИ page-токен (Authorization без Bearer) → principal в контексте, scope-энфорсинг по
  методу (403); authorizePage для обоих субъектов; API POST/GET/DELETE /tokens (только оператор).
  Контракт расширен с санкции человека (status_page_id в TokenCreate, TokenScope enum, GET /tokens,
  схема Token); типы перегенерированы. Юнит + интеграционный на PG16 PASS; build/test/vet/lint +
  admin build зелёные; миграция обратима. Дальше — 5.2 (полный write-API: status_page_id
  опционален при ApiToken — берётся из токена).
- 2026-06-30 — Этап 5.2 (полный write-API под page-токеном): общий `resolveManagedPage` (оператор —
  status_page_id обязателен → 422; токен — из токена, чужой → 404) на 10 list/create хендлеров;
  PATCH/DELETE/updates уже работали под токеном через authorizePage. Контракт: status_page_id убран
  из required в 4 Create-схемах (санкция человека); типы перегенерированы. Интеграционный на PG16
  (полный lifecycle инцидента + CRUD всех ресурсов под токеном без status_page_id) PASS; build/test/
  vet/lint + admin build зелёные. Дальше — 5.3 (входящие webhook'и + WebhookIntegration + HMAC).
- 2026-06-30 — Этап 5.3 (входящие webhook'и): только grafana+prometheus (generic/pagerduty → 501,
  по решению человека). Миграция 00012 (webhook_integrations + dedup-колонки в incidents,
  partial-unique по открытому); пакет internal/webhook (HMAC-подпись, парсинг Alertmanager-payload,
  маппинг на компоненты); CRUD /webhook-integrations (operator-only, секрет единожды); inbound
  handlers с HMAC + идемпотентным create/close инцидента по dedup-ключу. Контракт расширен (санкция
  человека), типы перегенерированы. Юнит + интеграционный на PG16 PASS; build/test/vet/lint + admin
  build зелёные; миграция обратима. Дальше — 5.4 (исходящие webhook'и, q.webhook.out).
- 2026-07-01 — Этап 5.4 (исходящие webhook'и): канал webhook стал доставляемым (Deliverable);
  движок роутит webhook-сообщения в webhooks.out (PublishWebhookOut), push — в notifications;
  привязка q.webhook.out←delayed.events для ретраев; пакет internal/webhookout (client/render/worker,
  payload Mattermost-совместимый); worker-webhook потребляет q.slack + q.webhook.out; POST /subscribers
  принимает channel=webhook (URL); admin +webhook в каналах. Контракт НЕ менялся. Юнит + интеграц.
  PG16 + живой e2e PG+RabbitMQ PASS; build/test/vet/lint + admin зелёные. **Этап 5 закрыт по коду**
  (5.1–5.4; inbound generic/pagerduty отложены, 501). Дальше — Этап 6 (биллинг) по приоритету человека.
- 2026-07-01 — **Этап 6 (Биллинг) закрыт по коду.** Контракт `/billing/*` уже был — не менялся.
  Миграция 00013 (subscriptions/payments). Пакет internal/billing (Provider + StubProvider +
  YooKassaProvider + Service: checkout/webhook/рекуррент/dunning/cancel). API `/billing/*` + webhook.
  worker-billing (периодический цикл). Feature-flags (6.7): `domain.PlanAllows` + гейтинг premium-фич
  в handlePatchPage/allowed-emails → 403. Оферта public-ssr `/offer`. Админка «Тариф». Решения
  человека: полный объём; реальная ЮKassa/цены/чеки — [ВЕРНУТЬСЯ], цены плейсхолдер. Юнит+интеграц.
  PG16 + живой smoke PASS; build/test/lint + фронты зелёные. Дальше — Этап 7.
- 2026-07-01 — **Этап 7 закрыт по коду** (кроме отложенного). 7.1 uptime (домен ComputeUptime,
  under_maintenance исключён; API + полоса на public-ssr); 7.2 changelog (контракт расширен;
  миграция 00014; админка+public-ssr «Релизы»); 7.3 **только /metrics** (Prometheus, решение человека);
  7.5 каркас импорта (миграция 00015, ImportJob/external_id_map/Importer/q.import/worker-import,
  контракт: ImportRequest.status_page_id); 7.6 адаптер **StatusPal** (только он); 7.9 UI импорта.
  Отложено человеком: 7.4 бэкапы + 7.7/7.8 Instatus/Статусмейт — прод/позже. Dep prometheus/client_golang.
  Юнит+интеграц. PG16 + живой smoke (metrics/uptime/changelog/import) PASS; всё зелёное. Дальше — деплой.
- 2026-07-01 — **Деплой/CI-CD.** Модель (решения человека): self-hosted PaaS **Dokploy** на одном
  РФ-VPS; отдельные приложения + managed Postgres/Redis Dokploy (бэкапы → закрывает 7.4); RabbitMQ —
  приложение; образы CI→GHCR, Dokploy тянет по вебхуку; ingress/TLS — Traefik. Артефакты: Dockerfile'ы
  фронтов (public-ssr standalone; admin nginx, контекст=корень репо), `docker-compose.prod.yml`+
  `Caddyfile` (Приложение B — ручная альтернатива), `.github/workflows/deploy.yml` (build+push GHCR +
  триггер вебхуков Dokploy), **DEPLOY.md** (полный runbook Dokploy). Провайдер РФ — [РЕШИТЬ]; сервер у
  человека есть. Провалидировано (compose config, Caddy validate, actionlint, сборка образов фронтов).
  Дальше — практическая установка Dokploy по DEPLOY.md.
- 2026-07-21/22 — **Прод развёрнут.** Через claude-in-chrome настроены и продеплоены в Dokploy:
  исправлен source type (был ошибочно nixpacks/GitHub у части приложений → Docker), Command-override
  для api+5 воркеров, домены (healthpage.ru+www→public-ssr:3000, app.healthpage.ru→admin:80+/api→
  api:8080, api.healthpage.ru→api:8080) с HTTPS/Let's Encrypt, 2 новых one-off приложения (`migrate`,
  `queue-setup`). Задеплоены по порядку migrate→queue-setup→api→(public-ssr,admin,worker-email,
  worker-webhook,worker-billing,worker-import); все живы по логам. `worker-telegram` оставлен
  недеплоенным (нет токена). Найден и исправлен баг: обе ссылки лендинга ведут на `/status/demo`,
  которой не было в чистой БД (404) — создана демо-страница вручную через API (нет seed-механизма в
  коде, зафиксировано в «Текущий статус» как TODO на будущее). Дальше — по приоритету человека: SMTP
  для worker-email, TELEGRAM_BOT_TOKEN, боевые ключи ЮKassa, либо возврат к стоп-маркерам биллинга/
  импорта/кастомных доменов.
- 2026-07-22 — **Стоп-маркер «кастомные домены» (4.3) снят.** Развилка решена человеком: вместо
  своего edge/tls-manager (конфликтует с Traefik на Dokploy) — интеграция с Dokploy API. Новый
  пакет `internal/dokploy` (CreateDomain/DeleteDomain, x-api-key, fallback на byApplicationId при
  нестабильной схеме ответа create — обнаружено через Swagger Dokploy, ответы `{}`-untyped).
  Миграция 00016 (`dokploy_domain_id`), sqlc-запрос `SetDokployDomainID`. `handleVerifyDomain`
  подключает домен при первой успешной верификации (idempotent); `handlePatchPage` отвязывает
  старый при смене/снятии (best-effort, не блокирует операцию при ошибке Dokploy). Контракт
  openapi.yaml НЕ менялся. Юнит-тесты клиента (httptest) + новый интеграционный тест на PG16
  (create/idempotent/delete через фейковый Dokploy-сервер) PASS; build/vet/lint + полный `go test
  ./...` зелёные. Не проверено на реальном клиентском домене — сделать перед стартом продаж
  white-label. DEPLOY.md §8 переписан (инструкция по `DOKPLOY_API_TOKEN`/`DOKPLOY_PUBLIC_SSR_APP_ID`).
  `cmd/edge`/`cmd/tls-manager` остаются в репо неиспользуемыми. Дальше — по приоритету человека.
- 2026-07-22 — **Пост-деплой проверка + 2 находки.** (1) Человек прогнал CI/CD (push→build→GHCR→
  webhook Dokploy) для коммита с кастомными доменами — подтверждено рабочим (Actions API: все джобы
  success; контейнер `api` пересоздан с новым digest, старый образ потерял тег `latest`). Кажущаяся
  «трёхнедельная давность» в GitHub Packages UI — не проблема пайплайна, просто неинформативное
  отображение там. (2) Обнаружено и исправлено: `migrate`/`queue-setup` зацикливались (перезапуск
  каждые ~3с часами) — Dokploy разворачивает их как Swarm `Replicated` с 1 репликой, и Swarm сам
  поддерживает число реплик независимо от Restart Policy (`None` не помогает; `Job`-режима в UI
  Dokploy нет). Фикс — вручную жать **Stop** после того, как одноразовая задача отработала
  (задокументировано в DEPLOY.md §8 как обязательный шаг после каждого их Deploy). (3) Обнаружено,
  не устранено: `cname.healthpage.ru` не резолвится (нет A-записи на `201.51.12.191`) — кастомные
  домены клиентов не заработают физически, пока её не завести у регистратора. DEPLOY.md дополнен
  §8.1 — пошаговый ручной прогон проверки кастомных доменов на своём тестовом домене.
- 2026-07-22 — **Найден и исправлен системный баг: BASE_URL конфликтовал со сплит-доменной схемой
  прода.** Код изначально написан под однодоменную схему (compose+Caddy), где один origin
  обслуживал и API, и public-ssr. На Dokploy — 3 разных домена (`api.healthpage.ru`,
  `healthpage.ru`, `app.healthpage.ru`), а `BASE_URL` использовался как «единственный публичный
  origin для всего» — из-за этого ссылки в письмах (страница/приватный доступ/отписка),
  RSS/iCal-фидах и уведомлениях Telegram/Slack/webhook вели на `api.healthpage.ru`, где таких
  маршрутов нет (реальному ущербу не привело, т.к. worker-email пока в dev-режиме логирования).
  Заодно баг с задвоенным путём `/billing/billing/stub-confirm` в checkout — та же причина.
  **Исправлено:** новые конфиги `PUBLIC_URL` (origin public-ssr) и `ADMIN_URL` (origin админки),
  разведены с `BASE_URL` (теперь только для API-ссылок). `internal/email/worker.go` разделён на
  `publicURL`/`apiURL`; `internal/{telegram,slack,webhookout}/worker.go` переведены на `publicURL`;
  `internal/api/feed.go` (RSS/iCal) — на `s.publicURL`; `internal/api/billing.go` — `s.adminURL`.
  `StubProvider.BaseURL` (ранее неиспользуемое поле) теперь реально используется для confirm-URL
  (не завязан на `ReturnURL`, убрано задвоение пути). Добавлена реальная dev-страница подтверждения
  оплаты — `frontend/admin/src/pages/billing/StubConfirmPage.tsx` (`/billing/stub-confirm`,
  зовёт `POST /billing/webhook/yookassa` сама, без ручного curl). Заодно поправлена кнопка
  лендинга «Начать бесплатно» → `{ADMIN_URL}/login?mode=register` (добавлена поддержка
  `?mode=register` в `AuthForm`, раньше такого deep-link'а не было); «Посмотреть пример» — как и
  было, на `/status/demo`. Контракт openapi.yaml НЕ менялся (сигнатуры Go-функций — внутренние).
  build/vet/lint + полный `go test ./...` (вкл. PG16) + оба фронта (`tsc`+`vite build`,
  `next build`) — зелёные. Требует новых env в проде: `api`+`worker-email`+`worker-telegram`+
  `worker-webhook` → `PUBLIC_URL`/`ADMIN_URL` (детали DEPLOY.md §5); `admin`/`public-ssr` уже
  собраны с новым кодом, но фронты нужно передеплоить после CI. Проверено вручную: полный флоу
  checkout→webhook (stub) на demo-аккаунте — `billing_plan` встал в `premium`.
- 2026-07-22 — **Подтверждено: платформенной суперадминки (кросс-аккаунтный доступ ко всем
  клиентам) в проекте нет и не планировалась в MVP** (DESIGN §2 — роли только внутри одной
  страницы: Owner/Admin/Editor/Viewer; ни ROADMAP.md, ни API, ни admin-фронт не содержат ничего
  похожего на `/platform`/`/admin/accounts`). Сейчас доступ к данным всех клиентов — только через
  прямой доступ к БД. Если понадобится реальным клиентам — отдельная фича, не заложена нигде.
- 2026-07-22 — **GitHub-секрет `DOKPLOY_WEBHOOKS` отсутствовал вообще** (не было заведено ни разу)
  — этим объясняется, почему автодеплой по пушу молча не срабатывал (workflow сам пишет «не
  задан — пропуск» и выходит 0, поэтому джоба всегда «зелёная», даже когда ничего не задеплоилось).
  Человек завёл секрет. Собран и передан человеку полный список webhook-URL 7 приложений
  (api/worker-email/worker-webhook/worker-billing/worker-import/public-ssr/admin);
  `worker-telegram` в список намеренно не включён (недеплоен, нет токена — деплой по вебхуку
  уронил бы его в тот же цикл рестартов, что раньше был у migrate/queue-setup).
- 2026-07-22 — **Прод-инцидент при первой попытке верификации кастомного домена клиента**
  (`status-test.livedigital.space`, DNS настроен человеком верно, `dig` с его машины проходил) —
  verify упорно возвращал `domain_verified=false`. Диагностика (с explicit-разрешением человека:
  Docker Terminal в `admin`, `/bin/sh`, `nslookup`) показала: дефолтный резолвер контейнера
  (127.0.0.11 → апстрим VPS-хостера) отдаёт NXDOMAIN на свежесозданную CNAME, тогда как прямой
  запрos к 8.8.8.8 резолвит верно. **Не баг кода** — резолвер хоста отстаёт/держит негативный кэш.
  **Исправлено системно** (не разово): `internal/api/dns_resolver.go` — верификация CNAME теперь
  всегда идёт напрямую в 8.8.8.8/1.1.1.1, в обход /etc/resolv.conf контейнера (иначе КАЖДЫЙ новый
  клиент рано или поздно упёрся бы в то же самое, требуя объяснять, почему его `dig` работает, а
  наша верификация — нет). `chainResolvers` — определённый NXDOMAIN возвращается сразу, к
  следующему серверу переходим только при сетевой ошибке. Юнит-тесты (4 сценария, без реальной
  сети). build/vet/lint + полный `go test ./...` зелёные. DEPLOY.md §8 дополнен.
- 2026-07-22 — **Прод-инцидент: `DOKPLOY_API_URL` на публичный IP давал `context deadline
  exceeded`.** Hairpin NAT VPS-хостера — контейнер `api` не достучался до публичного IP той же
  машины. **Исправлено:** везде указывать внутреннее имя `dokploy` (Swarm embedded DNS, порт 3000),
  не IP/домен сервера. DEPLOY.md §8 дополнен явным предупреждением. Заодно добавлено логирование
  ошибки `dokploy.CreateDomain` (`log.Printf`) для диагностики будущих таких случаев.
- 2026-07-22 — **Роутинг корня кастомного домена на статус-страницу (часть 4.3.3, без edge-прокси).**
  Раньше это делал бы `cmd/edge` (не используется в проде — Traefik/Dokploy сам терминирует TLS и
  шлёт трафик прямо в `public-ssr`), поэтому корень кастомного домена клиента открывал бы лендинг
  healthpage.ru, а не его статус-страницу. **Контракт расширен:** публичный `GET /pages/by-domain
  ?domain=` → `{slug}` (только для verified-доменов, иначе 404); openapi провалиден. Backend
  `handleGetSlugByDomain` (page_domain.go) — тонкая обёртка над уже существующим
  `store.SlugByCustomDomain`. Frontend: новый `frontend/public-ssr/middleware.ts` — матчится только
  на `/` (не трогает внутренние ссылки `/status/<slug>/...`), сверяет `Host` с `PUBLIC_URL`-хостом
  (+`www`+`localhost` — свои), иначе резолвит через `/pages/by-domain` и **rewrite** (не redirect)
  на `/status/{slug}`; при недоступном API/404 — тихо отдаёт лендинг как есть (не блокирует).
  Интеграционный тест на PG16 расширен (`by-domain`: неверифицированный→404, верифицированный→slug,
  неизвестный→404, после снятия домена→404 снова). build/test(PG16)/vet/gofmt/lint зелёные.
- 2026-07-22 — **Проверено сейчас: DNS-стоп-маркер снят.** `dig +short cname.healthpage.ru` →
  `201.51.12.191` (A-запись у регистратора заведена человеком после находки выше — раньше была
  пустой). Кастомные домены клиентов (verify + фактический трафик через CNAME) теперь физически
  работают, а не только проходят валидацию. `status-test.livedigital.space` резолвится корректно
  через CNAME. **Живой e2e подтверждён человеком (2026-07-22):** `status-test.livedigital.space`
  открывает статус-страницу клиента (не лендинг) — `middleware.ts`-роутинг корня кастомного домена
  работает в проде. **Этап 4.3 полностью закрыт, включая живую проверку.**
- 2026-07-22 — **7.6 StatusPal: адаптер сверен на живом read-only ключе клиента** (`statuspal.eu`,
  subdomain `edu-livedigital-space`, реальный аккаунт с историей) — не только по документации, как
  раньше. Найдено и исправлено **несколько реальных расхождений** со старой реализацией (которая
  была написана по докам и не смогла бы распарсить настоящий ответ сервера):
  - **Обёртки ответа:** все list-эндпоинты оборачивают массив в объект (`{"services":[...]}`,
    `{"incidents":[...],"links":{...},"meta":{...}}`, `{"subscriptions":[...]}`) — раньше код
    декодировал прямо в голый срез, упал бы на первом же вызове.
  - **Дерево сервисов:** `GET /services` отдаёт ТОЛЬКО корневые узлы (`parent_id=null`) с
    `children_ids` (просто id, без вложенных объектов) — полное дерево строится дозапросом
    `GET /services/{id}` рекурсивно по каждому id. Раньше код ожидал плоский список с `parent_id`
    у всех узлов сразу — не увидел бы дочерние сервисы вообще.
  - **`/maintenances` не существует** (реальный 404 на живом ключе) — работы это тот же ресурс
    `/incidents` с фильтром `?type=scheduled` (подтверждено: `type=major`+`type=minor` в сумме дают
    `total_count` обычных инцидентов). `FetchMaintenances` переписан на переиспользование
    `spIncident`/пагинации инцидентов; описание берётся из первого update (у инцидентов нет
    отдельного поля body).
  - **Subscriptions.id — строка (UUID)**, не число, как было в старом `spSubscription.ID int`
    (упало бы на decode). Поле `type` различает канал (email/sms/slack/...) — импортируются только
    `type=="email"` (уже было решение по ImportedSubscriber, теперь реально фильтруется).
  - **Пагинация** — курсорная, `links.next` отдаёт готовый абсолютный URL (иногда даже с опечаткой
    версии `/api/v1/` в пути — подтверждено, что это баг форматирования на стороне StatusPal,
    ответ по факту в формате v2; код просто переходит по URL как есть).
  - **Реальные значения `update.type`** в истории клиента: `issue`/`update`/`deescalate`/`resolved`
    (документация API их не перечисляет). У StatusPal нет прямых аналогов нашего
    investigating/identified/monitoring — маппинг (`spIncidentStatus`) это **решение агента**:
    issue→investigating, deescalate→monitoring, escalate→identified, resolved→resolved,
    остальное (в т.ч. `update`)→identified.
  - Юнит-тесты `statuspal_test.go` (httptest, все сценарии по реальным формам ответа: дерево
    сервисов, слияние major+minor, пагинация по links.next, maintenances через type=scheduled без
    обращения к несуществующему пути, фильтр email-подписчиков, 401). build/vet/gofmt/golangci-lint
    + `go test ./internal/importer/...` зелёные.
  - **Флаг снят частично:** схема services/incidents/subscribers подтверждена на реальных данных.
    **Остаётся неподтверждённым:** плановые работы (`type=scheduled`) — на тестовом аккаунте их 0,
    поля предполагаются идентичными обычным инцидентам (starts_at/ends_at/service_ids), но вживую не
    видел. Сверить при первом реальном импорте клиента, у которого есть хотя бы одна плановая
    работа в StatusPal, прежде чем снимать `[ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ ИМПОРТА]` полностью.
