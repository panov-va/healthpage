package api

import (
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

// decodeBody читает тело ответа в out (out может быть nil).
func decodeBody(t *testing.T, resp *http.Response, out any) {
	t.Helper()
	defer resp.Body.Close()
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

// wantStatus проверяет код ответа и (опционально) декодирует тело.
func wantStatus(t *testing.T, resp *http.Response, code int, out any) {
	t.Helper()
	if resp.StatusCode != code {
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("want %d, got %d (%s)", code, resp.StatusCode, b)
	}
	decodeBody(t, resp, out)
}

// TestWriteAPIByTokenIntegration проверяет этап 5.2: полный write-API под page-токеном БЕЗ явного
// status_page_id (берётся из токена) против реального PG16 — жизненный цикл инцидента
// (open→update→resolve→delete), CRUD компонентов/работ/подписчиков/шаблонов, и что чужой
// status_page_id в теле под токеном → 404, а оператор без status_page_id → 422.
// Запуск: HEALTHPAGE_TEST_DB=... go test ./internal/api/ -run TestWriteAPIByTokenIntegration
func TestWriteAPIByTokenIntegration(t *testing.T) {
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

	// Оператор + страница + write-токен.
	var a authResultResponse
	doJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]string{
		"email": "w-" + uuid.NewString() + "@example.test", "password": "supersecret",
	}, http.StatusCreated, &a)
	auid, _ := uuid.Parse(a.User.ID)
	t.Cleanup(func() {
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", auid)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", auid)
	})
	var pageA, pageB statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", a.AccessToken, map[string]string{"name": "A", "slug": "wa-" + uuid.NewString()[:8]}, http.StatusCreated, &pageA)
	doJSON(t, srv.URL+"/api/v1/pages", a.AccessToken, map[string]string{"name": "B", "slug": "wb-" + uuid.NewString()[:8]}, http.StatusCreated, &pageB)

	var tok tokenCreatedResponse
	doJSON(t, srv.URL+"/api/v1/tokens", a.AccessToken, map[string]any{
		"name": "ci", "status_page_id": pageA.ID, "scopes": []string{"write"},
	}, http.StatusCreated, &tok)
	api := tok.Token

	type idResp struct {
		ID string `json:"id"`
	}

	// ── Компонент: создание БЕЗ status_page_id (из токена), список ──
	var comp idResp
	wantStatus(t, rawTokenReq(t, http.MethodPost, srv.URL+"/api/v1/components", api, map[string]any{
		"name": "API",
	}), http.StatusCreated, &comp)
	if comp.ID == "" {
		t.Fatal("компонент не создан")
	}
	var comps []idResp
	wantStatus(t, rawTokenReq(t, http.MethodGet, srv.URL+"/api/v1/components", api, nil), http.StatusOK, &comps)
	if len(comps) != 1 {
		t.Fatalf("ожидался 1 компонент в списке (страница из токена), got %d", len(comps))
	}

	// ── Инцидент: open → update → resolve → delete, всё БЕЗ status_page_id ──
	var inc idResp
	wantStatus(t, rawTokenReq(t, http.MethodPost, srv.URL+"/api/v1/incidents", api, map[string]any{
		"title": "Сбой API", "status": "investigating", "impact": "major", "body": "Изучаем",
		"components": []map[string]string{{"component_id": comp.ID, "component_status_in_incident": "major_outage"}},
	}), http.StatusCreated, &inc)
	if inc.ID == "" {
		t.Fatal("инцидент не создан")
	}
	// Появился в админском списке (страница из токена).
	var incList incidentListResponse
	wantStatus(t, rawTokenReq(t, http.MethodGet, srv.URL+"/api/v1/incidents", api, nil), http.StatusOK, &incList)
	if len(incList.Items) != 1 {
		t.Fatalf("ожидался 1 инцидент, got %d", len(incList.Items))
	}
	// Обновление статуса (лента).
	wantStatus(t, rawTokenReq(t, http.MethodPost, srv.URL+"/api/v1/incidents/"+inc.ID+"/updates", api, map[string]any{
		"status": "identified", "body": "Нашли причину",
	}), http.StatusCreated, nil)
	// PATCH impact.
	wantStatus(t, rawTokenReq(t, http.MethodPatch, srv.URL+"/api/v1/incidents/"+inc.ID, api, map[string]any{
		"impact": "minor",
	}), http.StatusOK, nil)
	// Закрытие.
	wantStatus(t, rawTokenReq(t, http.MethodPost, srv.URL+"/api/v1/incidents/"+inc.ID+"/updates", api, map[string]any{
		"status": "resolved", "body": "Восстановлено",
	}), http.StatusCreated, nil)
	// Удаление.
	wantStatus(t, rawTokenReq(t, http.MethodDelete, srv.URL+"/api/v1/incidents/"+inc.ID, api, nil), http.StatusNoContent, nil)

	// ── Работы: create (без status_page_id) → in_progress → delete ──
	start := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	end := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)
	var mnt idResp
	wantStatus(t, rawTokenReq(t, http.MethodPost, srv.URL+"/api/v1/maintenances", api, map[string]any{
		"title": "Апгрейд БД", "scheduled_start": start, "scheduled_end": end,
		"component_ids": []string{comp.ID},
	}), http.StatusCreated, &mnt)
	wantStatus(t, rawTokenReq(t, http.MethodPatch, srv.URL+"/api/v1/maintenances/"+mnt.ID, api, map[string]any{
		"status": "in_progress",
	}), http.StatusOK, nil)
	wantStatus(t, rawTokenReq(t, http.MethodDelete, srv.URL+"/api/v1/maintenances/"+mnt.ID, api, nil), http.StatusNoContent, nil)

	// ── Подписчики: create (без status_page_id) → list → delete ──
	var sub idResp
	wantStatus(t, rawTokenReq(t, http.MethodPost, srv.URL+"/api/v1/subscribers", api, map[string]any{
		"channel": "email", "address": "ops@example.test",
	}), http.StatusCreated, &sub)
	var subs []idResp
	wantStatus(t, rawTokenReq(t, http.MethodGet, srv.URL+"/api/v1/subscribers", api, nil), http.StatusOK, &subs)
	if len(subs) != 1 {
		t.Fatalf("ожидался 1 подписчик, got %d", len(subs))
	}
	wantStatus(t, rawTokenReq(t, http.MethodDelete, srv.URL+"/api/v1/subscribers/"+sub.ID, api, nil), http.StatusNoContent, nil)

	// ── Шаблоны инцидентов: create (без status_page_id) → list ──
	var tmpl idResp
	wantStatus(t, rawTokenReq(t, http.MethodPost, srv.URL+"/api/v1/incident-templates", api, map[string]any{
		"name": "Сбой по умолчанию", "default_impact": "major",
	}), http.StatusCreated, &tmpl)
	var tmpls []idResp
	wantStatus(t, rawTokenReq(t, http.MethodGet, srv.URL+"/api/v1/incident-templates", api, nil), http.StatusOK, &tmpls)
	if len(tmpls) != 1 {
		t.Fatalf("ожидался 1 шаблон, got %d", len(tmpls))
	}

	// ── Чужой status_page_id под токеном → 404 (в теле и в query) ──
	wantStatus(t, rawTokenReq(t, http.MethodPost, srv.URL+"/api/v1/components", api, map[string]any{
		"name": "X", "status_page_id": pageB.ID,
	}), http.StatusNotFound, nil)
	wantStatus(t, rawTokenReq(t, http.MethodGet, srv.URL+"/api/v1/incidents?status_page_id="+pageB.ID, api, nil), http.StatusNotFound, nil)
	// Свой status_page_id под токеном — ок.
	wantStatus(t, rawTokenReq(t, http.MethodGet, srv.URL+"/api/v1/components?status_page_id="+pageA.ID, api, nil), http.StatusOK, nil)

	// ── Оператор без status_page_id → 422 (для него обязателен) ──
	doStatus(t, http.MethodPost, srv.URL+"/api/v1/components", a.AccessToken, jsonBody(map[string]any{"name": "noPage"}), http.StatusUnprocessableEntity)
	doStatus(t, http.MethodGet, srv.URL+"/api/v1/incidents", a.AccessToken, nil, http.StatusUnprocessableEntity)
}
