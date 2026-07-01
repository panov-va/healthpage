package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/healthpage/backend/internal/auth"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
)

// Интеграционный тест приватных страниц (этап 4.2): пароль → токен доступа → гейтинг публичных
// read через заголовок X-Page-Access. Запуск:
//
//	HEALTHPAGE_TEST_DB="postgres://healthpage:healthpage@localhost:5432/healthpage?sslmode=disable" \
//	  go test ./internal/api/ -run TestPrivatePageAccessIntegration
func TestPrivatePageAccessIntegration(t *testing.T) {
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

	tm, _ := security.NewTokenManager("test-secret", 15*time.Minute, time.Hour)
	srv := httptest.NewServer(NewRouter(Deps{
		Auth: auth.NewService(st, tm), Store: st, RefreshTTL: time.Hour, SubSecret: "test-sub-secret",
	}))
	defer srv.Close()

	var cleanup []uuid.UUID
	t.Cleanup(func() {
		for _, uid := range cleanup {
			_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", uid)
			_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", uid)
		}
	})

	var reg authResultResponse
	doJSON(t, srv.URL+"/api/v1/auth/register", "",
		map[string]string{"email": "priv-" + uuid.NewString() + "@example.test", "password": "supersecret"},
		http.StatusCreated, &reg)
	uid, _ := uuid.Parse(reg.User.ID)
	cleanup = append(cleanup, uid)
	token := reg.AccessToken
	// Приватные страницы — premium-фича (этап 6.7); поднимаем тариф аккаунта.
	if _, err := raw.Exec(ctx, "UPDATE accounts SET billing_plan='premium' WHERE owner_user_id=$1", uid); err != nil {
		t.Fatalf("upgrade premium: %v", err)
	}

	slug := "priv-" + uuid.NewString()[:8]
	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", token,
		map[string]string{"name": "Secret Co", "slug": slug}, http.StatusCreated, &page)

	summaryURL := srv.URL + "/api/v1/pages/" + slug + "/summary"
	accessURL := srv.URL + "/api/v1/pages/" + slug + "/access"

	// Пока публичная — сводка доступна без токена, /access → 404 (пароль не нужен).
	doStatus(t, http.MethodGet, summaryURL, "", nil, http.StatusOK)
	postStatus(t, accessURL, "", `{"password":"x"}`, http.StatusNotFound)

	// Делаем приватной + задаём пароль.
	patchJSON(t, srv.URL+"/api/v1/pages/"+page.ID, token,
		map[string]any{"visibility": "private", "password": "letmein"}, http.StatusOK, nil)

	// Без токена приватная сводка → 401 password_required.
	doStatus(t, http.MethodGet, summaryURL, "", nil, http.StatusUnauthorized)

	// Неверный пароль → 401.
	postStatus(t, accessURL, "", `{"password":"wrong"}`, http.StatusUnauthorized)

	// Верный пароль → 200 + токен.
	var grant struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	accessPostJSON(t, accessURL, `{"password":"letmein"}`, http.StatusOK, &grant)
	if grant.AccessToken == "" || grant.ExpiresIn <= 0 {
		t.Fatalf("bad grant: %+v", grant)
	}

	// С валидным токеном сводка отдаётся; visibility=private (для noindex на фронте).
	var summary pageSummaryResponse
	getWithAccess(t, summaryURL, grant.AccessToken, http.StatusOK, &summary)
	if summary.Page.Visibility != "private" {
		t.Fatalf("summary page visibility = %q, want private", summary.Page.Visibility)
	}

	// Чужой/битый токен → 401.
	getWithAccess(t, summaryURL, "garbage.token.here", http.StatusUnauthorized, nil)

	// Снимаем пароль (null) → теперь /access снова 401 (пароль не задан), страница всё ещё приватна.
	patchJSON(t, srv.URL+"/api/v1/pages/"+page.ID, token,
		map[string]any{"password": nil}, http.StatusOK, nil)
	postStatus(t, accessURL, "", `{"password":"letmein"}`, http.StatusUnauthorized)
}

// mustTestDSN возвращает DSN тестовой БД или пропускает тест.
func mustTestDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("HEALTHPAGE_TEST_DB")
	if dsn == "" {
		t.Skip("HEALTHPAGE_TEST_DB not set; skipping integration test")
	}
	return dsn
}

func accessPostReq(t *testing.T, url, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func accessPostJSON(t *testing.T, url, body string, wantStatus int, out any) {
	t.Helper()
	resp := accessPostReq(t, url, body)
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST %s: want %d, got %d (%s)", url, wantStatus, resp.StatusCode, b)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
}

func postStatus(t *testing.T, url, _, body string, wantStatus int) {
	t.Helper()
	resp := accessPostReq(t, url, body)
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST %s: want %d, got %d (%s)", url, wantStatus, resp.StatusCode, b)
	}
}

func getWithAccess(t *testing.T, url, accessToken string, wantStatus int, out any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("X-Page-Access", accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s: want %d, got %d (%s)", url, wantStatus, resp.StatusCode, b)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
}
