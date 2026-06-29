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

// Интеграционный тест API плановых работ (этап 2.6) против реального PostgreSQL. Запуск:
//
//	HEALTHPAGE_TEST_DB="postgres://healthpage:healthpage@localhost:5432/healthpage?sslmode=disable" \
//	  go test ./internal/api/ -run TestMaintenancesIntegration
func TestMaintenancesIntegration(t *testing.T) {
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
	register := func(email string) string {
		var out authResultResponse
		doJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]string{"email": email, "password": "supersecret"}, http.StatusCreated, &out)
		uid, _ := uuid.Parse(out.User.ID)
		cleanupUsers = append(cleanupUsers, uid)
		return out.AccessToken
	}
	componentStatus := func(token, pageID, compID string) string {
		var comps []componentResponse
		doJSON(t, srv.URL+"/api/v1/components?status_page_id="+pageID, token, nil, http.StatusOK, &comps)
		for _, c := range comps {
			if c.ID == compID {
				return c.CurrentStatus
			}
		}
		t.Fatalf("component %s not found", compID)
		return ""
	}

	token := register("mnt-" + uuid.NewString() + "@example.test")

	// страница + компонент
	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", token, map[string]string{"name": "Demo", "slug": "mnt-" + uuid.NewString()[:8]}, http.StatusCreated, &page)
	var comp componentResponse
	doJSON(t, srv.URL+"/api/v1/components", token, map[string]any{"status_page_id": page.ID, "name": "API"}, http.StatusCreated, &comp)

	start := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	end := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)

	// создать работы (scheduled) → компонент остаётся operational (scheduled статус не навязывает)
	var mnt maintenanceResponse
	doJSON(t, srv.URL+"/api/v1/maintenances", token, map[string]any{
		"status_page_id":  page.ID,
		"title":           "DB upgrade",
		"description":     "Плановое обновление",
		"scheduled_start": start,
		"scheduled_end":   end,
		"component_ids":   []string{comp.ID},
	}, http.StatusCreated, &mnt)
	if mnt.Status != "scheduled" {
		t.Fatalf("new maintenance status = %q, want scheduled", mnt.Status)
	}
	if len(mnt.ComponentIDs) != 1 || mnt.StartedAt != nil || mnt.CompletedAt != nil {
		t.Fatalf("maintenance: %+v", mnt)
	}
	if got := componentStatus(token, page.ID, comp.ID); got != "operational" {
		t.Fatalf("scheduled maintenance must not impose status, got %q", got)
	}

	// перевести в in_progress → компонент уходит в under_maintenance (авто-деривация)
	patchJSON(t, srv.URL+"/api/v1/maintenances/"+mnt.ID, token, map[string]any{"status": "in_progress"}, http.StatusOK, &mnt)
	if mnt.Status != "in_progress" || mnt.StartedAt == nil {
		t.Fatalf("in_progress maintenance: %+v", mnt)
	}
	if got := componentStatus(token, page.ID, comp.ID); got != "under_maintenance" {
		t.Fatalf("in_progress component = %q, want under_maintenance", got)
	}

	// публичная сводка отражает under_maintenance
	var summary pageSummaryResponse
	doJSON(t, srv.URL+"/api/v1/pages/"+page.Slug+"/summary", "", nil, http.StatusOK, &summary)
	if summary.OverallStatus != "under_maintenance" {
		t.Fatalf("public overall = %q, want under_maintenance", summary.OverallStatus)
	}

	// добавить обновление-заметку (без статуса) → 201
	var upd maintenanceUpdateResponse
	doJSON(t, srv.URL+"/api/v1/maintenances/"+mnt.ID+"/updates", token, map[string]any{"body": "Идём по плану"}, http.StatusCreated, &upd)
	if upd.Body != "Идём по плану" {
		t.Fatalf("update: %+v", upd)
	}

	// завершить → компонент возвращается в operational
	patchJSON(t, srv.URL+"/api/v1/maintenances/"+mnt.ID, token, map[string]any{"status": "completed"}, http.StatusOK, &mnt)
	if mnt.Status != "completed" || mnt.CompletedAt == nil {
		t.Fatalf("completed maintenance: %+v", mnt)
	}
	if got := componentStatus(token, page.ID, comp.ID); got != "operational" {
		t.Fatalf("after completion component = %q, want operational", got)
	}

	// валидация: недопустимый статус → 422
	doStatus(t, http.MethodPatch, srv.URL+"/api/v1/maintenances/"+mnt.ID, token,
		jsonBody(map[string]any{"status": "bogus"}), http.StatusUnprocessableEntity)

	// валидация: scheduled_end не позже scheduled_start → 422
	doStatus(t, http.MethodPost, srv.URL+"/api/v1/maintenances", token,
		jsonBody(map[string]any{"status_page_id": page.ID, "title": "x", "scheduled_start": end, "scheduled_end": start}),
		http.StatusUnprocessableEntity)

	// in_progress работы для проверки возврата компонента при удалении
	var mnt2 maintenanceResponse
	doJSON(t, srv.URL+"/api/v1/maintenances", token, map[string]any{
		"status_page_id": page.ID, "title": "Cache flush", "scheduled_start": start, "scheduled_end": end,
		"component_ids": []string{comp.ID},
	}, http.StatusCreated, &mnt2)
	patchJSON(t, srv.URL+"/api/v1/maintenances/"+mnt2.ID, token, map[string]any{"status": "in_progress"}, http.StatusOK, &mnt2)
	if got := componentStatus(token, page.ID, comp.ID); got != "under_maintenance" {
		t.Fatalf("mnt2 in_progress component = %q, want under_maintenance", got)
	}

	// изоляция: другой оператор не видит работы (404) и не создаёт под чужой страницей (404)
	other := register("mnt-other-" + uuid.NewString() + "@example.test")
	doStatus(t, http.MethodDelete, srv.URL+"/api/v1/maintenances/"+mnt2.ID, other, nil, http.StatusNotFound)
	doStatus(t, http.MethodPost, srv.URL+"/api/v1/maintenances", other,
		jsonBody(map[string]any{"status_page_id": page.ID, "title": "x", "scheduled_start": start, "scheduled_end": end}),
		http.StatusNotFound)

	// удалить in_progress работы → компонент возвращается в operational; повторный delete → 404
	doStatus(t, http.MethodDelete, srv.URL+"/api/v1/maintenances/"+mnt2.ID, token, nil, http.StatusNoContent)
	if got := componentStatus(token, page.ID, comp.ID); got != "operational" {
		t.Fatalf("after delete component = %q, want operational", got)
	}
	doStatus(t, http.MethodDelete, srv.URL+"/api/v1/maintenances/"+mnt2.ID, token, nil, http.StatusNotFound)

	// без авторизации → 401
	doStatus(t, http.MethodPost, srv.URL+"/api/v1/maintenances", "",
		jsonBody(map[string]any{"status_page_id": page.ID, "title": "x", "scheduled_start": start, "scheduled_end": end}),
		http.StatusUnauthorized)
}
