package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestUniSenderGo(t *testing.T, handler http.HandlerFunc) *UniSenderGoSender {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &UniSenderGoSender{APIKey: "test-key", Endpoint: srv.URL, HTTP: srv.Client()}
}

func TestUniSenderGoSenderHappyPath(t *testing.T) {
	var gotBody uniSenderGoRequest
	var gotAPIKey, gotContentType string
	s := newTestUniSenderGo(t, func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-API-KEY")
		gotContentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(uniSenderGoResponse{Status: "success", Emails: []string{"to@x.test"}})
	})

	err := s.Send(context.Background(), SMTP{From: "status@healthpage.ru"}, Email{
		To: "to@x.test", Subject: "Subj", TextBody: "text", HTMLBody: "<p>html</p>",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotAPIKey != "test-key" {
		t.Fatalf("X-API-KEY = %q", gotAPIKey)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q", gotContentType)
	}
	if len(gotBody.Message.Recipients) != 1 || gotBody.Message.Recipients[0].Email != "to@x.test" {
		t.Fatalf("recipients = %+v", gotBody.Message.Recipients)
	}
	if gotBody.Message.FromEmail != "status@healthpage.ru" {
		t.Fatalf("from_email = %q", gotBody.Message.FromEmail)
	}
	// track_links/track_read должны быть явно выключены (0) — API включает их по умолчанию (1),
	// а это требует настроенного tracking-домена в аккаунте, которого может не быть (найдено
	// 2026-07-22: без этого сервер отвечает "Custom backend domain or tracking domain required").
	if gotBody.Message.TrackLinks != 0 || gotBody.Message.TrackRead != 0 {
		t.Fatalf("track_links/track_read должны быть 0, получили %+v", gotBody.Message)
	}
	if gotBody.Message.Subject != "Subj" || gotBody.Message.Body.HTML != "<p>html</p>" || gotBody.Message.Body.PlainText != "text" {
		t.Fatalf("message body = %+v", gotBody.Message)
	}
}

func TestUniSenderGoSenderAPIError(t *testing.T) {
	s := newTestUniSenderGo(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(uniSenderGoResponse{Status: "error", Message: "invalid api key"})
	})

	err := s.Send(context.Background(), SMTP{From: "a@x.test"}, Email{To: "to@x.test"})
	if err == nil {
		t.Fatal("want error on status=error")
	}
}

func TestUniSenderGoSenderRejectedRecipient(t *testing.T) {
	s := newTestUniSenderGo(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(uniSenderGoResponse{
			Status:       "success",
			FailedEmails: map[string]string{"to@x.test": "invalid"},
		})
	})

	err := s.Send(context.Background(), SMTP{From: "a@x.test"}, Email{To: "to@x.test"})
	if err == nil {
		t.Fatal("want error when recipient is in failed_emails")
	}
}

func TestUniSenderGoSenderNoAPIKey(t *testing.T) {
	s := &UniSenderGoSender{}
	if err := s.Send(context.Background(), SMTP{}, Email{To: "to@x.test"}); err == nil {
		t.Fatal("want error when APIKey is empty")
	}
}
