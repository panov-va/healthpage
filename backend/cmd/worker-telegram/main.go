// Command worker-telegram — воркер канала Telegram (этап 3.7). Делает две вещи в одном процессе:
//  1. Потребляет очередь q.telegram (ручной ack, DESIGN §8.1): идемпотентность по Notification.id,
//     доставка через Bot API, ретраи с backoff через delayed.events, DLQ при исчерпании.
//  2. Long-poll getUpdates: бот управления подпиской — /start <slug> подписывает чат на страницу,
//     /stop отписывает. Подписка через бота, а не через POST /subscribe (DESIGN §3.4).
//
// Требует DATABASE_URL, RABBITMQ_URL и TELEGRAM_BOT_TOKEN.
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
	"github.com/healthpage/backend/internal/store"
	"github.com/healthpage/backend/internal/telegram"
)

const prefetch = 16

func main() {
	cfg := config.Load()
	token := cfg.MustTelegramBotToken()

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

	client := telegram.NewClient(token, nil)
	me, err := client.GetMe(initCtx)
	if err != nil {
		log.Fatalf("telegram getMe (проверьте TELEGRAM_BOT_TOKEN): %v", err)
	}
	log.Printf("worker-telegram: бот @%s авторизован", me.Username)

	worker := telegram.NewWorker(st, client, engine, cfg.BaseURL, nil)
	bot := telegram.NewBot(client, st, 30, nil)

	ctx, cancel := context.WithCancel(context.Background())

	// 1) Потребление очереди доставки.
	ch, err := conn.Consume(queue.WorkQueue("telegram"), prefetch, func(d queue.Delivery) {
		switch worker.Process(ctx, d.Body) {
		case telegram.Ack:
			_ = d.Ack(false)
		case telegram.Reject:
			_ = d.Nack(false, false) // → DLX → q.dlq.telegram
		case telegram.Requeue:
			_ = d.Nack(false, true)
		}
	})
	if err != nil {
		cancel()
		log.Fatalf("consume q.telegram: %v", err)
	}
	log.Println("worker-telegram: потребление q.telegram запущено")

	// 2) Бот управления подпиской (long polling) — в отдельной горутине.
	go func() {
		if err := bot.Run(ctx); err != nil && ctx.Err() == nil {
			log.Printf("worker-telegram: бот остановлен с ошибкой: %v", err)
		}
	}()
	log.Println("worker-telegram: бот подписки (getUpdates) запущен")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	cancel()
	_ = ch.Close()
	log.Println("worker-telegram: остановлен")
}
