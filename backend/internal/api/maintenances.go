package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
)

// ── DTO плановых работ (синхронны с openapi; конформность — контрактными тестами) ──

type maintenanceUpdateResponse struct {
	ID                string `json:"id"`
	Body              string `json:"body"`
	NotifySubscribers bool   `json:"notify_subscribers"`
	CreatedAt         string `json:"created_at"`
}

type maintenanceResponse struct {
	ID             string                      `json:"id"`
	Title          string                      `json:"title"`
	Description    *string                     `json:"description"`
	Status         string                      `json:"status"`
	ScheduledStart string                      `json:"scheduled_start"`
	ScheduledEnd   string                      `json:"scheduled_end"`
	StartedAt      *string                     `json:"started_at"`
	CompletedAt    *string                     `json:"completed_at"`
	ComponentIDs   []string                    `json:"component_ids"`
	Updates        []maintenanceUpdateResponse `json:"updates"`
}

func toMaintenanceResponse(m domain.Maintenance) maintenanceResponse {
	ids := make([]string, len(m.ComponentIDs))
	for i, c := range m.ComponentIDs {
		ids[i] = c.String()
	}
	updates := make([]maintenanceUpdateResponse, len(m.Updates))
	for i, u := range m.Updates {
		updates[i] = toMaintenanceUpdateResponse(u)
	}
	var description *string
	if m.Description != "" {
		d := m.Description
		description = &d
	}
	return maintenanceResponse{
		ID: m.ID.String(), Title: m.Title, Description: description, Status: string(m.Status),
		ScheduledStart: m.ScheduledStart.UTC().Format(time.RFC3339),
		ScheduledEnd:   m.ScheduledEnd.UTC().Format(time.RFC3339),
		StartedAt:      rfc3339Ptr(m.StartedAt), CompletedAt: rfc3339Ptr(m.CompletedAt),
		ComponentIDs: ids, Updates: updates,
	}
}

func toMaintenanceResponses(ms []domain.Maintenance) []maintenanceResponse {
	out := make([]maintenanceResponse, len(ms))
	for i, m := range ms {
		out[i] = toMaintenanceResponse(m)
	}
	return out
}

