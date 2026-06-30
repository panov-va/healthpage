package edge

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/healthpage/backend/internal/store"
)

type fakeResolver struct {
	challenges map[string]string
	slugs      map[string]string
}

func (f fakeResolver) DomainCertificate(context.Context, string) (store.DomainCertificate, error) {
	return store.DomainCertificate{}, store.ErrNotFound
}
func (f fakeResolver) ACMEChallenge(_ context.Context, token string) (string, error) {
	v, ok := f.challenges[token]
	if !ok {
		return "", store.ErrNotFound
	}
	return v, nil
}
func (f fakeResolver) SlugByCustomDomain(_ context.Context, domain string) (string, error) {
	v, ok := f.slugs[domain]
	if !ok {
		return "", store.ErrNotFound
	}
	return v, nil
}

func TestIsAPIPath(t *testing.T) {
	cases := map[string]bool{
		"/api":          true,
		"/api/v1/pages": true,
		"/apixyz":       false,
		"/status/x":     false,
		"/":             false,
	}
	for path, want := range cases {
		if got := isAPIPath(path); got != want {
			t.Errorf("isAPIPath(%q)=%v, want %v", path, got, want)
		}
	}
}

func TestHTTPHandlerChallenge(t *testing.T) {
	srv, _ := New(fakeResolver{challenges: map[string]string{"tok123": "keyauth-value"}}, "http://api", "http://ssr")
	h := srv.HTTPHandler()

	// challenge → отдаётся keyAuth
	req := httptest.NewRequest(http.MethodGet, "http://acme.test/.well-known/acme-challenge/tok123", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("challenge status: %d", rec.Code)
	}
	if body, _ := io.ReadAll(rec.Body); string(body) != "keyauth-value" {
		t.Fatalf("challenge body: %q", body)
	}

	// неизвестный токен → 404
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://acme.test/.well-known/acme-challenge/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown challenge: %d", rec.Code)
	}

	// прочее → редирект на https
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://acme.test/incidents", nil))
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("redirect status: %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://acme.test/incidents" {
		t.Fatalf("redirect location: %q", loc)
	}
}

func TestHostOnly(t *testing.T) {
	if hostOnly("acme.test:443") != "acme.test" {
		t.Error("порт не отброшен")
	}
	if hostOnly("acme.test") != "acme.test" {
		t.Error("хост без порта изменён")
	}
}
