package api

import (
	"context"
	"encoding/json"
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

// Интеграционный тест собственного домена (этап 4.3.1): установка custom_domain через PATCH,
// верификация CNAME через инъектированный резолвер, конфликт уникальности. Запуск:
//
//	HEALTHPAGE_TEST_DB="postgres://healthpage:healthpage@localhost:5432/healthpage?sslmode=disable" \
//	  go test ./internal/api/ -run TestCustomDomainIntegration
func TestCustomDomainIntegration(t *testing.T) {
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

	// Фейк-резолвер CNAME: status.acme.test → наш target; всё прочее → «чужой» хост.
	const target = "cname.healthpage.ru"
	resolver := func(_ context.Context, host string) (string, error) {
		if host == "status.acme.test" {
			return "cname.healthpage.ru.", nil // с завершающей точкой, как у настоящего LookupCNAME
		}
		return "elsewhere.example.net.", nil
	}

	tm, _ := security.NewTokenManager("test-secret", 15*time.Minute, time.Hour)
	srv := httptest.NewServer(NewRouter(Deps{
		Auth: auth.NewService(st, tm), Store: st, RefreshTTL: time.Hour,
		CNAMETarget: target, CNAMEResolver: resolver,
	}))
	defer srv.Close()

	var cleanup []uuid.UUID
	t.Cleanup(func() {
		for _, uid := range cleanup {
			_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", uid)
			_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", uid)
		}
	})

	register := func(email string) (string, uuid.UUID) {
		var out authResultResponse
		doJSON(t, srv.URL+"/api/v1/auth/register", "",
			map[string]string{"email": email, "password": "supersecret"}, http.StatusCreated, &out)
		uid, _ := uuid.Parse(out.User.ID)
		cleanup = append(cleanup, uid)
		// Собственный домен — premium-фича (этап 6.7); поднимаем тариф аккаунта.
		if _, err := raw.Exec(ctx, "UPDATE accounts SET billing_plan='premium' WHERE owner_user_id=$1", uid); err != nil {
			t.Fatalf("upgrade premium: %v", err)
		}
		return out.AccessToken, uid
	}

	token, _ := register("dom-" + uuid.NewString() + "@example.test")

	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", token,
		map[string]string{"name": "Acme", "slug": "dom-" + uuid.NewString()[:8]}, http.StatusCreated, &page)

	// Домен не задан → verify 422.
	doStatus(t, http.MethodPost, srv.URL+"/api/v1/pages/"+page.ID+"/domain/verify", token, nil, http.StatusUnprocessableEntity)

	// Задаём домен через PATCH (нормализуется в lower-case).
	var patched statusPageResponse
	patchJSON(t, srv.URL+"/api/v1/pages/"+page.ID, token,
		map[string]any{"custom_domain": "Status.ACME.test"}, http.StatusOK, &patched)
	if patched.CustomDomain == nil || *patched.CustomDomain != "status.acme.test" {
		t.Fatalf("custom_domain after patch: %v", patched.CustomDomain)
	}
	if patched.DomainVerified {
		t.Fatal("domain_verified должен быть false сразу после установки домена")
	}

	// Верификация (с токеном): резолвер указывает на наш target → verified=true.
	var verifyOK domainStatusResponse
	verifyDomain(t, srv.URL+"/api/v1/pages/"+page.ID+"/domain/verify", token, http.StatusOK, &verifyOK)
	if !verifyOK.DomainVerified || verifyOK.CNAMETarget != target {
		t.Fatalf("verify (match): %+v", verifyOK)
	}

	// Меняем домен на тот, что резолвится «не туда» → verify ставит false.
	patchJSON(t, srv.URL+"/api/v1/pages/"+page.ID, token,
		map[string]any{"custom_domain": "other.acme.test"}, http.StatusOK, nil)
	var verifyBad domainStatusResponse
	verifyDomain(t, srv.URL+"/api/v1/pages/"+page.ID+"/domain/verify", token, http.StatusOK, &verifyBad)
	if verifyBad.DomainVerified {
		t.Fatal("verify (mismatch) должен дать domain_verified=false")
	}

	// Конфликт уникальности: вторая страница того же оператора берёт занятый домен → 409.
	var page2 statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", token,
		map[string]string{"name": "Acme2", "slug": "dom2-" + uuid.NewString()[:8]}, http.StatusCreated, &page2)
	patchJSON(t, srv.URL+"/api/v1/pages/"+page2.ID, token,
		map[string]any{"custom_domain": "other.acme.test"}, http.StatusConflict, nil)

	// Снятие домена (null) → custom_domain очищен.
	patchJSON(t, srv.URL+"/api/v1/pages/"+page.ID, token,
		map[string]any{"custom_domain": nil}, http.StatusOK, &patched)
	if patched.CustomDomain != nil {
		t.Fatalf("custom_domain после снятия: %v", patched.CustomDomain)
	}
}

// verifyDomain выполняет POST /domain/verify с токеном, проверяет статус и декодирует ответ.
func verifyDomain(t *testing.T, url, token string, wantStatus int, out *domainStatusResponse) {
	t.Helper()
	resp := doReq(t, http.MethodPost, url, token, nil)
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Fatalf("POST %s: want %d, got %d", url, wantStatus, resp.StatusCode)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
}
