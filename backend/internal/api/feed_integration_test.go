package api

import (
	"context"
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
	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
)

// Интеграционный тест публичных фидов (RSS/iCal) против реального PostgreSQL. Запуск:
//
//	HEALTHPAGE_TEST_DB=... go test ./internal/api/ -run TestFeedIntegration
func TestFeedIntegration(t *testing.T) {
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
		Auth: auth.NewService(st, tm), Store: st, BaseURL: "https://h", RefreshTTL: time.Hour,
	}))
	defer srv.Close()

	em := "feed-" + uuid.NewString() + "@example.test"
	user, account, err := st.CreateUserWithAccount(ctx, em, "hash", "IT", "IT", "ru")
	if err != nil {
		t.Fatalf("CreateUserWithAccount: %v", err)
	}
	t.Cleanup(func() {
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE id=$1", account.ID)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", user.ID)
	})

	slug := "it-" + uuid.NewString()[:8]
	page, err := st.CreateStatusPage(ctx, account.ID, user.ID, "Acme", "", slug, "UTC", "ru", "public")
	if err != nil {
		t.Fatalf("CreateStatusPage: %v", err)
	}

	// Видимый инцидент + плановые работы.
	if _, err := st.CreateIncident(ctx, domain.Incident{
		StatusPageID: page.ID, Title: "API down", CurrentStatus: domain.IncidentInvestigating,
		Impact: domain.ImpactMajor, StartedAt: time.Now().UTC(), IsVisible: true,
	}, "Looking into it", false); err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	start := time.Now().UTC().Add(24 * time.Hour)
	if _, err := st.CreateMaintenance(ctx, domain.Maintenance{
		StatusPageID: page.ID, Title: "DB upgrade", Status: domain.MaintenanceScheduled,
		ScheduledStart: start, ScheduledEnd: start.Add(time.Hour),
	}); err != nil {
		t.Fatalf("CreateMaintenance: %v", err)
	}

	// RSS: 200, application/rss+xml, содержит обе записи.
	rssBody, rssCT := getBody(t, srv.URL+"/api/v1/pages/"+slug+"/rss")
	if !strings.Contains(rssCT, "application/rss+xml") {
		t.Errorf("RSS Content-Type = %q", rssCT)
	}
	for _, want := range []string{"<rss", "API down", "DB upgrade", "/status/" + slug} {
		if !strings.Contains(rssBody, want) {
			t.Errorf("RSS не содержит %q", want)
		}
	}

	// iCal: 200, text/calendar, VEVENT работ.
	icsBody, icsCT := getBody(t, srv.URL+"/api/v1/pages/"+slug+"/calendar.ics")
	if !strings.Contains(icsCT, "text/calendar") {
		t.Errorf("iCal Content-Type = %q", icsCT)
	}
	for _, want := range []string{"BEGIN:VCALENDAR", "BEGIN:VEVENT", "SUMMARY:DB upgrade"} {
		if !strings.Contains(icsBody, want) {
			t.Errorf("iCal не содержит %q", want)
		}
	}

	// Приватная страница → 404 для фидов.
	if err := st.SoftDeleteStatusPage(ctx, page.ID); err != nil {
		t.Fatalf("cleanup page: %v", err)
	}
	resp := doReq(t, http.MethodGet, srv.URL+"/api/v1/pages/"+slug+"/rss", "", nil)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("RSS удалённой страницы: статус %d, want 404", resp.StatusCode)
	}
}

func getBody(t *testing.T, url string) (body, contentType string) {
	t.Helper()
	resp := doReq(t, http.MethodGet, url, "", nil)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: статус %d, want 200", url, resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	return string(b), resp.Header.Get("Content-Type")
}
