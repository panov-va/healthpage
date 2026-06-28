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

// jsonBody сериализует v в io.Reader для doStatus.
func jsonBody(v any) io.Reader {
	buf, _ := json.Marshal(v)
	return bytes.NewReader(buf)
}

// Интеграционный тест API инцидентов (этап 2.5) против реального PostgreSQL. Запуск:
//
//	HEALTHPAGE_TEST_DB="postgres://healthpage:healthpage@localhost:5432/healthpage?sslmode=disable" \
//	  go test ./internal/api/ -run TestIncidentsIntegration
func TestIncidentsIntegration(t *testing.T) {
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

	token := register("inc-" + uuid.NewString() + "@example.test")

	// страница + компонент
	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", token, map[string]string{"name": "Demo", "slug": "inc-" + uuid.NewString()[:8]}, http.StatusCreated, &page)
	var comp componentResponse
	doJSON(t, srv.URL+"/api/v1/components", token, map[string]any{"status_page_id": page.ID, "name": "API"}, http.StatusCreated, &comp)

	// создать инцидент: компонент уходит в major_outage (авто-деривация)
	var inc incidentResponse
	doJSON(t, srv.URL+"/api/v1/incidents", token, map[string]any{
		"status_page_id": page.ID,
		"title":          "API down",
		"status":         "investigating",
		"impact":         "major",
		"body":           "Расследуем",
		"components":     []map[string]string{{"component_id": comp.ID, "component_status_in_incident": "major_outage"}},
	}, http.StatusCreated, &inc)
	if inc.CurrentStatus != "investigating" || inc.Impact != "major" {
		t.Fatalf("incident: %+v", inc)
	}
	if len(inc.Components) != 1 || len(inc.Updates) != 1 {
		t.Fatalf("incident components/updates: %+v", inc)
	}
	if inc.ResolvedAt != nil {
		t.Fatalf("new incident must not have resolved_at: %v", inc.ResolvedAt)
	}
	if got := componentStatus(token, page.ID, comp.ID); got != "major_outage" {
		t.Fatalf("derived component status = %q, want major_outage", got)
	}

	// публичная сводка отражает инцидент
	var summary pageSummaryResponse
	doJSON(t, srv.URL+"/api/v1/pages/"+page.Slug+"/summary", "", nil, http.StatusOK, &summary)
	if summary.OverallStatus != "major_outage" {
		t.Fatalf("public overall = %q, want major_outage", summary.OverallStatus)
	}

	// добавить обновление identified
	var upd incidentUpdateResponse
	doJSON(t, srv.URL+"/api/v1/incidents/"+inc.ID+"/updates", token, map[string]any{"status": "identified", "body": "Нашли причину"}, http.StatusCreated, &upd)
	if upd.Status != "identified" {
		t.Fatalf("update status = %q", upd.Status)
	}

	// resolve: компонент возвращается в operational (авто-деривация)
	doJSON(t, srv.URL+"/api/v1/incidents/"+inc.ID+"/updates", token, map[string]any{"status": "resolved", "body": "Исправлено"}, http.StatusCreated, &upd)
	if got := componentStatus(token, page.ID, comp.ID); got != "operational" {
		t.Fatalf("after resolve component = %q, want operational", got)
	}

	// постмортем разрешён только после resolved → теперь ок
	var patched incidentResponse
	patchJSON(t, srv.URL+"/api/v1/incidents/"+inc.ID, token, map[string]any{"postmortem": "Root cause: X"}, http.StatusOK, &patched)
	if patched.Postmortem == nil || *patched.Postmortem != "Root cause: X" {
		t.Fatalf("postmortem not saved: %+v", patched.Postmortem)
	}
	if patched.ResolvedAt == nil {
		t.Fatalf("resolved incident must have resolved_at")
	}

	// постмортем на неустранённом инциденте → 422
	var inc2 incidentResponse
	doJSON(t, srv.URL+"/api/v1/incidents", token, map[string]any{
		"status_page_id": page.ID, "title": "Other", "status": "investigating", "impact": "minor", "body": "...",
	}, http.StatusCreated, &inc2)
	doStatus(t, http.MethodPatch, srv.URL+"/api/v1/incidents/"+inc2.ID, token,
		jsonBody(map[string]any{"postmortem": "rc"}), http.StatusUnprocessableEntity)

	// валидация: недопустимый статус → 422
	doStatus(t, http.MethodPost, srv.URL+"/api/v1/incidents", token,
		jsonBody(map[string]any{"status_page_id": page.ID, "title": "x", "status": "bogus", "impact": "minor", "body": "b"}),
		http.StatusUnprocessableEntity)

	// изоляция: другой оператор не видит инцидент (404 на patch/delete и на создание под чужой страницей)
	other := register("inc-other-" + uuid.NewString() + "@example.test")
	doStatus(t, http.MethodDelete, srv.URL+"/api/v1/incidents/"+inc.ID, other, nil, http.StatusNotFound)
	doStatus(t, http.MethodPost, srv.URL+"/api/v1/incidents", other,
		jsonBody(map[string]any{"status_page_id": page.ID, "title": "x", "status": "investigating", "impact": "minor", "body": "b"}),
		http.StatusNotFound)

	// удалить активный инцидент inc2 → компонентов не затрагивал, просто 204; повторный delete → 404
	doStatus(t, http.MethodDelete, srv.URL+"/api/v1/incidents/"+inc2.ID, token, nil, http.StatusNoContent)
	doStatus(t, http.MethodDelete, srv.URL+"/api/v1/incidents/"+inc2.ID, token, nil, http.StatusNotFound)

	// без авторизации → 401
	doStatus(t, http.MethodPost, srv.URL+"/api/v1/incidents", "",
		jsonBody(map[string]any{"status_page_id": page.ID, "title": "x", "status": "investigating", "impact": "minor", "body": "b"}),
		http.StatusUnauthorized)
}
