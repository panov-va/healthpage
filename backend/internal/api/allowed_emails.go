package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
)

type allowedEmailResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

func toAllowedEmailResponse(a domain.AllowedEmail) allowedEmailResponse {
	return allowedEmailResponse{
		ID: a.ID.String(), Email: a.Email, CreatedAt: a.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// handleListAllowedEmails возвращает список email с доступом к приватной странице (4.2.1).
func (s *server) handleListAllowedEmails(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "page")
	if !ok {
		return
	}
	if _, ok := s.authorizePage(w, r, id); !ok {
		return
	}
	emails, err := s.store.ListAllowedEmails(r.Context(), id)
	if err != nil {
		writeServerError(w, err)
		return
	}
	out := make([]allowedEmailResponse, len(emails))
	for i, e := range emails {
		out[i] = toAllowedEmailResponse(e)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleAddAllowedEmail добавляет email в список доступа.
func (s *server) handleAddAllowedEmail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "page")
	if !ok {
		return
	}
	page, ok := s.authorizePage(w, r, id)
	if !ok {
		return
	}
	// Списки доступа — часть приватных страниц (premium, этап 6.7).
	if plan, ok := s.accountPlan(w, r, page.AccountID); !ok {
		return
	} else if !requireFeature(w, plan, domain.FeaturePrivatePages) {
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" || !strings.Contains(email, "@") {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "укажите корректный email")
		return
	}
	created, err := s.store.AddAllowedEmail(r.Context(), id, email)
	if err != nil {
		if errors.Is(err, store.ErrEmailAlreadyAllowed) {
			writeError(w, http.StatusConflict, "email_exists", "адрес уже в списке доступа")
			return
		}
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAllowedEmailResponse(created))
}

// handleDeleteAllowedEmail удаляет email из списка доступа (авторизация по владению страницей).
func (s *server) handleDeleteAllowedEmail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	row, err := s.store.AllowedEmailByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "запись не найдена")
		} else {
			writeServerError(w, err)
		}
		return
	}
	if _, ok := s.authorizePage(w, r, row.StatusPageID); !ok {
		return
	}
	if err := s.store.DeleteAllowedEmail(r.Context(), id); err != nil {
		writeServerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
