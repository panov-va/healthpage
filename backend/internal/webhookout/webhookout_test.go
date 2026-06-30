package webhookout

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/notify"
	"github.com/healthpage/backend/internal/store"
)

// ── Render ──

func TestRenderIncident(t *testing.T) {
	raw, err := Render(RenderInput{
		Event: domain.EventIncidentNew, Locale: "ru", PageName: "Acme", PageURL: "https://s/acme",
		Incident: &notify.IncidentPayload{Title: "API упал", Status: "investigating", Impact: "major", Body: "Чиним"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var p map[string]any
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("payload не JSON: %v", err)
	}
	text, _ := p["text"].(string)
	if text == "" || p["event"] != "incident_new" || p["status_page"] != "Acme" {
		t.Errorf("payload неполон: %+v", p)
	}
	if p["incident"] == nil {
		t.Error("ожидалось структурированное поле incident")
	}
	// Текст содержит заголовок и тело.
	for _, want := range []string{"API упал", "Чиним", "Новый инцидент"} {
		if !contains(text, want) {
			t.Errorf("text не содержит %q: %s", want, text)
		}
	}
}

func TestRenderMaintenanceEN(t *testing.T) {
	raw, err := Render(RenderInput{
		Event: domain.EventMaintenanceScheduled, Locale: "en", PageName: "Acme", PageURL: "https://s/acme",
		Maintenance: &notify.MaintenancePayload{Title: "DB upgrade", Status: "scheduled"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var p map[string]any
	_ = json.Unmarshal(raw, &p)
	text, _ := p["text"].(string)
	if !contains(text, "Scheduled maintenance") || !contains(text, "DB upgrade") {
		t.Errorf("EN maintenance text: %s", text)
	}
	if p["maintenance"] == nil {
		t.Error("ожидалось поле maintenance")
	}
}

func TestRenderErrors(t *testing.T) {
	if _, err := Render(RenderInput{Event: domain.EventIncidentNew}); err == nil {
		t.Error("nil incident payload → ошибка")
	}
	if _, err := Render(RenderInput{Event: "bogus"}); err == nil {
		t.Error("неизвестное событие → ошибка")
	}
}

// ── Client ──

func TestClientPostClassification(t *testing.T) {
	cases := []struct {
		code       int
		wantErr    bool
		permanent  bool
		retryAfter bool
	}{
		{200, false, false, false},
		{204, false, false, false},
		{400, true, true, false},
		{404, true, true, false},
		{429, true, false, true},
		{500, true, false, false},
		{503, true, false, false},
	}
	for _, c := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if c.code == 429 {
				w.Header().Set("Retry-After", "7")
			}
			w.WriteHeader(c.code)
		}))
		err := NewClient(nil).Post(context.Background(), srv.URL, []byte(`{"text":"x"}`))
		srv.Close()
		if c.wantErr != (err != nil) {
			t.Errorf("code %d: wantErr=%v, got %v", c.code, c.wantErr, err)
			continue
		}
		if err == nil {
			continue
		}
		perr, ok := err.(*PostError)
		if !ok {
			t.Errorf("code %d: ожидался *PostError, got %T", c.code, err)
			continue
		}
		if perr.Permanent != c.permanent {
			t.Errorf("code %d: Permanent=%v, want %v", c.code, perr.Permanent, c.permanent)
		}
		if c.retryAfter && perr.RetryAfter == 0 {
			t.Errorf("code %d: ожидался RetryAfter", c.code)
		}
	}
}

func TestClientBadURL(t *testing.T) {
	err := NewClient(nil).Post(context.Background(), "://bad", []byte(`{}`))
	perr, ok := err.(*PostError)
	if !ok || !perr.Permanent {
		t.Errorf("битый URL → Permanent PostError, got %v", err)
	}
}

// ── Worker ──

type fakeStore struct {
	notif    domain.Notification
	notFound bool
	page     domain.StatusPage
	sent     []uuid.UUID
}

func (f *fakeStore) NotificationByID(_ context.Context, _ uuid.UUID) (domain.Notification, error) {
	if f.notFound {
		return domain.Notification{}, store.ErrNotFound
	}
	return f.notif, nil
}
func (f *fakeStore) MarkNotificationSent(_ context.Context, id uuid.UUID) error {
	f.sent = append(f.sent, id)
	return nil
}
func (f *fakeStore) StatusPageByID(_ context.Context, _ uuid.UUID) (domain.StatusPage, error) {
	return f.page, nil
}

type fakePoster struct {
	err    error
	called int
}

func (p *fakePoster) Post(_ context.Context, _ string, _ []byte) error {
	p.called++
	return p.err
}

type fakeRetrier struct {
	scheduled bool
	err       error
	called    int
}

func (r *fakeRetrier) Retry(_ context.Context, _ notify.Message) (bool, error) {
	r.called++
	return r.scheduled, r.err
}

func msgBody(t *testing.T) []byte {
	t.Helper()
	pl, _ := json.Marshal(notify.IncidentPayload{Title: "x", Status: "investigating", Impact: "minor"})
	b, _ := json.Marshal(notify.Message{
		NotificationID: uuid.NewString(), Channel: "webhook", Event: string(domain.EventIncidentNew),
		Address: "https://mm/hook", Payload: pl, StatusPageID: uuid.NewString(),
	})
	return b
}

func newWorker(st WorkerStore, p Poster, r Retrier) *Worker {
	return NewWorker(st, p, r, "https://base", nil)
}

func TestWorkerHappyPath(t *testing.T) {
	st := &fakeStore{notif: domain.Notification{Status: domain.NotificationPending}, page: domain.StatusPage{Name: "Acme", Slug: "acme", DefaultLocale: "ru"}}
	p := &fakePoster{}
	w := newWorker(st, p, &fakeRetrier{})
	if d := w.Process(context.Background(), msgBody(t)); d != Ack {
		t.Fatalf("happy: want Ack, got %v", d)
	}
	if p.called != 1 || len(st.sent) != 1 {
		t.Errorf("ожидались Post + MarkSent: called=%d sent=%d", p.called, len(st.sent))
	}
}

func TestWorkerIdempotentAlreadySent(t *testing.T) {
	st := &fakeStore{notif: domain.Notification{Status: domain.NotificationSent}, page: domain.StatusPage{Slug: "acme"}}
	p := &fakePoster{}
	if d := newWorker(st, p, &fakeRetrier{}).Process(context.Background(), msgBody(t)); d != Ack {
		t.Fatalf("already sent: want Ack")
	}
	if p.called != 0 {
		t.Error("уже отправленное не должно постить повторно")
	}
}

func TestWorkerOrphan(t *testing.T) {
	st := &fakeStore{notFound: true}
	if d := newWorker(st, &fakePoster{}, &fakeRetrier{}).Process(context.Background(), msgBody(t)); d != Ack {
		t.Error("orphan notification → Ack (дроп)")
	}
}

func TestWorkerPermanentDrop(t *testing.T) {
	st := &fakeStore{notif: domain.Notification{Status: domain.NotificationPending}, page: domain.StatusPage{Slug: "acme"}}
	p := &fakePoster{err: &PostError{Code: 404, Permanent: true}}
	if d := newWorker(st, p, &fakeRetrier{}).Process(context.Background(), msgBody(t)); d != Ack {
		t.Error("permanent ошибка → Ack (дроп)")
	}
}

func TestWorkerTransientRetryScheduled(t *testing.T) {
	st := &fakeStore{notif: domain.Notification{Status: domain.NotificationPending}, page: domain.StatusPage{Slug: "acme"}}
	p := &fakePoster{err: &PostError{Code: 500}}
	r := &fakeRetrier{scheduled: true}
	if d := newWorker(st, p, r).Process(context.Background(), msgBody(t)); d != Ack {
		t.Error("транзиент + retry запланирован → Ack")
	}
	if r.called != 1 {
		t.Error("ожидался вызов Retry")
	}
}

func TestWorkerRetryExhausted(t *testing.T) {
	st := &fakeStore{notif: domain.Notification{Status: domain.NotificationPending}, page: domain.StatusPage{Slug: "acme"}}
	p := &fakePoster{err: &PostError{Code: 500}}
	r := &fakeRetrier{scheduled: false} // исчерпано
	if d := newWorker(st, p, r).Process(context.Background(), msgBody(t)); d != Reject {
		t.Error("исчерпание ретраев → Reject (DLQ)")
	}
}

func TestWorkerMalformed(t *testing.T) {
	if d := newWorker(&fakeStore{}, &fakePoster{}, &fakeRetrier{}).Process(context.Background(), []byte(`{bad`)); d != Reject {
		t.Error("битое сообщение → Reject")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
