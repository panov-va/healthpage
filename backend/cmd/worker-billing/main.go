// Command worker-billing — рекуррентные списания и dunning (этап 6.4, DESIGN §4.2).
// Периодически (BILLING_SCAN_INTERVAL) находит premium-подписки с истёкшим периодом и:
//   - списывает по сохранённому токену (автопродление) → продлевает период;
//   - при неуспехе наращивает счётчик dunning, держит grace (past_due);
//   - при исчерпании попыток или отмене автопродления — откатывает аккаунт на Free.
//
// Боевой провайдер — ЮKassa при наличии ключей, иначе stub (dev). Требует DATABASE_URL.
// [ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ БИЛЛИНГА]: реальные списания проверяются на прод-деплое.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/healthpage/backend/internal/billing"
	"github.com/healthpage/backend/internal/config"
	"github.com/healthpage/backend/internal/store"
)

const dueBatchLimit = 200

func main() {
	cfg := config.Load()

	initCtx, cancelInit := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelInit()
	st, err := store.New(initCtx, cfg.MustDatabaseURL())
	if err != nil {
		log.Fatalf("store init: %v", err)
	}
	defer st.Close()

	provider := billing.SelectProvider(cfg.YooKassaShopID, cfg.YooKassaSecretKey, cfg.BaseURL)
	if cfg.YooKassaShopID == "" || cfg.YooKassaSecretKey == "" {
		log.Println("worker-billing: ключи ЮKassa не заданы — stub-провайдер (dev)")
	}
	svc := billing.NewService(st, billing.Config{
		Provider:      provider,
		Pricing:       billing.DefaultPricing(cfg.PremiumMonthlyMinor, cfg.PremiumYearlyDiscountPct, cfg.TrialDays, cfg.BillingCurrency),
		MaxDunning:    cfg.BillingMaxDunning,
		RetryInterval: cfg.BillingRetryInterval,
	})
	log.Printf("worker-billing: scan_interval=%s max_dunning=%d retry_interval=%s",
		cfg.BillingScanInterval, cfg.BillingMaxDunning, cfg.BillingRetryInterval)

	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	runOnce := func() {
		runCtx, c := context.WithTimeout(ctx, 10*time.Minute)
		defer c()
		processed, err := svc.ProcessDue(runCtx, dueBatchLimit)
		if err != nil {
			log.Printf("worker-billing: cycle: processed=%d err=%v", processed, err)
		} else if processed > 0 {
			log.Printf("worker-billing: cycle ok: processed=%d", processed)
		}
	}

	runOnce() // первый прогон сразу при старте
	ticker := time.NewTicker(cfg.BillingScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			log.Println("worker-billing: stopping")
			cancel()
			return
		case <-ticker.C:
			runOnce()
		}
	}
}
