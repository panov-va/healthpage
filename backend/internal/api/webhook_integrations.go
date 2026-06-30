package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
	"github.com/healthpage/backend/internal/webhook"
)

// ── DTO интеграций (синхронны с openapi WebhookIntegration / *Create / *Patch / *Created) ──

type webhookIntegrationResponse struct {
	ID               string          `json:"id"`
	StatusPageID     string          `json:"status_page_id"`
	Source           string          `json:"source"`
	Name             string          `json:"name"`
	ComponentMapping json.RawMessage `json:"component_mapping"`
	Secret           *string         `json:"secret,omitempty"` // только при создании/ротации
	CreatedAt        string          `json:"created_at"`
	UpdatedAt        string          `json:"updated_at"`
}

func toWebhookIntegrationResponse(wi domain.WebhookIntegration, secret *string) webhookIntegrationResponse {
	mapping := wi.ComponentMapping
	if len(mapping) == 0 {
		mapping = []byte("{}")
	}
	return webhookIntegrationResponse{
		ID:               wi.ID.String(),
		StatusPageID:     wi.StatusPageID.String(),
		Source:           string(wi.Source),
		Name:             wi.Name,
		ComponentMapping: json.RawMessage(mapping),
		Secret:           secret,
		CreatedAt:        wi.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        wi.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

type webhookIntegrationCreateRequest struct {
	StatusPageID     string          `json:"status_page_id"`
	Source           string          `json:"source"`
	Name             string          `json:"name"`
	ComponentMapping json.RawMessage `json:"component_mapping"`
}

type webhookIntegrationPatchRequest struct {
	Name             *string         `json:"name"`
	ComponentMapping json.RawMessage `json:"component_mapping"`
	RegenerateSecret bool            `json:"regenerate_secret"`
}

// handleCreateWebhookIntegration создаёт интеграцию. Только оператор (как и токены — минтит секрет).
func (s *server) handleCreateWebhookIntegration(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireOperator(w, r); !ok {
		return
	}
	var req webhookIntegrationCreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	page, ok := s.resolveManagedPage(w, r, req.StatusPageID)
	if !ok {
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "name обязателен")
		return
	}
	source := domain.WebhookSource(req.Source)
	if !source.IsValid() {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый source")
		return
	}
	if !s.validMapping(w, req.ComponentMapping) {
		return
	}

	secret, err := security.GenerateWebhookSecret()
	if err != nil {
		writeServerError(w, err)
		return
	}
	created, err := s.store.CreateWebhookIntegration(r.Context(), domain.WebhookIntegration{
		StatusPageID:     page.ID,
		Source:           source,
		Name:             req.Name,
		Secret:           secret,
		ComponentMapping: req.ComponentMapping,
	})
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toWebhookIntegrationResponse(created, &secret))
}

// handleListWebhookIntegrations — список интеграций страницы (без секретов). Только оператор.
func (s *server) handleListWebhookIntegrations(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireOperator(w, r); !ok {
		return
	}
	page, ok := s.resolveManagedPage(w, r, r.URL.Query().Get("status_page_id"))
	if !ok {
		return
	}
	items, err := s.store.ListWebhookIntegrationsByPage(r.Context(), page.ID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	out := make([]webhookIntegrationResponse, len(items))
	for i, wi := range items {
		out[i] = toWebhookIntegrationResponse(wi, nil)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleGetWebhookIntegration(w http.ResponseWriter, r *http.Request) {
	wi, ok := s.loadAuthorizedIntegration(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toWebhookIntegrationResponse(wi, nil))
}

// handlePatchWebhookIntegration меняет name/component_mapping и опц. ротаирует секрет
// (regenerate_secret=true → новый секрет в ответе единожды).
func (s *server) handlePatchWebhookIntegration(w http.ResponseWriter, r *http.Request) {
	wi, ok := s.loadAuthorizedIntegration(w, r)
	if !ok {
		return
	}
	var req webhookIntegrationPatchRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name != nil {
		if *req.Name == "" {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "name не может быть пустым")
			return
		}
		wi.Name = *req.Name
	}
	if req.ComponentMapping != nil {
		if !s.validMapping(w, req.ComponentMapping) {
			return
		}
		wi.ComponentMapping = req.ComponentMapping
	}
	var newSecret *string
	if req.RegenerateSecret {
		secret, err := security.GenerateWebhookSecret()
		if err != nil {
			writeServerError(w, err)
			return
		}
		wi.Secret = secret
		newSecret = &secret
	}
	updated, err := s.store.UpdateWebhookIntegration(r.Context(), wi)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toWebhookIntegrationResponse(updated, newSecret))
}

func (s *server) handleDeleteWebhookIntegration(w http.ResponseWriter, r *http.Request) {
	wi, ok := s.loadAuthorizedIntegration(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteWebhookIntegration(r.Context(), wi.ID); err != nil {
		writeServerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// loadAuthorizedIntegration грузит интеграцию по {id} и авторизует доступ к её странице.
func (s *server) loadAuthorizedIntegration(w http.ResponseWriter, r *http.Request) (domain.WebhookIntegration, bool) {
	if _, ok := requireOperator(w, r); !ok {
		return domain.WebhookIntegration{}, false
	}
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return domain.WebhookIntegration{}, false
	}
	wi, err := s.store.WebhookIntegrationByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "интеграция не найдена")
		} else {
			writeServerError(w, err)
		}
		return domain.WebhookIntegration{}, false
	}
	if _, ok := s.authorizePage(w, r, wi.StatusPageID); !ok {
		return domain.WebhookIntegration{}, false
	}
	return wi, true
}

// validMapping проверяет, что component_mapping (если задан) — валидный JSON-объект маппинга.
func (s *server) validMapping(w http.ResponseWriter, raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	if _, err := webhook.ParseMapping(raw); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "некорректный component_mapping")
		return false
	}
	return true
}
