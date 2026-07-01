package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestMetricsHandlerAndMiddleware(t *testing.T) {
	r := chi.NewRouter()
	r.Use(Middleware)
	r.Handle("/metrics", Handler())
	r.Get("/pages/{slug}/summary", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(r)
	defer srv.Close()

	// Делаем учётный запрос по маршруту с параметром.
	resp, err := http.Get(srv.URL + "/pages/demo/summary")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()

	// Скрейпим метрики.
	m, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("metrics: %v", err)
	}
	defer func() { _ = m.Body.Close() }()
	if m.StatusCode != http.StatusOK {
		t.Fatalf("/metrics status=%d", m.StatusCode)
	}
	body, _ := io.ReadAll(m.Body)
	text := string(body)

	// Наш счётчик присутствует, маршрут — шаблон (не конкретный slug), код 200.
	if !strings.Contains(text, "healthpage_http_requests_total") {
		t.Fatal("missing healthpage_http_requests_total")
	}
	if !strings.Contains(text, `route="/pages/{slug}/summary"`) {
		t.Fatalf("expected templated route label, got:\n%s", firstLines(text, 40))
	}
	// Дефолтные коллекторы Go тоже экспонируются.
	if !strings.Contains(text, "go_goroutines") {
		t.Fatal("missing default go collectors")
	}
}

func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
