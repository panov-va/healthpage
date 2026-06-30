package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
)

// ── DTO инцидентов (синхронны с openapi; конформность — контрактными тестами) ──

type incidentComponentDTO struct {
	ComponentID               string `json:"component_id"`
	ComponentStatusInIncident string `json:"component_status_in_incident"`
}

type incidentUpdateResponse struct {
	ID                string `json:"id"`
	Status            string `json:"status"`
	Body              string `json:"body"`
	NotifySubscribers bool   `json:"notify_subscribers"`
	CreatedAt         string `json:"created_at"`
}

type incidentResponse struct {
	ID            string                   `json:"id"`
	Title         string                   `json:"title"`
	CurrentStatus string                   `json:"current_status"`
	Impact        string                   `json:"impact"`
	StartedAt     string                   `json:"started_at"`
	ResolvedAt    *string                  `json:"resolved_at"`
	Postmortem    *string                  `json:"postmortem"`
	IsVisible     bool                     `json:"is_visible"`
	Components    []incidentComponentDTO   `json:"components"`
	Updates       []incidentUpdateResponse `json:"updates"`
}

func toIncidentResponse(inc domain.Incident) incidentResponse {
	comps := make([]incidentComponentDTO, len(inc.Components))
	for i, c := range inc.Components {
		comps[i] = incidentComponentDTO{
			ComponentID:               c.ComponentID.String(),
			ComponentStatusInIncident: string(c.ComponentStatusInIncident),
		}
	}
	updates := make([]incidentUpdateResponse, len(inc.Updates))
	for i, u := range inc.Updates {
		updates[i] = incidentUpdateResponse{
			ID: u.ID.String(), Status: string(u.Status), Body: u.Body,
			NotifySubscribers: u.NotifySubscribers, CreatedAt: u.CreatedAt.UTC().Format(time.RFC3339),
		}
	}
	return incidentResponse{
		ID: inc.ID.String(), Title: inc.Title, CurrentStatus: string(inc.CurrentStatus),
		Impact: string(inc.Impact), StartedAt: inc.StartedAt.UTC().Format(time.RFC3339),
		ResolvedAt: rfc3339Ptr(inc.ResolvedAt), Postmortem: inc.Postmortem, IsVisible: inc.IsVisible,
		Components: comps, Updates: updates,
	}
}

func toIncidentResponses(incs []domain.Incident) []incidentResponse {
	out := make([]incidentResponse, len(incs))
	for i, inc := range incs {
		out[i] = toIncidentResponse(inc)
	}
	return out
}

func toIncidentUpdateResponse(u domain.IncidentUpdate) incidentUpdateResponse {
	return incidentUpdateResponse{
		ID: u.ID.String(), Status: string(u.Status), Body: u.Body,
		NotifySubscribers: u.NotifySubscribers, CreatedAt: u.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type createIncidentRequest struct {
	StatusPageID string                 `json:"status_page_id"`
	Title        string                 `json:"title"`
	Status       string                 `json:"status"`
	Impact       string                 `json:"impact"`
	Body         string                 `json:"body"`
	Notify       *bool                  `json:"notify"`
	StartedAt    *string                `json:"started_at"`
	Components   []incidentComponentDTO `json:"components"`
}

type patchIncidentRequest struct {
	Title      *string                 `json:"title"`
	Impact     *string                 `json:"impact"`
	Postmortem *string                 `json:"postmortem"`
	IsVisible  *bool                   `json:"is_visible"`
	Components *[]incidentComponentDTO `json:"components"`
}

type addIncidentUpdateRequest struct {
	Status string `json:"status"`
	Body   string `json:"body"`
	Notify *bool  `json:"notify"`
}

// ── Хендлеры ──

// handleListIncidents — админский список инцидентов страницы (включая скрытые) с фильтрами и
// пагинацией. Требует ?status_page_id (при операторском JWT); авторизация по владению страницей.
func (s *server) handleListIncidents(w http.ResponseWriter, r *http.Request) {
	page, ok := s.resolveManagedPage(w, r, r.URL.Query().Get("status_page_id"))
	if !ok {
		return
	}
	pageID := page.ID
	filter, ok := parseIncidentFilter(w, r)
	if !ok {
		return
	}
	pageNum, perPage, offset := parsePagination(r)
	incidents, total, err := s.store.ListIncidents(r.Context(), pageID, filter, perPage, offset)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, incidentListResponse{
		Items:      toIncidentResponses(incidents),
		Pagination: paginationResponse{Page: pageNum, PerPage: perPage, Total: total},
	})
}

// handleGetIncident — админский просмотр инцидента (включая скрытый), авторизация по владению.
func (s *server) handleGetIncident(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	inc, err := s.store.IncidentByID(r.Context(), id)
	if err != nil {
		s.writeLoadError(w, err, "инцидент не найден")
		return
	}
	if _, ok := s.authorizePage(w, r, inc.StatusPageID); !ok {
		return
	}
	writeJSON(w, http.StatusOK, toIncidentResponse(inc))
}

func (s *server) handleCreateIncident(w http.ResponseWriter, r *http.Request) {
	var req createIncidentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	page, ok := s.resolveManagedPage(w, r, req.StatusPageID)
	if !ok {
		return
	}
	pageID := page.ID
	if req.Title == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "title обязателен")
		return
	}
	if req.Body == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "body обязателен")
		return
	}
	status := domain.IncidentStatus(req.Status)
	if !status.IsValid() {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый status")
		return
	}
	impact := domain.IncidentImpact(req.Impact)
	if !impact.IsValid() {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый impact")
		return
	}
	comps, ok := s.parseIncidentComponents(w, r, pageID, req.Components)
	if !ok {
		return
	}

	now := time.Now().UTC()
	startedAt := now
	if req.StartedAt != nil && *req.StartedAt != "" {
		t, err := time.Parse(time.RFC3339, *req.StartedAt)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "started_at: ожидается RFC3339")
			return
		}
		startedAt = t
	}

	inc := domain.Incident{
		StatusPageID: pageID,
		Title:        req.Title,
		Impact:       impact,
		StartedAt:    startedAt,
		IsVisible:    true,
		Components:   comps,
	}
	// Жизненный цикл: фиксирует ResolvedAt, если инцидент сразу создаётся как resolved.
	if err := inc.ApplyStatusChange(status, now); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый status")
		return
	}

	notify := boolOr(req.Notify, true)
	created, err := s.store.CreateIncident(r.Context(), inc, req.Body, notify)
	if err != nil {
		writeServerError(w, err)
		return
	}
	// Уведомляем только о видимом инциденте и только если оператор не отключил рассылку.
	if notify && created.IsVisible {
		s.emitNotify(func() error { return s.notifier.IncidentCreated(r.Context(), created, req.Body) })
	}
	writeJSON(w, http.StatusCreated, toIncidentResponse(created))
}

