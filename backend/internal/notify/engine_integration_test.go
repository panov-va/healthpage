package notify_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/notify"
	"github.com/healthpage/backend/internal/queue"
	"github.com/healthpage/backend/internal/store"
)

// Живой e2e движка уведомлений: реальные PostgreSQL + RabbitMQ. Проверяет всю цепочку 3.3 —
// фан-аут по подтверждённым подписчикам, запись журнала и маршрутизацию сообщения в q.<channel>.
// Запуск:
//
//	HEALTHPAGE_TEST_DB="postgres://healthpage:healthpage@localhost:5432/healthpage?sslmode=disable" \
//	HEALTHPAGE_TEST_AMQP="amqp://healthpage:healthpage@localhost:5672/" \
//	  go test ./internal/notify/ -run TestEngineIntegration
func TestEngineIntegration(t *testing.T) {
	dsn := os.Getenv("HEALTHPAGE_TEST_DB")
	amqpURL := os.Getenv("HEALTHPAGE_TEST_AMQP")
	if dsn == "" || amqpURL == "" {
		t.Skip("HEALTHPAGE_TEST_DB / HEALTHPAGE_TEST_AMQP not set; skipping integration test")
	}
	ctx := context.Background()

	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	defer st.Close()

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
		t.Fatalf("declare topology: %v", err)
	}
	if _, err := ch.QueuePurge(queue.WorkQueue("email"), false); err != nil {
		t.Fatalf("purge: %v", err)
	}

	pub, err := queue.NewPublisher(conn)
	if err != nil {
		t.Fatalf("publisher: %v", err)
	}
	defer func() { _ = pub.Close() }()

	// Данные: оператор → страница → подтверждённый email-подписчик (scope=page).
	raw, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("raw pool: %v", err)
	}
	defer raw.Close()

	email := "it-" + uuid.NewString() + "@example.test"
	user, account, err := st.CreateUserWithAccount(ctx, email, "hash", "IT", "IT acc", "ru")
	if err != nil {
		t.Fatalf("CreateUserWithAccount: %v", err)
	}
	t.Cleanup(func() {
		// Каскад аккаунта чистит страницу/подписчиков/уведомления.
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE id=$1", account.ID)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", user.ID)
	})

	slug := "it-" + uuid.NewString()[:8]
	page, err := st.CreateStatusPage(ctx, account.ID, user.ID, "Demo", "", slug, "UTC", "ru", "public")
	if err != nil {
		t.Fatalf("CreateStatusPage: %v", err)
	}

	if _, err := st.CreateSubscriber(ctx, domain.Subscriber{
		StatusPageID: page.ID, Channel: domain.ChannelEmail, Address: "client@example.test",
		Confirmed: true, Scope: domain.ScopePage,
	}); err != nil {
		t.Fatalf("CreateSubscriber: %v", err)
	}

	// Движок рассылает событие нового инцидента.
	eng := notify.New(st, pub, nil)
	inc := domain.Incident{
		ID: uuid.New(), StatusPageID: page.ID, Title: "DB down",
		CurrentStatus: domain.IncidentInvestigating, Impact: domain.ImpactMajor,
	}
	if err := eng.IncidentCreated(ctx, inc, "Investigating"); err != nil {
		t.Fatalf("IncidentCreated: %v", err)
	}

	// Сообщение должно появиться в q.email с notification_id и корректным payload.
	var got notify.Message
	deadline := time.Now().Add(3 * time.Second)
	for {
		d, ok, err := ch.Get(queue.WorkQueue("email"), true)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if ok {
			if err := json.Unmarshal(d.Body, &got); err != nil {
				t.Fatalf("unmarshal message: %v", err)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("сообщение не пришло в q.email за 3s")
		}
		time.Sleep(50 * time.Millisecond)
	}

	if got.NotificationID == "" {
		t.Error("в сообщении нет notification_id")
	}
	if got.Channel != "email" || got.Event != string(domain.EventIncidentNew) {
		t.Errorf("channel/event = %s/%s, want email/incident_new", got.Channel, got.Event)
	}
	if got.Address != "client@example.test" {
		t.Errorf("address = %q", got.Address)
	}
	// notification_id должен указывать на реально созданную запись журнала (идемпотентность §8.1).
	nid, err := uuid.Parse(got.NotificationID)
	if err != nil {
		t.Fatalf("bad notification_id: %v", err)
	}
	n, err := st.NotificationByID(ctx, nid)
	if err != nil || n.Status != domain.NotificationPending {
		t.Errorf("запись журнала: err=%v status=%v", err, n.Status)
	}
	var p notify.IncidentPayload
	if err := json.Unmarshal(got.Payload, &p); err != nil || p.Title != "DB down" {
		t.Errorf("payload: %v / %+v", err, p)
	}
}
