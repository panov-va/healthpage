// Command edge — обратный прокси кастомных доменов (этап 4.3.3): :80 отдаёт HTTP-01 challenge и
// редиректит на HTTPS; :443 терминирует TLS по SNI (сертификаты из БД от tls-manager) и проксирует
// на public-ssr (страница по домену) и API (/api/*). Требует DATABASE_URL.
// Отладка — на прод-деплое (нужны публичный DNS, открытые :80/:443 и выпущенные сертификаты).
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/healthpage/backend/internal/config"
	"github.com/healthpage/backend/internal/edge"
	"github.com/healthpage/backend/internal/store"
)

func main() {
	cfg := config.Load()

	initCtx, cancelInit := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelInit()
	st, err := store.New(initCtx, cfg.MustDatabaseURL())
	if err != nil {
		log.Fatalf("store init: %v", err)
	}
	defer st.Close()

	srv, err := edge.New(st, cfg.EdgeAPIURL, cfg.EdgeSSRURL)
	if err != nil {
		log.Fatalf("edge init: %v", err)
	}

	httpSrv := &http.Server{
		Addr:              cfg.EdgeHTTPAddr,
		Handler:           srv.HTTPHandler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	httpsSrv := &http.Server{
		Addr:              cfg.EdgeHTTPSAddr,
		Handler:           srv.HTTPSHandler(),
		TLSConfig:         srv.TLSConfig(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("edge: HTTP (ACME/redirect) on %s", cfg.EdgeHTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("edge http: %v", err)
		}
	}()
	go func() {
		log.Printf("edge: HTTPS (TLS/proxy) on %s → api=%s ssr=%s", cfg.EdgeHTTPSAddr, cfg.EdgeAPIURL, cfg.EdgeSSRURL)
		// Сертификаты берутся из TLSConfig.GetCertificate (по SNI), поэтому файлы не нужны.
		if err := httpsSrv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Fatalf("edge https: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
	_ = httpsSrv.Shutdown(ctx)
	log.Println("edge stopped")
}