func (s *server) handlePatchIncident(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	inc, err := s.store.IncidentByID(r.Context(), id)
	if err != nil {
		s.writeLoadError(w, err, "инцидент не найден")
		return
	}
	if _, ok := s.authorizePage(w, r, inc.StatusPageID); !ok {
		return
	}
	var req patchIncidentRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.Title != nil {
		inc.Title = *req.Title
	}
	if req.Impact != nil {
		impact := domain.IncidentImpact(*req.Impact)
		if !impact.IsValid() {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый impact")
			return
		}
		inc.Impact = impact
	}
	if req.IsVisible != nil {
		inc.IsVisible = *req.IsVisible
	}
	if req.Postmortem != nil {
		// Постмортем допустим только для устранённого инцидента (DESIGN §3.3).
		if err := inc.SetPostmortem(*req.Postmortem); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request",
				"постмортем можно прикрепить только к устранённому инциденту")
			return
		}
	}

	replaceComponents := req.Components != nil
	if replaceComponents {
		comps, ok := s.parseIncidentComponents(w, r, inc.StatusPageID, *req.Components)
		if !ok {
			return
		}
		inc.Components = comps
	}

	updated, err := s.store.UpdateIncident(r.Context(), inc, replaceComponents)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toIncidentResponse(updated))
}

func (s *server) handleDeleteIncident(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	inc, err := s.store.IncidentByID(r.Context(), id)
	if err != nil {
		s.writeLoadError(w, err, "инцидент не найден")
		return
	}
	if _, ok := s.authorizePage(w, r, inc.StatusPageID); !ok {
		return
	}
	if err := s.store.SoftDeleteIncident(r.Context(), id); err != nil {
		writeServerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleAddIncidentUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	inc, err := s.store.IncidentByID(r.Context(), id)
	if err != nil {
		s.writeLoadError(w, err, "инцидент не найден")
		return
	}
	if _, ok := s.authorizePage(w, r, inc.StatusPageID); !ok {
		return
	}
	var req addIncidentUpdateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	status := domain.IncidentStatus(req.Status)
	if !status.IsValid() {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый status")
		return
	}
	if req.Body == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "body обязателен")
		return
	}
	notify := boolOr(req.Notify, true)
	update, updated, err := s.store.AddIncidentUpdate(
		r.Context(), id, status, req.Body, notify, time.Now().UTC(),
	)
	if err != nil {
		writeServerError(w, err)
		return
	}
	if notify && updated.IsVisible {
		s.emitNotify(func() error { return s.notifier.IncidentUpdated(r.Context(), updated, update) })
	}
	writeJSON(w, http.StatusCreated, toIncidentUpdateResponse(update))
}

// parseIncidentComponents валидирует список затронутых компонентов: корректный uuid, валидный
// статус и принадлежность компонента этой же странице (изоляция). При ошибке пишет 422.
func (s *server) parseIncidentComponents(
	w http.ResponseWriter, r *http.Request, pageID uuid.UUID, in []incidentComponentDTO,
) ([]domain.IncidentComponent, bool) {
	out := make([]domain.IncidentComponent, 0, len(in))
	for _, ic := range in {
		cid, err := uuid.Parse(ic.ComponentID)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "некорректный component_id")
			return nil, false
		}
		st := domain.ComponentStatus(ic.ComponentStatusInIncident)
		if !st.IsValid() {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый component_status_in_incident")
			return nil, false
		}
		comp, err := s.store.ComponentByID(r.Context(), cid)
		if err != nil || comp.StatusPageID != pageID {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "компонент не принадлежит странице")
			return nil, false
		}
		out = append(out, domain.IncidentComponent{ComponentID: cid, ComponentStatusInIncident: st})
	}
	return out, true
}

func rfc3339Ptr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}
