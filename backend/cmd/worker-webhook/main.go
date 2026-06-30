// Command worker-webhook — воркер HTTP-доставки (этапы 3.9 + 5.4). Потребляет две очереди (ручной
// ack, DESIGN §8.1):
//   - q.slack — доставка Block Kit в incoming-webhook URL Slack-подписчика (этап 3.9);
//   - q.webhook.out — исходящие webhook'и (Mattermost/произвольный URL, channel=webhook, этап 5.4).
//
// Общая механика: идемпотентность по Notification.id, доставка на Subscriber.address (URL), ретраи
// с backoff через delayed.events, DLQ при исчерпании. Транспорт — HTTP POST.
//
// Требует DATABASE_URL и RABBITMQ_URL.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/healthpage/backend/internal/config"
	"github.com/healthpage/backend/internal/notify"
	"github.com/healthpage/backend/internal/queue"
	"github.com/healthpage/backend/internal/slack"
	"github.com/healthpage/backend/internal/store"
	"github.com/healthpage/backend/internal/webhookout"
)

const prefetch = 16

func main() {
	cfg := config.Load()

	initCtx, cancelInit := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelInit()

	st, err := store.New(initCtx, cfg.MustDatabaseURL())
	if err != nil {
		log.Fatalf("store init: %v", err)
	}
	defer st.Close()

	conn, err := queue.Dial(cfg.MustRabbitMQURL())
	if err != nil {
		log.Fatalf("queue dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Топология должна быть объявлена (queue-setup); declare идемпотентен — подстрахуемся.
	setupCh, err := conn.Channel()
	if err != nil {
		log.Fatalf("queue channel: %v", err)
	}
	if err := queue.DeclareTopology(setupCh); err != nil {
		log.Fatalf("declare topology: %v", err)
	}
	_ = setupCh.Close()

	// Publisher для ретраев (отложенная републикация) + движок как Retrier.
	pub, err := queue.NewPublisher(conn)
	if err != nil {
		log.Fatalf("publisher: %v", err)
	}
	defer func() { _ = pub.Close() }()
	engine := notify.New(st, pub, nil)

	slackWorker := slack.NewWorker(st, slack.NewClient(nil), engine, cfg.BaseURL, nil)
	outWorker := webhookout.NewWorker(st, webhookout.NewClient(nil), engine, cfg.BaseURL, nil)

	ctx, cancel := context.WithCancel(context.Background())

	// q.slack — Slack-подписчики (этап 3.9).
	slackCh, err := conn.Consume(queue.WorkQueue("slack"), prefetch, func(d queue.Delivery) {
		switch slackWorker.Process(ctx, d.Body) {
		case slack.Ack:
			_ = d.Ack(false)
		case slack.Reject:
			_ = d.Nack(false, false) // → DLX → q.dlq.slack
		case slack.Requeue:
			_ = d.Nack(false, true)
		}
	})
	if err != nil {
		cancel()
		log.Fatalf("consume q.slack: %v", err)
	}

	// q.webhook.out — исходящие webhook'и (Mattermost/произвольный URL, этап 5.4).
	outCh, err := conn.Consume(queue.QueueWebhookOut, prefetch, func(d queue.Delivery) {
		switch outWorker.Process(ctx, d.Body) {
		case webhookout.Ack:
			_ = d.Ack(false)
		case webhookout.Reject:
			_ = d.Nack(false, false) // → DLX → q.dlq.webhook.out
		case webhookout.Requeue:
			_ = d.Nack(false, true)
		}
	})
	if err != nil {
		cancel()
		_ = slackCh.Close()
		log.Fatalf("consume q.webhook.out: %v", err)
	}
	log.Println("worker-webhook: потребление q.slack и q.webhook.out запущено")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	cancel()
	_ = slackCh.Close()
	_ = outCh.Close()
	log.Println("worker-webhook: остановлен")
}
