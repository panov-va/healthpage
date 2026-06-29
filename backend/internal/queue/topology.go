package queue

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Нормативная топология RabbitMQ — DESIGN §8.1. Объявление идемпотентно (повторный вызов
// ничего не ломает), поэтому безопасно выполнять при старте/деплое.
const (
	// ExchangeNotifications — фан-аут уведомлений по каналам: ключ notify.<channel>.<event>.
	ExchangeNotifications = "notifications"
	// ExchangeWebhooksOut — исходящие webhook'и (Mattermost/произвольные URL).
	ExchangeWebhooksOut = "webhooks.out"
	// ExchangeDelayed — отложенные события (анонсы работ, retry-with-backoff). Требует плагина
	// rabbitmq_delayed_message_exchange; маршрутизирует по тем же ключам, что notifications.
	ExchangeDelayed = "delayed.events"
	// ExchangeDLX — dead-letter exchange: упавшие после ретраев сообщения уходят в q.dlq.<channel>.
	ExchangeDLX = "dlx"

	// QueueWebhookOut — единственная очередь исходящих webhook'ов.
	QueueWebhookOut = "q.webhook.out"
	// webhookOutKey — ключ маршрутизации для direct-exchange webhooks.out.
	webhookOutKey = "webhook.out"
)

// NotificationChannels — каналы фан-аута уведомлений (DESIGN §8.1). Slack доставляется
// воркером webhook'ов, но имеет свою очередь q.slack.
var NotificationChannels = []string{"email", "telegram", "max", "slack"}

// WorkQueue возвращает имя рабочей очереди канала: q.<channel>.
func WorkQueue(channel string) string { return "q." + channel }

// DLQQueue возвращает имя dead-letter-очереди канала: q.dlq.<channel>.
func DLQQueue(channel string) string { return "q.dlq." + channel }

// dlqKey — ключ маршрутизации dead-letter для канала: dlq.<channel>.
func dlqKey(channel string) string { return "dlq." + channel }

// NotifyKey строит ключ маршрутизации одного события: notify.<channel>.<event>.
func NotifyKey(channel, event string) string { return "notify." + channel + "." + event }

// notifyBindingKey — ключ привязки очереди канала ко всем его событиям: notify.<channel>.*.
func notifyBindingKey(channel string) string { return "notify." + channel + ".*" }

// DeclareTopology объявляет все exchange'и, очереди и привязки §8.1. Идемпотентно.
func DeclareTopology(ch *amqp.Channel) error {
	// Exchange'и (durable, не auto-delete).
	if err := ch.ExchangeDeclare(ExchangeNotifications, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("queue: declare %s: %w", ExchangeNotifications, err)
	}
	if err := ch.ExchangeDeclare(ExchangeWebhooksOut, "direct", true, false, false, false, nil); err != nil {
		return fmt.Errorf("queue: declare %s: %w", ExchangeWebhooksOut, err)
	}
	if err := ch.ExchangeDeclare(ExchangeDLX, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("queue: declare %s: %w", ExchangeDLX, err)
	}
	// Отложенный exchange — плагин x-delayed-message; внутренний тип маршрутизации — topic.
	if err := ch.ExchangeDeclare(ExchangeDelayed, "x-delayed-message", true, false, false, false,
		amqp.Table{"x-delayed-type": "topic"}); err != nil {
		return fmt.Errorf("queue: declare %s: %w", ExchangeDelayed, err)
	}

	// Канальные очереди: рабочая (с DLX) + dead-letter, привязки к notifications и delayed.events.
	for _, channel := range NotificationChannels {
		if err := declareWorkQueue(ch, WorkQueue(channel), dlqKey(channel)); err != nil {
			return err
		}
		if err := declareDLQ(ch, DLQQueue(channel), dlqKey(channel)); err != nil {
			return err
		}
		key := notifyBindingKey(channel)
		if err := ch.QueueBind(WorkQueue(channel), key, ExchangeNotifications, false, nil); err != nil {
			return fmt.Errorf("queue: bind %s→%s: %w", WorkQueue(channel), ExchangeNotifications, err)
		}
		if err := ch.QueueBind(WorkQueue(channel), key, ExchangeDelayed, false, nil); err != nil {
			return fmt.Errorf("queue: bind %s→%s: %w", WorkQueue(channel), ExchangeDelayed, err)
		}
	}

	// Исходящие webhook'и: своя рабочая очередь + DLQ.
	if err := declareWorkQueue(ch, QueueWebhookOut, dlqKey("webhook.out")); err != nil {
		return err
	}
	if err := declareDLQ(ch, DLQQueue("webhook.out"), dlqKey("webhook.out")); err != nil {
		return err
	}
	if err := ch.QueueBind(QueueWebhookOut, webhookOutKey, ExchangeWebhooksOut, false, nil); err != nil {
		return fmt.Errorf("queue: bind %s→%s: %w", QueueWebhookOut, ExchangeWebhooksOut, err)
	}

	return nil
}

// declareWorkQueue объявляет durable-очередь с dead-letter в ExchangeDLX по ключу dlqRoutingKey.
func declareWorkQueue(ch *amqp.Channel, name, dlqRoutingKey string) error {
	_, err := ch.QueueDeclare(name, true, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    ExchangeDLX,
		"x-dead-letter-routing-key": dlqRoutingKey,
	})
	if err != nil {
		return fmt.Errorf("queue: declare %s: %w", name, err)
	}
	return nil
}

// declareDLQ объявляет durable dead-letter-очередь и привязывает её к ExchangeDLX по ключу.
func declareDLQ(ch *amqp.Channel, name, routingKey string) error {
	if _, err := ch.QueueDeclare(name, true, false, false, false, nil); err != nil {
		return fmt.Errorf("queue: declare %s: %w", name, err)
	}
	if err := ch.QueueBind(name, routingKey, ExchangeDLX, false, nil); err != nil {
		return fmt.Errorf("queue: bind %s→%s: %w", name, ExchangeDLX, err)
	}
	return nil
}
