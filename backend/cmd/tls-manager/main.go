// Command tls-manager — выпуск и автопродление TLS-сертификатов кастомных доменов через ACME
// (Let's Encrypt) по HTTP-01 (этап 4.3.2). Периодически выпускает/продлевает серты для
// верифицированных доменов и кладёт их в БД; edge-прокси (4.3.3) отдаёт их по SNI.
//
// Требует DATABASE_URL и ACME_EMAIL. HTTP-01 challenge'и кладутся в БД и отдаются edge на :80,
// поэтому реальный выпуск работает только при публично доступном домене — отладка на прод-деплое.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/healthpage/backend/internal/acme"
	"github.com/healthpage/backend/internal/config"
	"github.com/healthpage/backend/internal/store"
)

func main() {
	cfg := config.Load()
	if cfg.ACMEEmail == "" {
		log.Fatal("tls-manager: ACME_EMAIL не задан")
	}

	initCtx, cancelInit := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelInit()
	st, err := store.New(initCtx, cfg.MustDatabaseURL())
	if err != nil {
		log.Fatalf("store init: %v", err)
	}
	defer st.Close()

	mgr := acme.New(st, cfg.ACMEEmail, cfg.ACMEDirectoryURL)
	log.Printf("tls-manager: directory=%s renew_interval=%s renew_before=%s",
		cfg.ACMEDirectoryURL, cfg.ACMERenewInterval, cfg.ACMERenewBefore)

	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	runOnce := func() {
		runCtx, c := context.WithTimeout(ctx, 10*time.Minute)
		defer c()
		processed, err := mgr.RenewDue(runCtx, time.Now(), cfg.ACMERenewBefore)
		if err != nil {
			log.Printf("tls-manager: renew cycle: processed=%d err=%v", processed, err)
		} else {
			log.Printf("tls-manager: renew cycle ok: processed=%d", processed)
		}
	}

	runOnce() // первый прогон сразу при старте
	ticker := time.NewTicker(cfg.ACMERenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			log.Println("tls-manager: stopping")
			cancel()
			return
		case <-ticker.C:
			runOnce()
		}
	}
}
