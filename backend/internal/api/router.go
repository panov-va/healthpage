// Package api собирает HTTP-роутер сервиса.
// На этапе 0 здесь только служебные эндпоинты (healthz). Бизнес-эндпоинты
// по openapi.yaml появятся на этапе 1.
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter создаёт корневой роутер с базовым middleware и служебными эндпоинтами.
func NewRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", healthz)

	return r
}

// healthz — liveness-проба: отвечает 200, если процесс жив.
func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