func toMaintenanceUpdateResponse(u domain.MaintenanceUpdate) maintenanceUpdateResponse {
	return maintenanceUpdateResponse{
		ID: u.ID.String(), Body: u.Body, NotifySubscribers: u.NotifySubscribers,
		CreatedAt: u.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type createMaintenanceRequest struct {
	StatusPageID   string   `json:"status_page_id"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	ScheduledStart string   `json:"scheduled_start"`
	ScheduledEnd   string   `json:"scheduled_end"`
	ComponentIDs   []string `json:"component_ids"`
	Notify         *bool    `json:"notify"`
}

type patchMaintenanceRequest struct {
	Title          *string   `json:"title"`
	Description    *string   `json:"description"`
	Status         *string   `json:"status"`
	ScheduledStart *string   `json:"scheduled_start"`
	ScheduledEnd   *string   `json:"scheduled_end"`
	ComponentIDs   *[]string `json:"component_ids"`
}

type addMaintenanceUpdateRequest struct {
	Body   string `json:"body"`
	Notify *bool  `json:"notify"`
}

// ── Хендлеры ──

// handleListMaintenances — админский список работ страницы с фильтром по статусу и пагинацией.
// Требует ?status_page_id; авторизация по владению. Работы не имеют признака видимости, поэтому
// данные совпадают с публичным списком — переиспользуем тот же store-метод.
func (s *server) handleListMaintenances(w http.ResponseWriter, r *http.Request) {
	page, ok := s.resolveManagedPage(w, r, r.URL.Query().Get("status_page_id"))
	if !ok {
		return
	}
	pageID := page.ID
	statusFilter, ok := parseMaintenanceStatusFilter(w, r)
	if !ok {
		return
	}
	pageNum, perPage, offset := parsePagination(r)
	maintenances, total, err := s.store.ListPublicMaintenances(r.Context(), pageID, statusFilter, perPage, offset)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, maintenanceListResponse{
		Items:      toMaintenanceResponses(maintenances),
		Pagination: paginationResponse{Page: pageNum, PerPage: perPage, Total: total},
	})
}

// handleGetMaintenance — админский просмотр одной работы, авторизация по владению.
func (s *server) handleGetMaintenance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	m, err := s.store.MaintenanceByID(r.Context(), id)
	if err != nil {
		s.writeLoadError(w, err, "работы не найдены")
		return
	}
	if _, ok := s.authorizePage(w, r, m.StatusPageID); !ok {
		return
	}
	writeJSON(w, http.StatusOK, toMaintenanceResponse(m))
}

func (s *server) handleCreateMaintenance(w http.ResponseWriter, r *http.Request) {
	var req createMaintenanceRequest
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
	start, ok := parseRFC3339(w, req.ScheduledStart, "scheduled_start")
	if !ok {
		return
	}
	end, ok := parseRFC3339(w, req.ScheduledEnd, "scheduled_end")
	if !ok {
		return
	}
	componentIDs, ok := s.parseMaintenanceComponents(w, r, pageID, req.ComponentIDs)
	if !ok {
		return
	}

	m := domain.Maintenance{
		StatusPageID:   pageID,
		Title:          req.Title,
		Description:    req.Description,
		Status:         domain.MaintenanceScheduled,
		ScheduledStart: start,
		ScheduledEnd:   end,
		ComponentIDs:   componentIDs,
	}
	if err := m.ValidateSchedule(); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "scheduled_end должен быть позже scheduled_start")
		return
	}

	created, err := s.store.CreateMaintenance(r.Context(), m)
	if err != nil {
		writeServerError(w, err)
		return
	}
	// Анонс запланированных работ (DESIGN §8.1 notify.<channel>.maintenance_scheduled).
	if boolOr(req.Notify, true) {
		s.emitNotify(func() error {
			return s.notifier.MaintenanceEvent(r.Context(), created, domain.EventMaintenanceScheduled)
		})
	}
	writeJSON(w, http.StatusCreated, toMaintenanceResponse(created))
}

func (s *server) handlePatchMaintenance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	m, err := s.store.MaintenanceByID(r.Context(), id)
	if err != nil {
		s.writeLoadError(w, err, "работы не найдены")
		return
	}
	if _, ok := s.authorizePage(w, r, m.StatusPageID); !ok {
		return
	}
	var req patchMaintenanceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	prevStatus := m.Status

	if req.Title != nil {
		m.Title = *req.Title
	}
	if req.Description != nil {
		m.Description = *req.Description
	}
	if req.ScheduledStart != nil {
		t, ok := parseRFC3339(w, *req.ScheduledStart, "scheduled_start")
		if !ok {
			return
		}
		m.ScheduledStart = t
	}
	if req.ScheduledEnd != nil {
		t, ok := parseRFC3339(w, *req.ScheduledEnd, "scheduled_end")
		if !ok {
			return
		}
		m.ScheduledEnd = t
	}
	if req.ScheduledStart != nil || req.ScheduledEnd != nil {
		if err := m.ValidateSchedule(); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "scheduled_end должен быть позже scheduled_start")
			return
		}
	}
	if req.Status != nil {
		status := domain.MaintenanceStatus(*req.Status)
		if !status.IsValid() {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый status")
			return
		}
		// Жизненный цикл: фиксирует/сбрасывает StartedAt/CompletedAt.
		if err := m.ApplyStatusChange(status, time.Now().UTC()); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый status")
			return
		}
	}

	replaceComponents := req.ComponentIDs != nil
	if replaceComponents {
		ids, ok := s.parseMaintenanceComponents(w, r, m.StatusPageID, *req.ComponentIDs)
		if !ok {
			return
		}
		m.ComponentIDs = ids
	}

	updated, err := s.store.UpdateMaintenance(r.Context(), m, replaceComponents)
	if err != nil {
		writeServerError(w, err)
		return
	}
	// Уведомляем о начале/окончании работ при фактическом переходе статуса (DESIGN §3.5).
	if event, ok := maintenanceTransitionEvent(prevStatus, updated.Status); ok {
		s.emitNotify(func() error { return s.notifier.MaintenanceEvent(r.Context(), updated, event) })
	}
	writeJSON(w, http.StatusOK, toMaintenanceResponse(updated))
}

// maintenanceTransitionEvent возвращает тип уведомления для перехода статуса работ: вход в
// in_progress → started, в completed → completed. Прочие переходы (в т.ч. отсутствие смены)
// уведомлений не порождают.
func maintenanceTransitionEvent(from, to domain.MaintenanceStatus) (domain.EventType, bool) {
	if from == to {
		return "", false
	}
	switch to {
	case domain.MaintenanceInProgress:
		return domain.EventMaintenanceStarted, true
	case domain.MaintenanceCompleted:
		return domain.EventMaintenanceCompleted, true
	default:
		return "", false
	}
}

func (s *server) handleDeleteMaintenance(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	m, err := s.store.MaintenanceByID(r.Context(), id)
	if err != nil {
		s.writeLoadError(w, err, "работы не найдены")
		return
	}
	if _, ok := s.authorizePage(w, r, m.StatusPageID); !ok {
		return
	}
	if err := s.store.SoftDeleteMaintenance(r.Context(), id); err != nil {
		writeServerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleAddMaintenanceUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	m, err := s.store.MaintenanceByID(r.Context(), id)
	if err != nil {
		s.writeLoadError(w, err, "работы не найдены")
		return
	}
	if _, ok := s.authorizePage(w, r, m.StatusPageID); !ok {
		return
	}
	var req addMaintenanceUpdateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Body == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "body обязателен")
		return
	}
	update, _, err := s.store.AddMaintenanceUpdate(r.Context(), id, req.Body, boolOr(req.Notify, true))
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toMaintenanceUpdateResponse(update))
}

// parseMaintenanceComponents валидирует список затронутых компонентов: корректный uuid и
// принадлежность компонента этой же странице (изоляция). При ошибке пишет 422.
func (s *server) parseMaintenanceComponents(
	w http.ResponseWriter, r *http.Request, pageID uuid.UUID, in []string,
) ([]uuid.UUID, bool) {
	out := make([]uuid.UUID, 0, len(in))
	for _, raw := range in {
		cid, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "некорректный component_id")
			return nil, false
		}
		comp, err := s.store.ComponentByID(r.Context(), cid)
		if err != nil || comp.StatusPageID != pageID {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "компонент не принадлежит странице")
			return nil, false
		}
		out = append(out, cid)
	}
	return out, true
}

// parseRFC3339 разбирает обязательную дату-время RFC3339, при ошибке/пустоте пишет 422.
func parseRFC3339(w http.ResponseWriter, raw, field string) (time.Time, bool) {
	if raw == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", field+" обязателен")
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", field+": ожидается RFC3339")
		return time.Time{}, false
	}
	return t, true
}
