package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/healthpage/backend/internal/auth"
	"github.com/healthpage/backend/internal/dokploy"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
)

// Интеграционный тест прод-подключения кастомного домена через Dokploy API (замена
// edge/tls-manager, DEPLOY.md): успешная верификация CNAME один раз создаёт Domain в Dokploy
// (idempotent — повторный verify не создаёт второй раз), а смена/снятие домена его отвязывает.
// Запуск:
//
//	HEALTHPAGE_TEST_DB="postgres://healthpage:healthpage@localhost:5432/healthpage?sslmode=disable" \
//	  go test ./internal/api/ -run TestCustomDomainDokployIntegration
func TestCustomDomainDokployIntegration(t *testing.T) {
	dsn := mustTestDSN(t)
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

	const target = "cname.healthpage.ru"
	resolver := func(_ context.Context, host string) (string, error) {
		if host == "status.dokploytest.example" {
			return "cname.healthpage.ru.", nil
		}
		return "elsewhere.example.net.", nil
	}

	var createCalls, deleteCalls atomic.Int32
	fakeDokploy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/domain.create":
			createCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]string{"domainId": "dom-test-1"})
		case "/domain.delete":
			deleteCalls.Add(1)
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected dokploy call: %s", r.URL.Path)
		}
	}))
	defer fakeDokploy.Close()
	dokployClient := dokploy.NewClient(fakeDokploy.URL, "test-key", "app-public-ssr", nil)

	tm, _ := security.NewTokenManager("test-secret", 15*time.Minute, time.Hour)
	srv := httptest.NewServer(NewRouter(Deps{
		Auth: auth.NewService(st, tm), Store: st, RefreshTTL: time.Hour,
		CNAMETarget: target, CNAMEResolver: resolver, Dokploy: dokployClient,
	}))
	defer srv.Close()

	var cleanup []uuid.UUID
	t.Cleanup(func() {
		for _, uid := range cleanup {
			_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", uid)
			_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", uid)
		}
	})

	var out authResultResponse
	email := "dokploy-" + uuid.NewString() + "@example.test"
	doJSON(t, srv.URL+"/api/v1/auth/register", "",
		map[string]string{"email": email, "password": "supersecret"}, http.StatusCreated, &out)
	uid, _ := uuid.Parse(out.User.ID)
	cleanup = append(cleanup, uid)
	if _, err := raw.Exec(ctx, "UPDATE accounts SET billing_plan='premium' WHERE owner_user_id=$1", uid); err != nil {
		t.Fatalf("upgrade premium: %v", err)
	}
	token := out.AccessToken

	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", token,
		map[string]string{"name": "DokployTest", "slug": "dok-" + uuid.NewString()[:8]}, http.StatusCreated, &page)

	patchJSON(t, srv.URL+"/api/v1/pages/"+page.ID, token,
		map[string]any{"custom_domain": "status.dokploytest.example"}, http.StatusOK, nil)

	// Первая верификация: CNAME совпадает → domain_verified=true и создаётся Domain в Dokploy.
	var verify1 domainStatusResponse
	verifyDomain(t, srv.URL+"/api/v1/pages/"+page.ID+"/domain/verify", token, http.StatusOK, &verify1)
	if !verify1.DomainVerified {
		t.Fatalf("verify1: DomainVerified = false, want true")
	}
	if got := createCalls.Load(); got != 1 {
		t.Fatalf("createCalls после первого verify = %d, want 1", got)
	}
	pageID, _ := uuid.Parse(page.ID)
	assertDokployDomainID(ctx, t, raw, pageID, "dom-test-1")

	// Повторная верификация (уже подключено) — CreateDomain второй раз не вызывается (idempotent).
	var verify2 domainStatusResponse
	verifyDomain(t, srv.URL+"/api/v1/pages/"+page.ID+"/domain/verify", token, http.StatusOK, &verify2)
	if !verify2.DomainVerified {
		t.Fatalf("verify2: DomainVerified = false, want true")
	}
	if got := createCalls.Load(); got != 1 {
		t.Fatalf("createCalls после повторного verify = %d, want 1 (idempotent)", got)
	}

	// Снятие домена → отвязка в Dokploy (DeleteDomain) и dokploy_domain_id сбрасывается.
	patchJSON(t, srv.URL+"/api/v1/pages/"+page.ID, token,
		map[string]any{"custom_domain": nil}, http.StatusOK, nil)
	if got := deleteCalls.Load(); got != 1 {
		t.Fatalf("deleteCalls после снятия домена = %d, want 1", got)
	}
	assertDokployDomainID(ctx, t, raw, pageID, "")
}

func assertDokployDomainID(ctx context.Context, t *testing.T, raw *pgxpool.Pool, pageID uuid.UUID, want string) {
	t.Helper()
	var got *string
	if err := raw.QueryRow(ctx, "SELECT dokploy_domain_id FROM status_pages WHERE id=$1", pageID).Scan(&got); err != nil {
		t.Fatalf("query dokploy_domain_id: %v", err)
	}
	gotStr := ""
	if got != nil {
		gotStr = *got
	}
	if gotStr != want {
		t.Fatalf("dokploy_domain_id = %q, want %q", gotStr, want)
	}
}
