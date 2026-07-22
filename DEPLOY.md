# DEPLOY.md — деплой и CI/CD HealthPage (Dokploy)

> Как приложение разворачивается в проде и как устроен конвейер «закоммитил → развернулось».
> Логика/архитектура — в `DESIGN.md`; порты/env/сервисы — в `INFRASTRUCTURE.md`. Секреты — НИ здесь,
> ни в git: только имена и где взять (CLAUDE.md §2).
>
> **Выбранная модель (решения человека):** self-hosted PaaS **Dokploy** на одном РФ-VPS;
> компоненты — **отдельные приложения**, Postgres/Redis — **managed-БД Dokploy** (с их бэкапами),
> RabbitMQ — приложение из своего образа; образы **собираются в GitHub CI и пушатся в GHCR**,
> Dokploy тянет их по вебхуку. Ingress/TLS — **Traefik** (встроен в Dokploy).
> Ручная альтернатива (compose + Caddy + SSH) — в Приложении B.

---

## 1. Модель деплоя

```
 git push master
      │
      ▼
 ┌────────┐ success ┌────────────────────────────┐  push  ┌──────────┐
 │  CI    │ ───────►│ build+push образов          │ ─────► │  GHCR    │
 │test/lint│        │ (backend, rabbitmq, ssr,adm) │        └────┬─────┘
 └────────┘         └──────────────┬──────────────┘             │ pull (:latest)
                                   │ webhook                     ▼
                                   └───────────────────► ┌──────────────────────────┐
                                                         │  VPS (РФ): Dokploy        │
                                                         │  Traefik (TLS) + apps + БД│
                                                         └──────────────────────────┘
```

**Почему Dokploy:** TLS, деплой из реестра, бэкапы БД, логи, откат, роутинг доменов — из коробки,
через UI, без ручных скриптов. Traefik (встроен) хорошо ложится на будущие кастомные домены клиентов.
CI (тесты/линт/сборка образов) остаётся на GitHub — Dokploy отвечает только за выкатку и БД.

**Требования к серверу:** рекомендую **2 vCPU / 4 GB (лучше 6 GB) / 40+ GB SSD**, Ubuntu 22.04+.
Dokploy сам ест ~0.3–0.5 GB — на 4 GB работает, но с 6 GB спокойнее (control-plane + стек + сборки нет,
т.к. образы из GHCR). Данные (managed Postgres) — том на диске в РФ (152-ФЗ).

---

## 2. Установка Dokploy (один раз)

```bash
# На сервере (root):
curl -sSL https://dokploy.com/install.sh | sh
```

- Открыть UI `http://<IP>:3000`, создать администратора. **Закрыть UI наружу** (firewall/VPN)
  или повесить на отдельный домен с TLS — это control-plane.
- Traefik ставится автоматически и занимает :80/:443.

> Версии Dokploy отличаются по названиям полей UI — ниже описаны шаги по смыслу; сверяйтесь с UI.

---

## 3. Managed-БД Dokploy

Создать через **Databases → Create**:

| БД | Тип | Назначение | Бэкапы |
|----|-----|-----------|--------|
| PostgreSQL 16 | managed | источник истины | **включить бэкап в S3 по расписанию** (закрывает задачу 7.4) |
| Redis 7 | managed | кэш публичной сводки | не критично |

RabbitMQ в каталоге managed-БД Dokploy нет — он разворачивается как **приложение** (§4, свой образ
с delayed-плагином). После создания каждой БД Dokploy покажет **внутренний host, порт, пароль** —
они пойдут в `DATABASE_URL` / `REDIS_URL` приложений (§5).

---

## 4. Приложения (Docker image из GHCR)

Для каждого — **Create Application**, затем на вкладке **General** выбрать источник **Docker**
(один из табов рядом с GitHub/GitLab/Bitbucket/Gitea/Git/Drop — НЕ Nixpacks/Build Type, это для
сборки из исходников и здесь не используется). В открывшейся форме — поле **Docker Image**
(`ghcr.io/<owner>/healthpage-...:latest`) и опционально **Registry URL / Username / Password**
(авторизация GHCR — см. §7, можно оставить пустыми, если сервер уже авторизован через `docker login`).
Плюс своя вкладка **Environment** и (где нужно) **Domains**. Все приложения — в одном проекте
Dokploy, чтобы были в общей сети и видели друг друга по внутренним именам.

