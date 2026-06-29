package telegram

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
)

// ── фейки ──

type fakeBotStore struct {
	pages       map[string]domain.StatusPage // slug → page
	subs        map[uuid.UUID]domain.Subscriber
	createCalls int
	deleteCalls int
}

func newFakeBotStore() *fakeBotStore {
	return &fakeBotStore{pages: map[string]domain.StatusPage{}, subs: map[uuid.UUID]domain.Subscriber{}}
}

func (f *fakeBotStore) addPage(slug, locale string) domain.StatusPage {
	p := domain.StatusPage{ID: uuid.New(), Name: slug, Slug: slug, DefaultLocale: locale}
	f.pages[slug] = p
	return p
}

func (f *fakeBotStore) StatusPageBySlug(_ context.Context, slug string) (domain.StatusPage, error) {
	p, ok := f.pages[slug]
	if !ok {
		return domain.StatusPage{}, store.ErrNotFound
	}
	return p, nil
}

func (f *fakeBotStore) StatusPageByID(_ context.Context, id uuid.UUID) (domain.StatusPage, error) {
	for _, p := range f.pages {
		if p.ID == id {
			return p, nil
		}
	}
	return domain.StatusPage{}, store.ErrNotFound
}

func (f *fakeBotStore) SubscriberByPageChannelAddress(_ context.Context, pageID uuid.UUID, ch domain.SubscriberChannel, addr string) (domain.Subscriber, error) {
	for _, s := range f.subs {
		if s.StatusPageID == pageID && s.Channel == ch && s.Address == addr {
			return s, nil
		}
	}
	return domain.Subscriber{}, store.ErrNotFound
}

