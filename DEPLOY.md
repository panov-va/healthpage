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
PUBLIC_URL=https://healthpage.ru
ADMIN_URL=https://app.healthpage.ru
# Хосты — из Dokploy (managed-БД и приложения rabbitmq); НЕ localhost.
DATABASE_URL=postgres://<user>:<pass>@<pg-host>:5432/healthpage?sslmode=disable
REDIS_URL=redis://<redis-host>:6379/0
RABBITMQ_URL=amqp://<user>:<pass>@<rabbitmq-host>:5672/
JWT_SECRET=<long-random>
SUBSCRIPTION_SECRET=<long-random>   # ДОЛЖЕН быть ОДИНАКОВ в api и ВСЕХ воркерах
# Биллинг/SMTP/Telegram/Slack/ЮKassa/домены — по мере включения фич (INFRASTRUCTURE §3).
```

- **`BASE_URL` vs `PUBLIC_URL` vs `ADMIN_URL` — три разных домена, не путать (найдено 2026-07-22):**
  `BASE_URL` — origin самого API (для API-ссылок вроде `/api/v1/subscribe/confirm`). `PUBLIC_URL` —
  origin `public-ssr` (для ссылок в письмах/RSS/iCal/Telegram/Slack/webhook на страницы статуса,
  отписку, приватный доступ). `ADMIN_URL` — origin админки (для возврата после оплаты в биллинге).
  Раньше везде ошибочно использовался только `BASE_URL` — в сплит-доменной схеме (3 разных домена)
  это вело на несуществующие в API маршруты. Нужны на `api` и на `worker-email`, `worker-telegram`,
  `worker-webhook` (используются оба воркера — Slack и исходящие webhook'и).
- `public-ssr`: `HEALTHPAGE_API_URL=http://<api-host>:8080/api/v1` (внутренний адрес api-приложения),
  `ADMIN_URL=https://app.healthpage.ru` (кнопка «Начать бесплатно» на лендинге).
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
    `DOKPLOY_API_URL`, `DOKPLOY_API_TOKEN` (сам ключ, вписывается вручную человеком — не через
    агента), `DOKPLOY_PUBLIC_SSR_APP_ID` (ID приложения `public-ssr` в Dokploy, виден в URL его
    страницы). Без `DOKPLOY_API_TOKEN` домен остаётся только `domain_verified=true` без реального
    подключения (не ошибка, просто интеграция выключена).
  - **`DOKPLOY_API_URL` — только внутренний адрес `http://dokploy:3000/api`, НЕ публичный IP/домен
    VPS.** Прод-инцидент (2026-07-22): `DOKPLOY_API_URL=http://<публичный-IP>:3000/api` вызывал
    `context deadline exceeded` на каждом `POST /domain.create` — контейнер `api` не может
    достучаться до публичного IP той же машины, на которой сам работает (hairpin NAT, типичное
    ограничение сетей облачных VPS). Диагностировано через Docker Terminal контейнера: запрос на
    публичный IP:3000 подвисал и падал по таймауту, запрос на `http://dokploy:3000` (внутреннее имя
    сервиса Dokploy в Swarm-сети, резолвится через embedded DNS) отвечал мгновенно. Фикс — везде
    указывать внутреннее имя `dokploy` (порт Dokploy по умолчанию — 3000), а не IP/домен сервера.
  - **`CNAME_TARGET` должен реально резолвиться:** verify сравнивает CNAME клиента со значением
    `CNAME_TARGET` (дефолт `cname.healthpage.ru`) чисто текстово — но чтобы трафик клиента
    физически доходил до сервера, у самого `CNAME_TARGET` должна быть A-запись на IP сервера
    (`201.51.12.191`). Без неё домен будет `domain_verified=true` и подключён в Dokploy, но не
    откроется в браузере (цепочка CNAME клиента упрётся в несуществующий хост). Завести один раз
    у РФ-регистратора домена healthpage.ru: `cname.healthpage.ru A 201.51.12.191`.
  - `cmd/edge`/`cmd/tls-manager` код в репозитории оставлен (на случай ухода с Dokploy), но не
    входит в прод-деплой.
  - **Верификация CNAME идёт напрямую в публичные DNS (8.8.8.8/1.1.1.1), не через резолвер
    хоста** (`internal/api/dns_resolver.go`, решение принято 2026-07-22 после прод-инцидента):
    резолвер VPS (Docker embedded DNS → апстрим хостера) может ещё не видеть свежесозданную CNAME
    клиента (NXDOMAIN/негативный кэш) даже когда публичные DNS уже отдают верный ответ — иначе
    клиенту пришлось бы объяснять, почему его `dig` проходит, а наша верификация — нет. Определённый
    отрицательный ответ (реально нет такой CNAME) возвращается сразу; на следующий сервер переходим
    только при сетевой ошибке/таймауте. Требование: сервер должен иметь исходящий доступ к :53
    у 8.8.8.8/1.1.1.1 (обычно есть по умолчанию; если хостер режет исходящий DNS кроме своего
    резолвера — верификация не будет работать, значит нужны другие публичные резолверы в
    `publicDNSServers`).
  - Проверено вручную на реальном клиентском домене (`status-test.livedigital.space`,
    2026-07-22) — до фикса резолвера верификация не проходила (резолвер хоста отдавал NXDOMAIN),
    сам DNS-домена был настроен верно.