| Приложение | Образ (`${REGISTRY}` = `ghcr.io/<owner>`) | Домен (Traefik) | Особое |
|-----------|-------------------------------------------|-----------------|--------|
| `rabbitmq` | `${REGISTRY}/healthpage-rabbitmq:latest` | — | внутр.; env RABBITMQ_DEFAULT_USER/PASS |
| `api` | `${REGISTRY}/healthpage-backend:latest` | `api.healthpage.ru` + `app.healthpage.ru` path `/api` | миграции — отдельным one-off приложением (§6); порт 8080 |
| `worker-email` | `…/healthpage-backend:latest` | — | **Command** (Advanced → General): `/app/worker-email` |
| `worker-telegram` | `…/healthpage-backend:latest` | — | Command `/app/worker-telegram` (нужен TELEGRAM_BOT_TOKEN) |
| `worker-webhook` | `…/healthpage-backend:latest` | — | Command `/app/worker-webhook` |
| `worker-billing` | `…/healthpage-backend:latest` | — | Command `/app/worker-billing` |
| `worker-import` | `…/healthpage-backend:latest` | — | Command `/app/worker-import` |
| `public-ssr` | `${REGISTRY}/healthpage-public-ssr:latest` | `healthpage.ru`, `www.healthpage.ru` | порт 3000 |
| `admin` | `${REGISTRY}/healthpage-admin:latest` | `app.healthpage.ru` (path `/`) | порт 80 (nginx) |

> **Один образ `healthpage-backend` — семь приложений** (api + 6 воркеров), различаются только
> полем **Command** на вкладке **Advanced → General** (проверено по исходникам Dokploy: это поле
> маппится в `ContainerSpec.Command`, т.е. **заменяет** ENTRYPOINT образа, а не дописывается к нему).
> У `api` поле Command оставить пустым (дефолтный ENTRYPOINT `/app/api`). Значение вида
> `/app/worker-email` (без пробелов) достаточно — Dokploy сам разобьёт строку на массив.

---

## 5. Внутренняя сеть и переменные окружения

Приложения одного проекта видят друг друга по **внутренним именам** (Dokploy/Traefik network).
`api` и воркеры обращаются к БД/брокеру по хостам, которые Dokploy показал при создании managed-БД
и приложения `rabbitmq`. Заполняются в **Environment** каждого приложения:

```dotenv
APP_ENV=prod
BASE_URL=https://api.healthpage.ru
# Хосты — из Dokploy (managed-БД и приложения rabbitmq); НЕ localhost.
DATABASE_URL=postgres://<user>:<pass>@<pg-host>:5432/healthpage?sslmode=disable
REDIS_URL=redis://<redis-host>:6379/0
RABBITMQ_URL=amqp://<user>:<pass>@<rabbitmq-host>:5672/
JWT_SECRET=<long-random>
SUBSCRIPTION_SECRET=<long-random>   # ДОЛЖЕН быть ОДИНАКОВ в api и ВСЕХ воркерах
# Биллинг/SMTP/Telegram/Slack/ЮKassa/домены — по мере включения фич (INFRASTRUCTURE §3).
```

- `public-ssr`: `HEALTHPAGE_API_URL=http://<api-host>:8080/api/v1` (внутренний адрес api-приложения).
- `admin`: env не нужны (статика; API проксирует Traefik, см. §6).
- **`SUBSCRIPTION_SECRET` одинаков во всех 7 backend-приложениях** — иначе не сойдутся токены
  отписки/доступа/magic-link. Совет: держать общий набор env в переменных проекта Dokploy (Shared),
  если версия поддерживает, либо аккуратно продублировать.

---

## 6. Домены, TLS и роутинг (Traefik)

