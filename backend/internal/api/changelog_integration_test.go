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

// Интеграционный тест changelog (этап 7.2) на реальном PG: черновик скрыт публично, публикация
// открывает его, patch/delete, изоляция операторов. Запуск:
//
//	HEALTHPAGE_TEST_DB="postgres://healthpage:healthpage@localhost:5432/healthpage?sslmode=disable" \
//	  go test ./internal/api/ -run TestChangelogIntegration
func TestChangelogIntegration(t *testing.T) {
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

	register := func(email string) (string, string) {
		var out authResultResponse
		doJSON(t, srv.URL+"/api/v1/auth/register", "",
			map[string]string{"email": email, "password": "supersecret"}, http.StatusCreated, &out)
		uid, _ := uuid.Parse(out.User.ID)
		cleanup = append(cleanup, uid)
		return out.AccessToken, out.User.ID
	}

	token, _ := register("cl-" + uuid.NewString() + "@example.test")
	slug := "cl-" + uuid.NewString()[:8]
	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", token,
		map[string]string{"name": "CL Co", "slug": slug}, http.StatusCreated, &page)

	publicURL := srv.URL + "/api/v1/pages/" + slug + "/changelog"

	// Создаём черновик (published=false).
	var draft changelogResponse
	doJSON(t, srv.URL+"/api/v1/changelog", token,
		map[string]any{"status_page_id": page.ID, "title": "v1.0", "body": "Первый релиз"},
		http.StatusCreated, &draft)
	if draft.Published || draft.PublishedAt != nil {
		t.Fatalf("draft must be unpublished: %+v", draft)
	}

	// Публично черновик не виден.
	var pub []changelogResponse
	doJSON(t, publicURL, "", nil, http.StatusOK, &pub)
	if len(pub) != 0 {
		t.Fatalf("draft must be hidden publicly, got %d", len(pub))
	}

	// Админский список включает черновик.
	var adminList []changelogResponse
	doJSON(t, srv.URL+"/api/v1/changelog?status_page_id="+page.ID, token, nil, http.StatusOK, &adminList)
	if len(adminList) != 1 {
		t.Fatalf("admin list=%d want 1", len(adminList))
	}

	// Публикуем через PATCH.
	var published changelogResponse
	patchJSON(t, srv.URL+"/api/v1/changelog/"+draft.ID, token,
		map[string]any{"published": true}, http.StatusOK, &published)
	if !published.Published || published.PublishedAt == nil {
		t.Fatalf("after publish: %+v", published)
	}

	// Теперь запись видна публично.
	doJSON(t, publicURL, "", nil, http.StatusOK, &pub)
	if len(pub) != 1 || pub[0].Title != "v1.0" {
		t.Fatalf("published entry must be public: %+v", pub)
	}

	// Правим заголовок.
	patchJSON(t, srv.URL+"/api/v1/changelog/"+draft.ID, token,
		map[string]any{"title": "v1.0 — стабильный"}, http.StatusOK, nil)

	// Снятие с публикации снова скрывает.
	patchJSON(t, srv.URL+"/api/v1/changelog/"+draft.ID, token,
		map[string]any{"published": false}, http.StatusOK, &published)
	if published.PublishedAt != nil {
		t.Fatalf("unpublish must clear published_at: %+v", published)
	}
	doJSON(t, publicURL, "", nil, http.StatusOK, &pub)
	if len(pub) != 0 {
		t.Fatalf("unpublished must be hidden, got %d", len(pub))
	}

	// Изоляция: другой оператор не видит/не трогает запись.
	otherToken, _ := register("cl2-" + uuid.NewString() + "@example.test")
	doStatus(t, http.MethodGet, srv.URL+"/api/v1/changelog/"+draft.ID, otherToken, nil, http.StatusNotFound)

	// Пустой title → 422.
	doStatusBody(t, http.MethodPost, srv.URL+"/api/v1/changelog", token,
		map[string]any{"status_page_id": page.ID, "title": "  "}, http.StatusUnprocessableEntity)

	// Удаление.
	doStatus(t, http.MethodDelete, srv.URL+"/api/v1/changelog/"+draft.ID, token, nil, http.StatusNoContent)
	doStatus(t, http.MethodGet, srv.URL+"/api/v1/changelog/"+draft.ID, token, nil, http.StatusNotFound)
}
