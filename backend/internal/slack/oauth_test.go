package slack

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAuthorizeURL(t *testing.T) {
	o := NewOAuth("cid", "secret", "https://h/api/v1/subscribe/slack/callback", nil)
	raw := o.AuthorizeURL("st4te")
	if !strings.HasPrefix(raw, authorizeURL+"?") {
		t.Fatalf("неверный префикс: %s", raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := u.Query()
	if q.Get("client_id") != "cid" || q.Get("scope") != "incoming-webhook" ||
		q.Get("state") != "st4te" || q.Get("redirect_uri") == "" {
		t.Errorf("неверные параметры: %v", q)
	}
}

func TestExchangeOK(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.PostForm
		_, _ = w.Write([]byte(`{"ok":true,"incoming_webhook":{"url":"https://hooks.slack.com/services/X/Y/Z","channel":"#alerts"},"team":{"name":"Acme"}}`))
	}))
	defer srv.Close()

	o := NewOAuth("cid", "secret", "https://h/cb", srv.Client(), WithAccessURL(srv.URL))
	grant, err := o.Exchange(context.Background(), "the-code")
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if grant.WebhookURL != "https://hooks.slack.com/services/X/Y/Z" || grant.Channel != "#alerts" || grant.TeamName != "Acme" {
		t.Errorf("неверный grant: %+v", grant)
	}
	if gotForm.Get("code") != "the-code" || gotForm.Get("client_id") != "cid" || gotForm.Get("client_secret") != "secret" {
		t.Errorf("неверная форма обмена: %v", gotForm)
	}
}

func TestExchangeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"error":"invalid_code"}`))
	}))
	defer srv.Close()

	o := NewOAuth("cid", "secret", "https://h/cb", srv.Client(), WithAccessURL(srv.URL))
	if _, err := o.Exchange(context.Background(), "bad"); err == nil {
		t.Fatal("ожидалась ошибка при ok:false")
	}
}

func TestExchangeNoWebhook(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"incoming_webhook":{"url":""}}`))
	}))
	defer srv.Close()

	o := NewOAuth("cid", "secret", "https://h/cb", srv.Client(), WithAccessURL(srv.URL))
	if _, err := o.Exchange(context.Background(), "code"); err == nil {
		t.Fatal("ожидалась ошибка при пустом webhook url")
	}
}
