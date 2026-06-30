package api

import (
	"bytes"
	"context"
	"io"
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
	"github.com/healthpage/backend/internal/webhook"
)

// signedWebhook отправляет POST с телом и X-Signature = HMAC-SHA256(secret, body).
func signedWebhook(t *testing.T, url, secret string, body []byte) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", webhook.Sign(secret, body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// TestWebhookIntegrationsIntegration проверяет этап 5.3 против реального PG16: управление
// интеграциями (CRUD оператором, секрет единожды), входящий Grafana/Prometheus webhook с HMAC,
// идемпотентное создание/закрытие инцидента по dedup-ключу, маппинг на компонент, негативы
// (битая подпись/чужой источник/неизвестная интеграция → 401; generic → 501; изоляция операторов).
// Запуск: HEALTHPAGE_TEST_DB=... go test ./internal/api/ -run TestWebhookIntegrationsIntegration
func TestWebhookIntegrationsIntegration(t *testing.T) {
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
		Auth: auth.NewService(st, tm), Store: st, SubSecret: "s", RefreshTTL: time.Hour,
	}))
	defer srv.Close()

	var a authResultResponse
	doJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]string{
		"email": "wi-" + uuid.NewString() + "@example.test", "password": "supersecret",
	}, http.StatusCreated, &a)
	auid, _ := uuid.Parse(a.User.ID)
	t.Cleanup(func() {
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", auid)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", auid)
	})
	var page statusPageResponse
	doJSON(t, srv.URL+"/api/v1/pages", a.AccessToken, map[string]string{"name": "A", "slug": "wi-" + uuid.NewString()[:8]}, http.StatusCreated, &page)
	var comp struct {
		ID string `json:"id"`
	}
	doJSON(t, srv.URL+"/api/v1/components", a.AccessToken, map[string]any{
		"status_page_id": page.ID, "name": "API",
	}, http.StatusCreated, &comp)

	intURL := srv.URL + "/api/v1/webhook-integrations"

	// ── Создать grafana-интеграцию с маппингом метки service→компонент ──
	var created webhookIntegrationResponse
	doJSON(t, intURL, a.AccessToken, map[string]any{
		"status_page_id": page.ID, "source": "grafana", "name": "prod",
		"component_mapping": map[string]any{
			"match_label":    "service",
			"map":            map[string]string{"api": comp.ID},
			"default_impact": "major",
		},
	}, http.StatusCreated, &created)
	if created.Secret == nil || *created.Secret == "" {
		t.Fatal("секрет должен возвращаться при создании")
	}
	secret := *created.Secret
	grafanaURL := srv.URL + "/api/v1/integrations/" + created.ID + "/grafana"

	// Список — без секрета.
	var list []webhookIntegrationResponse
	doJSON(t, intURL+"?status_page_id="+page.ID, a.AccessToken, nil, http.StatusOK, &list)
	if len(list) != 1 || list[0].Secret != nil {
		t.Fatalf("список: ожидалась 1 интеграция без секрета, got %+v", list)
	}

	// ── Firing webhook → создаётся инцидент ──
	firing := []byte(`{"alerts":[{"status":"firing","fingerprint":"fp-1","labels":{"alertname":"HighLatency","service":"api"},"annotations":{"summary":"Высокая задержка","description":"p99>1s"}}]}`)
	resp := signedWebhook(t, grafanaURL, secret, firing)
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("firing webhook: want 202, got %d (%s)", resp.StatusCode, b)
	}
	_ = resp.Body.Close()

	var incList incidentListResponse
	doJSON(t, srv.URL+"/api/v1/incidents?status_page_id="+page.ID, a.AccessToken, nil, http.StatusOK, &incList)
	if len(incList.Items) != 1 {
		t.Fatalf("ожидался 1 инцидент после firing, got %d", len(incList.Items))
	}
	inc := incList.Items[0]
	if inc.Impact != "major" || inc.Title != "Высокая задержка" || inc.CurrentStatus != "investigating" {
		t.Errorf("инцидент неверный: %+v", inc)
	}
	if len(inc.Components) != 1 || inc.Components[0].ComponentID != comp.ID || inc.Components[0].ComponentStatusInIncident != "partial_outage" {
		t.Errorf("маппинг компонента неверный: %+v", inc.Components)
	}

	// Повторный firing → идемпотентно, дубля нет.
	resp = signedWebhook(t, grafanaURL, secret, firing)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("повторный firing: want 202, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	doJSON(t, srv.URL+"/api/v1/incidents?status_page_id="+page.ID, a.AccessToken, nil, http.StatusOK, &incList)
	if len(incList.Items) != 1 {
		t.Fatalf("повторный firing не должен плодить дубли, got %d", len(incList.Items))
	}

	// ── Resolved webhook → инцидент закрывается ──
	resolved := []byte(`{"alerts":[{"status":"resolved","fingerprint":"fp-1","labels":{"alertname":"HighLatency","service":"api"},"annotations":{"summary":"Высокая задержка"}}]}`)
	resp = signedWebhook(t, grafanaURL, secret, resolved)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("resolved webhook: want 202, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	var got incidentResponse
	doJSON(t, srv.URL+"/api/v1/incidents/"+inc.ID, a.AccessToken, nil, http.StatusOK, &got)
	if got.CurrentStatus != "resolved" || got.ResolvedAt == nil {
		t.Errorf("инцидент должен быть resolved: %+v", got)
	}

	// Resolved повторно (нет открытого) → no-op, дубля/ошибки нет.
	resp = signedWebhook(t, grafanaURL, secret, resolved)
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("повторный resolved: want 202, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// Новый firing с тем же ключом после resolve → новый инцидент (рецидив).
	resp = signedWebhook(t, grafanaURL, secret, firing)
	_ = resp.Body.Close()
	doJSON(t, srv.URL+"/api/v1/incidents?status_page_id="+page.ID, a.AccessToken, nil, http.StatusOK, &incList)
	if len(incList.Items) != 2 {
		t.Fatalf("рецидив после resolve должен создать новый инцидент, всего ожидалось 2, got %d", len(incList.Items))
	}

	// ── Негативы аутентификации ──
	// Битая подпись.
	req, _ := http.NewRequest(http.MethodPost, grafanaURL, bytes.NewReader(firing))
	req.Header.Set("X-Signature", "deadbeef")
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("битая подпись: want 401, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	// Чужой источник (prometheus-роут на grafana-интеграции).
	resp = signedWebhook(t, srv.URL+"/api/v1/integrations/"+created.ID+"/prometheus", secret, firing)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("чужой источник: want 401, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	// Неизвестная интеграция.
	resp = signedWebhook(t, srv.URL+"/api/v1/integrations/"+uuid.NewString()+"/grafana", secret, firing)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("неизвестная интеграция: want 401, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	// generic → 501.
	resp = signedWebhook(t, srv.URL+"/api/v1/integrations/"+created.ID+"/generic", secret, firing)
	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("generic: want 501, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// ── Prometheus-интеграция: отдельный секрет, firing создаёт инцидент ──
	var prom webhookIntegrationResponse
	doJSON(t, intURL, a.AccessToken, map[string]any{
		"status_page_id": page.ID, "source": "prometheus", "name": "alertmanager",
		"component_mapping": map[string]any{"default_component_ids": []string{comp.ID}, "default_impact": "critical"},
	}, http.StatusCreated, &prom)
	promFiring := []byte(`{"alerts":[{"status":"firing","fingerprint":"pfp-1","labels":{"alertname":"DiskFull"},"annotations":{"summary":"Диск заполнен"}}]}`)
	resp = signedWebhook(t, srv.URL+"/api/v1/integrations/"+prom.ID+"/prometheus", *prom.Secret, promFiring)
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("prometheus firing: want 202, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// ── Управление: get / patch (ротация секрета) / delete ──
	var fetched webhookIntegrationResponse
	doJSON(t, intURL+"/"+created.ID, a.AccessToken, nil, http.StatusOK, &fetched)
	if fetched.Secret != nil || fetched.Name != "prod" {
		t.Errorf("get не должен возвращать секрет: %+v", fetched)
	}
	var patched webhookIntegrationResponse
	patchJSON(t, intURL+"/"+created.ID, a.AccessToken, map[string]any{
		"name": "prod-2", "regenerate_secret": true,
	}, http.StatusOK, &patched)
	if patched.Name != "prod-2" || patched.Secret == nil || *patched.Secret == secret {
		t.Errorf("patch с ротацией: ожидался новый секрет и имя prod-2, got %+v", patched)
	}
	// PATCH без ротации → без секрета.
	var patched2 webhookIntegrationResponse
	patchJSON(t, intURL+"/"+created.ID, a.AccessToken, map[string]any{"name": "prod-3"}, http.StatusOK, &patched2)
	if patched2.Secret != nil {
		t.Errorf("patch без ротации не должен возвращать секрет: %+v", patched2)
	}

	// Изоляция: оператор B.
	var b authResultResponse
	doJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]string{
		"email": "wi-b-" + uuid.NewString() + "@example.test", "password": "supersecret",
	}, http.StatusCreated, &b)
	buid, _ := uuid.Parse(b.User.ID)
	t.Cleanup(func() {
		_, _ = raw.Exec(ctx, "DELETE FROM accounts WHERE owner_user_id=$1", buid)
		_, _ = raw.Exec(ctx, "DELETE FROM users WHERE id=$1", buid)
	})
	doJSON(t, intURL+"/"+created.ID, b.AccessToken, nil, http.StatusNotFound, nil)
	doStatus(t, http.MethodDelete, intURL+"/"+created.ID, b.AccessToken, nil, http.StatusNotFound)

	// Удаление оператором A → 204, повторный get → 404.
	doStatus(t, http.MethodDelete, intURL+"/"+created.ID, a.AccessToken, nil, http.StatusNoContent)
	doJSON(t, intURL+"/"+created.ID, a.AccessToken, nil, http.StatusNotFound, nil)
}
