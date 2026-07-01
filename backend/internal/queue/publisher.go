package queue

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher публикует сообщения с publisher confirms (DESIGN §8.1): сообщение считается принятым
// только после ack брокера. Держит отдельный канал в confirm-режиме; не потокобезопасен —
// по одному Publisher на горутину-издателя.
type Publisher struct {
	ch *amqp.Channel
}

// NewPublisher открывает канал и переводит его в режим подтверждений.
func NewPublisher(c *Conn) (*Publisher, error) {
	ch, err := c.Channel()
	if err != nil {
		return nil, err
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("queue: enable confirms: %w", err)
	}
	return &Publisher{ch: ch}, nil
}

// Close закрывает канал издателя.
func (p *Publisher) Close() error { return p.ch.Close() }

// publish публикует persistent-сообщение и ждёт подтверждения брокера (или ошибки/таймаута ctx).
func (p *Publisher) publish(ctx context.Context, exchange, key string, body []byte, headers amqp.Table) error {
	dc, err := p.ch.PublishWithDeferredConfirmWithContext(ctx, exchange, key, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
		Headers:      headers,
	})
	if err != nil {
		return fmt.Errorf("queue: publish exchange=%s key=%s: %w", exchange, key, err)
	}
	select {
	case <-dc.Done():
		if !dc.Acked() {
			return fmt.Errorf("queue: publish nacked by broker (exchange=%s key=%s)", exchange, key)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// PublishNotification публикует событие notify.<channel>.<event> в exchange notifications.
func (p *Publisher) PublishNotification(ctx context.Context, channel, event string, body []byte) error {
	return p.publish(ctx, ExchangeNotifications, NotifyKey(channel, event), body, nil)
}

// PublishNotificationDelayed публикует событие с задержкой через delayed.events (retry-with-backoff
// и отложенные анонсы работ). Сообщение попадёт в ту же канальную очередь по истечении delay.
func (p *Publisher) PublishNotificationDelayed(
	ctx context.Context, channel, event string, body []byte, delay time.Duration,
) error {
	headers := amqp.Table{"x-delay": int32(delay.Milliseconds())}
	return p.publish(ctx, ExchangeDelayed, NotifyKey(channel, event), body, headers)
}

// PublishWebhookOut публикует исходящий webhook в exchange webhooks.out.
func (p *Publisher) PublishWebhookOut(ctx context.Context, body []byte) error {
	return p.publish(ctx, ExchangeWebhooksOut, webhookOutKey, body, nil)
}

// PublishImport ставит задачу импорта в q.import (default exchange, routing key = имя очереди).
func (p *Publisher) PublishImport(ctx context.Context, body []byte) error {
	return p.publish(ctx, "", QueueImport, body, nil)
}
