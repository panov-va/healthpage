package dokploy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateDomain_IDInResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domain.create" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Fatalf("x-api-key = %q, want test-key", got)
		}
		var req createDomainRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Host != "status.client.com" || req.ApplicationID != "app-1" || req.Port != 3000 ||
			!req.HTTPS || req.CertificateType != "letsencrypt" || req.DomainType != "application" {
			t.Fatalf("unexpected request body: %+v", req)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"domainId": "dom-123"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", "app-1", nil)
	id, err := c.CreateDomain(context.Background(), "status.client.com")
	if err != nil {
		t.Fatalf("CreateDomain: %v", err)
	}
	if id != "dom-123" {
		t.Fatalf("id = %q, want dom-123", id)
	}
}

func TestCreateDomain_FallsBackToByApplicationId(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/domain.create":
			// Ответ без ID — как будто схема ответа не содержит ожидаемого поля.
			_ = json.NewEncoder(w).Encode(map[string]string{})
		case "/domain.byApplicationId":
			if got := r.URL.Query().Get("applicationId"); got != "app-1" {
				t.Fatalf("applicationId query = %q, want app-1", got)
			}
			_ = json.NewEncoder(w).Encode([]domainRecord{
				{ID: "other-id", Host: "other.example.com"},
				{DomainID: "dom-456", Host: "status.client.com"},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", "app-1", nil)
	id, err := c.CreateDomain(context.Background(), "status.client.com")
	if err != nil {
		t.Fatalf("CreateDomain: %v", err)
	}
	if id != "dom-456" {
		t.Fatalf("id = %q, want dom-456", id)
	}
}

func TestCreateDomain_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"BAD_REQUEST","message":"host already exists"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", "app-1", nil)
	_, err := c.CreateDomain(context.Background(), "status.client.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	dErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *dokploy.Error, got %T: %v", err, err)
	}
	if dErr.Code != http.StatusBadRequest {
		t.Fatalf("Code = %d, want 400", dErr.Code)
	}
}

func TestDeleteDomain_EmptyIDNoop(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", "app-1", nil)
	if err := c.DeleteDomain(context.Background(), ""); err != nil {
		t.Fatalf("DeleteDomain: %v", err)
	}
	if called {
		t.Fatal("expected no HTTP call for empty domainID")
	}
}

func TestDeleteDomain_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domain.delete" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["domainId"] != "dom-123" {
			t.Fatalf("unexpected body: %+v", body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key", "app-1", nil)
	if err := c.DeleteDomain(context.Background(), "dom-123"); err != nil {
		t.Fatalf("DeleteDomain: %v", err)
	}
}
