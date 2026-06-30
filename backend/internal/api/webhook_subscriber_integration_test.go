package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/healthpage/backend/internal/auth"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
)

// TestWebhookSubscriberRegistration проверяет этап 5.4: ручное добавление исходящего webhook'а как
// подписчика (channel=webhook, address=URL) против PG16 — принимается http(s)-URL, не-URL и
// pull-канал (rss) → 422.
// Запуск: HEALTHPAGE_TEST_DB=... go test ./internal/api/ -run TestWebhookSubscriberRegistration
func TestWebhookSubscriberRegistration(t *testing.T) {
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

	tm, _ := security.NewTokenManager("test-secret", 15*time.Minute, time.Hour)
	srv := httptest.NewServer(NewRouter(Deps{
		Auth: auth.NewService(st, tm), Store: st, SubSecret: "s", RefreshTTL: time.Hour,
	}))
	defer srv.Close()

	var a authResultResponse
	doJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]string{
		"email": "wsub-" + uuid.NewString() + "@example.test", "password": "supersecret",
	}, http.StatusCreated, &a)
	auid, _ := uuid.Parse(a.User.ID)
	t.Cleanup(func() {
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", auid)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", auid)
	})
	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", a.AccessToken, map[string]string{"name": "A", "slug": "ws-" + uuid.NewString()[:8]}, http.StatusCreated, &page)
	createURL := srv.URL + "/api/v1/subscribers"

	// channel=webhook с http(s)-URL → 201, confirmed=true.
	var created subscriberResponse
	doJSON(t, createURL, a.AccessToken, map[string]any{
		"status_page_id": page.ID, "channel": "webhook", "address": "https://mm.example/hooks/abc",
	}, http.StatusCreated, &created)
	if created.Channel != "webhook" || !created.Confirmed {
		t.Fatalf("webhook-подписчик некорректен: %+v", created)
	}

	// channel=webhook с не-URL → 422.
	doJSON(t, createURL, a.AccessToken, map[string]any{
		"status_page_id": page.ID, "channel": "webhook", "address": "not-a-url",
	}, http.StatusUnprocessableEntity, nil)

	// rss (pull-фид) по-прежнему отклоняется → 422.
	doJSON(t, createURL, a.AccessToken, map[string]any{
		"status_page_id": page.ID, "channel": "rss", "address": "x",
	}, http.StatusUnprocessableEntity, nil)
}
