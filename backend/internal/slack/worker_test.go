package slack

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/notify"
	"github.com/healthpage/backend/internal/store"
)

// ── фейки ──

type fakeWorkerStore struct {
	notif      *domain.Notification
	notifErr   error
	page       domain.StatusPage
	pageErr    error
	sentMarked uuid.UUID
	markErr    error
}

func (f *fakeWorkerStore) NotificationByID(_ context.Context, _ uuid.UUID) (domain.Notification, error) {
	if f.notifErr != nil {
		return domain.Notification{}, f.notifErr
	}
	return *f.notif, nil
}
func (f *fakeWorkerStore) StatusPageByID(_ context.Context, _ uuid.UUID) (domain.StatusPage, error) {
	return f.page, f.pageErr
}
func (f *fakeWorkerStore) MarkNotificationSent(_ context.Context, id uuid.UUID) error {
	f.sentMarked = id
	return f.markErr
}

type fakePoster struct {
	urls     []string
	payloads [][]byte
	err      error
}

func (p *fakePoster) PostMessage(_ context.Context, url string, payload []byte) error {
	if p.err != nil {
		return p.err
	}
	p.urls = append(p.urls, url)
	p.payloads = append(p.payloads, payload)
	return nil
}

type fakeRetrier struct {
	scheduled bool
	err       error
	calls     int
}

func (r *fakeRetrier) Retry(_ context.Context, _ notify.Message) (bool, error) {
	r.calls++
	return r.scheduled, r.err
}

// ── хелперы ──

func incidentMsg(t *testing.T, status domain.NotificationStatus, address string) ([]byte, *fakeWorkerStore) {
	t.Helper()
	nid := uuid.New()
	n := domain.Notification{ID: nid, Status: status}
	payload, _ := json.Marshal(notify.IncidentPayload{Title: "x", Status: "investigating", Impact: "major"})
	msg := notify.Message{
		NotificationID: nid.String(), SubscriberID: uuid.New().String(), StatusPageID: uuid.New().String(),
		Channel: "slack", Event: string(domain.EventIncidentNew), Address: address, Payload: payload,
	}
	body, _ := json.Marshal(msg)
	return body, &fakeWorkerStore{notif: &n, page: domain.StatusPage{Name: "Acme", Slug: "acme", DefaultLocale: "ru"}}
}

const hookURL = "https://hooks.slack.com/services/X/Y/Z"

func newWorker(st WorkerStore, poster Poster, retrier Retrier) *Worker {
	return NewWorker(st, poster, retrier, "https://h", nil)
}

// ── тесты ──

func TestProcessHappyPath(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending, hookURL)
	poster := &fakePoster{}
	w := newWorker(st, poster, &fakeRetrier{})

	if d := w.Process(context.Background(), body); d != Ack {
		t.Fatalf("disposition = %v, want Ack", d)
	}
	if len(poster.urls) != 1 || poster.urls[0] != hookURL {
		t.Fatalf("сообщение не отправлено в webhook: %+v", poster.urls)
	}
	if st.sentMarked != st.notif.ID {
		t.Errorf("MarkNotificationSent не вызван для %s", st.notif.ID)
	}
}

func TestProcessIdempotentSkip(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationSent, hookURL)
	poster := &fakePoster{}
	w := newWorker(st, poster, &fakeRetrier{})

	if d := w.Process(context.Background(), body); d != Ack {
		t.Fatalf("disposition = %v, want Ack", d)
	}
	if len(poster.payloads) != 0 {
		t.Errorf("уже отправленное не должно слаться повторно: %+v", poster.payloads)
	}
}

func TestProcessOrphanNotification(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending, hookURL)
	st.notifErr = store.ErrNotFound
	w := newWorker(st, &fakePoster{}, &fakeRetrier{})
	if d := w.Process(context.Background(), body); d != Ack {
		t.Fatalf("сирота-уведомление: disposition = %v, want Ack", d)
	}
}

func TestProcessEmptyAddressRejects(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending, "")
	w := newWorker(st, &fakePoster{}, &fakeRetrier{})
	if d := w.Process(context.Background(), body); d != Reject {
		t.Errorf("пустой webhook url: disposition = %v, want Reject", d)
	}
}

func TestProcessTransientFailureSchedulesRetry(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending, hookURL)
	poster := &fakePoster{err: &PostError{Code: 502}} // 5xx — транзиент
	retrier := &fakeRetrier{scheduled: true}
	w := newWorker(st, poster, retrier)

	if d := w.Process(context.Background(), body); d != Ack {
		t.Fatalf("при запланированном ретрае disposition = %v, want Ack", d)
	}
	if retrier.calls != 1 {
		t.Errorf("ретрай должен быть вызван 1 раз, got %d", retrier.calls)
	}
	if st.sentMarked != uuid.Nil {
		t.Error("при неуспешной отправке sent не помечаем")
	}
}

func TestProcessRetryExhaustedRejects(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending, hookURL)
	poster := &fakePoster{err: &PostError{Code: 502}}
	w := newWorker(st, poster, &fakeRetrier{scheduled: false})
	if d := w.Process(context.Background(), body); d != Reject {
		t.Fatalf("при исчерпании ретраев disposition = %v, want Reject", d)
	}
}

func TestProcessPermanentErrorDrops(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending, hookURL)
	poster := &fakePoster{err: &PostError{Code: 404, Permanent: true}} // webhook отозван
	retrier := &fakeRetrier{}
	w := newWorker(st, poster, retrier)

	if d := w.Process(context.Background(), body); d != Ack {
		t.Fatalf("перманентная ошибка: disposition = %v, want Ack", d)
	}
	if retrier.calls != 0 {
		t.Error("перманентную ошибку не ретраим")
	}
	if st.sentMarked != uuid.Nil {
		t.Error("недоставленное не помечаем sent")
	}
}

func TestProcessMalformedRejects(t *testing.T) {
	w := newWorker(&fakeWorkerStore{}, &fakePoster{}, &fakeRetrier{})
	if d := w.Process(context.Background(), []byte("{not json")); d != Reject {
		t.Errorf("битое сообщение: disposition = %v, want Reject", d)
	}
}

func TestProcessUnrenderableRejects(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending, hookURL)
	st.pageErr = store.ErrNotFound
	w := newWorker(st, &fakePoster{}, &fakeRetrier{})
	if d := w.Process(context.Background(), body); d != Reject {
		t.Errorf("неустранимая ошибка сборки: disposition = %v, want Reject", d)
	}
}

func TestProcessMarkSentErrorRequeues(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending, hookURL)
	st.markErr = errors.New("db down")
	w := newWorker(st, &fakePoster{}, &fakeRetrier{})
	if d := w.Process(context.Background(), body); d != Requeue {
		t.Errorf("ошибка отметки sent: disposition = %v, want Requeue", d)
	}
}
