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

// Интеграционный тест управляющего API против реального PostgreSQL. Запуск:
//
//	HEALTHPAGE_TEST_DB="postgres://healthpage:healthpage@localhost:5432/healthpage?sslmode=disable" \
//	  go test ./internal/api/ -run TestManagementIntegration
func TestManagementIntegration(t *testing.T) {
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
	srv := httptest.NewServer(NewRouter(Deps{Auth: auth.NewService(st, tm), Store: st, RefreshTTL: time.Hour}))
	defer srv.Close()

	cleanupUsers := []uuid.UUID{}
	t.Cleanup(func() {
		for _, uid := range cleanupUsers {
			_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", uid)
			_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", uid)
		}
	})

	// helper: register operator, return token + user id.
	register := func(email string) (string, uuid.UUID) {
		var out authResultResponse
		doJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]string{"email": email, "password": "supersecret"}, http.StatusCreated, &out)
		uid, _ := uuid.Parse(out.User.ID)
		cleanupUsers = append(cleanupUsers, uid)
		return out.AccessToken, uid
	}

	token, _ := register("mgr-" + uuid.NewString() + "@example.test")

	// create page
	slug := "it-" + uuid.NewString()[:8]
	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", token, map[string]string{"name": "Demo", "slug": slug}, http.StatusCreated, &page)
	if page.ID == "" || page.Visibility != "public" {
		t.Fatalf("create page: %+v", page)
	}

	// list pages
	var pages []statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", token, nil, http.StatusOK, &pages)
	if len(pages) != 1 {
		t.Fatalf("list pages: want 1, got %d", len(pages))
	}

	// create group
	var group componentGroupResponse
	doJSON(t, srv.URL+"/api/v1/pages/"+page.ID+"/component-groups", token, map[string]any{"name": "Core", "position": 0}, http.StatusCreated, &group)

	// create parent + child component
	var parent componentResponse
	doJSON(t, srv.URL+"/api/v1/components", token, map[string]any{"status_page_id": page.ID, "name": "API", "group_id": group.ID}, http.StatusCreated, &parent)
	if parent.CurrentStatus != "operational" {
		t.Fatalf("new component status = %q", parent.CurrentStatus)
	}
	var child componentResponse
	doJSON(t, srv.URL+"/api/v1/components", token, map[string]any{"status_page_id": page.ID, "name": "DB", "parent_id": parent.ID}, http.StatusCreated, &child)

	// list components
	var comps []componentResponse
	doJSON(t, srv.URL+"/api/v1/components?status_page_id="+page.ID, token, nil, http.StatusOK, &comps)
	if len(comps) != 2 {
		t.Fatalf("list components: want 2, got %d", len(comps))
	}

	// manual status change
	var patched componentResponse
	patchJSON(t, srv.URL+"/api/v1/components/"+parent.ID, token, map[string]any{"current_status": "major_outage"}, http.StatusOK, &patched)
	if patched.CurrentStatus != "major_outage" {
		t.Fatalf("status change: got %q", patched.CurrentStatus)
	}

	// patch page: имя + брендинг/тема/часовой пояс (этап 4.1) + white-label/SMTP (4.4/4.5)
	var renamed statusPageResponse
	patchJSON(t, srv.URL+"/api/v1/pages/"+page.ID, token, map[string]any{
		"name":            "Renamed",
		"timezone":        "Europe/Moscow",
		"logo_url":        "https://cdn.example.test/logo.png",
		"theme":           map[string]any{"primary_color": "#2563eb", "mode": "dark"},
		"hide_powered_by": true,
		"from_email":      "status@acme.test",
		"smtp_config":     map[string]any{"host": "smtp.acme.test", "port": 587, "username": "u", "password": "secret", "tls": false},
	}, http.StatusOK, &renamed)
	if renamed.Name != "Renamed" {
		t.Fatalf("patch page: got %q", renamed.Name)
	}
	if renamed.Timezone != "Europe/Moscow" {
		t.Fatalf("patch page timezone: got %q", renamed.Timezone)
	}
	if !renamed.HidePoweredBy {
		t.Fatal("hide_powered_by должен быть true после patch")
	}
	if renamed.FromEmail == nil || *renamed.FromEmail != "status@acme.test" {
		t.Fatalf("from_email: %v", renamed.FromEmail)
	}
	if !renamed.SMTPConfigured {
		t.Fatal("smtp_configured должен быть true после установки smtp_config")
	}

	// delete child
	doStatus(t, http.MethodDelete, srv.URL+"/api/v1/components/"+child.ID, token, nil, http.StatusNoContent)
	doJSON(t, srv.URL+"/api/v1/components?status_page_id="+page.ID, token, nil, http.StatusOK, &comps)
	if len(comps) != 1 {
		t.Fatalf("after delete: want 1, got %d", len(comps))
	}

	// приватный компонент — не должен попасть в публичную выдачу
	var priv componentResponse
	doJSON(t, srv.URL+"/api/v1/components", token, map[string]any{"status_page_id": page.ID, "name": "secret", "is_private": true}, http.StatusCreated, &priv)

	// публичная сводка (без авторизации): overall = major_outage (parent), приватный скрыт
	var summary pageSummaryResponse
	doJSON(t, srv.URL+"/api/v1/pages/"+page.Slug+"/summary", "", nil, http.StatusOK, &summary)
	if summary.OverallStatus != "major_outage" {
		t.Fatalf("public summary overall = %q, want major_outage", summary.OverallStatus)
	}
	if len(summary.Groups) != 1 || summary.Groups[0].AggregatedStatus != "major_outage" {
		t.Fatalf("public summary group: %+v", summary.Groups)
	}
	// брендинг страницы в сводке (этап 4.1): публично-безопасное подмножество
	if summary.Page.Name != "Renamed" || summary.Page.Slug != slug {
		t.Fatalf("public summary page meta: name=%q slug=%q", summary.Page.Name, summary.Page.Slug)
	}
	if summary.Page.Timezone != "Europe/Moscow" || summary.Page.DefaultLocale != "ru" {
		t.Fatalf("public summary page tz/locale: tz=%q locale=%q", summary.Page.Timezone, summary.Page.DefaultLocale)
	}
	if summary.Page.LogoURL == nil || *summary.Page.LogoURL != "https://cdn.example.test/logo.png" {
		t.Fatalf("public summary page logo: %v", summary.Page.LogoURL)
	}
	if string(summary.Page.Theme) == "" || string(summary.Page.Theme) == "{}" {
		t.Fatalf("public summary page theme empty: %s", summary.Page.Theme)
	}

	// публичный список компонентов: приватный исключён (остался только parent в группе)
	var pub []componentResponse
	doJSON(t, srv.URL+"/api/v1/pages/"+page.Slug+"/components", "", nil, http.StatusOK, &pub)
	for _, c := range pub {
		if c.ID == priv.ID {
			t.Fatal("приватный компонент попал в публичную выдачу")
		}
	}
	if len(pub) != 1 {
		t.Fatalf("public components = %d, want 1 (private excluded)", len(pub))
	}

	// isolation: another operator must not see this page (404)
	otherToken, _ := register("other-" + uuid.NewString() + "@example.test")
	doStatus(t, http.MethodGet, srv.URL+"/api/v1/pages/"+page.ID, otherToken, nil, http.StatusNotFound)

	// unauthenticated -> 401
	doStatus(t, http.MethodGet, srv.URL+"/api/v1/pages", "", nil, http.StatusUnauthorized)
}

