// Command worker-email — воркер доставки email-уведомлений (этап 3.4). Потребляет очередь
// q.email с ручным ack (DESIGN §8.1): идемпотентность по Notification.id, отправка по SMTP
// (системному или кастомному SMTP страницы), ретраи с backoff через delayed.events, DLQ при
// исчерпании. Требует DATABASE_URL и RABBITMQ_URL.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/healthpage/backend/internal/config"
	"github.com/healthpage/backend/internal/email"
	"github.com/healthpage/backend/internal/notify"
	"github.com/healthpage/backend/internal/queue"
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

	// Отправитель: реальный SMTP, если задан системный хост; иначе лог-заглушка (dev).
	sysSMTP := email.SMTP{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword, From: cfg.SMTPFrom, TLS: cfg.SMTPTLS,
	}
	var sender email.Sender = email.SMTPSender{}
	if sysSMTP.IsZero() {
		log.Println("worker-email: SMTP_HOST не задан — письма логируются (LogSender)")
		sender = email.LogSender{}
	}

	worker := email.NewWorker(st, sender, engine, sysSMTP, cfg.BaseURL, cfg.SubscriptionSecret, nil)

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := conn.Consume(queue.WorkQueue("email"), prefetch, func(d queue.Delivery) {
		switch worker.Process(ctx, d.Body) {
		case email.Ack:
			_ = d.Ack(false)
		case email.Reject:
			_ = d.Nack(false, false) // → DLX → q.dlq.email
		case email.Requeue:
			_ = d.Nack(false, true)
		}
	})
	if err != nil {
		cancel()
		log.Fatalf("consume q.email: %v", err)
	}
	log.Println("worker-email: потребление q.email запущено")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	cancel()
	_ = ch.Close()
	log.Println("worker-email: остановлен")
}
