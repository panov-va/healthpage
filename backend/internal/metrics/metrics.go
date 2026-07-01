// Package metrics экспортирует метрики Prometheus (этап 7.3). Регистрируется в дефолтном
// реестре client_golang (туда же автоматически попадают go_* и process_* коллекторы), отдаётся
// на GET /metrics для скрейпинга Prometheus. Дашборды/алерты (Grafana) — прод-решение.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "healthpage_http_requests_total",
		Help: "Число HTTP-запросов по методу, маршруту и коду ответа.",
	}, []string{"method", "route", "code"})

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "healthpage_http_request_duration_seconds",
		Help:    "Длительность обработки HTTP-запросов в секундах.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})
)

// Handler отдаёт метрики в формате экспозиции Prometheus (дефолтный реестр).
func Handler() http.Handler { return promhttp.Handler() }

// Middleware учитывает количество и длительность HTTP-запросов. Метка route — шаблон маршрута
// chi (напр. /api/v1/pages/{page}/summary), а не конкретный URL — чтобы избежать высокой
// кардинальности меток.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		httpRequests.WithLabelValues(r.Method, route, strconv.Itoa(ww.Status())).Inc()
		httpDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}
