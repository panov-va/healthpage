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
| worker-max | Доставка MAX | `worker-max` | — | не создан |
| worker-webhook | Slack-подписка + исходящие webhook | `worker-webhook` | — | не создан |
| worker-billing | Рекуррентные списания | `worker-billing` | — | не создан |
| worker-import | Миграция данных | `worker-import` | — | не создан |

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

### MAX (worker-max)
| Переменная | Назначение |
|-----------|-----------|
| `MAX_BOT_TOKEN` | токен бота MAX (получить после верификации самозанятого + модерации) |
| `MAX_API_BASE` | домен API MAX (свериться с dev.max.ru на момент реализации) |

### Slack (worker-webhook / OAuth)
| Переменная | Назначение |
|-----------|-----------|
| `SLACK_CLIENT_ID` `SLACK_CLIENT_SECRET` | OAuth-приложение «Add to Slack» |
| `SLACK_SIGNING_SECRET` | проверка подписи (если потребуется) |

### Платежи (worker-billing / ЮKassa)
| Переменная | Назначение |
|-----------|-----------|
| `YOOKASSA_SHOP_ID` `YOOKASSA_SECRET_KEY` | credentials ЮKassa |
| `YOOKASSA_WEBHOOK_SECRET` | проверка подписи webhook |

### Кастомные домены / TLS
| Переменная | Назначение |
|-----------|-----------|
| (ACME/Let's Encrypt настройки — заполнить на этапе 4) | автонастройка TLS для CNAME клиентов |

---

## 4. Внешние credentials (что нужно получить и где)

> Чек-лист внешних ключей. Сами значения — в `.env`, не здесь.

- [ ] **Telegram Bot** — токен через @BotFather.
- [ ] **MAX Bot** — профиль самозанятого на платформе MAX (верификация через Госуслуги),
      создание бота, модерация, затем токен. ⚠️ Организационная зависимость, запускать заранее.
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

> Заполнить при подготовке к запуску. Сейчас — план.

- On-prem **не делаем**, только SaaS (DESIGN §12).
- Rate-limiting — на уровне инфраструктуры (ingress / API-gateway), не в коде. В будущем
  кластере — ingress-контроллер (nginx ingress / Kong / APISIX). Для MVP — преждевременно.
- Публичная часть (`public-ssr`) деплоится отдельно от админки и должна быть доступнее, чем
  продукт клиента (DESIGN §9).
- CI/CD — _TBD_ (на этапе 0 завести сборку/тесты; деплой-пайплайн — позже).

---

## 7. Наблюдаемость / эксплуатация

> Этап 7. Заполнить по факту.

- Метрики: Prometheus — _TBD_.
- Логи: _TBD_.
- Трейсинг: _TBD_.
- Собственная статус-страница сервиса (HealthPage о самом себе) — _TBD_.
