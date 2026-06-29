package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestSubscriberChannelValidity(t *testing.T) {
	for _, c := range AllSubscriberChannels {
		if !c.IsValid() {
			t.Errorf("канал %q должен быть валиден", c)
		}
	}
	if SubscriberChannel("sms").IsValid() {
		t.Error("неизвестный канал не должен быть валиден")
	}
}

func TestSubscriberChannelIsPush(t *testing.T) {
	push := map[SubscriberChannel]bool{
		ChannelEmail: true, ChannelTelegram: true, ChannelMAX: true, ChannelSlack: true,
		ChannelRSS: false, ChannelICal: false, ChannelWebhook: false,
	}
	for c, want := range push {
		if got := c.IsPush(); got != want {
			t.Errorf("%q.IsPush() = %v, want %v", c, got, want)
		}
	}
}

func TestSubscriberScopeValidity(t *testing.T) {
	if !ScopePage.IsValid() || !ScopeComponents.IsValid() {
		t.Error("page/components должны быть валидны")
	}
	if SubscriberScope("group").IsValid() {
		t.Error("неизвестный scope не должен быть валиден")
	}
}

func TestSubscriberWantsEvent(t *testing.T) {
	c1, c2, c3 := uuid.New(), uuid.New(), uuid.New()

	pageSub := Subscriber{Scope: ScopePage}
	if !pageSub.WantsEvent(nil) {
		t.Error("scope=page хочет любое событие, даже без компонентов")
	}
	if !pageSub.WantsEvent([]uuid.UUID{c1}) {
		t.Error("scope=page хочет событие с компонентами")
	}

	compSub := Subscriber{Scope: ScopeComponents, ComponentIDs: []uuid.UUID{c1, c2}}
	if !compSub.WantsEvent([]uuid.UUID{c2, c3}) {
		t.Error("scope=components хочет событие при пересечении компонентов")
	}
	if compSub.WantsEvent([]uuid.UUID{c3}) {
		t.Error("scope=components не хочет событие без пересечения")
	}
	if compSub.WantsEvent(nil) {
		t.Error("scope=components не хочет событие без привязки к компонентам")
	}
}
