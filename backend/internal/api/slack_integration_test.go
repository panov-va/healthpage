package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/healthpage/backend/internal/auth"
	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/slack"
	"github.com/healthpage/backend/internal/store"
)

// TestSlackSubscribeIntegration проверяет OAuth-флоу подписки Slack против реального PostgreSQL
// со стаб-сервером Slack OAuth: start редиректит с подписанным state, callback обменивает code и
// создаёт Subscriber{channel=slack}; негативы (нет code / битый state / фича выключена).
// Запуск: HEALTHPAGE_TEST_DB=... go test ./internal/api/ -run TestSlackSubscribeIntegration
func TestSlackSubscribeIntegration(t *testing.T) {
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
	const webhookURL = "https://hooks.slack.com/services/T/B/abc123"

	// Стаб Slack oauth.v2.access.
	oauthStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"incoming_webhook":{"url":"` + webhookURL + `","channel":"#alerts"},"team":{"name":"Acme"}}`))
	}))
	defer oauthStub.Close()

	oa := slack.NewOAuth("cid", "csecret", "https://h/api/v1/subscribe/slack/callback", oauthStub.Client(), slack.WithAccessURL(oauthStub.URL))
	tm, _ := security.NewTokenManager("test-secret", 15*time.Minute, time.Hour)
	srv := httptest.NewServer(NewRouter(Deps{
		Auth: auth.NewService(st, tm), Store: st, SubSecret: secret, SlackOAuth: oa, RefreshTTL: time.Hour,
	}))
	defer srv.Close()

	// Клиент без авто-следования редиректам (start отвечает 302 на slack.com).
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	// Оператор + публичная страница.
	var authOut authResultResponse
	doJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]string{
		"email": "slack-" + uuid.NewString() + "@example.test", "password": "supersecret",
	}, http.StatusCreated, &authOut)
	uid, _ := uuid.Parse(authOut.User.ID)
	t.Cleanup(func() {
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", uid)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", uid)
	})
	slug := "sl-" + uuid.NewString()[:8]
	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", authOut.AccessToken, map[string]string{"name": "Demo", "slug": slug}, http.StatusCreated, &page)
	pageID, _ := uuid.Parse(page.ID)

	// 1) start → 302 на Slack authorize с подписанным state.
	resp, err := noRedirect.Get(srv.URL + "/api/v1/pages/" + slug + "/subscribe/slack/start")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("start status = %d, want 302", resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("location parse: %v", err)
	}
	state := loc.Query().Get("state")
	if state == "" || loc.Host != "slack.com" {
		t.Fatalf("неверный redirect: %s", resp.Header.Get("Location"))
	}

	// 2) callback с валидным code+state → 200, подписчик slack создан (confirmed, scope=page).
	doJSON(t, srv.URL+"/api/v1/subscribe/slack/callback?code=the-code&state="+url.QueryEscape(state), "", nil, http.StatusOK, nil)
	sub, err := st.SubscriberByPageChannelAddress(ctx, pageID, domain.ChannelSlack, webhookURL)
	if err != nil {
		t.Fatalf("slack-подписчик не создан: %v", err)
	}
	if !sub.Confirmed || sub.Scope != domain.ScopePage {
		t.Fatalf("ожидался confirmed page-подписчик: %+v", sub)
	}

	// Повторный callback с тем же URL — идемпотентно, без дубля (200).
	doJSON(t, srv.URL+"/api/v1/subscribe/slack/callback?code=the-code&state="+url.QueryEscape(state), "", nil, http.StatusOK, nil)
	all, _ := st.SubscribersByChannelAddress(ctx, domain.ChannelSlack, webhookURL)
	if len(all) != 1 {
		t.Fatalf("ожидался 1 подписчик, got %d", len(all))
	}

	// 3) Негативы.
	doJSON(t, srv.URL+"/api/v1/subscribe/slack/callback?state="+url.QueryEscape(state), "", nil, http.StatusBadRequest, nil) // нет code
	doJSON(t, srv.URL+"/api/v1/subscribe/slack/callback?code=x&state=garbage", "", nil, http.StatusBadRequest, nil)          // битый state

	// Приватная/несуществующая страница в start → 404.
	bad, err := noRedirect.Get(srv.URL + "/api/v1/pages/nope-" + uuid.NewString()[:6] + "/subscribe/slack/start")
	if err != nil {
		t.Fatalf("start unknown: %v", err)
	}
	_ = bad.Body.Close()
	if bad.StatusCode != http.StatusNotFound {
		t.Errorf("неизвестная страница: status = %d, want 404", bad.StatusCode)
	}
}

// TestSlackDisabledReturns404 проверяет, что без сконфигурированного OAuth эндпоинты Slack → 404.
func TestSlackDisabledReturns404(t *testing.T) {
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
		Auth: auth.NewService(st, tm), Store: st, SubSecret: "s", SlackOAuth: nil, RefreshTTL: time.Hour,
	}))
	defer srv.Close()

	var authOut authResultResponse
	doJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]string{
		"email": "slack-off-" + uuid.NewString() + "@example.test", "password": "supersecret",
	}, http.StatusCreated, &authOut)
	uid, _ := uuid.Parse(authOut.User.ID)
	t.Cleanup(func() {
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", uid)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", uid)
	})
	slug := "sl-off-" + uuid.NewString()[:8]
	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", authOut.AccessToken, map[string]string{"name": "Demo", "slug": slug}, http.StatusCreated, &page)

	doJSON(t, srv.URL+"/api/v1/pages/"+slug+"/subscribe/slack/start", "", nil, http.StatusNotFound, nil)
	doJSON(t, srv.URL+"/api/v1/subscribe/slack/callback?code=x&state=y", "", nil, http.StatusNotFound, nil)
}
