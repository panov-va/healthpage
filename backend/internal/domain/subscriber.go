package domain

import (
	"time"

	"github.com/google/uuid"
)

// ── Канал подписки ──

// SubscriberChannel — канал доставки уведомлений подписчику
// (нормативный enum, openapi SubscriberChannel; значения зеркалят CHECK в миграции 00008).
type SubscriberChannel string

const (
	ChannelEmail    SubscriberChannel = "email"
	ChannelTelegram SubscriberChannel = "telegram"
	ChannelRSS      SubscriberChannel = "rss"
	ChannelICal     SubscriberChannel = "ical"
	ChannelWebhook  SubscriberChannel = "webhook"
	ChannelMAX      SubscriberChannel = "max"
	ChannelSlack    SubscriberChannel = "slack"
)

// AllSubscriberChannels — все допустимые значения (для валидации/перебора).
var AllSubscriberChannels = []SubscriberChannel{
	ChannelEmail, ChannelTelegram, ChannelRSS, ChannelICal, ChannelWebhook, ChannelMAX, ChannelSlack,
}

// IsValid сообщает, входит ли значение в нормативный enum.
func (c SubscriberChannel) IsValid() bool {
	switch c {
	case ChannelEmail, ChannelTelegram, ChannelRSS, ChannelICal, ChannelWebhook, ChannelMAX, ChannelSlack:
		return true
	default:
		return false
	}
}

// IsPush сообщает, что канал получает push-уведомления через очередь notifications
// (фан-аут notify.<channel>.* → q.<channel>, DESIGN §8.1). RSS/iCal — pull-фиды (этап 3.6),
// webhook — исходящий webhook через webhooks.out (этап 5.4), поэтому движок уведомлений
// (этап 3.3) рассылает только по email/telegram/max/slack.
func (c SubscriberChannel) IsPush() bool {
	switch c {
	case ChannelEmail, ChannelTelegram, ChannelMAX, ChannelSlack:
		return true
	default:
		return false
	}
}

// ── Область подписки ──

// SubscriberScope — на что подписан клиент: вся страница или набор компонентов
// (нормативный enum, openapi SubscriberScope).
type SubscriberScope string

const (
	ScopePage       SubscriberScope = "page"       // вся страница
	ScopeComponents SubscriberScope = "components" // только выбранные компоненты
)

// IsValid сообщает, входит ли значение в нормативный enum.
func (s SubscriberScope) IsValid() bool {
	return s == ScopePage || s == ScopeComponents
}

// ── Сущность ──

// Subscriber — внешний клиент, подписанный на обновления страницы по каналу (DESIGN §3.5).
// Подтверждение — double opt-in (Confirmed + ConfirmToken); отписка — по UnsubscribeToken.
// Токены хранятся хэшированными (DESIGN §9); генерация/хэширование — в сервисе подписки (3.4/3.5).
type Subscriber struct {
	ID               uuid.UUID
	StatusPageID     uuid.UUID
	Channel          SubscriberChannel
	Address          string
	Confirmed        bool
	ConfirmToken     *string
	UnsubscribeToken *string
	Scope            SubscriberScope
	ComponentIDs     []uuid.UUID
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// WantsEvent сообщает, должно ли событие, затрагивающее affected-компоненты, быть доставлено
// этому подписчику (DESIGN §3.5):
//   - scope=page — интересует любое событие страницы;
//   - scope=components — только если пересекается с его набором компонентов. Событие без
//     привязки к компонентам (affected пуст) при scope=components не доставляется.
func (s Subscriber) WantsEvent(affected []uuid.UUID) bool {
	if s.Scope == ScopePage {
		return true
	}
	if len(affected) == 0 {
		return false
	}
	want := make(map[uuid.UUID]struct{}, len(s.ComponentIDs))
	for _, id := range s.ComponentIDs {
		want[id] = struct{}{}
	}
	for _, id := range affected {
		if _, ok := want[id]; ok {
			return true
		}
	}
	return false
}
