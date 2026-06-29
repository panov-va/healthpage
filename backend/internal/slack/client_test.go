package slack

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPostMessageOK(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	err := NewClient(srv.Client()).PostMessage(context.Background(), srv.URL, []byte(`{"x":1}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if gotBody != `{"x":1}` {
		t.Errorf("тело не дошло: %q", gotBody)
	}
}

func TestPostMessagePermanent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("no_service"))
	}))
	defer srv.Close()

	err := NewClient(srv.Client()).PostMessage(context.Background(), srv.URL, []byte(`{}`))
	var perr *PostError
	if !errors.As(err, &perr) {
		t.Fatalf("ожидался *PostError, got %T", err)
	}
	if !perr.Permanent || perr.Code != 404 {
		t.Errorf("404 должен быть Permanent: %+v", perr)
	}
}

func TestPostMessageRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "12")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("rate_limited"))
	}))
	defer srv.Close()

	err := NewClient(srv.Client()).PostMessage(context.Background(), srv.URL, []byte(`{}`))
	var perr *PostError
	if !errors.As(err, &perr) {
		t.Fatalf("ожидался *PostError, got %T", err)
	}
	if perr.Permanent {
		t.Error("429 не перманентна")
	}
	if perr.RetryAfter.Seconds() != 12 {
		t.Errorf("RetryAfter = %v, want 12s", perr.RetryAfter)
	}
}

func TestPostMessageTransient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	err := NewClient(srv.Client()).PostMessage(context.Background(), srv.URL, []byte(`{}`))
	var perr *PostError
	if !errors.As(err, &perr) {
		t.Fatalf("ожидался *PostError, got %T", err)
	}
	if perr.Permanent {
		t.Error("5xx должна быть транзиентной (не Permanent)")
	}
}
