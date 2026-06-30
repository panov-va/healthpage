package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
)

// ── DTO групп и компонентов ──

type componentGroupResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

func toGroupResponse(g domain.ComponentGroup) componentGroupResponse {
	return componentGroupResponse{ID: g.ID.String(), Name: g.Name, Position: g.Position}
}

type componentResponse struct {
	ID            string  `json:"id"`
	GroupID       *string `json:"group_id"`
	ParentID      *string `json:"parent_id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Position      int     `json:"position"`
	CurrentStatus string  `json:"current_status"`
	IsPrivate     bool    `json:"is_private"`
	ShowUptime    bool    `json:"show_uptime"`
	DisplayState  bool    `json:"display_state"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

func toComponentResponse(c domain.Component) componentResponse {
	return componentResponse{
		ID: c.ID.String(), GroupID: uuidPtrToStr(c.GroupID), ParentID: uuidPtrToStr(c.ParentID),
		Name: c.Name, Description: c.Description, Position: c.Position,
		CurrentStatus: string(c.CurrentStatus), IsPrivate: c.IsPrivate, ShowUptime: c.ShowUptime,
		DisplayState: c.DisplayState,
		CreatedAt:    c.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: c.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func uuidPtrToStr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

type createGroupRequest struct {
	Name     string `json:"name"`
	Position int    `json:"position"`
}

type patchGroupRequest struct {
	Name     *string `json:"name"`
	Position *int    `json:"position"`
}

type createComponentRequest struct {
	StatusPageID  string  `json:"status_page_id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	GroupID       *string `json:"group_id"`
	ParentID      *string `json:"parent_id"`
	Position      int     `json:"position"`
	CurrentStatus string  `json:"current_status"`
	IsPrivate     *bool   `json:"is_private"`
	ShowUptime    *bool   `json:"show_uptime"`
	DisplayState  *bool   `json:"display_state"`
}

type patchComponentRequest struct {
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	GroupID       *string `json:"group_id"`
	Position      *int    `json:"position"`
	CurrentStatus *string `json:"current_status"`
	IsPrivate     *bool   `json:"is_private"`
	ShowUptime    *bool   `json:"show_uptime"`
	DisplayState  *bool   `json:"display_state"`
}

// ── Группы ──

func (s *server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	pageID, ok := pathUUID(w, r, "page")
	if !ok {
		return
	}
	if _, ok := s.authorizePage(w, r, pageID); !ok {
		return
	}
	groups, err := s.store.ListComponentGroupsByPage(r.Context(), pageID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	out := make([]componentGroupResponse, len(groups))
	for i, g := range groups {
		out[i] = toGroupResponse(g)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	pageID, ok := pathUUID(w, r, "page")
	if !ok {
		return
	}
	if _, ok := s.authorizePage(w, r, pageID); !ok {
		return
	}
	var req createGroupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "name обязателен")
		return
	}
	g, err := s.store.CreateComponentGroup(r.Context(), pageID, req.Name, req.Position)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toGroupResponse(g))
}

func (s *server) handlePatchGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	group, err := s.store.ComponentGroupByID(r.Context(), id)
	if err != nil {
		s.writeLoadError(w, err, "группа не найдена")
		return
	}
	if _, ok := s.authorizePage(w, r, group.StatusPageID); !ok {
		return
	}
	var req patchGroupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name != nil {
		group.Name = *req.Name
	}
	if req.Position != nil {
		group.Position = *req.Position
	}
	updated, err := s.store.UpdateComponentGroup(r.Context(), group.ID, group.Name, group.Position)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toGroupResponse(updated))
}

func (s *server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	group, err := s.store.ComponentGroupByID(r.Context(), id)
	if err != nil {
		s.writeLoadError(w, err, "группа не найдена")
		return
	}
	if _, ok := s.authorizePage(w, r, group.StatusPageID); !ok {
		return
	}
	if err := s.store.SoftDeleteComponentGroup(r.Context(), id); err != nil {
		writeServerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Компоненты ──

func (s *server) handleListComponents(w http.ResponseWriter, r *http.Request) {
	page, ok := s.resolveManagedPage(w, r, r.URL.Query().Get("status_page_id"))
	if !ok {
		return
	}
	pageID := page.ID
	comps, err := s.store.ListComponentsByPage(r.Context(), pageID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	out := make([]componentResponse, len(comps))
	for i, c := range comps {
		out[i] = toComponentResponse(c)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleCreateComponent(w http.ResponseWriter, r *http.Request) {
	var req createComponentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	page, ok := s.resolveManagedPage(w, r, req.StatusPageID)
	if !ok {
		return
	}
	pageID := page.ID
	if req.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "name обязателен")
		return
	}
	groupID, ok := parseOptionalUUID(w, req.GroupID, "group_id")
	if !ok {
		return
	}
	parentID, ok := parseOptionalUUID(w, req.ParentID, "parent_id")
	if !ok {
		return
	}
	status := domain.ComponentStatus(req.CurrentStatus)
	if req.CurrentStatus != "" && !status.IsValid() {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый current_status")
		return
	}

	c := domain.Component{
		StatusPageID:  pageID,
		GroupID:       groupID,
		ParentID:      parentID,
		Name:          req.Name,
		Description:   req.Description,
		Position:      req.Position,
		CurrentStatus: status,
		IsPrivate:     boolOr(req.IsPrivate, false),
		ShowUptime:    boolOr(req.ShowUptime, true),
		DisplayState:  boolOr(req.DisplayState, true),
	}
	created, err := s.store.CreateComponent(r.Context(), c)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toComponentResponse(created))
}

func (s *server) handlePatchComponent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	comp, err := s.store.ComponentByID(r.Context(), id)
	if err != nil {
		s.writeLoadError(w, err, "компонент не найден")
		return
	}
	if _, ok := s.authorizePage(w, r, comp.StatusPageID); !ok {
		return
	}
	var req patchComponentRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	// Валидация статуса (если передан) до изменений.
	var newStatus *domain.ComponentStatus
	if req.CurrentStatus != nil {
		st := domain.ComponentStatus(*req.CurrentStatus)
		if !st.IsValid() {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый current_status")
			return
		}
		newStatus = &st
	}

	// Атрибуты (кроме статуса).
	if req.Name != nil {
		comp.Name = *req.Name
	}
	if req.Description != nil {
		comp.Description = *req.Description
	}
	if req.GroupID != nil {
		gid, ok := parseOptionalUUID(w, req.GroupID, "group_id")
		if !ok {
			return
		}
		comp.GroupID = gid
	}
	if req.Position != nil {
		comp.Position = *req.Position
	}
	if req.IsPrivate != nil {
		comp.IsPrivate = *req.IsPrivate
	}
	if req.ShowUptime != nil {
		comp.ShowUptime = *req.ShowUptime
	}
	if req.DisplayState != nil {
		comp.DisplayState = *req.DisplayState
	}

	updated, err := s.store.UpdateComponent(r.Context(), comp)
	if err != nil {
		writeServerError(w, err)
		return
	}

	// Ручная смена статуса — с ведением истории (DESIGN §6).
	if newStatus != nil && *newStatus != updated.CurrentStatus {
		updated, err = s.store.ChangeComponentStatus(r.Context(), updated.ID, *newStatus, domain.SourceManual)
		if err != nil {
			writeServerError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, toComponentResponse(updated))
}

func (s *server) handleDeleteComponent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	comp, err := s.store.ComponentByID(r.Context(), id)
	if err != nil {
		s.writeLoadError(w, err, "компонент не найден")
		return
	}
	if _, ok := s.authorizePage(w, r, comp.StatusPageID); !ok {
		return
	}
	if err := s.store.SoftDeleteComponent(r.Context(), id); err != nil {
		writeServerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeLoadError мапит ошибку загрузки сущности: ErrNotFound -> 404, иначе 500.
func (s *server) writeLoadError(w http.ResponseWriter, err error, notFoundMsg string) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", notFoundMsg)
		return
	}
	writeServerError(w, err)
}

func parseOptionalUUID(w http.ResponseWriter, raw *string, field string) (*uuid.UUID, bool) {
	if raw == nil || *raw == "" {
		return nil, true
	}
	id, err := uuid.Parse(*raw)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "некорректный "+field)
		return nil, false
	}
	return &id, true
}

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}