func (f *fakeBotStore) SubscribersByChannelAddress(_ context.Context, ch domain.SubscriberChannel, addr string) ([]domain.Subscriber, error) {
	var out []domain.Subscriber
	for _, s := range f.subs {
		if s.Channel == ch && s.Address == addr {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeBotStore) CreateSubscriber(_ context.Context, sub domain.Subscriber) (domain.Subscriber, error) {
	f.createCalls++
	sub.ID = uuid.New()
	f.subs[sub.ID] = sub
	return sub, nil
}

func (f *fakeBotStore) DeleteSubscriber(_ context.Context, id uuid.UUID) error {
	f.deleteCalls++
	delete(f.subs, id)
	return nil
}

// fakeAPI ловит ответы бота (GetUpdates не используется в этих тестах).
type fakeAPI struct{ replies []string }

func (a *fakeAPI) GetUpdates(_ context.Context, _ int64, _ int) ([]Update, error) { return nil, nil }
func (a *fakeAPI) SendMessage(_ context.Context, _ int64, text string) error {
	a.replies = append(a.replies, text)
	return nil
}

func msg(chatID int64, text string) Message {
	return Message{Chat: Chat{ID: chatID, Type: "private"}, Text: text, From: &User{ID: chatID, LanguageCode: "ru"}}
}

// ── parseCommand ──

func TestParseCommand(t *testing.T) {
	cases := []struct{ in, cmd, arg string }{
		{"/start acme", "/start", "acme"},
		{"/start@HealthPageBot acme", "/start", "acme"},
		{"  /STOP  ", "/stop", ""},
		{"/stop acme", "/stop", "acme"},
		{"hello", "hello", ""},
	}
	for _, c := range cases {
		cmd, arg := parseCommand(c.in)
		if cmd != c.cmd || arg != c.arg {
			t.Errorf("parseCommand(%q) = (%q,%q), want (%q,%q)", c.in, cmd, arg, c.cmd, c.arg)
		}
	}
}

// ── /start ──

func TestStartSubscribes(t *testing.T) {
	st := newFakeBotStore()
	page := st.addPage("acme", "ru")
	api := &fakeAPI{}
	b := NewBot(api, st, 0, nil)

	b.handleMessage(context.Background(), msg(777, "/start acme"))

	if st.createCalls != 1 {
		t.Fatalf("ожидалось создание подписчика, createCalls=%d", st.createCalls)
	}
	sub, err := st.SubscriberByPageChannelAddress(context.Background(), page.ID, domain.ChannelTelegram, "777")
	if err != nil {
		t.Fatalf("подписчик не найден: %v", err)
	}
	if !sub.Confirmed {
		t.Error("подписка Telegram должна быть подтверждена сразу")
	}
	if sub.Scope != domain.ScopePage {
		t.Errorf("scope = %q, want page", sub.Scope)
	}
	if len(api.replies) != 1 {
		t.Fatalf("ожидался один ответ, got %d", len(api.replies))
	}
}

func TestStartIdempotent(t *testing.T) {
	st := newFakeBotStore()
	st.addPage("acme", "ru")
	api := &fakeAPI{}
	b := NewBot(api, st, 0, nil)

	b.handleMessage(context.Background(), msg(777, "/start acme"))
	b.handleMessage(context.Background(), msg(777, "/start acme"))

	if st.createCalls != 1 {
		t.Errorf("повторный /start не должен создавать дубликат, createCalls=%d", st.createCalls)
	}
}

func TestStartNoArg(t *testing.T) {
	st := newFakeBotStore()
	api := &fakeAPI{}
	b := NewBot(api, st, 0, nil)

	b.handleMessage(context.Background(), msg(777, "/start"))

	if st.createCalls != 0 {
		t.Error("/start без аргумента не подписывает")
	}
	if len(api.replies) != 1 {
		t.Fatalf("ожидалась подсказка, got %d ответов", len(api.replies))
	}
}

func TestStartUnknownPage(t *testing.T) {
	st := newFakeBotStore()
	api := &fakeAPI{}
	b := NewBot(api, st, 0, nil)

	b.handleMessage(context.Background(), msg(777, "/start nope"))

	if st.createCalls != 0 {
		t.Error("неизвестная страница не подписывает")
	}
	if len(api.replies) != 1 {
		t.Fatalf("ожидался ответ «не найдено», got %d", len(api.replies))
	}
}

// ── /stop ──

func TestStopOnePage(t *testing.T) {
	st := newFakeBotStore()
	st.addPage("acme", "ru")
	st.addPage("beta", "ru")
	api := &fakeAPI{}
	b := NewBot(api, st, 0, nil)
	b.handleMessage(context.Background(), msg(777, "/start acme"))
	b.handleMessage(context.Background(), msg(777, "/start beta"))

	b.handleMessage(context.Background(), msg(777, "/stop acme"))

	if st.deleteCalls != 1 {
		t.Fatalf("ожидалось удаление одной подписки, deleteCalls=%d", st.deleteCalls)
	}
	left, _ := st.SubscribersByChannelAddress(context.Background(), domain.ChannelTelegram, "777")
	if len(left) != 1 || st.pages["beta"].ID != left[0].StatusPageID {
		t.Errorf("должна остаться подписка на beta: %+v", left)
	}
}

func TestStopAll(t *testing.T) {
	st := newFakeBotStore()
	st.addPage("acme", "ru")
	st.addPage("beta", "ru")
	api := &fakeAPI{}
	b := NewBot(api, st, 0, nil)
	b.handleMessage(context.Background(), msg(777, "/start acme"))
	b.handleMessage(context.Background(), msg(777, "/start beta"))

	b.handleMessage(context.Background(), msg(777, "/stop"))

	if st.deleteCalls != 2 {
		t.Fatalf("ожидалось удаление всех подписок, deleteCalls=%d", st.deleteCalls)
	}
	left, _ := st.SubscribersByChannelAddress(context.Background(), domain.ChannelTelegram, "777")
	if len(left) != 0 {
		t.Errorf("подписок не должно остаться: %+v", left)
	}
}

func TestStopNothing(t *testing.T) {
	st := newFakeBotStore()
	api := &fakeAPI{}
	b := NewBot(api, st, 0, nil)

	b.handleMessage(context.Background(), msg(777, "/stop"))

	if st.deleteCalls != 0 {
		t.Error("нечего удалять")
	}
	if len(api.replies) != 1 {
		t.Fatalf("ожидался ответ «нет подписок», got %d", len(api.replies))
	}
}
