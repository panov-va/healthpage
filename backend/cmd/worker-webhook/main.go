// Command worker-webhook — воркер HTTP-доставки (этап 3.9). Потребляет очередь q.slack (ручной
// ack, DESIGN §8.1): идемпотентность по Notification.id, доставка сообщения Block Kit в
// incoming-webhook URL канала (Subscriber.address), ретраи с backoff через delayed.events, DLQ при
// исчерпании. Транспорт — HTTP POST; секреты Slack OAuth здесь не нужны (адрес уже сохранён при
// подписке). На этапе 5.4 этот же воркер обслужит исходящие webhook'и (q.webhook.out).
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

	worker := slack.NewWorker(st, slack.NewClient(nil), engine, cfg.BaseURL, nil)

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := conn.Consume(queue.WorkQueue("slack"), prefetch, func(d queue.Delivery) {
		switch worker.Process(ctx, d.Body) {
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
	log.Println("worker-webhook: потребление q.slack запущено")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	cancel()
	_ = ch.Close()
	log.Println("worker-webhook: остановлен")
}