Traefik в Dokploy выпускает TLS (Let's Encrypt) автоматически при добавлении домена приложению.
Для DNS — A-записи на IP сервера: `healthpage.ru`, `www`, `app.`, `api.` (+ `cname.healthpage.ru`
для кастомных доменов клиентов позже).

- `public-ssr` → домены `healthpage.ru`, `www.healthpage.ru`, контейнерный порт **3000**.
- `admin` → домен `app.healthpage.ru`, path `/`, порт **80**.
- `api` → домен `api.healthpage.ru` (порт **8080**) **и** второй домен `app.healthpage.ru` с path
  **`/api`** — чтобы относительные запросы админки (`/api/v1/...`) уходили в api. (Path-роут в Dokploy:
  добавить домен с указанием path; Traefik разрулит `app.*/api/*`→api, `app.*/*`→admin.)
- `/metrics` наружу не публикуем (Prometheus скрейпит api по внутренней сети, §10).

### Миграции — one-off приложения (в Dokploy нет хука «Pre-Deploy command»)
Проверено по исходникам Dokploy: отдельного «pre-deploy»-шага перед стартом контейнера там нет
(есть только «Run Command» — выполнение команды в уже запущенном контейнере, для отладки, и
«Schedules» — команда по cron). Миграции — это **отдельное одноразовое приложение**:
- Создать приложение `migrate`: источник **Docker**, тот же образ `${REGISTRY}/healthpage-backend:latest`,
  на вкладке **Advanced → General → Command** указать `/app/migrate up`. Env — только `DATABASE_URL`.
  Домен не нужен. Перед каждым релизом со сменой схемы — **Deploy** (или Redeploy) этого приложения
  вручную; миграции идемпотентны (goose), схема меняется редко.
- Топологию RabbitMQ — так же: приложение `queue-setup`, Command `/app/queue-setup`, env
  `RABBITMQ_URL`, один раз (идемпотентно, повторные запуски безвредны).

---

## 7. GitHub → GHCR → Dokploy (CI/CD)

1. **CI** (`.github/workflows/ci.yml`, уже есть): на push/PR — тесты/линт/сборка фронтов (гейт).
2. **Deploy** (`.github/workflows/deploy.yml`): после успешного CI на `master` → собирает и пушит в
   GHCR 4 образа (`healthpage-backend|rabbitmq|public-ssr|admin`, теги `<sha>`+`latest`) → **дёргает
   deploy-вебхуки Dokploy** (POST на каждый URL) → Dokploy тянет `:latest` и передеплоивает.

**Что настроить:**
- **Доступ к приватным GHCR-образам** — два равноценных варианта (Dokploy `Settings → Registry` для
  этого НЕ подходит: тот каталог — для регистри, куда Dokploy сам пушит собранные образы, к pull'у
  готовых образов отношения не имеет):
  1. **На каждом приложении** (вкладка Docker) заполнить Registry URL=`ghcr.io`, Username=GitHub-логин,
     Password=PAT с `read:packages` — Dokploy сам сделает `docker login` перед `pull`.
  2. **Один раз на сервере**: `docker login ghcr.io -u <логин> -p <PAT>` (по SSH, от имени пользователя,
     под которым Dokploy дёргает Docker) — тогда поля Username/Password в приложениях можно оставить
     пустыми, `docker pull` сработает по сохранённым в Docker credentials на хосте.
  Либо сделать пакеты публичными в GitHub Packages — тогда авторизация не нужна вовсе.
- **Вебхуки приложений:** у каждого приложения в Dokploy включить Auto-Deploy / получить Deploy Webhook
  URL. Собрать URL всех приложений, которые надо обновлять на релизе (api + 6 воркеров + 2 фронта +
  rabbitmq по необходимости), в **GitHub Secret `DOKPLOY_WEBHOOKS`** (через пробел/перенос строки).
- Секрет `GITHUB_TOKEN` в CI (встроенный) даёт `packages: write` — пуш в GHCR без доп. секретов.

> `api` и воркеры используют один образ `healthpage-backend` — на релизе дёргаются вебхуки каждого
> из них (все подтянут новый `:latest`). Порядок нестрогий; миграции — до api (§6).

Откат: образы тегируются по git sha — в Dokploy переключить приложение на конкретный `…:<sha>` и
передеплоить (или повторно запустить прошлый релиз).

---

## 8. Бэкапы, мониторинг, кастомные домены

- **Бэкапы (7.4):** managed-Postgres Dokploy → расписание бэкапа в S3-совместимое РФ-хранилище;
  периодически проверять восстановление. Redis/RabbitMQ — не критичны.
- **Мониторинг (7.3):** `api` отдаёт `/metrics` (внутр. сеть). Dokploy даёт логи/базовые метрики
  контейнеров в UI; для дашбордов — Prometheus+Grafana поверх (скрейп api по внутреннему хосту).
- **Кастомные домены клиентов — решено (2026-07-22):** свой `cmd/edge`+`cmd/tls-manager` в проде
  на Dokploy **не используется** (конфликтовал бы с Traefik на :80/:443 на той же машине). Вместо
  этого при успешной верификации CNAME (`POST /pages/{id}/domain/verify`) бэкенд сам вызывает
  Dokploy API (`POST /domain.create` с `applicationId` приложения `public-ssr`, `port:3000`,
  `https:true`, `certificateType:letsencrypt`) — дальше Traefik и Let's Encrypt обслуживает домен
  клиента сам Dokploy. При смене/снятии домена бэкенд аналогично вызывает `POST /domain.delete`.
  Реализация — `backend/internal/dokploy` (клиент), подключается через `POST /pages/{id}` →
  `handleVerifyDomain` (`backend/internal/api/page_domain.go`).
  - **Настройка:** Dokploy → Profile → API/CLI Keys → Generate New Key. Прописать в env `api`:
    `DOKPLOY_API_URL` (`http://<host>:3000/api`), `DOKPLOY_API_TOKEN` (сам ключ, вписывается
    вручную человеком — не через агента), `DOKPLOY_PUBLIC_SSR_APP_ID` (ID приложения `public-ssr`
    в Dokploy, виден в URL его страницы). Без `DOKPLOY_API_TOKEN` домен остаётся только
    `domain_verified=true` без реального подключения (не ошибка, просто интеграция выключена).
  - `cmd/edge`/`cmd/tls-manager` код в репозитории оставлен (на случай ухода с Dokploy), но не
    входит в прод-деплой.
  - Не проверено на реальном клиентском домене (только юнит+интеграционные тесты с фейковым
    Dokploy API) — стоит один раз проверить на тестовом поддомене перед тем, как включать фичу
    платным клиентам.

---

## 9. Чек-лист перед первым прод-запуском

- [ ] Dokploy установлен, UI закрыт наружу; Traefik держит :80/:443; DNS настроен.
- [ ] Managed Postgres + Redis созданы; бэкап Postgres в S3 включён и проверен.
- [ ] `rabbitmq` + `api` + 6 воркеров + `public-ssr` + `admin` подняты из GHCR; env заполнены;
      `SUBSCRIPTION_SECRET` одинаков во всех backend-приложениях.
- [ ] Домены привязаны, TLS выпущен; `app.*/api` роутится в api; `/metrics` не публичен.
- [ ] Миграции: `migrate up` (Pre-Deploy или one-off) прошёл; `queue-setup` выполнен один раз.
- [ ] GHCR-доступ в Dokploy настроен; `DOKPLOY_WEBHOOKS` в GitHub Secrets; тестовый push выкатился.
- [ ] **Биллинг:** ключи ЮKassa, рекурренты согласованы, цены финальны, оферта готова.
- [ ] **Импорт:** StatusPal API сверен на живом ключе; 152-ФЗ по импортированным подписчикам (opt-in).
- [ ] Первый оператор зарегистрирован; smoke основных сценариев.

---

## Приложение A. GitHub Secrets

| Secret | Назначение |
|--------|-----------|
| `DOKPLOY_WEBHOOKS` | список deploy-вебхуков приложений Dokploy (пробел/перенос строки) |
| `GITHUB_TOKEN` | встроенный, `packages: write` для пуша в GHCR — задавать не нужно |

(SSH-секреты `DEPLOY_*` нужны только для альтернативы из Приложения B.)

---

## Приложение B. Альтернатива без Dokploy (compose + Caddy + SSH)

Если откажетесь от Dokploy — в репозитории есть готовый ручной путь (собран и провалидирован):
`docker-compose.prod.yml` (весь стек, образы из GHCR) + `Caddyfile` (ingress, авто-TLS) и вариант
`deploy.yml` с SSH-деплоем (scp compose+Caddyfile → `docker login` → `compose pull` → `migrate up` →
`up` → `queue-setup`). Нужны GitHub Secrets `DEPLOY_HOST/USER/SSH_KEY/PATH` и server `.env`
(REGISTRY, домены, ACME_EMAIL, все секреты). Этот путь прозрачнее и без control-plane, но без
бэкапов/логов/отката «из коробки» — их пришлось бы настраивать отдельно.
```
