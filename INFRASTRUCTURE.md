# INFRASTRUCTURE.md — операционный runbook HealthPage

> **Назначение.** Этот файл описывает, как система **развёрнута и запускается** — в отличие от
> `DESIGN.md §8`, который описывает, как она **спроектирована** (логика и связи сервисов).
> Здесь — порты, переменные окружения, внешние credentials, команды запуска, dev vs prod.
> Если меняется логическая архитектура — правится DESIGN.md; если меняется развёртывание/конфиг —
> правится этот файл. **Дублировать диаграмму сервисов из DESIGN.md сюда НЕ нужно.**
>
> **Статус:** наполняется по мере разработки (с этапа 0). Пока кода нет — большинство разделов
> заглушки, агент заполняет их фактическими значениями по ходу.

---

## 0. Правила работы с этим файлом (для агента)

- Сюда пишем **факт развёртывания**: env-переменные, порты, имена docker-сервисов, volume'ы,
  команды запуска, какие внешние ключи нужны и куда кладутся.
- **НИКОГДА не записывать сами значения секретов** (токены, пароли, ключи API) — ни здесь, ни
  в git. Здесь — только **имена** переменных и **где** взять/положить значение (`.env`).
- `.env` — в `.gitignore`. В репозитории — `.env.example` с пустыми/примерными значениями.
- При добавлении нового сервиса/воркера/зависимости — добавить его сюда (порт, env, запуск).
- Держать актуальным: устаревшие порты/переменные удалять.

---

## 1. Локальный запуск (dev)

```
cp .env.example .env          # заполнить значения локально (файл в .gitignore)
docker compose up -d --build  # postgres, redis, rabbitmq, api (или: make up)

# миграции (goose) — внутри контейнера api или с хоста с заданным DATABASE_URL:
docker compose exec api /app/migrate up        # применить
docker compose exec api /app/migrate status    # статус
docker compose exec api /app/migrate down      # откатить одну
# с хоста (нужен DATABASE_URL, см. .env): make migrate-up / migrate-status / migrate-down

# топология RabbitMQ (exchange'и/очереди/DLQ/delayed — DESIGN §8.1), идемпотентно:
docker compose run --rm --entrypoint /app/queue-setup api
# (--entrypoint обязателен: образ api имеет ENTRYPOINT=/app/api, иначе путь уйдёт аргументом в api;
#  требует RABBITMQ_URL; внутри сети compose хост брокера — rabbitmq:5672)

# проверка:
curl http://localhost:8080/healthz   # -> 200 {"status":"ok"}
```

> **RabbitMQ-образ собирается локально** из `docker/rabbitmq/Dockerfile` (базовый
> `rabbitmq:3.13-management-alpine` + плагин `rabbitmq_delayed_message_exchange` для отложенных
> событий). Сборка плагина требует доступа в интернет (`docker compose build rabbitmq`).
> Топология объявляется командой `queue-setup` (см. выше); сами воркеры-потребители — этап 3.4+.

Сборка/тесты/линт без docker: `make build`, `make test`, `make lint` (см. `make help`).
Кодогенерация: `make gen` = типы из openapi.yaml (`gen-go` + `gen-ts`) **и** store-код из
SQL-запросов (`gen-sqlc`). sqlc локально на macOS требует установленного бинаря `sqlc`
(сборка через `go run` падает на cgo); на CI (linux) `go run` работает.

- Требования к машине разработчика:
  - **Docker + Docker Compose** (проверено: Docker 27.x, Compose v2.29).
  - **Go 1.24+** — go.mod требует toolchain go1.25.7; Go сам подтянет нужный toolchain.
  - **Node 18+** (проверено на 18.19) и npm 10 — для сборки фронтов и генерации TS-типов.
- Если host-порты 5432/6379/5672/15672 уже заняты другими сервисами на машине —
  переопредели публикацию портов локальным `compose.override.yml` (не коммитить) или
  останови конфликтующие сервисы. Внутренняя сеть compose от host-портов не зависит.

---

## 2. Сервисы и порты (dev)

> Таблица наполняется по мере появления сервисов. Колонка «docker-сервис» = имя в compose.

