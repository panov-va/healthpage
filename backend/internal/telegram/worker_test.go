package telegram

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

type fakeSender struct {
	chatIDs []int64
	texts   []string
	err     error
}

func (s *fakeSender) SendMessage(_ context.Context, chatID int64, text string) error {
	if s.err != nil {
		return s.err
	}
	s.chatIDs = append(s.chatIDs, chatID)
	s.texts = append(s.texts, text)
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
		Channel: "telegram", Event: string(domain.EventIncidentNew), Address: address, Payload: payload,
	}
	body, _ := json.Marshal(msg)
	return body, &fakeWorkerStore{notif: &n, page: domain.StatusPage{Name: "Acme", Slug: "acme", DefaultLocale: "ru"}}
}

func newWorker(st WorkerStore, sender Sender, retrier Retrier) *Worker {
	return NewWorker(st, sender, retrier, "https://h", nil)
}

// ── тесты ──

func TestProcessHappyPath(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending, "12345")
	sender := &fakeSender{}
	w := newWorker(st, sender, &fakeRetrier{})

	if d := w.Process(context.Background(), body); d != Ack {
		t.Fatalf("disposition = %v, want Ack", d)
	}
	if len(sender.chatIDs) != 1 || sender.chatIDs[0] != 12345 {
		t.Fatalf("сообщение не отправлено в чат: %+v", sender.chatIDs)
	}
	if st.sentMarked != st.notif.ID {
		t.Errorf("MarkNotificationSent не вызван для %s", st.notif.ID)
	}
}

func TestProcessIdempotentSkip(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationSent, "12345")
	sender := &fakeSender{}
	w := newWorker(st, sender, &fakeRetrier{})

	if d := w.Process(context.Background(), body); d != Ack {
		t.Fatalf("disposition = %v, want Ack", d)
	}
	if len(sender.texts) != 0 {
		t.Errorf("уже отправленное не должно слаться повторно: %+v", sender.texts)
	}
}

func TestProcessOrphanNotification(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending, "12345")
	st.notifErr = store.ErrNotFound
	sender := &fakeSender{}
	w := newWorker(st, sender, &fakeRetrier{})

	if d := w.Process(context.Background(), body); d != Ack {
		t.Fatalf("сирота-уведомление: disposition = %v, want Ack", d)
	}
	if len(sender.texts) != 0 {
		t.Error("по отсутствующей записи сообщение не шлём")
	}
}

func TestProcessBadAddressRejects(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending, "not-a-number")
	w := newWorker(st, &fakeSender{}, &fakeRetrier{})
	if d := w.Process(context.Background(), body); d != Reject {
		t.Errorf("нечисловой chat_id: disposition = %v, want Reject", d)
	}
}

func TestProcessTransientFailureSchedulesRetry(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending, "12345")
	sender := &fakeSender{err: &APIError{Method: "sendMessage", Code: 500}} // 5xx — транзиентная
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
	body, st := incidentMsg(t, domain.NotificationPending, "12345")
	sender := &fakeSender{err: &APIError{Method: "sendMessage", Code: 500}}
	retrier := &fakeRetrier{scheduled: false} // исчерпано
	w := newWorker(st, sender, retrier)

	if d := w.Process(context.Background(), body); d != Reject {
		t.Fatalf("при исчерпании ретраев disposition = %v, want Reject", d)
	}
}

func TestProcessPermanentErrorDrops(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending, "12345")
	// 403 — бот заблокирован пользователем (Permanent). Ретрай не нужен, доставку дропаем (Ack).
	sender := &fakeSender{err: &APIError{Method: "sendMessage", Code: 403, Permanent: true}}
	retrier := &fakeRetrier{}
	w := newWorker(st, sender, retrier)

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
	st := &fakeWorkerStore{}
	w := newWorker(st, &fakeSender{}, &fakeRetrier{})
	if d := w.Process(context.Background(), []byte("{not json")); d != Reject {
		t.Errorf("битое сообщение: disposition = %v, want Reject", d)
	}
}

func TestProcessUnrenderableRejects(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending, "12345")
	st.pageErr = store.ErrNotFound // страница не найдена → собрать сообщение нельзя
	w := newWorker(st, &fakeSender{}, &fakeRetrier{})
	if d := w.Process(context.Background(), body); d != Reject {
		t.Errorf("неустранимая ошибка сборки: disposition = %v, want Reject", d)
	}
}

func TestProcessMarkSentErrorRequeues(t *testing.T) {
	body, st := incidentMsg(t, domain.NotificationPending, "12345")
	st.markErr = errors.New("db down")
	w := newWorker(st, &fakeSender{}, &fakeRetrier{})
	if d := w.Process(context.Background(), body); d != Requeue {
		t.Errorf("ошибка отметки sent: disposition = %v, want Requeue", d)
	}
}
