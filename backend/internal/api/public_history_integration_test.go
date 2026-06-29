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

// Интеграционный тест публичной истории инцидентов/работ и наполнения сводки (этап 2.8) против
// реального PostgreSQL. Запуск:
//
//	HEALTHPAGE_TEST_DB="postgres://healthpage:healthpage@localhost:5432/healthpage?sslmode=disable" \
//	  go test ./internal/api/ -run TestPublicHistoryIntegration
func TestPublicHistoryIntegration(t *testing.T) {
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

	token := register("pub-" + uuid.NewString() + "@example.test")

	// страница + два компонента
	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", token, map[string]string{"name": "Demo", "slug": "pub-" + uuid.NewString()[:8]}, http.StatusCreated, &page)
	var compA, compB componentResponse
	doJSON(t, srv.URL+"/api/v1/components", token, map[string]any{"status_page_id": page.ID, "name": "API"}, http.StatusCreated, &compA)
	doJSON(t, srv.URL+"/api/v1/components", token, map[string]any{"status_page_id": page.ID, "name": "Web"}, http.StatusCreated, &compB)

	// активный инцидент на compA (major), resolved инцидент на compB (minor), скрытый инцидент
	var incActive, incResolved, incHidden incidentResponse
	doJSON(t, srv.URL+"/api/v1/incidents", token, map[string]any{
		"status_page_id": page.ID, "title": "API down", "status": "investigating", "impact": "major",
		"body": "...", "components": []map[string]string{{"component_id": compA.ID, "component_status_in_incident": "major_outage"}},
	}, http.StatusCreated, &incActive)

	doJSON(t, srv.URL+"/api/v1/incidents", token, map[string]any{
		"status_page_id": page.ID, "title": "Web blip", "status": "investigating", "impact": "minor",
		"body": "...", "components": []map[string]string{{"component_id": compB.ID, "component_status_in_incident": "degraded_performance"}},
	}, http.StatusCreated, &incResolved)
	var upd incidentUpdateResponse
	doJSON(t, srv.URL+"/api/v1/incidents/"+incResolved.ID+"/updates", token, map[string]any{"status": "resolved", "body": "ok"}, http.StatusCreated, &upd)

	doJSON(t, srv.URL+"/api/v1/incidents", token, map[string]any{
		"status_page_id": page.ID, "title": "Secret", "status": "investigating", "impact": "critical", "body": "...",
	}, http.StatusCreated, &incHidden)
	var patched incidentResponse
	patchJSON(t, srv.URL+"/api/v1/incidents/"+incHidden.ID, token, map[string]any{"is_visible": false}, http.StatusOK, &patched)

	pub := func(path string) string { return srv.URL + "/api/v1/pages/" + page.Slug + path }

	// история: видимые инциденты (active + resolved), скрытый исключён
	var hist incidentListResponse
	doJSON(t, pub("/incidents"), "", nil, http.StatusOK, &hist)
	if hist.Pagination.Total != 2 || len(hist.Items) != 2 {
		t.Fatalf("history total=%d items=%d, want 2/2", hist.Pagination.Total, len(hist.Items))
	}
	for _, it := range hist.Items {
		if it.ID == incHidden.ID {
			t.Fatalf("hidden incident leaked into history")
		}
	}

	// фильтр по impact=major → только активный
	var byImpact incidentListResponse
	doJSON(t, pub("/incidents?impact=major"), "", nil, http.StatusOK, &byImpact)
	if byImpact.Pagination.Total != 1 || byImpact.Items[0].ID != incActive.ID {
		t.Fatalf("impact filter: %+v", byImpact)
	}

	// фильтр по status=resolved → только устранённый
	var byStatus incidentListResponse
	doJSON(t, pub("/incidents?status=resolved"), "", nil, http.StatusOK, &byStatus)
	if byStatus.Pagination.Total != 1 || byStatus.Items[0].ID != incResolved.ID {
		t.Fatalf("status filter: %+v", byStatus)
	}

	// фильтр по component_id=compB → только Web blip
	var byComp incidentListResponse
	doJSON(t, pub("/incidents?component_id="+compB.ID), "", nil, http.StatusOK, &byComp)
	if byComp.Pagination.Total != 1 || byComp.Items[0].ID != incResolved.ID {
		t.Fatalf("component filter: %+v", byComp)
	}

	// пагинация: per_page=1 → 1 элемент, total=2
	var paged incidentListResponse
	doJSON(t, pub("/incidents?per_page=1&page=1"), "", nil, http.StatusOK, &paged)
	if len(paged.Items) != 1 || paged.Pagination.Total != 2 || paged.Pagination.PerPage != 1 {
		t.Fatalf("pagination: %+v", paged.Pagination)
	}

	// невалидный фильтр → 422
	doStatus(t, http.MethodGet, pub("/incidents?status=bogus"), "", nil, http.StatusUnprocessableEntity)

	// detail: видимый ок, скрытый 404, чужой uuid 404
	var detail incidentResponse
	doJSON(t, pub("/incidents/"+incActive.ID), "", nil, http.StatusOK, &detail)
	if detail.ID != incActive.ID || len(detail.Updates) == 0 {
		t.Fatalf("detail: %+v", detail)
	}
	doStatus(t, http.MethodGet, pub("/incidents/"+incHidden.ID), "", nil, http.StatusNotFound)
	doStatus(t, http.MethodGet, pub("/incidents/"+uuid.NewString()), "", nil, http.StatusNotFound)

	// работы: scheduled + in_progress
	var mScheduled, mActive maintenanceResponse
	start := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	end := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)
	doJSON(t, srv.URL+"/api/v1/maintenances", token, map[string]any{
		"status_page_id": page.ID, "title": "Future", "scheduled_start": start, "scheduled_end": end,
	}, http.StatusCreated, &mScheduled)
	doJSON(t, srv.URL+"/api/v1/maintenances", token, map[string]any{
		"status_page_id": page.ID, "title": "Now", "scheduled_start": start, "scheduled_end": end,
		"component_ids": []string{compB.ID},
	}, http.StatusCreated, &mActive)
	patchJSON(t, srv.URL+"/api/v1/maintenances/"+mActive.ID, token, map[string]any{"status": "in_progress"}, http.StatusOK, &mActive)

	// публичный список работ: обе (не завершённые)
	var mlist maintenanceListResponse
	doJSON(t, pub("/maintenances"), "", nil, http.StatusOK, &mlist)
	if mlist.Pagination.Total != 2 {
		t.Fatalf("maintenances total=%d, want 2", mlist.Pagination.Total)
	}
	// фильтр по status=in_progress → одна
	var mInProgress maintenanceListResponse
	doJSON(t, pub("/maintenances?status=in_progress"), "", nil, http.StatusOK, &mInProgress)
	if mInProgress.Pagination.Total != 1 || mInProgress.Items[0].ID != mActive.ID {
		t.Fatalf("maintenance status filter: %+v", mInProgress)
	}

	// сводка: active_incidents = 1 (active, не resolved/hidden), active_maintenances = 2 (не completed)
	var summary pageSummaryResponse
	doJSON(t, pub("/summary"), "", nil, http.StatusOK, &summary)
	if len(summary.ActiveIncidents) != 1 || summary.ActiveIncidents[0].ID != incActive.ID {
		t.Fatalf("active_incidents: %+v", summary.ActiveIncidents)
	}
	if len(summary.ActiveMaintenances) != 2 {
		t.Fatalf("active_maintenances=%d, want 2", len(summary.ActiveMaintenances))
	}
}
