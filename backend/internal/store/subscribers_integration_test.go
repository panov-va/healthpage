package store_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
)

// Интеграционный тест подписчиков и журнала уведомлений против реального PostgreSQL.
// Запуск: HEALTHPAGE_TEST_DB=... go test ./internal/store/ -run TestSubscribersIntegration
func TestSubscribersIntegration(t *testing.T) {
	dsn := os.Getenv("HEALTHPAGE_TEST_DB")
	if dsn == "" {
		t.Skip("HEALTHPAGE_TEST_DB not set; skipping integration test")
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

	email := "it-" + uuid.NewString() + "@example.test"
	user, account, err := st.CreateUserWithAccount(ctx, email, "hash", "IT", "IT acc", "ru")
	if err != nil {
		t.Fatalf("CreateUserWithAccount: %v", err)
	}
	t.Cleanup(func() {
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE id=$1", account.ID)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", user.ID)
	})

	slug := "it-" + uuid.NewString()[:8]
	page, err := st.CreateStatusPage(ctx, account.ID, user.ID, "Demo", "", slug, "UTC", "ru", "public")
	if err != nil {
		t.Fatalf("CreateStatusPage: %v", err)
	}

	// Подтверждённый подписчик (попадёт в рассылку) и неподтверждённый (не попадёт).
	comp := uuid.New()
	confirmed, err := st.CreateSubscriber(ctx, domain.Subscriber{
		StatusPageID: page.ID, Channel: domain.ChannelEmail, Address: "sub@example.test",
		Confirmed: true, Scope: domain.ScopeComponents, ComponentIDs: []uuid.UUID{comp},
	})
	if err != nil {
		t.Fatalf("CreateSubscriber confirmed: %v", err)
	}
	if _, err := st.CreateSubscriber(ctx, domain.Subscriber{
		StatusPageID: page.ID, Channel: domain.ChannelTelegram, Address: "12345",
		Confirmed: false, Scope: domain.ScopePage,
	}); err != nil {
		t.Fatalf("CreateSubscriber pending: %v", err)
	}

	// ListConfirmedSubscribers возвращает только подтверждённого, с сохранённым component_ids.
	subs, err := st.ListConfirmedSubscribers(ctx, page.ID)
	if err != nil {
		t.Fatalf("ListConfirmedSubscribers: %v", err)
	}
	if len(subs) != 1 || subs[0].ID != confirmed.ID {
		t.Fatalf("ожидался 1 подтверждённый подписчик, got %d %+v", len(subs), subs)
	}
	if len(subs[0].ComponentIDs) != 1 || subs[0].ComponentIDs[0] != comp {
		t.Errorf("component_ids не сохранились: %+v", subs[0].ComponentIDs)
	}

	// Дубль (page, channel, address) — unique violation (идемпотентность повторной подписки).
	if _, err := st.CreateSubscriber(ctx, domain.Subscriber{
		StatusPageID: page.ID, Channel: domain.ChannelEmail, Address: "sub@example.test",
		Confirmed: true, Scope: domain.ScopePage,
	}); err == nil {
		t.Error("ожидалась ошибка уникальности на дубль подписки")
	}

	// Журнал уведомлений: создание → pending/attempts=0; инкремент попыток; sent; failed.
	payload := []byte(`{"incident_id":"x","title":"t"}`)
	n, err := st.CreateNotification(ctx, confirmed.ID, domain.EventIncidentNew, payload)
	if err != nil {
		t.Fatalf("CreateNotification: %v", err)
	}
	if n.Status != domain.NotificationPending || n.Attempts != 0 {
		t.Errorf("новая запись: status=%s attempts=%d, want pending/0", n.Status, n.Attempts)
	}

	// jsonb нормализует/переупорядочивает ключи, поэтому сверяем по содержимому, не побайтно.
	got, err := st.NotificationByID(ctx, n.ID)
	if err != nil {
		t.Fatalf("NotificationByID: %v", err)
	}
	var gotPayload map[string]string
	if err := json.Unmarshal(got.Payload, &gotPayload); err != nil || gotPayload["incident_id"] != "x" || gotPayload["title"] != "t" {
		t.Fatalf("payload round-trip: %v / %+v", err, gotPayload)
	}

	a1, err := st.IncrementNotificationAttempts(ctx, n.ID)
	if err != nil || a1 != 1 {
		t.Fatalf("IncrementNotificationAttempts: got %d err %v", a1, err)
	}
	a2, _ := st.IncrementNotificationAttempts(ctx, n.ID)
	if a2 != 2 {
		t.Errorf("второй инкремент = %d, want 2", a2)
	}

	if err := st.MarkNotificationSent(ctx, n.ID); err != nil {
		t.Fatalf("MarkNotificationSent: %v", err)
	}
	sent, _ := st.NotificationByID(ctx, n.ID)
	if sent.Status != domain.NotificationSent || sent.SentAt == nil {
		t.Errorf("после MarkSent: status=%s sent_at=%v", sent.Status, sent.SentAt)
	}

	if err := st.MarkNotificationFailed(ctx, n.ID); err != nil {
		t.Fatalf("MarkNotificationFailed: %v", err)
	}
	failed, _ := st.NotificationByID(ctx, n.ID)
	if failed.Status != domain.NotificationFailed {
		t.Errorf("после MarkFailed: status=%s, want failed", failed.Status)
	}

	// Каскад: удаление страницы убирает подписчиков и их уведомления.
	if err := st.SoftDeleteStatusPage(ctx, page.ID); err != nil {
		// SoftDelete не каскадит подписчиков (FK на жёсткое удаление строки страницы);
		// для проверки каскада удалим страницу физически через raw.
		t.Logf("SoftDeleteStatusPage: %v (ожидаемо — каскад проверяем через hard delete)", err)
	}
	if _, err := raw.Exec(ctx, "DELETE FROM status_pages WHERE id=$1", page.ID); err != nil {
		t.Fatalf("hard delete page: %v", err)
	}
	var cnt int
	if err := raw.QueryRow(ctx, "SELECT count(*) FROM subscribers WHERE status_page_id=$1", page.ID).Scan(&cnt); err != nil {
		t.Fatalf("count subscribers: %v", err)
	}
	if cnt != 0 {
		t.Errorf("после удаления страницы подписчики должны исчезнуть, got %d", cnt)
	}
}
