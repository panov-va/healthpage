package importer

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
)

// fakeImporter — источник-заглушка для теста движка (без сети).
type fakeImporter struct {
	comps  []domain.ImportedComponent
	incs   []domain.ImportedIncident
	maints []domain.ImportedMaintenance
	subs   []domain.ImportedSubscriber
}

func (fakeImporter) Source() domain.ImportSource { return domain.SourceStatusPal }
func (f fakeImporter) Probe(context.Context, domain.ImportCreds) (domain.ImportPreview, error) {
	return domain.ImportPreview{}, nil
}
func (f fakeImporter) FetchComponents(context.Context, domain.ImportCreds) ([]domain.ImportedComponent, error) {
	return f.comps, nil
}
func (f fakeImporter) FetchIncidents(context.Context, domain.ImportCreds) ([]domain.ImportedIncident, error) {
	return f.incs, nil
}
func (f fakeImporter) FetchMaintenances(context.Context, domain.ImportCreds) ([]domain.ImportedMaintenance, error) {
	return f.maints, nil
}
func (f fakeImporter) FetchSubscribers(context.Context, domain.ImportCreds) ([]domain.ImportedSubscriber, error) {
	return f.subs, nil
}

// TestImportEngineIntegration прогоняет движок на реальном PG. Запуск:
//
//	HEALTHPAGE_TEST_DB="postgres://healthpage:healthpage@localhost:5432/healthpage?sslmode=disable" \
//	  go test ./internal/importer/ -run TestImportEngineIntegration
func TestImportEngineIntegration(t *testing.T) {
	dsn := os.Getenv("HEALTHPAGE_TEST_DB")
	if dsn == "" {
		t.Skip("HEALTHPAGE_TEST_DB не задан — пропуск интеграционного теста")
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

	user, acc, err := st.CreateUserWithAccount(ctx, "imp-"+randSuffix()+"@example.test", "hash", "Imp", "Imp Co", "ru")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	t.Cleanup(func() {
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", user.ID)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", user.ID)
	})
	page, err := st.CreateStatusPage(ctx, acc.ID, user.ID, "Imported", "", "imp-"+randSuffix(), "UTC", "ru", "public")
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	start := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	fake := fakeImporter{
		comps: []domain.ImportedComponent{
			{ExternalID: "1", Name: "API", Status: domain.StatusOperational, GroupName: "Core"},
			{ExternalID: "2", Name: "Worker", Status: domain.StatusMajorOutage, ParentExternalID: "1"},
		},
		incs: []domain.ImportedIncident{{
			ExternalID: "10", Title: "Outage", Impact: domain.ImpactMajor, Status: domain.IncidentResolved,
			StartedAt: start, ResolvedAt: &end, Components: []string{"1"},
			Updates: []domain.ImportedIncidentUpdate{
				{Status: domain.IncidentInvestigating, Body: "Looking", CreatedAt: start},
				{Status: domain.IncidentResolved, Body: "Fixed", CreatedAt: end},
			},
		}},
		maints: []domain.ImportedMaintenance{{
			ExternalID: "20", Title: "DB upgrade", Status: domain.MaintenanceCompleted,
			StartAt: start, EndAt: end, Components: []string{"1"},
		}},
		subs: []domain.ImportedSubscriber{{ExternalID: "30", Email: "sub@example.test"}},
	}

	eng := NewEngine(st, fake)
	job := domain.ImportJob{ID: page.ID, StatusPageID: page.ID, AccountID: acc.ID, Source: domain.SourceStatusPal, Mode: domain.ModeSkip}

	rep, err := eng.Run(ctx, job, domain.ImportCreds{Subdomain: "x"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if rep.ComponentsCreated != 2 || rep.IncidentsCreated != 1 || rep.MaintenancesCreated != 1 || rep.SubscribersImported != 1 {
		t.Fatalf("report after 1st run: %+v", rep)
	}

	// Дерево: Worker имеет parent = API.
	comps, err := st.ListComponentsByPage(ctx, page.ID)
	if err != nil {
		t.Fatalf("list components: %v", err)
	}
	var apiID, workerParent string
	for _, c := range comps {
		if c.Name == "API" {
			apiID = c.ID.String()
		}
		if c.Name == "Worker" && c.ParentID != nil {
			workerParent = c.ParentID.String()
		}
	}
	if apiID == "" || workerParent != apiID {
		t.Fatalf("parent tree not set: apiID=%s workerParent=%s", apiID, workerParent)
	}

	// Импортированный подписчик НЕ подтверждён (152-ФЗ).
	var confirmed bool
	if err := raw.QueryRow(ctx, "SELECT confirmed FROM subscribers WHERE status_page_id=$1 AND address='sub@example.test'", page.ID).Scan(&confirmed); err != nil {
		t.Fatalf("query subscriber: %v", err)
	}
	if confirmed {
		t.Fatal("импортированный подписчик должен быть confirmed=false (opt-in)")
	}

	// Идемпотентность: повторный прогон (skip) ничего не создаёт заново.
	rep2, err := eng.Run(ctx, job, domain.ImportCreds{Subdomain: "x"})
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	if rep2.ComponentsCreated != 0 || rep2.IncidentsCreated != 0 || rep2.MaintenancesCreated != 0 || rep2.SubscribersImported != 0 {
		t.Fatalf("2nd run must create nothing: %+v", rep2)
	}
	if rep2.ComponentsSkipped != 2 || rep2.IncidentsSkipped != 1 {
		t.Fatalf("2nd run must skip existing: %+v", rep2)
	}
}

func randSuffix() string {
	return time.Now().Format("150405.000000")
}