| Сервис | Назначение | docker-сервис | Порт (dev) | Статус |
|--------|-----------|---------------|-----------|--------|
| api | REST API (Go, chi) | `api` | 8080 | каркас (этап 0) |
| public-ssr | Публичные страницы + лендинг (Next.js) | `public-ssr` | 3000 (dev) | каркас, не в compose |
| admin | Админка (React SPA, Vite) | `admin` | 5173 (dev) | каркас, не в compose |
| postgres | БД (источник истины) | `postgres` | 5432 | в compose |
| redis | Кэш публичной сводки | `redis` | 6379 | в compose |
| rabbitmq | Очередь уведомлений/webhook | `rabbitmq` | 5672 / 15672 (UI) | в compose (свой образ: +delayed-плагин) |
| worker-email | Доставка email (SMTP) | `worker-email` | — | создан (3.4), в compose |
| worker-telegram | Доставка Telegram + бот подписки | `worker-telegram` | — | создан (3.7), в compose |
| worker-max | Доставка MAX | `worker-max` | — | ⏸ отложен (после Этапа 7) |
| worker-webhook | Доставка Slack (Block Kit) + исходящие webhook | `worker-webhook` | — | создан (3.9, q.slack), в compose |
| worker-billing | Рекуррентные списания + dunning | `worker-billing` | — | создан (6.4), в compose |
| worker-import | Миграция данных (q.import; MVP: StatusPal) | `worker-import` | — | создан (7.5/7.6), в compose |
| tls-manager | ACME-выпуск/продление TLS кастом-доменов | `tls-manager` | — | создан (4.3.2), compose-профиль `edge` |
| edge | Прокси кастом-доменов: TLS по SNI + HTTP-01 + роутинг | `edge` | 80/443 | создан (4.3.3), compose-профиль `edge` |

> Логические связи сервисов и очередей — в `DESIGN.md §8` и `§8.1`, не дублировать здесь.

---

## 3. Переменные окружения

> Только **имена** и назначение. Значения — в `.env` (не в git). Заполнять по мере появления
> сервисов. Поддерживать в синхроне с `.env.example`.

### Общие / БД / инфраструктура
| Переменная | Назначение |
|-----------|-----------|
| `DATABASE_URL` | строка подключения PostgreSQL (для api и migrate) |
| `REDIS_URL` | подключение Redis |
| `RABBITMQ_URL` | подключение RabbitMQ |
| `APP_ENV` | `dev` / `prod` |
| `HTTP_PORT` | порт HTTP-сервера api (дефолт 8080) |
| `BASE_URL` | базовый публичный URL сервиса |
| `POSTGRES_USER` `POSTGRES_PASSWORD` `POSTGRES_DB` | инициализация контейнера postgres (dev) |
| `RABBITMQ_DEFAULT_USER` `RABBITMQ_DEFAULT_PASS` | инициализация контейнера rabbitmq (dev) |

### Auth
| Переменная | Назначение |
|-----------|-----------|
| `JWT_SECRET` | секрет подписи операторских access-JWT (HS256). Обязателен для запуска api |
| `ACCESS_TTL` | TTL access-токена (Go duration, дефолт `15m`) |
| `REFRESH_TTL` | TTL refresh-токена (Go duration, дефолт `720h` = 30 дней) |

### Email (worker-email)
| Переменная | Назначение |
|-----------|-----------|
| `SMTP_HOST` `SMTP_PORT` `SMTP_USER` `SMTP_PASS` | дефолтный SMTP сервиса |
| (кастомный SMTP страницы хранится в БД, не в env) | per-page override |

### Telegram (worker-telegram)
| Переменная | Назначение |
|-----------|-----------|
| `TELEGRAM_BOT_TOKEN` | токен бота от @BotFather; без него воркер не стартует |

> Подписка — через бота, не через `POST /subscribe`: пользователь открывает
> `t.me/<bot>?start=<slug>` → команда `/start <slug>` создаёт подписчика на страницу (подтверждён
> сразу). `/stop [<slug>]` — отписка. Один процесс совмещает доставку из `q.telegram` и long-poll
> бота. MVP: подписка только на всю страницу (scope=page).

### MAX (worker-max) — ⏸ ОТЛОЖЕНО (после Этапа 7)
| Переменная | Назначение |
|-----------|-----------|
| `MAX_BOT_TOKEN` | токен бота MAX (получить после верификации самозанятого + модерации) |
| `MAX_API_BASE` | домен API MAX (свериться с dev.max.ru на момент реализации) |

### Slack (worker-webhook / OAuth, этап 3.9)
| Переменная | Назначение |
|-----------|-----------|
| `SLACK_CLIENT_ID` `SLACK_CLIENT_SECRET` | OAuth-приложение «Add to Slack»; задаются для **api**. Пусто → эндпоинты подписки Slack отвечают 404 |

> redirect_uri Slack App = `<BASE_URL>/api/v1/subscribe/slack/callback` (выводится из BASE_URL).
> Подписка — через OAuth: «Add to Slack» (`/pages/{slug}/subscribe/slack/start`) → выбор канала →
> callback сохраняет `Subscriber{channel=slack, address=<incoming-webhook URL>}`. Доставка —
> `worker-webhook` (потребляет `q.slack`, шлёт Block Kit в этот URL); **секреты OAuth воркеру не
> нужны**. MVP: подписка на всю страницу (scope=page).

