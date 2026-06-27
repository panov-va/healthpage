package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
)

// Интеграционный тест store против реального PostgreSQL. Запуск:
//
//	HEALTHPAGE_TEST_DB="postgres://healthpage:healthpage@localhost:5432/healthpage?sslmode=disable" \
//	  go test ./internal/store/ -run TestStoreIntegration
//
// Без переменной — пропускается (например, в CI без БД).
func TestStoreIntegration(t *testing.T) {
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

	// Отдельный пул для подготовки/очистки сырым SQL.
	raw, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("raw pool: %v", err)
	}
	defer raw.Close()

	// Уникальный оператор+аккаунт для изоляции прогона.
	email := "it-" + uuid.NewString() + "@example.test"
	user, account, err := st.CreateUserWithAccount(ctx, email, "hash", "IT", "IT acc", "ru")
	if err != nil {
		t.Fatalf("CreateUserWithAccount: %v", err)
	}
	t.Cleanup(func() {
		// Удаление аккаунта каскадно убирает страницы/группы/компоненты/историю.
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE id=$1", account.ID)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", user.ID)
	})

	// Страница.
	slug := "it-" + uuid.NewString()[:8]
	page, err := st.CreateStatusPage(ctx, account.ID, "Demo", "", slug, "UTC", "ru", "public")
	if err != nil {
		t.Fatalf("CreateStatusPage: %v", err)
	}
	if got, err := st.StatusPageBySlug(ctx, slug); err != nil || got.ID != page.ID {
		t.Fatalf("StatusPageBySlug: got %v err %v", got.ID, err)
	}

	// Дубль slug -> ErrSlugTaken.
	if _, err := st.CreateStatusPage(ctx, account.ID, "Dup", "", slug, "UTC", "ru", "public"); err != store.ErrSlugTaken {
		t.Fatalf("duplicate slug: want ErrSlugTaken, got %v", err)
	}

	// Группа.
	group, err := st.CreateComponentGroup(ctx, page.ID, "Core", 0)
	if err != nil {
		t.Fatalf("CreateComponentGroup: %v", err)
	}

	// Компоненты: родитель (в группе) + ребёнок.
	parent, err := st.CreateComponent(ctx, domain.Component{
		StatusPageID: page.ID, GroupID: &group.ID, Name: "API", ShowUptime: true, DisplayState: true,
	})
	if err != nil {
		t.Fatalf("CreateComponent parent: %v", err)
	}
	if parent.CurrentStatus != domain.StatusOperational {
		t.Fatalf("new component default status = %q, want operational", parent.CurrentStatus)
	}
	if _, err := st.CreateComponent(ctx, domain.Component{
		StatusPageID: page.ID, ParentID: &parent.ID, Name: "API-DB", ShowUptime: true, DisplayState: true,
	}); err != nil {
		t.Fatalf("CreateComponent child: %v", err)
	}

	// Список + дерево (домен поверх store).
	comps, err := st.ListComponentsByPage(ctx, page.ID)
	if err != nil || len(comps) != 2 {
		t.Fatalf("ListComponentsByPage: len=%d err=%v", len(comps), err)
	}
	roots := domain.BuildComponentTree(comps)
	if len(roots) != 1 || len(roots[0].Children) != 1 {
		t.Fatalf("tree: want 1 root with 1 child, got %d roots", len(roots))
	}

	// Смена статуса: история закрывает старый период и открывает новый.
	if _, err := st.ChangeComponentStatus(ctx, parent.ID, domain.StatusMajorOutage, domain.SourceManual); err != nil {
		t.Fatalf("ChangeComponentStatus 1: %v", err)
	}
	if _, err := st.ChangeComponentStatus(ctx, parent.ID, domain.StatusOperational, domain.SourceManual); err != nil {
		t.Fatalf("ChangeComponentStatus 2: %v", err)
	}
	hist, err := st.ListStatusHistory(ctx, parent.ID)
	if err != nil || len(hist) != 2 {
		t.Fatalf("history: len=%d err=%v", len(hist), err)
	}
	if hist[0].EndedAt == nil {
		t.Fatal("first history period must be closed (ended_at set)")
	}
	if hist[1].EndedAt != nil {
		t.Fatal("last history period must be open (ended_at nil)")
	}

	// Обновление страницы.
	page.Name = "Demo Renamed"
	if upd, err := st.UpdateStatusPage(ctx, page); err != nil || upd.Name != "Demo Renamed" {
		t.Fatalf("UpdateStatusPage: name=%q err=%v", upd.Name, err)
	}

	// Мягкое удаление компонента-ребёнка.
	if err := st.SoftDeleteComponent(ctx, roots[0].Children[0].ID); err != nil {
		t.Fatalf("SoftDeleteComponent: %v", err)
	}
	comps, _ = st.ListComponentsByPage(ctx, page.ID)
	if len(comps) != 1 {
		t.Fatalf("after soft-delete: want 1 component, got %d", len(comps))
	}
}
