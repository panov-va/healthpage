// Package dokploy — клиент API self-hosted Dokploy для подключения кастомных доменов клиентов
// (этап 4.3, прод-интеграция вместо cmd/edge+cmd/tls-manager — см. DEPLOY.md). Домен подключается
// как Domain приложения public-ssr в Dokploy: дальше Traefik-роутинг и выпуск Let's Encrypt
// обслуживает сам Dokploy, свой ACME/edge не нужен.
package dokploy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client вызывает Dokploy API (аутентификация — заголовок x-api-key) для управления Domain
// приложения public-ssr.
type Client struct {
	http          *http.Client
	baseURL       string // например http://<host>:3000/api
	apiKey        string
	applicationID string // ID приложения public-ssr в Dokploy
}

// NewClient собирает клиент. httpClient=nil → клиент с таймаутом 15с.
func NewClient(baseURL, apiKey, applicationID string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{
		http:          httpClient,
		baseURL:       strings.TrimSuffix(baseURL, "/"),
		apiKey:        apiKey,
		applicationID: applicationID,
	}
}

// Error — ошибка вызова Dokploy API: код HTTP + тело ответа (для сообщения оператору/логов).
type Error struct {
	Op   string
	Code int
	Body string
}

func (e *Error) Error() string {
	return fmt.Sprintf("dokploy: %s: код %d: %s", e.Op, e.Code, e.Body)
}

type createDomainRequest struct {
	Host            string `json:"host"`
	Path            string `json:"path"`
	Port            int    `json:"port"`
	HTTPS           bool   `json:"https"`
	ApplicationID   string `json:"applicationId"`
	CertificateType string `json:"certificateType"`
	DomainType      string `json:"domainType"`
}

// domainRecord — форма записи Domain в ответах Dokploy. ID-поле дублируется под двумя
// возможными именами: точная схема ответа tRPC-обёртки Dokploy не задокументирована (OpenAPI
// отдаёт её как {} — см. DEPLOY.md), поэтому CreateDomain не полагается на одно конкретное имя.
type domainRecord struct {
	DomainID string `json:"domainId"`
	ID       string `json:"id"`
	Host     string `json:"host"`
}

func (r domainRecord) recordID() string {
	if r.DomainID != "" {
		return r.DomainID
	}
	return r.ID
}

// CreateDomain подключает host как Domain приложения public-ssr (путь "/", порт 3000, HTTPS,
// certificateType=letsencrypt) — дальше Traefik/Let's Encrypt обслуживает его сам Dokploy.
// Возвращает ID записи Domain в Dokploy (для последующего DeleteDomain при смене/снятии домена).
//
// Если тело ответа create не содержит ID напрямую, догоняет его через domain.byApplicationId,
// сопоставляя по host — подстраховка от нестабильности схемы ответа (см. domainRecord).
func (c *Client) CreateDomain(ctx context.Context, host string) (string, error) {
	body, err := json.Marshal(createDomainRequest{
		Host:            host,
		Path:            "/",
		Port:            3000,
		HTTPS:           true,
		ApplicationID:   c.applicationID,
		CertificateType: "letsencrypt",
		DomainType:      "application",
	})
	if err != nil {
		return "", fmt.Errorf("dokploy: marshal create request: %w", err)
	}

	var rec domainRecord
	if err := c.do(ctx, http.MethodPost, "/domain.create", body, &rec); err != nil {
		return "", err
	}
	if id := rec.recordID(); id != "" {
		return id, nil
	}

	id, err := c.findDomainIDByHost(ctx, host)
	if err != nil {
		return "", fmt.Errorf("dokploy: домен создан, но не удалось получить его id: %w", err)
	}
	return id, nil
}

// DeleteDomain отвязывает домен от приложения (Traefik перестаёт его обслуживать).
// domainID="" — нечего удалять, запрос не выполняется.
func (c *Client) DeleteDomain(ctx context.Context, domainID string) error {
	if domainID == "" {
		return nil
	}
	body, err := json.Marshal(map[string]string{"domainId": domainID})
	if err != nil {
		return fmt.Errorf("dokploy: marshal delete request: %w", err)
	}
	return c.do(ctx, http.MethodPost, "/domain.delete", body, nil)
}

func (c *Client) findDomainIDByHost(ctx context.Context, host string) (string, error) {
	var records []domainRecord
	path := "/domain.byApplicationId?applicationId=" + url.QueryEscape(c.applicationID)
	if err := c.do(ctx, http.MethodGet, path, nil, &records); err != nil {
		return "", err
	}
	for _, r := range records {
		if strings.EqualFold(r.Host, host) {
			if id := r.recordID(); id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("домен %q не найден среди доменов приложения", host)
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, out any) error {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("dokploy: build request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("dokploy: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode >= 300 {
		return &Error{Op: path, Code: resp.StatusCode, Body: string(respBody)}
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("dokploy: decode response %s: %w", path, err)
	}
	return nil
}
