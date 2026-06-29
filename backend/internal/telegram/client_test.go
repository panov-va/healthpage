package telegram

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient направляет клиента на тестовый сервер.
func newTestClient(srv *httptest.Server) *Client {
	c := NewClient("TESTTOKEN", srv.Client())
	c.baseURL = srv.URL
	return c
}

func TestSendMessageOK(t *testing.T) {
	var gotPath string
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer srv.Close()

	if err := newTestClient(srv).SendMessage(context.Background(), 42, "hi"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotPath != "/botTESTTOKEN/sendMessage" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(gotBody, `"chat_id":"42"`) || !strings.Contains(gotBody, `"parse_mode":"HTML"`) {
		t.Errorf("body = %s", gotBody)
	}
}

func TestSendMessagePermanentError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`))
	}))
	defer srv.Close()

	err := newTestClient(srv).SendMessage(context.Background(), 42, "hi")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("ожидался *APIError, got %T", err)
	}
	if !apiErr.Permanent || apiErr.Code != 403 {
		t.Errorf("403 должен быть Permanent: %+v", apiErr)
	}
}

func TestSendMessageRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":7}}`))
	}))
	defer srv.Close()

	err := newTestClient(srv).SendMessage(context.Background(), 42, "hi")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("ожидался *APIError, got %T", err)
	}
	if apiErr.Permanent {
		t.Error("429 не перманентна")
	}
	if apiErr.RetryAfter.Seconds() != 7 {
		t.Errorf("RetryAfter = %v, want 7s", apiErr.RetryAfter)
	}
}

func TestGetUpdatesParses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":[
			{"update_id":10,"message":{"message_id":1,"chat":{"id":5,"type":"private"},"text":"/start acme","from":{"id":5,"language_code":"ru"}}}
		]}`))
	}))
	defer srv.Close()

	ups, err := newTestClient(srv).GetUpdates(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("getUpdates: %v", err)
	}
	if len(ups) != 1 || ups[0].UpdateID != 10 || ups[0].Message.Text != "/start acme" {
		t.Fatalf("распарсилось неверно: %+v", ups)
	}
	if ups[0].Message.From.LanguageCode != "ru" {
		t.Errorf("language_code не распарсился: %+v", ups[0].Message.From)
	}
}
