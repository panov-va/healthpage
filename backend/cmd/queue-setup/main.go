// Command queue-setup объявляет топологию RabbitMQ (DESIGN §8.1): exchange'и, очереди, привязки,
// dead-letter и отложенный exchange. Идемпотентно — безопасно запускать при каждом деплое.
//
// Использование:
//
//	queue-setup            — объявить топологию и выйти
//
// Требует RABBITMQ_URL. Локально: docker compose run --rm api /app/queue-setup
package main

import (
	"log"

	"github.com/healthpage/backend/internal/config"
	"github.com/healthpage/backend/internal/queue"
)

func main() {
	cfg := config.Load()

	conn, err := queue.Dial(cfg.MustRabbitMQURL())
	if err != nil {
		log.Fatalf("queue dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("queue channel: %v", err)
	}
	defer func() { _ = ch.Close() }()

	if err := queue.DeclareTopology(ch); err != nil {
		log.Fatalf("declare topology: %v", err)
	}
	log.Println("RabbitMQ topology declared (DESIGN §8.1)")
}
