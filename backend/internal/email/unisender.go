package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// uniSenderGoEndpoint — Web API UniSender Go (HTTPS:443, https://godocs.unisender.ru/web-api-ref).
// Хост зависит от дата-центра аккаунта (go1/go2/...) — единого для всех "goapi.unisender.ru" нет
// (найдено 2026-07-22: с ним API отвечал "user not found", код 114). У этого аккаунта дата-центр —
// go2 (виден в SMTP-хосте smtp.go2.unisender.ru); переопределяется UniSenderGoSender.Endpoint или
// конфигом UNISENDER_GO_API_URL, если аккаунт когда-нибудь переедет на другой дата-центр.
const uniSenderGoEndpoint = "https://go2.unisender.ru/ru/transactional/api/v1/email/send.json"

// UniSenderGoSender доставляет письма через HTTP Web API UniSender Go вместо SMTP — решение
// 2026-07-22, когда выяснилось, что у VPS-провайдера прод-сервера исходящие SMTP-порты (587/465)
// заблокированы на уровне сети (таймаут даже до smtp.gmail.com); HTTPS не блокируется.
// Используется ТОЛЬКО как системный (платформенный) отправитель — кастомный SMTP страницы (4.5)
// всегда идёт через настоящий SMTP-протокол (SMTPSender), см. Worker.customSender.
type UniSenderGoSender struct {
	APIKey   string
	Endpoint string // инъекция для тестов; пусто → uniSenderGoEndpoint
	HTTP     *http.Client
}

// NewUniSenderGoSender создаёт отправителя с дефолтным HTTP-клиентом (таймаут 15с). apiURL пустой
// → дефолт пакета (дата-центр go2, см. uniSenderGoEndpoint); передайте непустой, если аккаунт на
// другом дата-центре UniSender Go (UNISENDER_GO_API_URL).
func NewUniSenderGoSender(apiKey, apiURL string) *UniSenderGoSender {
	if apiURL == "" {
		apiURL = uniSenderGoEndpoint
	}
	return &UniSenderGoSender{APIKey: apiKey, Endpoint: apiURL, HTTP: &http.Client{Timeout: 15 * time.Second}}
}

func (s *UniSenderGoSender) endpoint() string {
	if s.Endpoint != "" {
		return s.Endpoint
	}
	return uniSenderGoEndpoint
}

type uniSenderGoRequest struct {
	Message uniSenderGoMessage `json:"message"`
}

type uniSenderGoMessage struct {
	Recipients []uniSenderGoRecipient `json:"recipients"`
	Body       uniSenderGoBody        `json:"body"`
	Subject    string                 `json:"subject"`
	FromEmail  string                 `json:"from_email"`
}

type uniSenderGoRecipient struct {
	Email string `json:"email"`
}

type uniSenderGoBody struct {
	HTML      string `json:"html"`
	PlainText string `json:"plaintext"`
}

type uniSenderGoResponse struct {
	Status       string            `json:"status"`
	Message      string            `json:"message"`
	Emails       []string          `json:"emails"`
	FailedEmails map[string]string `json:"failed_emails"`
}

// Send публикует письмо на одного получателя через email/send.json. cfg.From задаёт отправителя
// (тот же From, что и для системного SMTP — cfg берётся из effectiveSMTP, host/port/username/
// password в cfg этим отправителем игнорируются, аутентификация — по API-ключу в заголовке).
func (s *UniSenderGoSender) Send(ctx context.Context, cfg SMTP, msg Email) error {
	if s.APIKey == "" {
		return fmt.Errorf("email: UniSender Go API-ключ не задан")
	}
	body, err := json.Marshal(uniSenderGoRequest{Message: uniSenderGoMessage{
		Recipients: []uniSenderGoRecipient{{Email: msg.To}},
		Body:       uniSenderGoBody{HTML: msg.HTMLBody, PlainText: msg.TextBody},
		Subject:    msg.Subject,
		FromEmail:  cfg.From,
	}})
	if err != nil {
		return fmt.Errorf("email: unisender marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", s.APIKey)

	resp, err := s.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("email: unisender http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var out uniSenderGoResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("email: unisender decode (http %d): %w", resp.StatusCode, err)
	}
	if out.Status != "success" {
		return fmt.Errorf("email: unisender error: %s", out.Message)
	}
	if reason, rejected := out.FailedEmails[msg.To]; rejected {
		return fmt.Errorf("email: unisender rejected %s: %s", msg.To, reason)
	}
	return nil
}
