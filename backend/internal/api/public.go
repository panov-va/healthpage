package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
)

// ── Публичные DTO сводки ──

type publicGroupResponse struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	Position         int                 `json:"position"`
	AggregatedStatus string              `json:"aggregated_status"`
	Components       []componentResponse `json:"components"`
}

// publicPageResponse — публично-безопасное подмножество страницы (openapi PublicPage):
// брендинг/тема/часовой пояс, без приватных полей (account_id, password_hash, smtp_config,
// custom_domain, redirect_url и т.п.).
type publicPageResponse struct {
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Slug          string          `json:"slug"`
	Timezone      string          `json:"timezone"`
	DefaultLocale string          `json:"default_locale"`
	Theme         json.RawMessage `json:"theme"`
	LogoURL       *string         `json:"logo_url"`
	FaviconURL    *string         `json:"favicon_url"`
	HidePoweredBy bool            `json:"hide_powered_by"`
}

func toPublicPageResponse(p domain.StatusPage) publicPageResponse {
	theme := p.Theme
	if len(theme) == 0 {
		theme = []byte("{}")
	}
	return publicPageResponse{
		Name:          p.Name,
		Description:   p.Description,
		Slug:          p.Slug,
		Timezone:      p.Timezone,
		DefaultLocale: p.DefaultLocale,
		Theme:         json.RawMessage(theme),
		LogoURL:       p.LogoURL,
		FaviconURL:    p.FaviconURL,
		HidePoweredBy: p.HidePoweredBy,
	}
}

type pageSummaryResponse struct {
	OverallStatus       string                `json:"overall_status"`
	Page                publicPageResponse    `json:"page"`
	UpdatedAt           string                `json:"updated_at"`
	Groups              []publicGroupResponse `json:"groups"`
	UngroupedComponents []componentResponse   `json:"ungrouped_components"`
	ActiveIncidents     []incidentResponse    `json:"active_incidents"`
	ActiveMaintenances  []maintenanceResponse `json:"active_maintenances"`
}

// loadPublicPage загружает публичную страницу по slug. Приватные страницы недоступны анонимно
// (доступ по паролю/email — этап 4), поэтому скрываются как 404. Возвращает false, если ответ уже записан.
func (s *server) loadPublicPage(w http.ResponseWriter, r *http.Request) (domain.StatusPage, bool) {
	slug := chi.URLParam(r, "page")
	page, err := s.store.StatusPageBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "страница не найдена")
		} else {
			writeServerError(w, err)
		}
		return domain.StatusPage{}, false
	}
	if page.IsPrivate() {
		writeError(w, http.StatusNotFound, "not_found", "страница не найдена")
		return domain.StatusPage{}, false
	}
	return page, true
}

func (s *server) handlePublicSummary(w http.ResponseWriter, r *http.Request) {
	page, ok := s.loadPublicPage(w, r)
	if !ok {
		return
	}
	groups, err := s.store.ListComponentGroupsByPage(r.Context(), page.ID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	components, err := s.store.ListComponentsByPage(r.Context(), page.ID)
	if err != nil {
		writeServerError(w, err)
		return
	}

	summary := domain.BuildPublicSummary(groups, components)

	activeIncidents, err := s.store.ListActiveIncidents(r.Context(), page.ID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	activeMaintenances, err := s.store.ListActiveMaintenances(r.Context(), page.ID)
	if err != nil {
		writeServerError(w, err)
		return
	}

	resp := pageSummaryResponse{
		OverallStatus:       string(summary.OverallStatus),
		Page:                toPublicPageResponse(page),
		UpdatedAt:           summaryUpdatedAt(page, components).UTC().Format(time.RFC3339),
		Groups:              make([]publicGroupResponse, len(summary.Groups)),
		UngroupedComponents: toComponentResponses(summary.Ungrouped),
		ActiveIncidents:     toIncidentResponses(activeIncidents),
		ActiveMaintenances:  toMaintenanceResponses(activeMaintenances),
	}
	for i, g := range summary.Groups {
		resp.Groups[i] = publicGroupResponse{
			ID:               g.Group.ID.String(),
			Name:             g.Group.Name,
			Position:         g.Group.Position,
			AggregatedStatus: string(g.AggregatedStatus),
			Components:       toComponentResponses(g.Components),
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) handlePublicComponents(w http.ResponseWriter, r *http.Request) {
	page, ok := s.loadPublicPage(w, r)
	if !ok {
		return
	}
	components, err := s.store.ListComponentsByPage(r.Context(), page.ID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	// На публичной странице приватные компоненты не показываются (DESIGN §3.2).
	visible := make([]domain.Component, 0, len(components))
	for _, c := range components {
		if !c.IsPrivate {
			visible = append(visible, c)
		}
	}
	writeJSON(w, http.StatusOK, toComponentResponses(visible))
}

func toComponentResponses(comps []domain.Component) []componentResponse {
	out := make([]componentResponse, len(comps))
	for i, c := range comps {
		out[i] = toComponentResponse(c)
	}
	return out
}

// summaryUpdatedAt — свежесть сводки: максимум из updated_at страницы и компонентов.
func summaryUpdatedAt(page domain.StatusPage, components []domain.Component) time.Time {
	latest := page.UpdatedAt
	for _, c := range components {
		if c.UpdatedAt.After(latest) {
			latest = c.UpdatedAt
		}
	}
	return latest
}
