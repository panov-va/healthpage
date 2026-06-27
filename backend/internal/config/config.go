// Package config загружает конфигурацию сервиса из переменных окружения.
// На этапе 0 — минимальный набор для запуска api-сервиса и миграций.
package config

import (
	"fmt"
	"os"
)

// Config — конфигурация, общая для процессов backend (api, migrate, ...).
type Config struct {
	AppEnv      string // dev / prod
	HTTPPort    string // порт HTTP-сервера api
	DatabaseURL string // строка подключения PostgreSQL
	RedisURL    string // подключение Redis
	RabbitMQURL string // подключение RabbitMQ
	BaseURL     string // базовый публичный URL сервиса
}

// Load читает конфигурацию из окружения, подставляя dev-дефолты.
func Load() Config {
	return Config{
		AppEnv:      env("APP_ENV", "dev"),
		HTTPPort:    env("HTTP_PORT", "8080"),
		DatabaseURL: env("DATABASE_URL", ""),
		RedisURL:    env("REDIS_URL", ""),
		RabbitMQURL: env("RABBITMQ_URL", ""),
		BaseURL:     env("BASE_URL", "http://localhost:8080"),
	}
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

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
