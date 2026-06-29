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
	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
)

// TestAdminSubscribersIntegration проверяет админ-управление подписчиками против реального PG16:
// список (вкл. pending) с фильтром по странице, ручное добавление (confirmed=true), дубль/невалидный
// канал → 422, удаление → 204/повтор 404, изоляция операторов (404), 401.
// Запуск: HEALTHPAGE_TEST_DB=... go test ./internal/api/ -run TestAdminSubscribersIntegration
func TestAdminSubscribersIntegration(t *testing.T) {
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

	// Оператор A + страница.
	var a authResultResponse
	doJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]string{
		"email": "subadm-" + uuid.NewString() + "@example.test", "password": "supersecret",
	}, http.StatusCreated, &a)
	auid, _ := uuid.Parse(a.User.ID)
	t.Cleanup(func() {
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", auid)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", auid)
	})
	slug := "su-" + uuid.NewString()[:8]
	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", a.AccessToken, map[string]string{"name": "Demo", "slug": slug}, http.StatusCreated, &page)
	pageID, _ := uuid.Parse(page.ID)
	listURL := srv.URL + "/api/v1/subscribers?status_page_id=" + page.ID
	createURL := srv.URL + "/api/v1/subscribers"

	// Пустой список.
	var list []subscriberResponse
	doJSON(t, listURL, a.AccessToken, nil, http.StatusOK, &list)
	if len(list) != 0 {
		t.Fatalf("ожидался пустой список, got %d", len(list))
	}

	// Ручное добавление email → 201, confirmed=true.
	var created subscriberResponse
	doJSON(t, createURL, a.AccessToken, map[string]any{
		"status_page_id": page.ID, "channel": "email", "address": "manual@example.test",
	}, http.StatusCreated, &created)
	if !created.Confirmed || created.Channel != "email" || created.StatusPageID != page.ID {
		t.Fatalf("неверный созданный подписчик: %+v", created)
	}

	// Pending-подписчик напрямую через store (имитация незавершённого double opt-in).
	// confirm_token уникален глобально (partial-unique) — берём случайный, чтобы не ловить
	// остатки прежних прогонов.
	pendingHash := uuid.NewString()
	if _, err := st.CreateSubscriber(ctx, domain.Subscriber{
		StatusPageID: pageID, Channel: domain.ChannelEmail, Address: "pending@example.test",
		Confirmed: false, ConfirmToken: &pendingHash, Scope: domain.ScopePage,
	}); err != nil {
		t.Fatalf("create pending: %v", err)
	}

	// Список содержит обоих (вкл. pending).
	doJSON(t, listURL, a.AccessToken, nil, http.StatusOK, &list)
	if len(list) != 2 {
		t.Fatalf("ожидалось 2 подписчика (вкл. pending), got %d", len(list))
	}

	// Дубль (page,channel,address) → 422.
	doJSON(t, createURL, a.AccessToken, map[string]any{
		"status_page_id": page.ID, "channel": "email", "address": "manual@example.test",
	}, http.StatusUnprocessableEntity, nil)

	// Невалидный канал (rss — pull-фид) → 422.
	doJSON(t, createURL, a.AccessToken, map[string]any{
		"status_page_id": page.ID, "channel": "rss", "address": "x",
	}, http.StatusUnprocessableEntity, nil)

	// Удаление → 204, список уменьшается.
	delURL := srv.URL + "/api/v1/subscribers/" + created.ID
	resp := doReq(t, http.MethodDelete, delURL, a.AccessToken, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	doJSON(t, listURL, a.AccessToken, nil, http.StatusOK, &list)
	if len(list) != 1 {
		t.Fatalf("после удаления ожидался 1 подписчик, got %d", len(list))
	}
	// Повторное удаление → 404.
	resp = doReq(t, http.MethodDelete, delURL, a.AccessToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("повторное удаление: want 404, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// Изоляция: оператор B не видит/не управляет подписчиками A.
	var b authResultResponse
	doJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]string{
		"email": "subadm-b-" + uuid.NewString() + "@example.test", "password": "supersecret",
	}, http.StatusCreated, &b)
	buid, _ := uuid.Parse(b.User.ID)
	t.Cleanup(func() {
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", buid)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", buid)
	})
	doJSON(t, listURL, b.AccessToken, nil, http.StatusNotFound, nil) // чужая страница → 404
	rem := list[0]
	resp = doReq(t, http.MethodDelete, srv.URL+"/api/v1/subscribers/"+rem.ID, b.AccessToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("чужой подписчик: delete want 404, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// 401 без токена.
	doJSON(t, listURL, "", nil, http.StatusUnauthorized, nil)
}
