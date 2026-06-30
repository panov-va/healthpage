package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
)

// ── DTO страниц (синхронны с openapi StatusPage / *Create / *Update) ──

type statusPageResponse struct {
	ID             string          `json:"id"`
	AccountID      string          `json:"account_id"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Slug           string          `json:"slug"`
	Timezone       string          `json:"timezone"`
	DefaultLocale  string          `json:"default_locale"`
	Visibility     string          `json:"visibility"`
	CustomDomain   *string         `json:"custom_domain"`
	DomainVerified bool            `json:"domain_verified"`
	Theme          json.RawMessage `json:"theme"`
	LogoURL        *string         `json:"logo_url"`
	FaviconURL     *string         `json:"favicon_url"`
	HidePoweredBy  bool            `json:"hide_powered_by"`
	RedirectURL    *string         `json:"redirect_url"`
	FromEmail      *string         `json:"from_email"`
	SMTPConfigured bool            `json:"smtp_configured"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
}

func toStatusPageResponse(p domain.StatusPage) statusPageResponse {
	theme := p.Theme
	if len(theme) == 0 {
		theme = []byte("{}")
	}
	return statusPageResponse{
		ID: p.ID.String(), AccountID: p.AccountID.String(), Name: p.Name, Description: p.Description,
		Slug: p.Slug, Timezone: p.Timezone, DefaultLocale: p.DefaultLocale, Visibility: string(p.Visibility),
		CustomDomain: p.CustomDomain, DomainVerified: p.DomainVerified, Theme: json.RawMessage(theme),
		LogoURL: p.LogoURL, FaviconURL: p.FaviconURL, HidePoweredBy: p.HidePoweredBy, RedirectURL: p.RedirectURL,
		FromEmail: p.FromEmail, SMTPConfigured: len(p.SMTPConfig) > 0 && string(p.SMTPConfig) != "null",
		CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: p.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

type createPageRequest struct {
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Description   string `json:"description"`
	Timezone      string `json:"timezone"`
	DefaultLocale string `json:"default_locale"`
	Visibility    string `json:"visibility"`
}

type patchPageRequest struct {
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	Timezone      *string `json:"timezone"`
	DefaultLocale *string `json:"default_locale"`
	Visibility    *string `json:"visibility"`
	// Пароль приватной страницы (этап 4.2). Отсутствует → не трогаем; null или "" → снять;
	// непустая строка → задать/сменить. RawMessage, чтобы отличить отсутствие от null.
	Password json.RawMessage `json:"password"`
	// Собственный домен (этап 4.3). Та же семантика RawMessage: отсутствие → не трогаем;
	// null/"" → снять; строка → задать (сбрасывает domain_verified).
	CustomDomain  json.RawMessage `json:"custom_domain"`
	Theme         json.RawMessage `json:"theme"`
	LogoURL       *string         `json:"logo_url"`
	FaviconURL    *string         `json:"favicon_url"`
	HidePoweredBy *bool           `json:"hide_powered_by"`
	RedirectURL   *string         `json:"redirect_url"`
	// Кастомный SMTP (этап 4.5). RawMessage: отсутствие → не трогаем; null → снять; объект → задать.
	SMTPConfig json.RawMessage `json:"smtp_config"`
	FromEmail  json.RawMessage `json:"from_email"`
}

func (s *server) handleListPages(w http.ResponseWriter, r *http.Request) {
	user, ok := requireOperator(w, r)
	if !ok {
		return
	}
	acc, err := s.store.AccountByOwner(r.Context(), user.ID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	pages, err := s.store.ListStatusPagesByAccount(r.Context(), acc.ID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	out := make([]statusPageResponse, len(pages))
	for i, p := range pages {
		out[i] = toStatusPageResponse(p)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleCreatePage(w http.ResponseWriter, r *http.Request) {
	var req createPageRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" || req.Slug == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "name и slug обязательны")
		return
	}
	visibility := req.Visibility
	if visibility == "" {
		visibility = string(domain.VisibilityPublic)
	}
	if !domain.Visibility(visibility).IsValid() {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый visibility")
		return
	}

	user, ok := requireOperator(w, r)
	if !ok {
		return
	}
	acc, err := s.store.AccountByOwner(r.Context(), user.ID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	locale := req.DefaultLocale
	if locale == "" {
		locale = "ru"
	}
	timezone := req.Timezone
	if timezone == "" {
		timezone = "UTC"
	}
	page, err := s.store.CreateStatusPage(r.Context(), acc.ID, user.ID, req.Name, req.Description, req.Slug, timezone, locale, visibility)
	if err != nil {
		if errors.Is(err, store.ErrSlugTaken) {
			writeError(w, http.StatusConflict, "slug_taken", "slug уже занят")
			return
		}
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toStatusPageResponse(page))
}

func (s *server) handleGetPage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "page")
	if !ok {
		return
	}
	page, ok := s.authorizePage(w, r, id)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toStatusPageResponse(page))
}

func (s *server) handlePatchPage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "page")
	if !ok {
		return
	}
	page, ok := s.authorizePage(w, r, id)
	if !ok {
		return
	}
	var req patchPageRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name != nil {
		page.Name = *req.Name
	}
	if req.Description != nil {
		page.Description = *req.Description
	}
	if req.Timezone != nil {
		page.Timezone = *req.Timezone
	}
	if req.DefaultLocale != nil {
		page.DefaultLocale = *req.DefaultLocale
	}
	if req.Visibility != nil {
		if !domain.Visibility(*req.Visibility).IsValid() {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый visibility")
			return
		}
		page.Visibility = domain.Visibility(*req.Visibility)
	}
	if req.Theme != nil {
		page.Theme = req.Theme
	}
	if req.LogoURL != nil {
		page.LogoURL = req.LogoURL
	}
	if req.FaviconURL != nil {
		page.FaviconURL = req.FaviconURL
	}
	if req.HidePoweredBy != nil {
		page.HidePoweredBy = *req.HidePoweredBy
	}
	if req.RedirectURL != nil {
		page.RedirectURL = req.RedirectURL
	}

	updated, err := s.store.UpdateStatusPage(r.Context(), page)
	if err != nil {
		writeServerError(w, err)
		return
	}

	// Пароль приватной страницы (этап 4.2) — отдельная колонка (хранится хэш, §9), не в Update.
	if req.Password != nil {
		hash, ok := hashPagePassword(w, req.Password)
		if !ok {
			return
		}
		if err := s.store.SetStatusPagePassword(r.Context(), updated.ID, hash); err != nil {
			writeServerError(w, err)
			return
		}
	}

	// Собственный домен (этап 4.3) — отдельный запрос (сбрасывает domain_verified, уникален).
	if req.CustomDomain != nil {
		domain, ok := parseCustomDomain(w, req.CustomDomain)
		if !ok {
			return
		}
		if err := s.store.SetCustomDomain(r.Context(), updated.ID, domain); err != nil {
			if errors.Is(err, store.ErrDomainTaken) {
				writeError(w, http.StatusConflict, "domain_taken", "домен уже привязан к другой странице")
				return
			}
			writeServerError(w, err)
			return
		}
		// Перечитываем, чтобы ответ отразил новый домен и сброшенный domain_verified.
		if refreshed, err := s.store.StatusPageByID(r.Context(), updated.ID); err == nil {
			updated = refreshed
		}
	}

	// Кастомный SMTP / from_email (этап 4.5) — отдельные колонки; обновляются вместе.
	if req.SMTPConfig != nil || req.FromEmail != nil {
		smtpCfg := updated.SMTPConfig
		fromEmail := updated.FromEmail
		if req.SMTPConfig != nil {
			if string(req.SMTPConfig) == "null" {
				smtpCfg = nil
			} else {
				var obj map[string]any
				if err := json.Unmarshal(req.SMTPConfig, &obj); err != nil {
					writeError(w, http.StatusUnprocessableEntity, "invalid_request", "smtp_config должен быть объектом или null")
					return
				}
				smtpCfg = []byte(req.SMTPConfig)
			}
		}
		if req.FromEmail != nil {
			from, ok := parseNullableString(w, req.FromEmail, "from_email")
			if !ok {
				return
			}
			fromEmail = from
		}
		if err := s.store.SetStatusPageSMTP(r.Context(), updated.ID, smtpCfg, fromEmail); err != nil {
			writeServerError(w, err)
			return
		}
		if refreshed, err := s.store.StatusPageByID(r.Context(), updated.ID); err == nil {
			updated = refreshed
		}
	}

	writeJSON(w, http.StatusOK, toStatusPageResponse(updated))
}

// parseNullableString разбирает JSON-поле: null/"" → nil; непустая строка (trim) → &s. 422 при ошибке.
func parseNullableString(w http.ResponseWriter, raw json.RawMessage, field string) (*string, bool) {
	if string(raw) == "null" {
		return nil, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", field+" должен быть строкой или null")
		return nil, false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, true
	}
	return &s, true
}

// parseCustomDomain разбирает поле custom_domain из PATCH: JSON null или пустая строка → снять
// домен (nil); непустая строка → нормализованный (lower, trim) домен. При ошибке пишет 422.
func parseCustomDomain(w http.ResponseWriter, raw json.RawMessage) (*string, bool) {
	if string(raw) == "null" {
		return nil, true
	}
	var d string
	if err := json.Unmarshal(raw, &d); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "custom_domain должен быть строкой или null")
		return nil, false
	}
	d = strings.ToLower(strings.TrimSpace(d))
	if d == "" {
		return nil, true
	}
	return &d, true
}

// hashPagePassword разбирает поле password из PATCH: JSON null или пустая строка → снять
// пароль (nil); непустая строка → argon2id-хэш. Возвращает (хэш|nil, ok); при ошибке пишет 422.
func hashPagePassword(w http.ResponseWriter, raw json.RawMessage) (*string, bool) {
	if string(raw) == "null" {
		return nil, true
	}
	var pw string
	if err := json.Unmarshal(raw, &pw); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "password должен быть строкой или null")
		return nil, false
	}
	if pw == "" {
		return nil, true
	}
	hash, err := security.HashPassword(pw)
	if err != nil {
		writeServerError(w, err)
		return nil, false
	}
	return &hash, true
}

func (s *server) handleDeletePage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "page")
	if !ok {
		return
	}
	if _, ok := s.authorizePage(w, r, id); !ok {
		return
	}
	if err := s.store.SoftDeleteStatusPage(r.Context(), id); err != nil {
		writeServerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