- **Одноразовые приложения (`migrate`, `queue-setup`) зацикливаются после запуска — известная
  особенность Dokploy.** Dokploy разворачивает их как Swarm-сервис в режиме `Replicated` с 1
  репликой: после того как процесс завершается (успешно, exit 0), Swarm сам поддерживает
  заданное число реплик и почти сразу пересоздаёт задачу — независимо от Restart Policy (пробовал
  `None`, не помогло: `Replicated`/`Global` — единственные режимы, доступные в UI Dokploy, режима
  `Job` там нет). **Правило:** после каждого `Deploy` этих двух приложений — дождаться в логах
  `successfully migrated`/`no migrations to run` (`migrate`) или `RabbitMQ topology declared`
  (`queue-setup`), затем нажать **Stop**. Иначе они будут перезапускаться каждые несколько
  секунд бесконечно (не ломает данные — обе операции идемпотентны — но впустую грузит сервер).

### 8.1. Как проверить кастомные домены (ручной прогон на тестовом домене)

Нужен домен, которым вы реально управляете (свой домен/поддомен с доступом к DNS). Прогон
end-to-end:

1. **Предусловие (один раз):** убедиться, что `cname.healthpage.ru` резолвится на сервер —
   `dig +short cname.healthpage.ru` должен вернуть `201.51.12.191`. Если пусто — завести A-запись
   у регистратора (см. пункт выше) и подождать её распространения (`dig` до положительного результата).
2. Зайти в админку HealthPage (`https://app.healthpage.ru`) под аккаунтом с тарифом **Premium**
   (кастомные домены — платная фича, DESIGN §10; демо-аккаунту тариф не хватит — либо повысить
   его billing_plan напрямую в БД для теста, либо завести отдельный тестовый аккаунт/страницу).
3. В настройках своей страницы статуса задать **собственный домен**, например
   `status.ваш-домен.ru`.
4. У себя в DNS (регистратор/DNS-провайдер вашего домена) добавить запись:
   `status.ваш-домен.ru CNAME cname.healthpage.ru`.
5. Подождать распространения DNS (`dig +short status.ваш-домен.ru CNAME` должен показать
   `cname.healthpage.ru.`) — может занять от минут до часа в зависимости от TTL.
6. В админке нажать **«Проверить домен»** (или `POST /pages/{id}/domain/verify` с токеном) —
   должно появиться `domain_verified: true`. Если Dokploy интеграция настроена (`DOKPLOY_API_TOKEN`
   задан) — в этот момент бэкенд сам создаёт домен в Dokploy.
7. Проверить в Dokploy: `public-ssr` → вкладка **Domains** — должна появиться новая запись с
   вашим `status.ваш-домен.ru`, `certificateType: letsencrypt`.
8. Подождать 1–2 минуты (Traefik запрашивает сертификат Let's Encrypt по HTTP-01 при первом
   обращении/по расписанию) и открыть `https://status.ваш-домен.ru/` в браузере — должна
   открыться ваша страница статуса с валидным HTTPS (без предупреждения браузера о сертификате).
9. **Проверка отвязки:** в админке очистить/сменить собственный домен на странице → в Dokploy
   `public-ssr` → Domains запись должна исчезнуть, `https://status.ваш-домен.ru/` перестанет
   открываться (или начнёт отдавать ошибку сертификата/соединения — это ожидаемо, домен отвязан).

Если на шаге 6 verify возвращает ошибку `dokploy_error` — CNAME указан верно, но вызов Dokploy API
не прошёл (неверный/просроченный `DOKPLOY_API_TOKEN`, неверный `DOKPLOY_PUBLIC_SSR_APP_ID`, или
Dokploy недоступен) — проверить логи `api` (вкладка Logs в Dokploy) на сообщение `dokploy:`.

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
