package slack

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Client отправляет сообщения в incoming-webhook URL канала.
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

// PostError — ошибка доставки в webhook. Permanent=true означает, что повтор бесполезен
// (канал удалён/архивирован, webhook отозван, невалидный payload). 429 → RetryAfter.
type PostError struct {
	Code       int
	Body       string
	Permanent  bool
	RetryAfter time.Duration
}

func (e *PostError) Error() string {
	return fmt.Sprintf("slack: post failed: code=%d body=%q", e.Code, e.Body)
}

// PostMessage отправляет JSON-payload (Block Kit) в webhook URL. Классифицирует ответ:
// 2xx — успех; 429 — троттлинг (RetryAfter); 5xx/сеть — транзиент; прочие 4xx — Permanent.
func (c *Client) PostMessage(ctx context.Context, webhookURL string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("slack: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		// Сетевая ошибка — транзиентная (Permanent=false).
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
		// серверная ошибка Slack — транзиентная
	default:
		// 4xx (кроме 429): отозванный/удалённый webhook, невалидный payload — повтор бесполезен.
		perr.Permanent = true
	}
	return perr
}
