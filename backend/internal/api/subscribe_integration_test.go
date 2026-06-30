package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/healthpage/backend/internal/auth"
	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/notify"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
	"github.com/healthpage/backend/internal/subscription"
)

// capturePublisher перехватывает опубликованные движком сообщения (без RabbitMQ).
type capturePublisher struct {
	mu   sync.Mutex
	msgs []notify.Message
}

func (p *capturePublisher) PublishNotification(_ context.Context, _, _ string, body []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var m notify.Message
	_ = json.Unmarshal(body, &m)
	p.msgs = append(p.msgs, m)
	return nil
}

func (p *capturePublisher) PublishNotificationDelayed(_ context.Context, _, _ string, _ []byte, _ time.Duration) error {
	return nil
}

func (p *capturePublisher) PublishWebhookOut(_ context.Context, body []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var m notify.Message
	_ = json.Unmarshal(body, &m)
	p.msgs = append(p.msgs, m)
	return nil
}

func (p *capturePublisher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.msgs)
}

// Интеграционный тест флоу подписки (double opt-in) против реального PostgreSQL. Запуск:
//
//	HEALTHPAGE_TEST_DB=... go test ./internal/api/ -run TestSubscribeIntegration
func TestSubscribeIntegration(t *testing.T) {
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

	const secret = "test-sub-secret"
	pub := &capturePublisher{}
	engine := notify.New(st, pub, nil)
	tm, _ := security.NewTokenManager("test-secret", 15*time.Minute, time.Hour)
	srv := httptest.NewServer(NewRouter(Deps{
		Auth: auth.NewService(st, tm), Store: st, Notifier: engine, SubSecret: secret, RefreshTTL: time.Hour,
	}))
	defer srv.Close()

	// Оператор + публичная страница.
	var authOut authResultResponse
	doJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]string{
		"email": "sub-" + uuid.NewString() + "@example.test", "password": "supersecret",
	}, http.StatusCreated, &authOut)
	uid, _ := uuid.Parse(authOut.User.ID)
	t.Cleanup(func() {
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", uid)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", uid)
	})
	slug := "it-" + uuid.NewString()[:8]
	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", authOut.AccessToken, map[string]string{"name": "Demo", "slug": slug}, http.StatusCreated, &page)
	pageID, _ := uuid.Parse(page.ID)

	subURL := srv.URL + "/api/v1/pages/" + slug + "/subscribe"
	addr := "client@example.test"

	// 1) Подписка email → 202, опубликовано письмо подтверждения.
	doJSON(t, subURL, "", map[string]string{"channel": "email", "address": addr}, http.StatusAccepted, nil)
	if pub.count() != 1 {
		t.Fatalf("ожидалось 1 письмо подтверждения, got %d", pub.count())
	}
	confirmMsg := pub.msgs[0]
	if confirmMsg.Event != string(domain.EventSubscriberConfirm) {
		t.Fatalf("event = %s, want subscriber_confirm", confirmMsg.Event)
	}
	var cp notify.ConfirmPayload
	if err := json.Unmarshal(confirmMsg.Payload, &cp); err != nil || cp.ConfirmToken == "" {
		t.Fatalf("confirm payload: %v / %+v", err, cp)
	}

	// Подписчик создан неподтверждённым.
	sub, err := st.SubscriberByPageChannelAddress(ctx, pageID, domain.ChannelEmail, addr)
	if err != nil || sub.Confirmed {
		t.Fatalf("подписчик: err=%v confirmed=%v (ожидался pending)", err, sub.Confirmed)
	}

	// 2) Повторная подписка (не подтверждена) → 202, перевыпуск токена + новое письмо.
	doJSON(t, subURL, "", map[string]string{"channel": "email", "address": addr}, http.StatusAccepted, nil)
	if pub.count() != 2 {
		t.Fatalf("ожидалось 2 письма после повторной подписки, got %d", pub.count())
	}
	newToken := mustConfirmToken(t, pub.msgs[1])

	// Старый токен больше не валиден (перевыпущен).
	doJSON(t, srv.URL+"/api/v1/subscribe/confirm?token="+cp.ConfirmToken, "", nil, http.StatusBadRequest, nil)

	// 3) Подтверждение свежим токеном → 200, confirmed=true.
	doJSON(t, srv.URL+"/api/v1/subscribe/confirm?token="+newToken, "", nil, http.StatusOK, nil)
	sub, _ = st.SubscriberByPageChannelAddress(ctx, pageID, domain.ChannelEmail, addr)
	if !sub.Confirmed {
		t.Fatal("подписчик должен стать confirmed после confirm")
	}

	// 4) Повторное подтверждение — токен погашен → 400.
	doJSON(t, srv.URL+"/api/v1/subscribe/confirm?token="+newToken, "", nil, http.StatusBadRequest, nil)

	// 5) Подписка уже подтверждённым адресом → 202 без нового письма.
	doJSON(t, subURL, "", map[string]string{"channel": "email", "address": addr}, http.StatusAccepted, nil)
	if pub.count() != 2 {
		t.Errorf("подтверждённому не должно слаться письмо, писем %d (want 2)", pub.count())
	}

	// 6) Отписка по HMAC-токену → 200, подписчик удалён.
	unsub := subscription.UnsubscribeToken(secret, sub.ID)
	doJSON(t, srv.URL+"/api/v1/unsubscribe?token="+unsub, "", nil, http.StatusOK, nil)
	if _, err := st.SubscriberByPageChannelAddress(ctx, pageID, domain.ChannelEmail, addr); err == nil {
		t.Error("после отписки подписчика быть не должно")
	}

	// 7) Негативы: битый токен отписки, чужой slug, не-email канал.
	doJSON(t, srv.URL+"/api/v1/unsubscribe?token=garbage", "", nil, http.StatusBadRequest, nil)
	doJSON(t, srv.URL+"/api/v1/pages/nope-"+uuid.NewString()[:6]+"/subscribe", "",
		map[string]string{"channel": "email", "address": addr}, http.StatusNotFound, nil)
	doJSON(t, subURL, "", map[string]string{"channel": "telegram", "address": "123"}, http.StatusUnprocessableEntity, nil)
}

func mustConfirmToken(t *testing.T, m notify.Message) string {
	t.Helper()
	var cp notify.ConfirmPayload
	if err := json.Unmarshal(m.Payload, &cp); err != nil || cp.ConfirmToken == "" {
		t.Fatalf("confirm payload: %v / %+v", err, cp)
	}
	return cp.ConfirmToken
}
