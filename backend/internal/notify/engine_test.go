package notify

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
)

// ── фейки ──

type fakeStore struct {
	subs      []domain.Subscriber
	created   []domain.Notification
	attempts  map[uuid.UUID]int
	failed    map[uuid.UUID]bool
	createErr error
}

func newFakeStore(subs ...domain.Subscriber) *fakeStore {
	return &fakeStore{subs: subs, attempts: map[uuid.UUID]int{}, failed: map[uuid.UUID]bool{}}
}

func (f *fakeStore) ListConfirmedSubscribers(_ context.Context, _ uuid.UUID) ([]domain.Subscriber, error) {
	return f.subs, nil
}

func (f *fakeStore) CreateNotification(_ context.Context, sid uuid.UUID, ev domain.EventType, payload []byte) (domain.Notification, error) {
	if f.createErr != nil {
		return domain.Notification{}, f.createErr
	}
	n := domain.Notification{ID: uuid.New(), SubscriberID: sid, EventType: ev, Payload: payload, Status: domain.NotificationPending}
	f.created = append(f.created, n)
	return n, nil
}

func (f *fakeStore) MarkNotificationFailed(_ context.Context, id uuid.UUID) error {
	f.failed[id] = true
	return nil
}

func (f *fakeStore) IncrementNotificationAttempts(_ context.Context, id uuid.UUID) (int, error) {
	f.attempts[id]++
	return f.attempts[id], nil
}

type publishedMsg struct {
	channel string
	event   string
	delay   time.Duration
	delayed bool
	msg     Message
}

type fakePublisher struct {
	msgs []publishedMsg
	err  error
}

func (p *fakePublisher) PublishNotification(_ context.Context, channel, event string, body []byte) error {
	if p.err != nil {
		return p.err
	}
	var m Message
	_ = json.Unmarshal(body, &m)
	p.msgs = append(p.msgs, publishedMsg{channel: channel, event: event, msg: m})
	return nil
}

func (p *fakePublisher) PublishNotificationDelayed(_ context.Context, channel, event string, body []byte, delay time.Duration) error {
	if p.err != nil {
		return p.err
	}
	var m Message
	_ = json.Unmarshal(body, &m)
	p.msgs = append(p.msgs, publishedMsg{channel: channel, event: event, delay: delay, delayed: true, msg: m})
	return nil
}

// PublishWebhookOut — публикация исходящего webhook'а (этап 5.4): фиксируем как channel=webhook.
func (p *fakePublisher) PublishWebhookOut(_ context.Context, body []byte) error {
	if p.err != nil {
		return p.err
	}
	var m Message
	_ = json.Unmarshal(body, &m)
	p.msgs = append(p.msgs, publishedMsg{channel: m.Channel, event: m.Event, msg: m})
	return nil
}

// ── тесты ──

func sub(channel domain.SubscriberChannel, scope domain.SubscriberScope, comps ...uuid.UUID) domain.Subscriber {
	return domain.Subscriber{ID: uuid.New(), Channel: channel, Address: "a@b.c", Scope: scope, ComponentIDs: comps, Confirmed: true}
}

func TestIncidentCreatedFanOut(t *testing.T) {
	c1, c2 := uuid.New(), uuid.New()
	emailPage := sub(domain.ChannelEmail, domain.ScopePage)
	tgComp := sub(domain.ChannelTelegram, domain.ScopeComponents, c1)
	maxOther := sub(domain.ChannelMAX, domain.ScopeComponents, c2) // не пересекается
	rssPage := sub(domain.ChannelRSS, domain.ScopePage)            // не push

	st := newFakeStore(emailPage, tgComp, maxOther, rssPage)
	pub := &fakePublisher{}
	e := New(st, pub, nil)

	inc := domain.Incident{
		ID:            uuid.New(),
		StatusPageID:  uuid.New(),
		Title:         "API down",
		CurrentStatus: domain.IncidentInvestigating,
		Impact:        domain.ImpactMajor,
		Components:    []domain.IncidentComponent{{ComponentID: c1, ComponentStatusInIncident: domain.StatusMajorOutage}},
	}
	if err := e.IncidentCreated(context.Background(), inc, "Investigating"); err != nil {
		t.Fatalf("IncidentCreated: %v", err)
	}

	// Доставляется email(page) и telegram(c1); max(c2) и rss(не push) — нет.
	if len(pub.msgs) != 2 {
		t.Fatalf("опубликовано %d сообщений, want 2: %+v", len(pub.msgs), pub.msgs)
	}
	if len(st.created) != 2 {
		t.Fatalf("создано %d записей журнала, want 2", len(st.created))
	}
	gotChannels := map[string]bool{}
	for _, m := range pub.msgs {
		gotChannels[m.channel] = true
		if m.event != string(domain.EventIncidentNew) {
			t.Errorf("event = %q, want incident_new", m.event)
		}
		if m.msg.NotificationID == "" {
			t.Error("в сообщении нет notification_id (ключ идемпотентности)")
		}
		var p IncidentPayload
		if err := json.Unmarshal(m.msg.Payload, &p); err != nil || p.Title != "API down" {
			t.Errorf("payload некорректен: %v / %+v", err, p)
		}
	}
	if !gotChannels["email"] || !gotChannels["telegram"] {
		t.Errorf("ожидались каналы email+telegram, got %v", gotChannels)
	}
	if gotChannels["max"] || gotChannels["rss"] {
		t.Errorf("max/rss не должны были получить уведомление, got %v", gotChannels)
	}
}

