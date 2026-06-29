// Command api — HTTP-сервер REST API HealthPage.
// На этапе 0 поднимает сервер с эндпоинтом /healthz и корректно завершается.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/healthpage/backend/internal/api"
	"github.com/healthpage/backend/internal/auth"
	"github.com/healthpage/backend/internal/config"
	"github.com/healthpage/backend/internal/notify"
	"github.com/healthpage/backend/internal/queue"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
)

func main() {
	// -health: режим проверки живости для docker HEALTHCHECK (образ distroless
	// без shell/curl, поэтому пробу выполняет сам бинарь).
	health := flag.Bool("health", false, "выполнить healthcheck и выйти (0 — ok)")
	flag.Parse()

	cfg := config.Load()

	if *health {
		runHealthCheck(cfg.HTTPPort)
		return
	}

	// Контекст инициализации зависимостей.
	initCtx, cancelInit := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelInit()

	st, err := store.New(initCtx, cfg.MustDatabaseURL())
	if err != nil {
		log.Fatalf("store init: %v", err)
	}
	defer st.Close()

	tokens, err := security.NewTokenManager(cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL)
	if err != nil {
		log.Fatalf("token manager: %v (задан ли JWT_SECRET?)", err)
	}
	authSvc := auth.NewService(st, tokens)

	// Движок уведомлений — мягкая зависимость: без RabbitMQ API работает, но рассылка отключена.
	notifier, closeQueue := setupNotifier(cfg.RabbitMQURL, st)
	defer closeQueue()

	srv := &http.Server{
		Addr: ":" + cfg.HTTPPort,
		Handler: api.NewRouter(api.Deps{
			Auth:       authSvc,
			Store:      st,
			Notifier:   notifier,
			SubSecret:  cfg.SubscriptionSecret,
			BaseURL:    cfg.BaseURL,
			Prod:       cfg.IsProd(),
			RefreshTTL: cfg.RefreshTTL,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Запуск сервера в фоне.
	go func() {
		log.Printf("api listening on :%s (env=%s)", cfg.HTTPPort, cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Ожидание сигнала завершения.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	// Graceful shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("api stopped")
}

// setupNotifier подключается к RabbitMQ и собирает движок уведомлений. При пустом URL или ошибке
// подключения возвращает (nil, no-op): API продолжает работать без рассылки (записи журнала не
// создаются, события просто не публикуются). Возвращает функцию закрытия ресурсов брокера.
func setupNotifier(rabbitURL string, st *store.Store) (*notify.Engine, func()) {
	if rabbitURL == "" {
		log.Println("notify: RABBITMQ_URL не задан — рассылка уведомлений отключена")
		return nil, func() {}
	}
	conn, err := queue.Dial(rabbitURL)
	if err != nil {
		log.Printf("notify: не удалось подключиться к RabbitMQ (%v) — рассылка отключена", err)
		return nil, func() {}
	}
	pub, err := queue.NewPublisher(conn)
	if err != nil {
		log.Printf("notify: не удалось открыть publisher (%v) — рассылка отключена", err)
		_ = conn.Close()
		return nil, func() {}
	}
	log.Println("notify: движок уведомлений подключён к RabbitMQ")
	return notify.New(st, pub, nil), func() {
		_ = pub.Close()
		_ = conn.Close()
	}
}

// runHealthCheck дёргает локальный /healthz и завершает процесс с кодом 0/1.
func runHealthCheck(port string) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		log.Printf("healthcheck failed: %v", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("healthcheck status: %d", resp.StatusCode)
		os.Exit(1)
	}
}
