// Package queue инкапсулирует работу с RabbitMQ: подключение, объявление топологии (DESIGN §8.1),
// публикацию с publisher confirms и потребление с ручным ack. Источник истины топологии — §8.1;
// здесь — её код-реализация. На этапе 3.2 объявляется только топология и публикатор; воркеры
// (consumer'ы) подключаются на 3.4+.
package queue

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Conn — обёртка над AMQP-соединением. Один Conn на процесс; каналы открываются под задачу.
type Conn struct {
	conn *amqp.Connection
}

// Dial устанавливает соединение с брокером по AMQP-URL (amqp://user:pass@host:5672/).
func Dial(url string) (*Conn, error) {
	if url == "" {
		return nil, fmt.Errorf("queue: пустой RABBITMQ_URL")
	}
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("queue: dial: %w", err)
	}
	return &Conn{conn: conn}, nil
}

// Channel открывает новый AMQP-канал. Вызывающий закрывает его сам.
func (c *Conn) Channel() (*amqp.Channel, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("queue: open channel: %w", err)
	}
	return ch, nil
}

// Close закрывает соединение (и все его каналы).
func (c *Conn) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
