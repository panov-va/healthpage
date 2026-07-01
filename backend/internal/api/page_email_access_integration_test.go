package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/healthpage/backend/internal/auth"
	"github.com/healthpage/backend/internal/notify"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
)

// Интеграционный тест приватности по списку email + magic-link (этап 4.2.1). Запуск:
//
//	HEALTHPAGE_TEST_DB=... go test ./internal/api/ -run TestEmailAccessIntegration
func TestEmailAccessIntegration(t *testing.T) {
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

	const secret = "test-sub-secret"
	pub := &capturePublisher{}
	engine := notify.New(st, pub, nil)
	tm, _ := security.NewTokenManager("test-secret", 15*time.Minute, time.Hour)
	srv := httptest.NewServer(NewRouter(Deps{
		Auth: auth.NewService(st, tm), Store: st, Notifier: engine, SubSecret: secret, RefreshTTL: time.Hour,
	}))
	defer srv.Close()

	var cleanup []uuid.UUID
	t.Cleanup(func() {
		for _, uid := range cleanup {
			_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", uid)
			_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", uid)
		}
	})
	register := func(email string) string {
		var out authResultResponse
		doJSON(t, srv.URL+"/api/v1/auth/register", "",
			map[string]string{"email": email, "password": "supersecret"}, http.StatusCreated, &out)
		uid, _ := uuid.Parse(out.User.ID)
		cleanup = append(cleanup, uid)
		// Приватные страницы (список email) — premium-фича (этап 6.7); поднимаем тариф.
		if _, err := raw.Exec(ctx, "UPDATE accounts SET billing_plan='premium' WHERE owner_user_id=$1", uid); err != nil {
			t.Fatalf("upgrade premium: %v", err)
		}
		return out.AccessToken
	}

	token := register("eacc-" + uuid.NewString() + "@example.test")

	slug := "eacc-" + uuid.NewString()[:8]
	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", token,
		map[string]string{"name": "Closed", "slug": slug}, http.StatusCreated, &page)
	// делаем приватной (без пароля — доступ по списку email)
	patchJSON(t, srv.URL+"/api/v1/pages/"+page.ID, token,
		map[string]any{"visibility": "private"}, http.StatusOK, nil)

	// admin: добавить email в список доступа
	var ae allowedEmailResponse
	doJSON(t, srv.URL+"/api/v1/pages/"+page.ID+"/allowed-emails", token,
		map[string]string{"email": "Guest@Acme.test"}, http.StatusCreated, &ae)
	// дубль → 409
	doStatusBody(t, http.MethodPost, srv.URL+"/api/v1/pages/"+page.ID+"/allowed-emails", token,
		map[string]any{"email": "guest@acme.test"}, http.StatusConflict)
	// невалидный → 422
	doStatusBody(t, http.MethodPost, srv.URL+"/api/v1/pages/"+page.ID+"/allowed-emails", token,
		map[string]any{"email": "notanemail"}, http.StatusUnprocessableEntity)
	// список содержит адрес
	var list []allowedEmailResponse
	doJSON(t, srv.URL+"/api/v1/pages/"+page.ID+"/allowed-emails", token, nil, http.StatusOK, &list)
	if len(list) != 1 {
		t.Fatalf("allowed-emails list: want 1, got %d", len(list))
	}

	// public: запрос ссылки разрешённым адресом → 202 + опубликовано письмо magic-link
	before := pub.count()
	accessPostJSON(t, srv.URL+"/api/v1/pages/"+slug+"/access/request-link", `{"email":"guest@acme.test"}`, http.StatusAccepted, nil)
	if pub.count() != before+1 {
		t.Fatalf("request-link (allowed): publish count %d → %d", before, pub.count())
	}
	tokenLink := extractAccessLinkToken(t, pub)

	// запрос неразрешённым адресом → тоже 202, но письмо не публикуется
	before = pub.count()
	accessPostJSON(t, srv.URL+"/api/v1/pages/"+slug+"/access/request-link", `{"email":"stranger@acme.test"}`, http.StatusAccepted, nil)
	if pub.count() != before {
		t.Fatal("request-link (not allowed) не должен публиковать письмо")
	}

	// verify magic-link токеном → access_token; им открывается приватная сводка
	var grant struct {
		AccessToken string `json:"access_token"`
	}
	getJSONStatus(t, srv.URL+"/api/v1/pages/"+slug+"/access/verify?token="+tokenLink, http.StatusOK, &grant)
	if grant.AccessToken == "" {
		t.Fatal("verify: пустой access_token")
	}
	getWithAccess(t, srv.URL+"/api/v1/pages/"+slug+"/summary", grant.AccessToken, http.StatusOK, nil)

	// битый токен → 401
	getJSONStatus(t, srv.URL+"/api/v1/pages/"+slug+"/access/verify?token=garbage", http.StatusUnauthorized, nil)

	// удалить адрес → старый magic-link больше не верифицируется (доступ отозван)
	doStatus(t, http.MethodDelete, srv.URL+"/api/v1/allowed-emails/"+ae.ID, token, nil, http.StatusNoContent)
	getJSONStatus(t, srv.URL+"/api/v1/pages/"+slug+"/access/verify?token="+tokenLink, http.StatusUnauthorized, nil)

	// изоляция: чужой оператор не удалит/не увидит список
	other := register("other-" + uuid.NewString() + "@example.test")
	doStatus(t, http.MethodGet, srv.URL+"/api/v1/pages/"+page.ID+"/allowed-emails", other, nil, http.StatusNotFound)
}

func extractAccessLinkToken(t *testing.T, pub *capturePublisher) string {
	t.Helper()
	pub.mu.Lock()
	defer pub.mu.Unlock()
	for i := len(pub.msgs) - 1; i >= 0; i-- {
		if pub.msgs[i].Event == "access_link" {
			var p notify.AccessLinkPayload
			if err := json.Unmarshal(pub.msgs[i].Payload, &p); err != nil {
				t.Fatalf("unmarshal access link payload: %v", err)
			}
			return p.Token
		}
	}
	t.Fatal("access_link сообщение не найдено")
	return ""
}

// doStatusBody — запрос с JSON-телом, проверка только статуса.
func doStatusBody(t *testing.T, method, url, token string, body any, wantStatus int) {
	t.Helper()
	buf, _ := json.Marshal(body)
	resp := doReq(t, method, url, token, strings.NewReader(string(buf)))
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s: want %d, got %d", method, url, wantStatus, resp.StatusCode)
	}
}

// getJSONStatus — публичный GET без токена, проверка статуса + опц. декод.
func getJSONStatus(t *testing.T, url string, wantStatus int, out any) {
	t.Helper()
	resp := doReq(t, http.MethodGet, url, "", nil)
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Fatalf("GET %s: want %d, got %d", url, wantStatus, resp.StatusCode)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
}
