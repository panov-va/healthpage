package email

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

type fakeStore struct {
	notif      *domain.Notification
	notifErr   error
	page       domain.StatusPage
	pageErr    error
	sentMarked uuid.UUID
	markErr    error
}

func (f *fakeStore) NotificationByID(_ context.Context, _ uuid.UUID) (domain.Notification, error) {
	if f.notifErr != nil {
		return domain.Notification{}, f.notifErr
	}
	return *f.notif, nil
}

func (f *fakeStore) StatusPageByID(_ context.Context, _ uuid.UUID) (domain.StatusPage, error) {
	return f.page, f.pageErr
}

func (f *fakeStore) MarkNotificationSent(_ context.Context, id uuid.UUID) error {
	f.sentMarked = id
	return f.markErr
}

type fakeSender struct {
	sent []Email
	err  error
}

func (s *fakeSender) Send(_ context.Context, _ SMTP, msg Email) error {
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, msg)
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

func incidentMsg(t *testing.T, status domain.NotificationStatus) ([]byte, *fakeStore) {
	t.Helper()
	nid := uuid.New()
	n := domain.Notification{ID: nid, Status: status}
	payload, _ := json.Marshal(notify.IncidentPayload{Title: "x", Status: "investigating", Impact: "major"})
	msg := notify.Message{
		NotificationID: nid.String(), SubscriberID: uuid.New().String(), StatusPageID: uuid.New().String(),
		Channel: "email", Event: string(domain.EventIncidentNew), Address: "to@x.test", Payload: payload,
	}
	body, _ := json.Marshal(msg)
	return body, &fakeStore{notif: &n, page: domain.StatusPage{Name: "Acme", Slug: "acme", DefaultLocale: "ru"}}
}

func newWorker(st WorkerStore, sender Sender, retrier Retrier) *Worker {
	return NewWorker(st, sender, retrier, SMTP{}, "https://h", "secret", nil)
}

// ── тесты ──

func TestProcessHappyPath(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending)
	sender := &fakeSender{}
	w := newWorker(st, sender, &fakeRetrier{})

	if d := w.Process(context.Background(), body); d != Ack {
		t.Fatalf("disposition = %v, want Ack", d)
	}
	if len(sender.sent) != 1 || sender.sent[0].To != "to@x.test" {
		t.Fatalf("письмо не отправлено: %+v", sender.sent)
	}
	if st.sentMarked != st.notif.ID {
		t.Errorf("MarkNotificationSent не вызван для %s", st.notif.ID)
	}
}

func TestProcessIdempotentSkip(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationSent)
	sender := &fakeSender{}
	w := newWorker(st, sender, &fakeRetrier{})

	if d := w.Process(context.Background(), body); d != Ack {
		t.Fatalf("disposition = %v, want Ack", d)
	}
	if len(sender.sent) != 0 {
		t.Errorf("уже отправленное не должно слаться повторно: %+v", sender.sent)
	}
}

func TestProcessOrphanNotification(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending)
	st.notifErr = store.ErrNotFound
	sender := &fakeSender{}
	w := newWorker(st, sender, &fakeRetrier{})

	if d := w.Process(context.Background(), body); d != Ack {
		t.Fatalf("сирота-уведомление: disposition = %v, want Ack", d)
	}
	if len(sender.sent) != 0 {
		t.Error("по отсутствующей записи письмо не шлём")
	}
}

func TestProcessSendFailureSchedulesRetry(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending)
	sender := &fakeSender{err: errors.New("smtp down")}
	retrier := &fakeRetrier{scheduled: true}
	w := newWorker(st, sender, retrier)

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
	body, st := incidentMsg(t, domain.NotificationPending)
	sender := &fakeSender{err: errors.New("smtp down")}
	retrier := &fakeRetrier{scheduled: false} // исчерпано
	w := newWorker(st, sender, retrier)

	if d := w.Process(context.Background(), body); d != Reject {
		t.Fatalf("при исчерпании ретраев disposition = %v, want Reject", d)
	}
}

func TestProcessMalformedRejects(t *testing.T) {
	st := &fakeStore{}
	w := newWorker(st, &fakeSender{}, &fakeRetrier{})
	if d := w.Process(context.Background(), []byte("{not json")); d != Reject {
		t.Errorf("битое сообщение: disposition = %v, want Reject", d)
	}
}

func TestProcessUnrenderableRejects(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending)
	st.pageErr = store.ErrNotFound // страница не найдена → собрать письмо нельзя
	w := newWorker(st, &fakeSender{}, &fakeRetrier{})
	if d := w.Process(context.Background(), body); d != Reject {
		t.Errorf("неустранимая ошибка сборки: disposition = %v, want Reject", d)
	}
}
