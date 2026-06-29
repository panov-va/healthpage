// Package slack — канал подписки Slack (этап 3.9, DESIGN §4.4). Содержит OAuth-клиент «Add to
// Slack» (получение incoming-webhook URL канала), HTTP-клиент доставки в этот URL, рендер
// сообщений в формате Block Kit и воркер доставки из очереди q.slack. Подписка — через OAuth
// (эндпоинты /subscribe/slack/start|callback), а не через POST /subscribe.
package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	authorizeURL = "https://slack.com/oauth/v2/authorize"
	accessURL    = "https://slack.com/api/oauth.v2.access"
	// scope incoming-webhook — Slack возвращает webhook URL выбранного пользователем канала.
	oauthScope = "incoming-webhook"
)

// OAuth — клиент OAuth-флоу «Add to Slack».
type OAuth struct {
	clientID     string
	clientSecret string
	redirectURI  string
	http         *http.Client
	accessURL    string // переопределяется в тестах
}

// Option настраивает OAuth-клиент (тестовый seam).
type Option func(*OAuth)

// WithAccessURL переопределяет endpoint oauth.v2.access (для тестов).
func WithAccessURL(u string) Option { return func(o *OAuth) { o.accessURL = u } }

// NewOAuth собирает OAuth-клиент. httpClient=nil → клиент с таймаутом 10с.
func NewOAuth(clientID, clientSecret, redirectURI string, httpClient *http.Client, opts ...Option) *OAuth {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	o := &OAuth{
		clientID: clientID, clientSecret: clientSecret, redirectURI: redirectURI,
		http: httpClient, accessURL: accessURL,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// AuthorizeURL строит ссылку, на которую редиректится пользователь («Add to Slack»). state —
// подписанный CSRF-токен, привязанный к странице.
func (o *OAuth) AuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("client_id", o.clientID)
	q.Set("scope", oauthScope)
	q.Set("redirect_uri", o.redirectURI)
	q.Set("state", state)
	return authorizeURL + "?" + q.Encode()
}

// accessResponse — ответ oauth.v2.access (нужны ok/error + incoming_webhook.url + team).
type accessResponse struct {
	OK              bool   `json:"ok"`
	Error           string `json:"error"`
	IncomingWebhook struct {
		URL     string `json:"url"`
		Channel string `json:"channel"`
	} `json:"incoming_webhook"`
	Team struct {
		Name string `json:"name"`
	} `json:"team"`
}

// WebhookGrant — результат успешного обмена кода: webhook URL канала и его человекочитаемое имя.
type WebhookGrant struct {
	WebhookURL string
	Channel    string
	TeamName   string
}

// Exchange обменивает OAuth-код на incoming-webhook URL выбранного канала.
func (o *OAuth) Exchange(ctx context.Context, code string) (WebhookGrant, error) {
	form := url.Values{}
	form.Set("client_id", o.clientID)
	form.Set("client_secret", o.clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", o.redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.accessURL, strings.NewReader(form.Encode()))
	if err != nil {
		return WebhookGrant{}, fmt.Errorf("slack: build access request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := o.http.Do(req)
	if err != nil {
		return WebhookGrant{}, fmt.Errorf("slack: oauth access: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return WebhookGrant{}, fmt.Errorf("slack: read access response: %w", err)
	}
	var ar accessResponse
	if err := json.Unmarshal(raw, &ar); err != nil {
		return WebhookGrant{}, fmt.Errorf("slack: decode access response: %w", err)
	}
	if !ar.OK {
		return WebhookGrant{}, fmt.Errorf("slack: oauth access failed: %s", ar.Error)
	}
	if ar.IncomingWebhook.URL == "" {
		return WebhookGrant{}, fmt.Errorf("slack: access response has no incoming_webhook.url")
	}
	return WebhookGrant{
		WebhookURL: ar.IncomingWebhook.URL,
		Channel:    ar.IncomingWebhook.Channel,
		TeamName:   ar.Team.Name,
	}, nil
}
