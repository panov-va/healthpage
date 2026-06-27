package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/healthpage/backend/internal/domain"
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
	Name          *string         `json:"name"`
	Description   *string         `json:"description"`
	Timezone      *string         `json:"timezone"`
	DefaultLocale *string         `json:"default_locale"`
	Visibility    *string         `json:"visibility"`
	Theme         json.RawMessage `json:"theme"`
	LogoURL       *string         `json:"logo_url"`
	FaviconURL    *string         `json:"favicon_url"`
	HidePoweredBy *bool           `json:"hide_powered_by"`
	RedirectURL   *string         `json:"redirect_url"`
}

func (s *server) handleListPages(w http.ResponseWriter, r *http.Request) {
	user, _ := userFromContext(r.Context())
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

	user, _ := userFromContext(r.Context())
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
	writeJSON(w, http.StatusOK, toStatusPageResponse(updated))
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