// doJSON выполняет запрос (POST если body!=nil, иначе GET), проверяет статус и декодирует ответ.
func doJSON(t *testing.T, url, token string, body any, wantStatus int, out any) {
	t.Helper()
	method := http.MethodGet
	var rdr io.Reader
	if body != nil {
		method = http.MethodPost
		buf, _ := json.Marshal(body)
		rdr = bytes.NewReader(buf)
	}
	resp := doReq(t, method, url, token, rdr)
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: want %d, got %d (%s)", method, url, wantStatus, resp.StatusCode, b)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
}

// patchJSON выполняет PATCH с JSON-телом, проверяет статус и декодирует ответ.
func patchJSON(t *testing.T, url, token string, body any, wantStatus int, out any) {
	t.Helper()
	buf, _ := json.Marshal(body)
	resp := doReq(t, http.MethodPatch, url, token, bytes.NewReader(buf))
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("PATCH %s: want %d, got %d (%s)", url, wantStatus, resp.StatusCode, b)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
}

func doStatus(t *testing.T, method, url, token string, body io.Reader, wantStatus int) {
	t.Helper()
	resp := doReq(t, method, url, token, body)
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: want %d, got %d (%s)", method, url, wantStatus, resp.StatusCode, b)
	}
}

func doReq(t *testing.T, method, url, token string, body io.Reader) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, url, body)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}
