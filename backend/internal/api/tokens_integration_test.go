package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

// rawTokenReq выполняет запрос с заголовком Authorization БЕЗ префикса Bearer (page-токен).
func rawTokenReq(t *testing.T, method, url, apiToken string, body any) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		rdr = bytes.NewReader(buf)
	}
	req, _ := http.NewRequest(method, url, rdr)
	req.Header.Set("Content-Type", "application/json")
	if apiToken != "" {
		req.Header.Set("Authorization", apiToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

// TestAPITokensIntegration проверяет этап 5.1 против реального PG16: создание/список/отзыв токенов
// (оператором), аутентификацию управляющих запросов page-токеном (Authorization без Bearer),
// энфорсинг scope (read/write по методу), привязку к одной странице, запрет управления токенами
// самим токеном, изоляцию операторов, 401 на невалидном/отозванном токене.
// Запуск: HEALTHPAGE_TEST_DB=... go test ./internal/api/ -run TestAPITokensIntegration
func TestAPITokensIntegration(t *testing.T) {
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

	// Оператор A + две страницы.
	var a authResultResponse
	doJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]string{
		"email": "tok-a-" + uuid.NewString() + "@example.test", "password": "supersecret",
	}, http.StatusCreated, &a)
	auid, _ := uuid.Parse(a.User.ID)
	t.Cleanup(func() {
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", auid)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", auid)
	})
	var pageA, pageB statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", a.AccessToken, map[string]string{"name": "A", "slug": "tka-" + uuid.NewString()[:8]}, http.StatusCreated, &pageA)
	doJSON(t, srv.URL+"/api/v1/pages", a.AccessToken, map[string]string{"name": "B", "slug": "tkb-" + uuid.NewString()[:8]}, http.StatusCreated, &pageB)

	tokensURL := srv.URL + "/api/v1/tokens"

	// Пустой список токенов страницы A.
	var list []tokenResponse
	doJSON(t, tokensURL+"?status_page_id="+pageA.ID, a.AccessToken, nil, http.StatusOK, &list)
	if len(list) != 0 {
		t.Fatalf("ожидался пустой список токенов, got %d", len(list))
	}

	// Создать write-токен для страницы A.
	var writeTok tokenCreatedResponse
	doJSON(t, tokensURL, a.AccessToken, map[string]any{
		"name": "ci", "status_page_id": pageA.ID, "scopes": []string{"write"},
	}, http.StatusCreated, &writeTok)
	if writeTok.Token == "" || writeTok.StatusPageID != pageA.ID {
		t.Fatalf("неверный созданный токен: %+v", writeTok)
	}
	if len(writeTok.Scopes) != 1 || writeTok.Scopes[0] != "write" {
		t.Fatalf("ожидался scope write, got %v", writeTok.Scopes)
	}

	// Создать read-токен для страницы A (scopes по умолчанию пусто → ниже отдельный тест).
	var readTok tokenCreatedResponse
	doJSON(t, tokensURL, a.AccessToken, map[string]any{
		"name": "dashboard", "status_page_id": pageA.ID, "scopes": []string{"read"},
	}, http.StatusCreated, &readTok)

	// Список теперь содержит 2 токена (без значений).
	doJSON(t, tokensURL+"?status_page_id="+pageA.ID, a.AccessToken, nil, http.StatusOK, &list)
	if len(list) != 2 {
		t.Fatalf("ожидалось 2 токена, got %d", len(list))
	}
	for _, tr := range list {
		if tr.LastUsedAt != nil {
			t.Errorf("last_used_at должен быть null до использования: %+v", tr)
		}
	}

	// write-токен создаёт компонент на странице A (Authorization без Bearer).
	compURL := srv.URL + "/api/v1/components"
	resp := rawTokenReq(t, http.MethodPost, compURL, writeTok.Token, map[string]any{
		"status_page_id": pageA.ID, "name": "API",
	})
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("write-токен POST /components: want 201, got %d (%s)", resp.StatusCode, b)
	}
	_ = resp.Body.Close()

	// last_used_at теперь проставлен у write-токена.
	doJSON(t, tokensURL+"?status_page_id="+pageA.ID, a.AccessToken, nil, http.StatusOK, &list)
	touched := false
	for _, tr := range list {
		if tr.ID == writeTok.ID && tr.LastUsedAt != nil {
			touched = true
		}
	}
	if !touched {
		t.Error("ожидался проставленный last_used_at у write-токена после использования")
	}

	// write-токен может читать (write подразумевает read).
	resp = rawTokenReq(t, http.MethodGet, compURL+"?status_page_id="+pageA.ID, writeTok.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("write-токен GET /components: want 200, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// read-токен НЕ может писать → 403.
	resp = rawTokenReq(t, http.MethodPost, compURL, readTok.Token, map[string]any{
		"status_page_id": pageA.ID, "name": "nope",
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("read-токен POST /components: want 403, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// read-токен может читать → 200.
	resp = rawTokenReq(t, http.MethodGet, compURL+"?status_page_id="+pageA.ID, readTok.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("read-токен GET /components: want 200, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// Токен привязан к странице A: доступ к странице B → 404.
	resp = rawTokenReq(t, http.MethodGet, compURL+"?status_page_id="+pageB.ID, writeTok.Token, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("write-токен на чужую страницу: want 404, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// Токен НЕ может управлять токенами (операция только для оператора) → 403.
	resp = rawTokenReq(t, http.MethodPost, tokensURL, writeTok.Token, map[string]any{
		"name": "escalate", "status_page_id": pageA.ID,
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("токен создаёт токен: want 403, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// Невалидный scope при создании → 422.
	doJSON(t, tokensURL, a.AccessToken, map[string]any{
		"name": "bad", "status_page_id": pageA.ID, "scopes": []string{"admin"},
	}, http.StatusUnprocessableEntity, nil)

	// Невалидный токен → 401.
	resp = rawTokenReq(t, http.MethodGet, compURL+"?status_page_id="+pageA.ID, "hp_nonexistent", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("невалидный токен: want 401, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// Изоляция: оператор B не видит/не отзывает токены A.
	var b authResultResponse
	doJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]string{
		"email": "tok-b-" + uuid.NewString() + "@example.test", "password": "supersecret",
	}, http.StatusCreated, &b)
	buid, _ := uuid.Parse(b.User.ID)
	t.Cleanup(func() {
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", buid)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", buid)
	})
	doJSON(t, tokensURL+"?status_page_id="+pageA.ID, b.AccessToken, nil, http.StatusNotFound, nil)
	doStatus(t, http.MethodDelete, tokensURL+"/"+writeTok.ID, b.AccessToken, nil, http.StatusNotFound)

	// Отзыв токена оператором A → 204, затем токен больше не работает (401).
	doStatus(t, http.MethodDelete, tokensURL+"/"+writeTok.ID, a.AccessToken, nil, http.StatusNoContent)
	resp = rawTokenReq(t, http.MethodGet, compURL+"?status_page_id="+pageA.ID, writeTok.Token, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("отозванный токен: want 401, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	// Повторный отзыв → 404.
	doStatus(t, http.MethodDelete, tokensURL+"/"+writeTok.ID, a.AccessToken, nil, http.StatusNotFound)

	// Создание без scopes → полный доступ (read+write).
	var fullTok tokenCreatedResponse
	doJSON(t, tokensURL, a.AccessToken, map[string]any{
		"name": "full", "status_page_id": pageA.ID,
	}, http.StatusCreated, &fullTok)
	if len(fullTok.Scopes) != 2 {
		t.Errorf("пустой scopes → полный доступ (2 scope), got %v", fullTok.Scopes)
	}
	resp = rawTokenReq(t, http.MethodPost, compURL, fullTok.Token, map[string]any{
		"status_page_id": pageA.ID, "name": "full-write",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("full-токен POST /components: want 201, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
}
