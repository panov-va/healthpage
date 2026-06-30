package api

import (
	"net/http"

	"github.com/healthpage/backend/internal/domain"
)

// ── DTO шаблонов инцидентов (синхронны с openapi; конформность — контрактными тестами) ──

type incidentTemplateResponse struct {
	ID                string                 `json:"id"`
	StatusPageID      string                 `json:"status_page_id"`
	Name              string                 `json:"name"`
	TitleTmpl         string                 `json:"title_tmpl"`
	BodyTmpl          string                 `json:"body_tmpl"`
	DefaultImpact     string                 `json:"default_impact"`
	DefaultComponents []incidentComponentDTO `json:"default_components"`
}

func toIncidentTemplateResponse(t domain.IncidentTemplate) incidentTemplateResponse {
	comps := make([]incidentComponentDTO, len(t.DefaultComponents))
	for i, c := range t.DefaultComponents {
		comps[i] = incidentComponentDTO{
			ComponentID:               c.ComponentID.String(),
			ComponentStatusInIncident: string(c.ComponentStatusInIncident),
		}
	}
	return incidentTemplateResponse{
		ID: t.ID.String(), StatusPageID: t.StatusPageID.String(), Name: t.Name,
		TitleTmpl: t.TitleTmpl, BodyTmpl: t.BodyTmpl, DefaultImpact: string(t.DefaultImpact),
		DefaultComponents: comps,
	}
}

type createIncidentTemplateRequest struct {
	StatusPageID      string                 `json:"status_page_id"`
	Name              string                 `json:"name"`
	TitleTmpl         string                 `json:"title_tmpl"`
	BodyTmpl          string                 `json:"body_tmpl"`
	DefaultImpact     *string                `json:"default_impact"`
	DefaultComponents []incidentComponentDTO `json:"default_components"`
}

type patchIncidentTemplateRequest struct {
	Name              *string                 `json:"name"`
	TitleTmpl         *string                 `json:"title_tmpl"`
	BodyTmpl          *string                 `json:"body_tmpl"`
	DefaultImpact     *string                 `json:"default_impact"`
	DefaultComponents *[]incidentComponentDTO `json:"default_components"`
}

// ── Хендлеры ──

func (s *server) handleCreateIncidentTemplate(w http.ResponseWriter, r *http.Request) {
	var req createIncidentTemplateRequest
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
	impact := domain.ImpactNone
	if req.DefaultImpact != nil {
		impact = domain.IncidentImpact(*req.DefaultImpact)
		if !impact.IsValid() {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый default_impact")
			return
		}
	}
	comps, ok := s.parseIncidentComponents(w, r, pageID, req.DefaultComponents)
	if !ok {
		return
	}

	tmpl := domain.IncidentTemplate{
		StatusPageID:      pageID,
		Name:              req.Name,
		TitleTmpl:         req.TitleTmpl,
		BodyTmpl:          req.BodyTmpl,
		DefaultImpact:     impact,
		DefaultComponents: comps,
	}
	created, err := s.store.CreateIncidentTemplate(r.Context(), tmpl)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toIncidentTemplateResponse(created))
}

func (s *server) handleListIncidentTemplates(w http.ResponseWriter, r *http.Request) {
	page, ok := s.resolveManagedPage(w, r, r.URL.Query().Get("status_page_id"))
	if !ok {
		return
	}
	pageID := page.ID
	templates, err := s.store.ListIncidentTemplates(r.Context(), pageID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	out := make([]incidentTemplateResponse, len(templates))
	for i, t := range templates {
		out[i] = toIncidentTemplateResponse(t)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleGetIncidentTemplate(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadAuthorizedTemplate(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toIncidentTemplateResponse(t))
}

func (s *server) handlePatchIncidentTemplate(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadAuthorizedTemplate(w, r)
	if !ok {
		return
	}
	var req patchIncidentTemplateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.Name != nil {
		if *req.Name == "" {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "name не может быть пустым")
			return
		}
		t.Name = *req.Name
	}
	if req.TitleTmpl != nil {
		t.TitleTmpl = *req.TitleTmpl
	}
	if req.BodyTmpl != nil {
		t.BodyTmpl = *req.BodyTmpl
	}
	if req.DefaultImpact != nil {
		impact := domain.IncidentImpact(*req.DefaultImpact)
		if !impact.IsValid() {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый default_impact")
			return
		}
		t.DefaultImpact = impact
	}

	replaceComponents := req.DefaultComponents != nil
	if replaceComponents {
		comps, ok := s.parseIncidentComponents(w, r, t.StatusPageID, *req.DefaultComponents)
		if !ok {
			return
		}
		t.DefaultComponents = comps
	}

	updated, err := s.store.UpdateIncidentTemplate(r.Context(), t, replaceComponents)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toIncidentTemplateResponse(updated))
}

func (s *server) handleDeleteIncidentTemplate(w http.ResponseWriter, r *http.Request) {
	t, ok := s.loadAuthorizedTemplate(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteIncidentTemplate(r.Context(), t.ID); err != nil {
		writeServerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// loadAuthorizedTemplate грузит шаблон по {id} из пути и проверяет, что вызывающий владеет его
// страницей. При отсутствии/чужом шаблоне пишет 404 (изоляция) и возвращает ok=false.
func (s *server) loadAuthorizedTemplate(w http.ResponseWriter, r *http.Request) (domain.IncidentTemplate, bool) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return domain.IncidentTemplate{}, false
	}
	t, err := s.store.IncidentTemplateByID(r.Context(), id)
	if err != nil {
		s.writeLoadError(w, err, "шаблон не найден")
		return domain.IncidentTemplate{}, false
	}
	if _, ok := s.authorizePage(w, r, t.StatusPageID); !ok {
		return domain.IncidentTemplate{}, false
	}
	return t, true
}
