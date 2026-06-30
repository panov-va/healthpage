// Package webhookout — доставка исходящих webhook'ов (этап 5.4, DESIGN §4, §8.1): POST JSON в
// произвольный URL (Mattermost / любой endpoint). Потребляется worker-webhook из q.webhook.out.
// Симметрично internal/slack (тот же транспорт — HTTP POST), но payload — generic + text.
package webhookout

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Client отправляет payload в произвольный webhook URL.
type Client struct {
	http *http.Client
}

// NewClient собирает HTTP-клиент доставки. httpClient=nil → клиент с таймаутом 10с.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{http: httpClient}
}

// PostError — ошибка доставки. Permanent=true → повтор бесполезен (невалидный URL/payload, 4xx).
// 429 → RetryAfter.
type PostError struct {
	Code       int
	Body       string
	Permanent  bool
	RetryAfter time.Duration
}

func (e *PostError) Error() string {
	return fmt.Sprintf("webhookout: post failed: code=%d body=%q", e.Code, e.Body)
}

// Post отправляет JSON-payload в webhook URL. Классификация: 2xx — успех; 429 — троттлинг
// (RetryAfter); 5xx/сеть — транзиент; прочие 4xx — Permanent.
func (c *Client) Post(ctx context.Context, webhookURL string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		// Невалидный URL — повтор бесполезен.
		return &PostError{Body: err.Error(), Permanent: true}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		// Сетевая ошибка — транзиентная.
		return &PostError{Body: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<12))
	perr := &PostError{Code: resp.StatusCode, Body: string(body)}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		if v := resp.Header.Get("Retry-After"); v != "" {
			if secs, err := strconv.Atoi(v); err == nil {
				perr.RetryAfter = time.Duration(secs) * time.Second
			}
		}
	case resp.StatusCode >= 500:
		// серверная ошибка получателя — транзиентная
	default:
		perr.Permanent = true // 4xx (кроме 429)
	}
	return perr
}
