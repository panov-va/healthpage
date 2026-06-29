package queue

import (
	"context"
	"os"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Интеграционный тест топологии/надёжности против реального RabbitMQ (с плагином
// rabbitmq_delayed_message_exchange). Запуск:
//
//	HEALTHPAGE_TEST_AMQP="amqp://healthpage:healthpage@localhost:5672/" \
//	  go test ./internal/queue/ -run TestQueueIntegration
func TestQueueIntegration(t *testing.T) {
	url := os.Getenv("HEALTHPAGE_TEST_AMQP")
	if url == "" {
		t.Skip("HEALTHPAGE_TEST_AMQP not set; skipping integration test")
	}
	conn, err := Dial(url)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("channel: %v", err)
	}
	defer func() { _ = ch.Close() }()

	if err := DeclareTopology(ch); err != nil {
		t.Fatalf("declare topology: %v", err)
	}
	// Изолируем тест: очищаем рабочую и dead-letter очереди email.
	if _, err := ch.QueuePurge(WorkQueue("email"), false); err != nil {
		t.Fatalf("purge work: %v", err)
	}
	if _, err := ch.QueuePurge(DLQQueue("email"), false); err != nil {
		t.Fatalf("purge dlq: %v", err)
	}

	pub, err := NewPublisher(conn)
	if err != nil {
		t.Fatalf("publisher: %v", err)
	}
	defer func() { _ = pub.Close() }()
	ctx := context.Background()

	// 1) publisher confirm + маршрутизация notify.email.* → q.email.
	if err := pub.PublishNotification(ctx, "email", "incident_new", []byte(`{"n":1}`)); err != nil {
		t.Fatalf("publish notification: %v", err)
	}
	d, ok := getWithin(t, ch, WorkQueue("email"), 2*time.Second)
	if !ok || string(d.Body) != `{"n":1}` {
		t.Fatalf("expected message in q.email, got ok=%v body=%s", ok, d.Body)
	}

	// 2) DLX: Nack(requeue=false) отправляет сообщение в q.dlq.email.
	if err := d.Nack(false, false); err != nil {
		t.Fatalf("nack: %v", err)
	}
	dd, ok := getWithin(t, ch, DLQQueue("email"), 2*time.Second)
	if !ok || string(dd.Body) != `{"n":1}` {
		t.Fatalf("expected dead-lettered message in q.dlq.email, got ok=%v body=%s", ok, dd.Body)
	}
	_ = dd.Ack(false)

	// 3) delayed.events: сообщение приходит в q.email не раньше, чем через ~delay.
	start := time.Now()
	if err := pub.PublishNotificationDelayed(ctx, "email", "maintenance_scheduled", []byte(`{"n":2}`), time.Second); err != nil {
		t.Fatalf("publish delayed: %v", err)
	}
	dl, ok := getWithin(t, ch, WorkQueue("email"), 5*time.Second)
	if !ok || string(dl.Body) != `{"n":2}` {
		t.Fatalf("expected delayed message in q.email, got ok=%v body=%s", ok, dl.Body)
	}
	if elapsed := time.Since(start); elapsed < 700*time.Millisecond {
		t.Fatalf("delayed message arrived too early: %v (want ≥ ~1s)", elapsed)
	}
	_ = dl.Ack(false)
}

// getWithin поллит очередь (manual ack: autoAck=false) до появления сообщения или таймаута.
func getWithin(t *testing.T, ch *amqp.Channel, queueName string, timeout time.Duration) (amqp.Delivery, bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		d, ok, err := ch.Get(queueName, false)
		if err != nil {
			t.Fatalf("get %s: %v", queueName, err)
		}
		if ok {
			return d, true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return amqp.Delivery{}, false
}
