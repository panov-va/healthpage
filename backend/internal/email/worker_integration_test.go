package email_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/email"
	"github.com/healthpage/backend/internal/notify"
	"github.com/healthpage/backend/internal/queue"
	"github.com/healthpage/backend/internal/store"
)

// captureSender запоминает отправленные письма (вместо реального SMTP).
type captureSender struct{ sent []email.Email }

func (s *captureSender) Send(_ context.Context, _ email.SMTP, msg email.Email) error {
	s.sent = append(s.sent, msg)
	return nil
}

// Живой e2e worker-email: engine публикует событие → q.email → worker.Process доставляет и
// помечает уведомление sent; повторная обработка того же сообщения идемпотентна.
// Запуск: HEALTHPAGE_TEST_DB=... HEALTHPAGE_TEST_AMQP=... go test ./internal/email/ -run TestWorkerIntegration
func TestWorkerIntegration(t *testing.T) {
	dsn := os.Getenv("HEALTHPAGE_TEST_DB")
	amqpURL := os.Getenv("HEALTHPAGE_TEST_AMQP")
	if dsn == "" || amqpURL == "" {
		t.Skip("HEALTHPAGE_TEST_DB / HEALTHPAGE_TEST_AMQP not set; skipping")
	}
	ctx := context.Background()

	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	defer st.Close()
	raw, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("raw pool: %v", err)
	}
	defer raw.Close()

	conn, err := queue.Dial(amqpURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("channel: %v", err)
	}
	defer func() { _ = ch.Close() }()
	if err := queue.DeclareTopology(ch); err != nil {
		t.Fatalf("topology: %v", err)
	}
	if _, err := ch.QueuePurge(queue.WorkQueue("email"), false); err != nil {
		t.Fatalf("purge: %v", err)
	}
	pub, err := queue.NewPublisher(conn)
	if err != nil {
		t.Fatalf("publisher: %v", err)
	}
	defer func() { _ = pub.Close() }()

	// Данные: страница + подтверждённый email-подписчик.
	em := "it-" + uuid.NewString() + "@example.test"
	user, account, err := st.CreateUserWithAccount(ctx, em, "hash", "IT", "IT", "ru")
	if err != nil {
		t.Fatalf("CreateUserWithAccount: %v", err)
	}
	t.Cleanup(func() {
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE id=$1", account.ID)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", user.ID)
	})
	slug := "it-" + uuid.NewString()[:8]
	page, err := st.CreateStatusPage(ctx, account.ID, user.ID, "Acme", "", slug, "UTC", "ru", "public")
	if err != nil {
		t.Fatalf("CreateStatusPage: %v", err)
	}
	if _, err := st.CreateSubscriber(ctx, domain.Subscriber{
		StatusPageID: page.ID, Channel: domain.ChannelEmail, Address: "client@example.test",
		Confirmed: true, Scope: domain.ScopePage,
	}); err != nil {
		t.Fatalf("CreateSubscriber: %v", err)
	}

	// Engine публикует новый инцидент → сообщение попадает в q.email.
	engine := notify.New(st, pub, nil)
	inc := domain.Incident{
		ID: uuid.New(), StatusPageID: page.ID, Title: "Down",
		CurrentStatus: domain.IncidentInvestigating, Impact: domain.ImpactMajor,
	}
	if err := engine.IncidentCreated(ctx, inc, "Investigating"); err != nil {
		t.Fatalf("IncidentCreated: %v", err)
	}

	// Достаём сообщение и прогоняем через воркер с capture-отправителем.
	var body []byte
	deadline := time.Now().Add(3 * time.Second)
	for {
		d, ok, err := ch.Get(queue.WorkQueue("email"), true)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if ok {
			body = d.Body
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("сообщение не пришло в q.email за 3s")
		}
		time.Sleep(50 * time.Millisecond)
	}

	sender := &captureSender{}
	worker := email.NewWorker(st, sender, engine, email.SMTP{}, "https://h", "https://h", "secret", nil)

	if d := worker.Process(ctx, body); d != email.Ack {
		t.Fatalf("Process = %v, want Ack", d)
	}
	if len(sender.sent) != 1 || sender.sent[0].To != "client@example.test" {
		t.Fatalf("письмо не отправлено: %+v", sender.sent)
	}

	// Повторная обработка того же сообщения — идемпотентна (письмо не дублируется).
	if d := worker.Process(ctx, body); d != email.Ack {
		t.Fatalf("повторный Process = %v, want Ack", d)
	}
	if len(sender.sent) != 1 {
		t.Errorf("идемпотентность нарушена: писем %d, want 1", len(sender.sent))
	}
}
