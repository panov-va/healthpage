// Package config загружает конфигурацию сервиса из переменных окружения.
// На этапе 0 — минимальный набор для запуска api-сервиса и миграций.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config — конфигурация, общая для процессов backend (api, migrate, ...).
type Config struct {
	AppEnv      string // dev / prod
	HTTPPort    string // порт HTTP-сервера api
	DatabaseURL string // строка подключения PostgreSQL
	RedisURL    string // подключение Redis
	RabbitMQURL string // подключение RabbitMQ
	BaseURL     string // базовый публичный URL самого API (ссылки на API-эндпоинты — confirm и т.п.)

	// PublicURL — публичный origin public-ssr (лендинг, страницы статуса, отписка, приватный
	// доступ). В сплит-доменном проде (healthpage.ru отдельно от api.healthpage.ru) это НЕ то же
	// самое, что BaseURL — раньше ссылки в письмах/фидах/уведомлениях ошибочно строились от
	// BaseURL и вели на несуществующие в API маршруты.
	PublicURL string
	// AdminURL — публичный origin админки (React SPA). Используется для redirect после оплаты
	// (billing checkout return_url).
	AdminURL string

	JWTSecret  string        // секрет подписи операторских access-JWT
	AccessTTL  time.Duration // срок жизни access-токена
	RefreshTTL time.Duration // срок жизни refresh-токена

	// SubscriptionSecret — секрет подписи capability-токенов отписки (HMAC). Если не задан —
	// падает на JWTSecret (dev), чтобы воркер и эндпоинты подписки сходились.
	SubscriptionSecret string

	// Системный SMTP (дефолтный отправитель). Страница может переопределить своим smtp_config.
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string // адрес отправителя по умолчанию
	SMTPTLS      bool   // true — неявный TLS (порт 465); иначе STARTTLS

	// UniSenderGoAPIKey (если задан) — системный отправитель шлёт через HTTP Web API UniSender Go
	// (порт 443) вместо SMTP. Решение 2026-07-22: у VPS-провайдера прод-сервера исходящие SMTP-порты
	// (587/465) заблокированы на уровне сети (подтверждено — таймаут даже до smtp.gmail.com), а HTTPS
	// не блокируется. Кастомный SMTP страницы (4.5, произвольный провайдер клиента) не затрагивается —
	// для него всегда используется настоящий SMTP-протокол (SMTPSender), т.к. мы не можем провести
	// чужой почтовый сервер через свой аккаунт UniSender Go.
	UniSenderGoAPIKey string
	// UniSenderGoAPIURL переопределяет эндпоинт Web API (дата-центр аккаунта — go1/go2/...,
	// см. комментарий в internal/email/unisender.go). Пусто → дефолт пакета (go2, текущий аккаунт).
	UniSenderGoAPIURL string
	// UniSenderGoBackendID — custom_backend_id, если у проекта UniSender Go нет назначенного по
	// умолчанию backend-домена (см. internal/email/unisender.go). 0 → не передавать.
	UniSenderGoBackendID int

	// TelegramBotToken — токен бота от @BotFather (worker-telegram). Если пуст — воркер не
	// стартует (бот без токена бессмыслен).
	TelegramBotToken string

	// Slack OAuth (канал подписки 3.9). Если ClientID/Secret пусты — эндпоинты подписки Slack
	// отвечают 404 (фича выключена). Доставка (worker-webhook) секретов не требует — шлёт в
	// сохранённый incoming-webhook URL.
	SlackClientID     string
	SlackClientSecret string

	// Целевой хост для CNAME собственных доменов (этап 4.3): оператор направляет CNAME своего
	// домена сюда, верификация сверяет резолв с этим значением.
	CNAMETarget string

	// ACME (этап 4.3.2): автоматический выпуск TLS для кастомных доменов.
	ACMEEmail         string        // контактный email для Let's Encrypt
	ACMEDirectoryURL  string        // ACME-директория (prod/staging)
	ACMERenewInterval time.Duration // период запуска проверки продления
	ACMERenewBefore   time.Duration // продлевать, если до истечения меньше этого срока

	// Edge-прокси кастомных доменов (этап 4.3.3). Держим на случай ухода от Dokploy; в прод-деплое
	// на Dokploy не используется — см. DokployAPIURL и DEPLOY.md.
	EdgeHTTPAddr  string // адрес :80 (HTTP-01 + redirect)
	EdgeHTTPSAddr string // адрес :443 (TLS-терминация)
	EdgeAPIURL    string // origin API для /api/*
	EdgeSSRURL    string // origin public-ssr для страниц

	// Dokploy API (прод-интеграция кастомных доменов вместо edge/tls-manager, DEPLOY.md): при
	// верификации CNAME домен подключается как Domain приложения public-ssr — Traefik/Let's
	// Encrypt дальше обслуживает его сам Dokploy. Пусто (DokployAPIToken) → интеграция выключена,
	// домен остаётся только verified без реального подключения.
	DokployAPIURL         string // например http://<host>:3000/api
	DokployAPIToken       string // API-ключ Dokploy (Settings → Profile → API/CLI Keys)
	DokployPublicSSRAppID string // ID приложения public-ssr в Dokploy

	// Биллинг (этап 6). Цены — плейсхолдер, финализируются перед запуском (Статусмейт −10%,
	// DESIGN §10). Суммы в копейках. Боевой провайдер — ЮKassa; без credentials используется
	// stub (dev). [ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ БИЛЛИНГА]: реальные ключи и боевые списания.
	PremiumMonthlyMinor      int64         // цена Premium в месяц, копейки
	PremiumYearlyDiscountPct int           // скидка годовой подписки, %
	BillingCurrency          string        // ISO-код валюты (RUB)
	TrialDays                int           // длительность пробного периода
	BillingScanInterval      time.Duration // период цикла worker-billing
	BillingMaxDunning        int           // число неуспешных попыток до отката на Free
	BillingRetryInterval     time.Duration // пауза между попытками списания (dunning)
	YooKassaShopID           string        // Shop ID ЮKassa (пусто → stub-провайдер)
	YooKassaSecretKey        string        // секретный ключ ЮKassa
}