### Биллинг / тарифы (api + worker-billing, этап 6)
| Переменная | Назначение |
|-----------|-----------|
| `PREMIUM_MONTHLY_MINOR` | цена Premium в месяц, копейки (дефолт 99000 = 990 ₽). **Плейсхолдер** |
| `PREMIUM_YEARLY_DISCOUNT_PCT` | скидка годовой подписки, % (дефолт 20) |
| `BILLING_CURRENCY` | валюта (дефолт RUB) |
| `TRIAL_DAYS` | длительность триала (дефолт 14; флоу с картой-на-файле — после боевой ЮKassa) |
| `BILLING_SCAN_INTERVAL` | период цикла worker-billing (дефолт 1h) |
| `BILLING_MAX_DUNNING` | число неуспешных списаний до отката на Free (дефолт 3) |
| `BILLING_RETRY_INTERVAL` | пауза между попытками списания в past_due (дефолт 72h) |
| `YOOKASSA_SHOP_ID` `YOOKASSA_SECRET_KEY` | credentials ЮKassa (api + worker-billing). **Пусто → stub-провайдер (dev, без реальных денег)** |

> **Цена — плейсхолдер** (финализировать перед запуском: цена Статусмейта −10%, DESIGN §10).
> Эффективный флаг тарифа — `accounts.billing_plan`; его включает webhook успешной оплаты, откат на
> free делает `worker-billing`. Webhook ЮKassa (`POST /api/v1/billing/webhook/yookassa`) — публичный,
> подлинность по IP-allowlist провайдера (не HMAC), ограничить на ingress.
>
> **[ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ БИЛЛИНГА]:** реальные ключи ЮKassa, согласование рекуррентов с менеджером,
> боевые списания и фискальные чеки («Мой налог», режим НПД) **локально не проверены** — на прод-деплое
> (sandbox→prod). Публичная оферта `/offer` — черновик, финализировать с юристом.

### Кастомные домены / TLS (этап 4.3)
| Переменная | Сервис | Назначение |
|-----------|--------|-----------|
| `CNAME_TARGET` | api | Хост, на который клиент направляет CNAME своего домена; verify сверяет резолв. Дефолт `cname.healthpage.ru` |
| `ACME_EMAIL` | tls-manager | Контактный email Let's Encrypt. Пусто → tls-manager не стартует |
| `ACME_DIRECTORY_URL` | tls-manager | ACME-директория; для тестов — staging `https://acme-staging-v02.api.letsencrypt.org/directory` |
| `ACME_RENEW_INTERVAL` / `ACME_RENEW_BEFORE` | tls-manager | Период цикла (12h) / порог продления (30d) |
| `EDGE_API_URL` / `EDGE_SSR_URL` | edge | Origin'ы api и public-ssr (в сети compose — имена сервисов) |
| `EDGE_HTTP_ADDR` / `EDGE_HTTPS_ADDR` | edge | Адреса :80 / :443 |

