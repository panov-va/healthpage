// Package telegram — канал доставки уведомлений через Telegram (этап 3.7). Содержит минимальный
// клиент Bot API (sendMessage/getUpdates), воркер доставки сообщений из очереди q.telegram
// (идемпотентность по Notification.id, ретраи через notify.Engine) и бота управления подпиской
// (long polling getUpdates: команда /start <slug> создаёт Subscriber{channel=telegram},
// /stop отписывает). Подписка идёт через бота (DESIGN §3.4), а не через POST /subscribe.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const defaultBaseURL = "https://api.telegram.org"

// Client — минимальный клиент Telegram Bot API (только нужные методы).
type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

// NewClient создаёт клиента бота. http=nil → клиент с разумным таймаутом.
// Таймаут должен превышать таймаут long polling getUpdates — задаём с запасом.
func NewClient(token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 65 * time.Second}
	}
	return &Client{token: token, baseURL: defaultBaseURL, http: httpClient}
}

// apiResponse — обёртка ответа Bot API (ok + result | error_code/description).
type apiResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	ErrorCode   int             `json:"error_code"`
	Description string          `json:"description"`
	Parameters  *struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters,omitempty"`
}

// APIError — ошибка вызова Bot API. Permanent=true означает, что повтор бесполезен
// (например, бот заблокирован пользователем 403 или чат не найден 400).
type APIError struct {
	Method     string
	Code       int
	Desc       string
	Permanent  bool
	RetryAfter time.Duration // для 429 — рекомендованная пауза
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram: %s failed: code=%d desc=%q", e.Method, e.Code, e.Desc)
}

// call выполняет метод Bot API с JSON-телом и декодирует result в out (если не nil).
func (c *Client) call(ctx context.Context, method string, params any, out any) error {
	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("telegram: marshal %s params: %w", method, err)
	}
	url := c.baseURL + "/bot" + c.token + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: build %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		// Сетевая ошибка — транзиентная (Permanent=false), повтор имеет смысл.
		return &APIError{Method: method, Desc: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return &APIError{Method: method, Code: resp.StatusCode, Desc: err.Error()}
	}
	var ar apiResponse
	if err := json.Unmarshal(raw, &ar); err != nil {
		return &APIError{Method: method, Code: resp.StatusCode, Desc: "bad response: " + string(raw)}
	}
	if !ar.OK {
		apiErr := &APIError{Method: method, Code: ar.ErrorCode, Desc: ar.Description}
		switch {
		case ar.ErrorCode == http.StatusTooManyRequests:
			if ar.Parameters != nil {
				apiErr.RetryAfter = time.Duration(ar.Parameters.RetryAfter) * time.Second
			}
		case ar.ErrorCode >= 400 && ar.ErrorCode < 500:
			// 400 (chat not found / bad request), 403 (бот заблокирован) и пр. — повтор бесполезен.
			apiErr.Permanent = true
		}
		return apiErr
	}
	if out != nil {
		if err := json.Unmarshal(ar.Result, out); err != nil {
			return fmt.Errorf("telegram: decode %s result: %w", method, err)
		}
	}
	return nil
}

// User — описание бота/пользователя (getMe и поле from входящих сообщений).
type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"` // у отправителя сообщения: "ru", "en", "en-US", ...
}

// GetMe возвращает данные бота (проверка токена + получение username при старте).
func (c *Client) GetMe(ctx context.Context) (User, error) {
	var u User
	if err := c.call(ctx, "getMe", struct{}{}, &u); err != nil {
		return User{}, err
	}
	return u, nil
}

// SendMessage отправляет текстовое сообщение в чат с parse_mode=HTML (без превью ссылок).
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	return c.call(ctx, "sendMessage", map[string]any{
		"chat_id":                  strconv.FormatInt(chatID, 10),
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}, nil)
}

// ── getUpdates (long polling, для бота управления подпиской) ──

// Update — обновление от Telegram (нужны только id и сообщение).
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

// Message — входящее сообщение (только нужные поля).
type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

// Chat — чат сообщения.
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// GetUpdates долго опрашивает обновления начиная с offset (long polling timeout секунд).
// Запрашиваются только сообщения (allowed_updates=["message"]).
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeout int) ([]Update, error) {
	var updates []Update
	err := c.call(ctx, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         timeout,
		"allowed_updates": []string{"message"},
	}, &updates)
	if err != nil {
		return nil, err
	}
	return updates, nil
}