func TestIncidentCreatedFanOutWebhook(t *testing.T) {
	// Исходящий webhook (этап 5.4) — доставляемый канал: попадает в фан-аут через PublishWebhookOut.
	whPage := sub(domain.ChannelWebhook, domain.ScopePage)
	whPage.Address = "https://mm.example/hooks/abc"
	st := newFakeStore(whPage, sub(domain.ChannelRSS, domain.ScopePage)) // rss не доставляется
	pub := &fakePublisher{}
	e := New(st, pub, nil)

	inc := domain.Incident{ID: uuid.New(), StatusPageID: uuid.New(), Title: "API down", CurrentStatus: domain.IncidentInvestigating, Impact: domain.ImpactMajor}
	if err := e.IncidentCreated(context.Background(), inc, "Investigating"); err != nil {
		t.Fatalf("IncidentCreated: %v", err)
	}
	if len(pub.msgs) != 1 {
		t.Fatalf("опубликовано %d, want 1 (только webhook): %+v", len(pub.msgs), pub.msgs)
	}
	if pub.msgs[0].channel != string(domain.ChannelWebhook) || pub.msgs[0].msg.Address != "https://mm.example/hooks/abc" {
		t.Errorf("webhook-сообщение некорректно: %+v", pub.msgs[0])
	}
	if pub.msgs[0].msg.NotificationID == "" {
		t.Error("нет notification_id у webhook-сообщения")
	}
}

func TestIncidentUpdatedRoutingKey(t *testing.T) {
	st := newFakeStore(sub(domain.ChannelEmail, domain.ScopePage))
	pub := &fakePublisher{}
	e := New(st, pub, nil)

	inc := domain.Incident{ID: uuid.New(), StatusPageID: uuid.New(), Title: "x", CurrentStatus: domain.IncidentMonitoring, Impact: domain.ImpactMinor}
	upd := domain.IncidentUpdate{ID: uuid.New(), Status: domain.IncidentMonitoring, Body: "watching"}
	if err := e.IncidentUpdated(context.Background(), inc, upd); err != nil {
		t.Fatalf("IncidentUpdated: %v", err)
	}
	if len(pub.msgs) != 1 || pub.msgs[0].event != string(domain.EventIncidentUpdate) {
		t.Fatalf("ожидалось 1 сообщение incident_update, got %+v", pub.msgs)
	}
	var p IncidentPayload
	_ = json.Unmarshal(pub.msgs[0].msg.Payload, &p)
	if p.Body != "watching" {
		t.Errorf("payload.body = %q, want watching", p.Body)
	}
}

func TestMaintenanceEvent(t *testing.T) {
	c1 := uuid.New()
	st := newFakeStore(sub(domain.ChannelSlack, domain.ScopeComponents, c1))
	pub := &fakePublisher{}
	e := New(st, pub, nil)

	m := domain.Maintenance{ID: uuid.New(), StatusPageID: uuid.New(), Title: "upgrade", Status: domain.MaintenanceInProgress, ComponentIDs: []uuid.UUID{c1}}
	if err := e.MaintenanceEvent(context.Background(), m, domain.EventMaintenanceStarted); err != nil {
		t.Fatalf("MaintenanceEvent: %v", err)
	}
	if len(pub.msgs) != 1 || pub.msgs[0].event != string(domain.EventMaintenanceStarted) || pub.msgs[0].channel != "slack" {
		t.Fatalf("ожидалось 1 slack-сообщение maintenance_started, got %+v", pub.msgs)
	}
}

func TestRetrySchedulesDelayed(t *testing.T) {
	st := newFakeStore()
	pub := &fakePublisher{}
	e := New(st, pub, nil)
	nid := uuid.New()

	msg := Message{NotificationID: nid.String(), Channel: "email", Event: string(domain.EventIncidentNew)}
	scheduled, err := e.Retry(context.Background(), msg)
	if err != nil || !scheduled {
		t.Fatalf("первый ретрай должен быть запланирован: scheduled=%v err=%v", scheduled, err)
	}
	if len(pub.msgs) != 1 || !pub.msgs[0].delayed || pub.msgs[0].delay != time.Minute {
		t.Fatalf("ожидалась отложенная публикация с delay=1m, got %+v", pub.msgs)
	}
	if pub.msgs[0].msg.Attempt != 1 {
		t.Errorf("attempt в сообщении = %d, want 1", pub.msgs[0].msg.Attempt)
	}
}

func TestRetryExhaustedMarksFailed(t *testing.T) {
	st := newFakeStore()
	pub := &fakePublisher{}
	e := New(st, pub, nil)
	nid := uuid.New()
	st.attempts[nid] = MaxAttempts // следующий инкремент превысит расписание

	msg := Message{NotificationID: nid.String(), Channel: "email", Event: string(domain.EventIncidentNew)}
	scheduled, err := e.Retry(context.Background(), msg)
	if err != nil {
		t.Fatalf("Retry: %v", err)
	}
	if scheduled {
		t.Error("ретрай не должен планироваться после исчерпания")
	}
	if !st.failed[nid] {
		t.Error("исчерпанная запись должна быть помечена failed")
	}
	if len(pub.msgs) != 0 {
		t.Errorf("после исчерпания не должно быть публикаций, got %+v", pub.msgs)
	}
}

func TestDispatchContinuesOnPublishError(t *testing.T) {
	st := newFakeStore(sub(domain.ChannelEmail, domain.ScopePage))
	pub := &fakePublisher{err: errors.New("broker down")}
	e := New(st, pub, nil)

	inc := domain.Incident{ID: uuid.New(), StatusPageID: uuid.New(), Title: "x", CurrentStatus: domain.IncidentInvestigating, Impact: domain.ImpactNone}
	err := e.IncidentCreated(context.Background(), inc, "body")
	if err == nil {
		t.Error("ожидалась ошибка публикации")
	}
	// Запись журнала всё равно создана (pending — восстановима).
	if len(st.created) != 1 {
		t.Errorf("запись журнала должна остаться, created=%d", len(st.created))
	}
}