> **Поток:** клиент ставит `status.его-домен` CNAME → `CNAME_TARGET`; оператор жмёт «Проверить домен»
> (`POST /pages/{id}/domain/verify` → `domain_verified=true`). `tls-manager` периодически выпускает/
> продлевает серты для verified-доменов (ACME HTTP-01: challenge кладётся в БД, `edge` отдаёт на :80),
> хранит их в `domain_certificates`. `edge` терминирует TLS по SNI (серт из БД), роутит `/api/*` → api,
> корень кастом-домена → `/status/{slug}`, прочее → public-ssr. Запуск: `docker compose --profile edge up`.
>
> **[ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ КАСТОМНЫХ ДОМЕНОВ]:** выпуск реальных сертификатов и HTTPS-доступ по
> домену **не проверены локально** (нужны публичный DNS, открытые :80/:443, доступность Let's Encrypt).
> Проверить на прод-деплое: сначала на ACME staging, затем prod. Серты — в БД (не на диске).

---

## 4. Внешние credentials (что нужно получить и где)

> Чек-лист внешних ключей. Сами значения — в `.env`, не здесь.

- [ ] **Telegram Bot** — токен через @BotFather.
- [ ] **MAX Bot** — ⏸ ОТЛОЖЕНО (разработка 3.8 перенесена на после Этапа 7). Профиль самозанятого
      на платформе MAX (верификация через Госуслуги), создание бота, модерация, затем токен.
- [ ] **Slack App** — создать приложение, OAuth scope `incoming-webhook`, прямая ссылка
      «Add to Slack» (без публикации в App Directory для MVP).
- [ ] **ЮKassa** — подключить как самозанятый (через «Мой налог»), согласовать рекурренты с
      менеджером (скриншот отвязки карты + оценка оборота), получить Shop ID и секретный ключ.
- [ ] **Домен** healthpage.ru — DNS, основной сертификат.

---

## 5. Хранилища и данные

| Что | Где (dev) | Где (prod) | Примечание |
|-----|-----------|-----------|-----------|
| Основная БД | postgres в compose | _TBD_ | источник истины |
| Кэш | redis в compose | _TBD_ | публичная сводка |
| Очередь | rabbitmq в compose | _TBD_ | DLQ, delayed exchange |
| Загрузки (логотипы/favicon) | _TBD_ | _TBD_ | object storage? решить на этапе 4 |
| Бэкапы БД | — | _TBD_ | PITR — этап 7 |

**[РЕШИТЬ при выходе в prod]** Где размещается прод: данные должны храниться в РФ (152-ФЗ).
Кандидаты — российские облака (Yandex Cloud, VK Cloud, Selectel и т.п.). Заполнить при
подготовке prod-окружения.

---

## 6. Деплой / prod

> **Полный процесс деплоя и CI/CD — в `DEPLOY.md`.** Здесь — краткая сводка.

- On-prem **не делаем**, только SaaS (DESIGN §12).
- **Модель (решение человека):** self-hosted PaaS **Dokploy** на одном VPS в РФ (152-ФЗ).
  Компоненты — отдельные приложения; Postgres/Redis — **managed-БД Dokploy** (с бэкапами в S3 —
  закрывает 7.4); RabbitMQ — приложение из своего образа; ingress/TLS — **Traefik** (встроен).
  Образы **собираются в GitHub CI и пушатся в GHCR**, Dokploy тянет их по deploy-вебхуку.
- **CD** (`.github/workflows/deploy.yml`): push в `master` → CI → build+push 4 образов в GHCR →
  триггер вебхуков Dokploy. Полный runbook — **`DEPLOY.md`** (Dokploy — основной путь; compose+Caddy+SSH —
  Приложение B, готовый ручной вариант).
- **Прод-образы:** `healthpage-backend` (api + все воркеры + migrate/queue-setup, distroless),
  `healthpage-rabbitmq` (+delayed-плагин), `healthpage-public-ssr` (Next standalone),
  `healthpage-admin` (nginx+SPA). Dockerfile'ы: `backend/Dockerfile`, `docker/rabbitmq/Dockerfile`,
  `frontend/*/Dockerfile` (admin собирается с контекстом = корень репо ради `shared/api-types`).
- Rate-limiting — на уровне ingress (Traefik/будущий gateway), не в коде MVP.
- Публичная часть (`public-ssr`) деплоится отдельным образом от админки (DESIGN §9).
- Секреты — в Environment приложений Dokploy + GitHub Secret `DOKPLOY_WEBHOOKS`; список — DEPLOY.md §5, A.
- **[ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ КАСТОМНЫХ ДОМЕНОВ]:** `edge`/`tls-manager` конфликтуют с Traefik по :443 —
  предпочтительно перевести кастомные домены на Traefik on-demand TLS (DEPLOY.md §8).

---

## 7. Наблюдаемость / эксплуатация

- **Метрики (этап 7.3): готово в коде.** `api` отдаёт Prometheus-метрики на `GET /metrics`
  (вне `/api/v1`, без авторизации): дефолтные `go_*`/`process_*` + прикладные
  `healthpage_http_requests_total{method,route,code}` и `healthpage_http_request_duration_seconds`
  (метка `route` — шаблон chi, не конкретный URL). Скрейпить Prometheus с `api:8080/metrics`.
- **[РЕШЕНО человеком, на прод] Grafana/дашборды/вывод статуса, бэкапы — прод-задачи**, не MVP-код:
  - Дашборды и алерты — Grafana поверх Prometheus (`/metrics`). Завести на прод-деплое.
  - **Бэкапы БД (7.4):** PITR PostgreSQL + снапшоты + регулярная проверка восстановления —
    средствами прод-БД/облака (не в коде). Настроить перед запуском.
  - **Собственная статус-страница сервиса** (HealthPage о самом себе) — завести на проде (можно
    отдельным инстансом HealthPage или внешним пробером; вывести туда состояние + метрики).
  - Логи/трейсинг — на прод-инфраструктуре (сбор логов контейнеров; трейсинг — при необходимости).

> **Импорт (этап 7.5/7.6):** только **StatusPal** (адаптеры Instatus/Статусмейт — 7.7/7.8 — отложены).
> Ключ источника (`api_key`) **не хранится** в БД: передаётся воркеру в сообщении `q.import`.
> **[ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ ИМПОРТА]:** (1) сверить схему StatusPal API v2 на живом ключе
> (структуры/пути/маппинг статусов в `internal/importer/statuspal.go` — по документации, не проверены);
> (2) 152-ФЗ: импортированные email создаются `confirmed=false` (повторный opt-in) — пересмотреть с юристом.
