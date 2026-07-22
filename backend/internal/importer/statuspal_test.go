package importer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/healthpage/backend/internal/domain"
)

// newTestStatusPal поднимает httptest-сервер, воспроизводящий реальные ответы StatusPal API v2
// (сверено на живом read-only ключе клиента 2026-07-22 — см. комментарий у типа StatusPal).
func newTestStatusPal(t *testing.T, handler http.HandlerFunc) (*StatusPal, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	sp := &StatusPal{
		BaseFor: func(domain.ImportRegion) string { return srv.URL },
		HTTP:    srv.Client(),
	}
	return sp, srv
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode: %v", err)
	}
}

func TestFetchComponentsBuildsFullTreeFromChildrenIDs(t *testing.T) {
	sp, _ := newTestStatusPal(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status_pages/acme/services":
			writeJSON(t, w, spServicesResponse{Services: []spService{
				{ID: 1, Name: "Website", Private: false, ChildrenIDs: []int{2}},
			}})
		case "/status_pages/acme/services/2":
			writeJSON(t, w, spServiceResponse{Service: spService{
				ID: 2, Name: "Auth", ParentID: intPtr(1), ChildrenIDs: []int{},
			}})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})

	got, err := sp.FetchComponents(context.Background(), domain.ImportCreds{Subdomain: "acme"})
	if err != nil {
		t.Fatalf("FetchComponents: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 components (root+child), got %d: %+v", len(got), got)
	}
	byID := map[string]domain.ImportedComponent{}
	for _, c := range got {
		byID[c.ExternalID] = c
	}
	if byID["1"].ParentExternalID != "" {
		t.Fatalf("root parent should be empty, got %q", byID["1"].ParentExternalID)
	}
	if byID["2"].ParentExternalID != "1" {
		t.Fatalf("child parent = %q, want 1", byID["2"].ParentExternalID)
	}
}

func TestFetchIncidentsMergesMajorAndMinorMapsUpdateTypes(t *testing.T) {
	sp, _ := newTestStatusPal(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("type") {
		case "major":
			writeJSON(t, w, spIncidentsResponse{Incidents: []spIncident{
				{ID: 1, Title: "Down", Type: "major", ServiceIDs: []int{10}, Updates: []spIncidentUpdate{
					{Type: "issue", Description: "investigating", InsertedAt: time.Now()},
					{Type: "deescalate", Description: "better", InsertedAt: time.Now()},
				}},
			}})
		case "minor":
			writeJSON(t, w, spIncidentsResponse{Incidents: []spIncident{
				{ID: 2, Title: "Slow", Type: "minor", Updates: []spIncidentUpdate{
					{Type: "resolved", Description: "fixed", InsertedAt: time.Now()},
				}},
			}})
		case "scheduled":
			t.Fatalf("FetchIncidents must not request type=scheduled")
		default:
			t.Fatalf("unexpected type filter %q", r.URL.Query().Get("type"))
		}
	})

	got, err := sp.FetchIncidents(context.Background(), domain.ImportCreds{Subdomain: "acme"})
	if err != nil {
		t.Fatalf("FetchIncidents: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 incidents, got %d", len(got))
	}
	var major, minor *domain.ImportedIncident
	for i := range got {
		switch got[i].ExternalID {
		case "1":
			major = &got[i]
		case "2":
			minor = &got[i]
		}
	}
	if major == nil || minor == nil {
		t.Fatalf("missing incident: %+v", got)
	}
	if major.Impact != domain.ImpactMajor {
		t.Fatalf("major impact = %v", major.Impact)
	}
	if major.Status != domain.IncidentMonitoring {
		t.Fatalf("major status (last update=deescalate) = %v, want monitoring", major.Status)
	}
	if major.Updates[0].Status != domain.IncidentInvestigating {
		t.Fatalf("first update (issue) = %v, want investigating", major.Updates[0].Status)
	}
	if minor.Status != domain.IncidentResolved {
		t.Fatalf("minor status = %v, want resolved (has EndsAt via resolved update path)", minor.Status)
	}
}

