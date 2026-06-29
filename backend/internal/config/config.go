// Package config загружает конфигурацию сервиса из переменных окружения.
// На этапе 0 — минимальный набор для запуска api-сервиса и миграций.
package config

import (
	"fmt"
	"os"
	"time"
)

// Config — конфигурация, общая для процессов backend (api, migrate, ...).
type Config struct {
	AppEnv      string // dev / prod
	HTTPPort    string // порт HTTP-сервера api
	DatabaseURL string // строка подключения PostgreSQL
	RedisURL    string // подключение Redis
	RabbitMQURL string // подключение RabbitMQ
	BaseURL     string // базовый публичный URL сервиса

	JWTSecret  string        // секрет подписи операторских access-JWT
	AccessTTL  time.Duration // срок жизни access-токена
	RefreshTTL time.Duration // срок жизни refresh-токена
}

// IsProd сообщает, работаем ли в prod-режиме (влияет, напр., на флаг Secure у cookie).
func (c Config) IsProd() bool { return c.AppEnv == "prod" }

// Load читает конфигурацию из окружения, подставляя dev-дефолты.
func Load() Config {
	return Config{
		AppEnv:      env("APP_ENV", "dev"),
		HTTPPort:    env("HTTP_PORT", "8080"),
		DatabaseURL: env("DATABASE_URL", ""),
		RedisURL:    env("REDIS_URL", ""),
		RabbitMQURL: env("RABBITMQ_URL", ""),
		BaseURL:     env("BASE_URL", "http://localhost:8080"),
		JWTSecret:   env("JWT_SECRET", ""),
		AccessTTL:   envDuration("ACCESS_TTL", 15*time.Minute),
		RefreshTTL:  envDuration("REFRESH_TTL", 30*24*time.Hour),
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

// MustRabbitMQURL возвращает строку подключения к RabbitMQ или завершает процесс,
// если она не задана (используется командой queue-setup и воркерами этапа 3).
func (c Config) MustRabbitMQURL() string {
	if c.RabbitMQURL == "" {
		fmt.Fprintln(os.Stderr, "RABBITMQ_URL is required")
		os.Exit(1)
	}
	return c.RabbitMQURL
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
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
