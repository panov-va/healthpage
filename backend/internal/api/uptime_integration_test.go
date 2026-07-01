package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/healthpage/backend/internal/auth"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
)

// Интеграционный тест uptime (этап 7.1) на реальном PG: расчёт по истории статусов, гейтинг
// приватных/чужих компонентов. Запуск:
//
//	HEALTHPAGE_TEST_DB="postgres://healthpage:healthpage@localhost:5432/healthpage?sslmode=disable" \
//	  go test ./internal/api/ -run TestUptimeIntegration
func TestUptimeIntegration(t *testing.T) {
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
		Auth: auth.NewService(st, tm), Store: st, RefreshTTL: time.Hour,
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
		map[string]string{"email": "up-" + uuid.NewString() + "@example.test", "password": "supersecret"},
		http.StatusCreated, &reg)
	uid, _ := uuid.Parse(reg.User.ID)
	cleanup = append(cleanup, uid)
	token := reg.AccessToken

	slug := "up-" + uuid.NewString()[:8]
	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", token,
		map[string]string{"name": "Up Co", "slug": slug}, http.StatusCreated, &page)

	var comp componentResponse
	doJSON(t, srv.URL+"/api/v1/components", token,
		map[string]any{"status_page_id": page.ID, "name": "API"}, http.StatusCreated, &comp)

	uptimeURL := srv.URL + "/api/v1/pages/" + slug + "/uptime?component_id=" + comp.ID

	// Без истории простоев → 100%, daily на 90 дней.
	var rep uptimeReportResponse
	doJSON(t, uptimeURL+"&days=90", "", nil, http.StatusOK, &rep)
	if rep.UptimePercent != 100 || len(rep.Daily) != 90 {
		t.Fatalf("no-history uptime: percent=%v daily=%d", rep.UptimePercent, len(rep.Daily))
	}

	// Состариваем компонент (окно uptime клиппируется датой создания) и вставляем полный день
	// простоя 3 дня назад (major_outage).
	cid, _ := uuid.Parse(comp.ID)
	if _, err := raw.Exec(ctx, "UPDATE components SET created_at = now() - interval '100 days' WHERE id=$1", cid); err != nil {
		t.Fatalf("backdate component: %v", err)
	}
	dayStart := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -3)
	dayEnd := dayStart.AddDate(0, 0, 1)
	if _, err := raw.Exec(ctx,
		"INSERT INTO component_status_history (component_id, status, started_at, ended_at, source) VALUES ($1,'major_outage',$2,$3,'incident')",
		cid, dayStart, dayEnd); err != nil {
		t.Fatalf("insert outage: %v", err)
	}

	doJSON(t, uptimeURL+"&days=90", "", nil, http.StatusOK, &rep)
	if rep.UptimePercent >= 100 || rep.UptimePercent < 98 {
		t.Fatalf("after 1-day outage in 90d window uptime=%v (want <100, ~98.9)", rep.UptimePercent)
	}

	// Приватный компонент → 404 (не раскрывается публично).
	var priv componentResponse
	doJSON(t, srv.URL+"/api/v1/components", token,
		map[string]any{"status_page_id": page.ID, "name": "Secret", "is_private": true}, http.StatusCreated, &priv)
	doStatus(t, http.MethodGet, srv.URL+"/api/v1/pages/"+slug+"/uptime?component_id="+priv.ID, "", nil, http.StatusNotFound)

	// Несуществующий компонент → 404.
	doStatus(t, http.MethodGet, srv.URL+"/api/v1/pages/"+slug+"/uptime?component_id="+uuid.NewString(), "", nil, http.StatusNotFound)

	// Без component_id → 422.
	doStatus(t, http.MethodGet, srv.URL+"/api/v1/pages/"+slug+"/uptime", "", nil, http.StatusUnprocessableEntity)
}