func TestFetchIncidentsFollowsPaginationLinks(t *testing.T) {
	// links.next в реальном API — уже готовый абсолютный URL (иногда даже с опечаткой версии в
	// пути, см. комментарий у типа StatusPal); handler строится через httptest.NewServer с
	// заглушкой, а сам URL сервера подставляется в тело ответа первой страницы.
	page2Hit := false
	sp, srv := newTestStatusPal(t, nil)
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") != "major" {
			writeJSON(t, w, spIncidentsResponse{})
			return
		}
		if r.URL.Query().Get("after") == "p2" {
			page2Hit = true
			writeJSON(t, w, spIncidentsResponse{Incidents: []spIncident{{ID: 2, Title: "b", Type: "major"}}})
			return
		}
		next := srv.URL + "/status_pages/acme/incidents?type=major&after=p2"
		writeJSON(t, w, spIncidentsResponse{
			Incidents: []spIncident{{ID: 1, Title: "a", Type: "major"}},
			Links:     spLinks{Next: &next},
		})
	})

	got, err := sp.FetchIncidents(context.Background(), domain.ImportCreds{Subdomain: "acme"})
	if err != nil {
		t.Fatalf("FetchIncidents: %v", err)
	}
	if !page2Hit {
		t.Fatal("pagination did not follow links.next")
	}
	if len(got) != 2 {
		t.Fatalf("want 2 incidents across both pages, got %d", len(got))
	}
}

func TestFetchMaintenancesUsesScheduledIncidentTypeNotDedicatedPath(t *testing.T) {
	sp, _ := newTestStatusPal(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status_pages/acme/maintenances" {
			t.Fatal("must not call the non-existent dedicated /maintenances path")
		}
		if r.URL.Query().Get("type") != "scheduled" {
			t.Fatalf("want type=scheduled, got %q", r.URL.Query().Get("type"))
		}
		future := time.Now().Add(24 * time.Hour)
		writeJSON(t, w, spIncidentsResponse{Incidents: []spIncident{
			{ID: 5, Title: "DB upgrade", Type: "scheduled", StartsAt: future,
				Updates: []spIncidentUpdate{{Type: "issue", Description: "planned window"}}},
		}})
	})

	got, err := sp.FetchMaintenances(context.Background(), domain.ImportCreds{Subdomain: "acme"})
	if err != nil {
		t.Fatalf("FetchMaintenances: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 maintenance, got %d", len(got))
	}
	if got[0].Description != "planned window" {
		t.Fatalf("description = %q, want first update description", got[0].Description)
	}
	if got[0].Status != domain.MaintenanceScheduled {
		t.Fatalf("status = %v, want scheduled (starts in future, no ends_at)", got[0].Status)
	}
	if !got[0].EndAt.IsZero() {
		t.Fatalf("EndAt should be zero value when ends_at is null, got %v", got[0].EndAt)
	}
}

func TestFetchSubscribersFiltersEmailOnlyAndKeepsUUIDIDs(t *testing.T) {
	sp, _ := newTestStatusPal(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, spSubscriptionsResponse{Subscriptions: []spSubscription{
			{ID: "325b26a3-d060-4850-8325-c0fcf97f47a6", Email: "a@example.com", Type: "email"},
			{ID: "67953399-0b16-4bdc-953d-bbaaeef3c15c", Email: "", Type: "slack"},
		}})
	})

	got, err := sp.FetchSubscribers(context.Background(), domain.ImportCreds{Subdomain: "acme"})
	if err != nil {
		t.Fatalf("FetchSubscribers: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 (slack filtered out), got %d: %+v", len(got), got)
	}
	if got[0].ExternalID != "325b26a3-d060-4850-8325-c0fcf97f47a6" {
		t.Fatalf("ExternalID = %q, want the UUID string preserved", got[0].ExternalID)
	}
}

func TestGetUnauthorized(t *testing.T) {
	sp, _ := newTestStatusPal(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := sp.FetchComponents(context.Background(), domain.ImportCreds{Subdomain: "acme"})
	if err == nil {
		t.Fatal("want error on 401")
	}
}

func intPtr(v int) *int { return &v }
