package queue

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Delivery — переэкспорт типа доставки, чтобы воркеры (этап 3.4+) не импортировали amqp напрямую.
type Delivery = amqp.Delivery

// Consume запускает потребление очереди с ручным ack (DESIGN §8.1): autoAck выключен, handler сам
// вызывает d.Ack/d.Nack только после успешной (или окончательно неуспешной) обработки. prefetch
// ограничивает число неподтверждённых сообщений на потребителя. Возвращает канал (закрыть для
// остановки потребления). Воркеры подключают это на этапе 3.4+.
func (c *Conn) Consume(queueName string, prefetch int, handler func(Delivery)) (*amqp.Channel, error) {
	ch, err := c.Channel()
	if err != nil {
		return nil, err
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("queue: qos: %w", err)
	}
	// autoAck=false — подтверждение строго вручную в handler.
	deliveries, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("queue: consume %s: %w", queueName, err)
	}
	go func() {
		for d := range deliveries {
			handler(d)
		}
	}()
	return ch, nil
}
