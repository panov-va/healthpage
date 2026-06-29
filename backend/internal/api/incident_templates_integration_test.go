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

// Интеграционный тест API шаблонов инцидентов (этап 2.7) против реального PostgreSQL. Запуск:
//
//	HEALTHPAGE_TEST_DB="postgres://healthpage:healthpage@localhost:5432/healthpage?sslmode=disable" \
//	  go test ./internal/api/ -run TestIncidentTemplatesIntegration
func TestIncidentTemplatesIntegration(t *testing.T) {
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

	token := register("tmpl-" + uuid.NewString() + "@example.test")

	// страница + компонент
	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", token, map[string]string{"name": "Demo", "slug": "tmpl-" + uuid.NewString()[:8]}, http.StatusCreated, &page)
	var comp componentResponse
	doJSON(t, srv.URL+"/api/v1/components", token, map[string]any{"status_page_id": page.ID, "name": "API"}, http.StatusCreated, &comp)

	// создать шаблон с преднастроенным компонентом
	var tmpl incidentTemplateResponse
	doJSON(t, srv.URL+"/api/v1/incident-templates", token, map[string]any{
		"status_page_id": page.ID,
		"name":           "DB degradation",
		"title_tmpl":     "БД деградирует",
		"body_tmpl":      "Расследуем замедление БД",
		"default_impact": "major",
		"default_components": []map[string]string{
			{"component_id": comp.ID, "component_status_in_incident": "partial_outage"},
		},
	}, http.StatusCreated, &tmpl)
	if tmpl.Name != "DB degradation" || tmpl.DefaultImpact != "major" {
		t.Fatalf("template: %+v", tmpl)
	}
	if len(tmpl.DefaultComponents) != 1 || tmpl.DefaultComponents[0].ComponentStatusInIncident != "partial_outage" {
		t.Fatalf("template components: %+v", tmpl.DefaultComponents)
	}

	// создание шаблона НЕ влияет на статус компонента (это лишь заготовка)
	var comps []componentResponse
	doJSON(t, srv.URL+"/api/v1/components?status_page_id="+page.ID, token, nil, http.StatusOK, &comps)
	if comps[0].CurrentStatus != "operational" {
		t.Fatalf("template must not change component status, got %q", comps[0].CurrentStatus)
	}

	// список шаблонов страницы
	var list []incidentTemplateResponse
	doJSON(t, srv.URL+"/api/v1/incident-templates?status_page_id="+page.ID, token, nil, http.StatusOK, &list)
	if len(list) != 1 || list[0].ID != tmpl.ID {
		t.Fatalf("list templates: %+v", list)
	}

	// получить шаблон по id
	var got incidentTemplateResponse
	doJSON(t, srv.URL+"/api/v1/incident-templates/"+tmpl.ID, token, nil, http.StatusOK, &got)
	if got.ID != tmpl.ID || got.BodyTmpl != "Расследуем замедление БД" {
		t.Fatalf("get template: %+v", got)
	}

	// patch: сменить impact и заменить компоненты на пустой набор
	var patched incidentTemplateResponse
	patchJSON(t, srv.URL+"/api/v1/incident-templates/"+tmpl.ID, token, map[string]any{
		"default_impact": "minor", "default_components": []map[string]string{},
	}, http.StatusOK, &patched)
	if patched.DefaultImpact != "minor" || len(patched.DefaultComponents) != 0 {
		t.Fatalf("patched template: %+v", patched)
	}

	// валидация: недопустимый default_impact → 422
	doStatus(t, http.MethodPost, srv.URL+"/api/v1/incident-templates", token,
		jsonBody(map[string]any{"status_page_id": page.ID, "name": "x", "default_impact": "bogus"}),
		http.StatusUnprocessableEntity)

	// валидация: пустое имя → 422
	doStatus(t, http.MethodPost, srv.URL+"/api/v1/incident-templates", token,
		jsonBody(map[string]any{"status_page_id": page.ID, "name": ""}),
		http.StatusUnprocessableEntity)

	// изоляция: другой оператор не видит шаблон (404 на get/patch/delete) и не создаёт под чужой страницей
	other := register("tmpl-other-" + uuid.NewString() + "@example.test")
	doStatus(t, http.MethodGet, srv.URL+"/api/v1/incident-templates/"+tmpl.ID, other, nil, http.StatusNotFound)
	doStatus(t, http.MethodDelete, srv.URL+"/api/v1/incident-templates/"+tmpl.ID, other, nil, http.StatusNotFound)
	doStatus(t, http.MethodPost, srv.URL+"/api/v1/incident-templates", other,
		jsonBody(map[string]any{"status_page_id": page.ID, "name": "x"}), http.StatusNotFound)

	// удалить шаблон → 204; повторный delete → 404
	doStatus(t, http.MethodDelete, srv.URL+"/api/v1/incident-templates/"+tmpl.ID, token, nil, http.StatusNoContent)
	doStatus(t, http.MethodDelete, srv.URL+"/api/v1/incident-templates/"+tmpl.ID, token, nil, http.StatusNotFound)

	// без авторизации → 401
	doStatus(t, http.MethodPost, srv.URL+"/api/v1/incident-templates", "",
		jsonBody(map[string]any{"status_page_id": page.ID, "name": "x"}), http.StatusUnauthorized)
}