// IsProd сообщает, работаем ли в prod-режиме (влияет, напр., на флаг Secure у cookie).
func (c Config) IsProd() bool { return c.AppEnv == "prod" }

// Load читает конфигурацию из окружения, подставляя dev-дефолты.
func Load() Config {
	c := Config{
		AppEnv:      env("APP_ENV", "dev"),
		HTTPPort:    env("HTTP_PORT", "8080"),
		DatabaseURL: env("DATABASE_URL", ""),
		RedisURL:    env("REDIS_URL", ""),
		RabbitMQURL: env("RABBITMQ_URL", ""),
		BaseURL:     env("BASE_URL", "http://localhost:8080"),
		PublicURL:   env("PUBLIC_URL", "http://localhost:3000"),
		AdminURL:    env("ADMIN_URL", "http://localhost:5173"),
		JWTSecret:   env("JWT_SECRET", ""),
		AccessTTL:   envDuration("ACCESS_TTL", 15*time.Minute),
		RefreshTTL:  envDuration("REFRESH_TTL", 30*24*time.Hour),

		SubscriptionSecret: env("SUBSCRIPTION_SECRET", ""),

		SMTPHost:     env("SMTP_HOST", ""),
		SMTPPort:     envInt("SMTP_PORT", 587),
		SMTPUsername: env("SMTP_USERNAME", ""),
		SMTPPassword: env("SMTP_PASSWORD", ""),
		SMTPFrom:     env("SMTP_FROM", ""),
		SMTPTLS:      env("SMTP_TLS", "") == "true",

		UniSenderGoAPIKey:    env("UNISENDER_GO_API_KEY", ""),
		UniSenderGoAPIURL:    env("UNISENDER_GO_API_URL", ""),
		UniSenderGoBackendID: envInt("UNISENDER_GO_BACKEND_ID", 0),

		TelegramBotToken: env("TELEGRAM_BOT_TOKEN", ""),

		SlackClientID:     env("SLACK_CLIENT_ID", ""),
		SlackClientSecret: env("SLACK_CLIENT_SECRET", ""),

		CNAMETarget: env("CNAME_TARGET", "cname.healthpage.ru"),

		ACMEEmail:         env("ACME_EMAIL", ""),
		ACMEDirectoryURL:  env("ACME_DIRECTORY_URL", "https://acme-v02.api.letsencrypt.org/directory"),
		ACMERenewInterval: envDuration("ACME_RENEW_INTERVAL", 12*time.Hour),
		ACMERenewBefore:   envDuration("ACME_RENEW_BEFORE", 30*24*time.Hour),

		EdgeHTTPAddr:  env("EDGE_HTTP_ADDR", ":80"),
		EdgeHTTPSAddr: env("EDGE_HTTPS_ADDR", ":443"),
		EdgeAPIURL:    env("EDGE_API_URL", "http://api:8080"),
		EdgeSSRURL:    env("EDGE_SSR_URL", "http://public-ssr:3000"),

		DokployAPIURL:         env("DOKPLOY_API_URL", ""),
		DokployAPIToken:       env("DOKPLOY_API_TOKEN", ""),
		DokployPublicSSRAppID: env("DOKPLOY_PUBLIC_SSR_APP_ID", ""),

		PremiumMonthlyMinor:      int64(envInt("PREMIUM_MONTHLY_MINOR", 99000)),
		PremiumYearlyDiscountPct: envInt("PREMIUM_YEARLY_DISCOUNT_PCT", 20),
		BillingCurrency:          env("BILLING_CURRENCY", "RUB"),
		TrialDays:                envInt("TRIAL_DAYS", 14),
		BillingScanInterval:      envDuration("BILLING_SCAN_INTERVAL", time.Hour),
		BillingMaxDunning:        envInt("BILLING_MAX_DUNNING", 3),
		BillingRetryInterval:     envDuration("BILLING_RETRY_INTERVAL", 72*time.Hour),
		YooKassaShopID:           env("YOOKASSA_SHOP_ID", ""),
		YooKassaSecretKey:        env("YOOKASSA_SECRET_KEY", ""),
	}
	// Дефолт секрета отписки — операторский JWT-секрет (для dev/одно-процессного запуска).
	if c.SubscriptionSecret == "" {
		c.SubscriptionSecret = c.JWTSecret
	}
	return c
}

// MustDatabaseURL возвращает строку подключения к БД или завершает процесс,
// если она не задана (используется командой миграций).
func (c Config) MustDatabaseURL() string {
	if c.DatabaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}
	return c.DatabaseURL
}

// MustRabbitMQURL возвращает строку подключения к RabbitMQ или завершает процесс,
// если она не задана (используется командой queue-setup и воркерами этапа 3).
func (c Config) MustRabbitMQURL() string {
	if c.RabbitMQURL == "" {
		fmt.Fprintln(os.Stderr, "RABBITMQ_URL is required")
		os.Exit(1)
	}
	return c.RabbitMQURL
}

// MustTelegramBotToken возвращает токен бота или завершает процесс, если он не задан
// (используется командой worker-telegram).
func (c Config) MustTelegramBotToken() string {
	if c.TelegramBotToken == "" {
		fmt.Fprintln(os.Stderr, "TELEGRAM_BOT_TOKEN is required")
		os.Exit(1)
	}
	return c.TelegramBotToken
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
