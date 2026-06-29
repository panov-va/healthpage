package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
)

// ── Пагинация ──

type paginationResponse struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

const (
	defaultPerPage = 20
	maxPerPage     = 100
)

// parsePagination читает query-параметры page/per_page (openapi: Page/PerPage) и нормализует их:
// page ≥ 1 (дефолт 1), per_page в [1, 100] (дефолт 20). Возвращает page, per_page и offset.
func parsePagination(r *http.Request) (page, perPage, offset int) {
	page = atoiDefault(r.URL.Query().Get("page"), 1)
	if page < 1 {
		page = 1
	}
	perPage = atoiDefault(r.URL.Query().Get("per_page"), defaultPerPage)
	if perPage < 1 {
		perPage = defaultPerPage
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	return page, perPage, (page - 1) * perPage
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// ── Списки ──

type incidentListResponse struct {
	Items      []incidentResponse `json:"items"`
	Pagination paginationResponse `json:"pagination"`
}

type maintenanceListResponse struct {
	Items      []maintenanceResponse `json:"items"`
	Pagination paginationResponse    `json:"pagination"`
}

// handlePublicIncidents — публичная история инцидентов страницы с фильтрами (status, impact,
// component_id) и пагинацией. Невалидное значение фильтра → 422.
func (s *server) handlePublicIncidents(w http.ResponseWriter, r *http.Request) {
	page, ok := s.loadPublicPage(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	var filter store.IncidentFilter

	if raw := q.Get("status"); raw != "" {
		st := domain.IncidentStatus(raw)
		if !st.IsValid() {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый status")
			return
		}
		filter.Status = &st
	}
	if raw := q.Get("impact"); raw != "" {
		im := domain.IncidentImpact(raw)
		if !im.IsValid() {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый impact")
			return
		}
		filter.Impact = &im
	}
	if raw := q.Get("component_id"); raw != "" {
		cid, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "некорректный component_id")
			return
		}
		filter.ComponentID = &cid
	}

	pageNum, perPage, offset := parsePagination(r)
	incidents, total, err := s.store.ListPublicIncidents(r.Context(), page.ID, filter, perPage, offset)
	if err != nil {
		writeServerError(w, err)
		return
	}
	items := make([]incidentResponse, len(incidents))
	for i, inc := range incidents {
		items[i] = toIncidentResponse(inc)
	}
	writeJSON(w, http.StatusOK, incidentListResponse{
		Items:      items,
		Pagination: paginationResponse{Page: pageNum, PerPage: perPage, Total: total},
	})
}

// handlePublicIncidentDetail — публичный инцидент с лентой обновлений. Невидимые/удалённые/чужие
// (не с этой страницы) — 404 (не раскрываем).
func (s *server) handlePublicIncidentDetail(w http.ResponseWriter, r *http.Request) {
	page, ok := s.loadPublicPage(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "инцидент не найден")
		return
	}
	inc, err := s.store.IncidentByID(r.Context(), id)
	if err != nil || inc.StatusPageID != page.ID || !inc.IsVisible {
		writeError(w, http.StatusNotFound, "not_found", "инцидент не найден")
		return
	}
	writeJSON(w, http.StatusOK, toIncidentResponse(inc))
}

// handlePublicMaintenances — публичный список плановых работ с фильтром по статусу и пагинацией.
func (s *server) handlePublicMaintenances(w http.ResponseWriter, r *http.Request) {
	page, ok := s.loadPublicPage(w, r)
	if !ok {
		return
	}
	var statusFilter *domain.MaintenanceStatus
	if raw := r.URL.Query().Get("status"); raw != "" {
		st := domain.MaintenanceStatus(raw)
		if !st.IsValid() {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый status")
			return
		}
		statusFilter = &st
	}

	pageNum, perPage, offset := parsePagination(r)
	maintenances, total, err := s.store.ListPublicMaintenances(r.Context(), page.ID, statusFilter, perPage, offset)
	if err != nil {
		writeServerError(w, err)
		return
	}
	items := make([]maintenanceResponse, len(maintenances))
	for i, m := range maintenances {
		items[i] = toMaintenanceResponse(m)
	}
	writeJSON(w, http.StatusOK, maintenanceListResponse{
		Items:      items,
		Pagination: paginationResponse{Page: pageNum, PerPage: perPage, Total: total},
	})
}
