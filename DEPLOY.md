# DEPLOY.md — деплой и CI/CD HealthPage

> Как приложение разворачивается в проде и как устроен конвейер «закоммитил → развернулось».
> Логика/архитектура — в `DESIGN.md`; порты/env/сервисы — в `INFRASTRUCTURE.md`; здесь — **процесс
> деплоя**. Значения секретов НИ здесь, ни в git — только имена и где взять (CLAUDE.md §2).

---

## 1. Модель деплоя (MVP)

Выбрана простая, подходящая MVP модель (не «на вырост»):

- **Один сервер (VPS) в РФ** (152-ФЗ: ПД в России), Ubuntu 22.04+, Docker + Docker Compose.
- **Образы** собираются в CI и хранятся в **GHCR** (GitHub Container Registry). На сервере — только
  `docker compose pull && up` (сервер ничего не собирает).
- **Ingress — Caddy**: терминирует HTTPS (автоматический Let's Encrypt), роутит по доменам.
- **CD — GitHub Actions**: `push в main → CI (тесты/линт) → Deploy (сборка образов + SSH-деплой)`.

```
 git push main
      │
      ▼
 ┌─────────┐   success   ┌──────────────────────────┐   ssh    ┌──────────────────────┐
 │  CI      │ ─────────► │ Deploy: build+push GHCR   │ ───────► │  VPS (РФ)             │
 │ test/lint│            │ (backend, rabbitmq, ssr,  │          │  compose pull+migrate │
 └─────────┘            │  admin)                    │          │  + up (Caddy/api/...) │
                        └──────────────────────────┘          └──────────────────────┘
```

**Альтернативы (не сейчас):** managed k8s (Yandex/VK) — для масштаба; PaaS — быстрее, но 152-ФЗ и
кастомные домены усложняют. MVP держим на одном VPS + compose.

### [РЕШИТЬ] Провайдер и сервер
Выбрать РФ-провайдера и создать VPS. Кандидаты: **Yandex Cloud / VK Cloud / Selectel / Timeweb**.
Рекомендация для старта: VPS 2 vCPU / 4 GB / 40+ GB SSD (postgres+redis+rabbitmq+api+воркеры+2 фронта+caddy).
Данные (postgres volume) — на диске в РФ; бэкапы — там же/в РФ-объектном хранилище.

---

## 2. Первичный провижининг сервера (один раз)

```bash
# 1) Docker + compose-plugin
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER   # перелогиниться

# 2) Каталог деплоя (совпадает с секретом DEPLOY_PATH)
sudo mkdir -p /opt/healthpage && sudo chown $USER /opt/healthpage
cd /opt/healthpage

# 3) Заполнить ./.env (см. §4) — секреты только здесь, НЕ в git
nano .env
```

CD сам кладёт на сервер `docker-compose.prod.yml` и `Caddyfile` при каждом деплое (scp). Первый
деплой можно инициировать вручную из вкладки **Actions → Deploy → Run workflow**, либо запустить
локально скопировав файлы и выполнив шаги из §5.

---

## 3. DNS

A-записи на IP сервера (значения доменов должны совпадать с `.env`):

| Запись | Тип | Значение | Назначение |
|--------|-----|----------|-----------|
| `healthpage.ru` | A | IP сервера | лендинг + публичные страницы (public-ssr) |
| `www.healthpage.ru` | A | IP сервера | редирект/алиас |
| `app.healthpage.ru` | A | IP сервера | админка (SPA) + `/api/*` |
| `api.healthpage.ru` | A | IP сервера | публичный REST API (опционально) |
| `cname.healthpage.ru` | A | IP edge | цель CNAME кастомных доменов клиентов (этап 4.3, позже) |

Caddy выпустит TLS автоматически, когда домены зарезолвятся на сервер.

---

## 4. Переменные окружения сервера (`/opt/healthpage/.env`)

Наследует `.env.example` (dev-дефолты) + **прод-специфичные**. Все секреты — сильные случайные значения.

```dotenv
# --- Реестр образов и деплой ---
REGISTRY=ghcr.io/<owner>          # префикс образов GHCR (owner = организация/пользователь GitHub)
TAG=latest                        # деплой переопределяет на git sha; для ручного up — latest

# --- Домены / TLS (Caddy) ---
PUBLIC_DOMAIN=healthpage.ru
ADMIN_DOMAIN=app.healthpage.ru
API_DOMAIN=api.healthpage.ru
ACME_EMAIL=ops@healthpage.ru      # контакт Let's Encrypt

# --- Общие ---
APP_ENV=prod
BASE_URL=https://api.healthpage.ru   # публичный origin API (ссылки в письмах/OAuth/редиректах)
HTTP_PORT=8080

# --- PostgreSQL (сильный пароль!) ---
POSTGRES_USER=healthpage
POSTGRES_PASSWORD=<strong-random>
POSTGRES_DB=healthpage
DATABASE_URL=postgres://healthpage:<strong-random>@postgres:5432/healthpage?sslmode=disable

# --- Redis / RabbitMQ ---
REDIS_URL=redis://redis:6379/0
RABBITMQ_DEFAULT_USER=healthpage
RABBITMQ_DEFAULT_PASS=<strong-random>
RABBITMQ_URL=amqp://healthpage:<strong-random>@rabbitmq:5672/

# --- Auth (обязателен) ---
JWT_SECRET=<long-random>
SUBSCRIPTION_SECRET=<long-random>   # ДОЛЖЕН совпадать между api и воркерами

# --- Email (SMTP), Telegram, Slack, ЮKassa, биллинг, домены ---
# см. INFRASTRUCTURE.md §3 — задать перед включением соответствующих фич.
```

> `SUBSCRIPTION_SECRET` одинаков во всех процессах (иначе не сойдутся токены отписки/доступа/magic-link).
> Пустой `TELEGRAM_BOT_TOKEN` → worker-telegram не стартует: либо задать токен, либо убрать сервис.

---

## 5. GitHub Secrets (Settings → Secrets and variables → Actions)

CD использует (значения задаёт человек):

| Secret | Назначение |
|--------|-----------|
| `DEPLOY_HOST` | IP/хост сервера (SSH) |
| `DEPLOY_USER` | SSH-пользователь (в группе docker) |
| `DEPLOY_SSH_KEY` | приватный SSH-ключ для доступа на сервер |
| `DEPLOY_PATH` | каталог деплоя, напр. `/opt/healthpage` |

GHCR-логин в CI использует встроенный `GITHUB_TOKEN` (права `packages: write`) — отдельный секрет
не нужен. Пул приватных образов на сервере в шаге деплоя логинится тем же токеном.

> Если репозиторий приватный, а сервер должен тянуть образы — токен пробрасывается в SSH-шаг
> (`GHCR_TOKEN`). Для долгоживущего доступа можно вместо этого сделать пакеты públic или завести
> отдельный read-only PAT и логинить им сервер.

---

## 6. Что делает конвейер (`.github/workflows/deploy.yml`)

1. Триггер: успешное завершение workflow **CI** на `main` (или ручной `workflow_dispatch`).
2. **build** (matrix): собирает и пушит в GHCR 4 образа с тегами `<sha>` и `latest`:
   `healthpage-backend`, `healthpage-rabbitmq`, `healthpage-public-ssr`, `healthpage-admin`.
3. **deploy**: scp `docker-compose.prod.yml` + `Caddyfile` на сервер; по SSH:
   - `docker login ghcr.io`, `docker compose pull` (тег = git sha);
   - **миграции** `migrate up` (аддитивные — до перезапуска api);
   - `docker compose up -d` + идемпотентный `queue-setup`; `docker image prune`.

Downtime при MVP-деплое — секунды (пересоздание контейнеров). Zero-downtime — позже (не в MVP).

---

## 7. Откат

Образы тегируются по git sha. Откат — задеплоить предыдущий тег:

```bash
cd /opt/healthpage
export REGISTRY=ghcr.io/<owner> TAG=<предыдущий-sha>
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

Миграции аддитивны и назад не откатываются автоматически (при необходимости — `migrate down`
вручную, осознанно). Не деплоить одновременно несовместимую схему и старый код.

---

## 8. Бэкапы (этап 7.4 — прод)

- **PostgreSQL:** ежедневный `pg_dump` в РФ-объектное хранилище + снапшоты диска провайдера;
  регулярная проверка восстановления. Volume `pgdata` — источник истины.
- RabbitMQ/Redis — не критичны (очереди/кэш восстановимы), спец-бэкап не нужен.

---

## 9. Кастомные домены клиентов (профиль `edge`)

`edge` + `tls-manager` (этап 4.3) обслуживают HTTPS на доменах клиентов (TLS по SNI из БД).
**[ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ КАСТОМНЫХ ДОМЕНОВ]:** `edge` и `caddy` оба слушают :443 — на одном
сервере это конфликт. Варианты: (а) единый ingress — перевести кастомные домены на Caddy on-demand TLS
(эндпоинт-проверка допустимости домена), отказавшись от edge; (б) вынести edge на отдельный сервер/порт;
(в) фронтить всё через edge (добавить ему роутинг основных доменов). Решить перед включением профиля.

---

## 10. Мониторинг (этап 7.3 — прод)

`api` отдаёт метрики на `api:8080/metrics` (внутренняя сеть; наружу через `API_DOMAIN` закрыто).
Prometheus на сервере скрейпит `api:8080/metrics`; Grafana — дашборды/алерты. Собственная
статус-страница сервиса — отдельным инстансом/пробером.

---

## 11. Чек-лист перед первым прод-запуском (стоп-маркеры)

- [ ] Провайдер РФ выбран, VPS создан, `.env` заполнен сильными секретами.
- [ ] GitHub Secrets заданы (`DEPLOY_*`); DNS настроен; Caddy выпустил TLS.
- [ ] **Биллинг:** реальные ключи ЮKassa, согласование рекуррентов, финальные цены; оферта финализирована.
- [ ] **Импорт:** сверить StatusPal API v2 на живом ключе; 152-ФЗ по импортированным подписчикам (opt-in).
- [ ] **Кастомные домены:** решён единый ingress (§9), проверен выпуск TLS.
- [ ] Бэкапы БД настроены и проверено восстановление.
- [ ] Первый оператор зарегистрирован; smoke основных сценариев на проде.
```
