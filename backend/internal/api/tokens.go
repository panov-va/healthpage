package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
)

// ── DTO токенов (синхронны с openapi Token / TokenCreate / TokenCreated) ──

type tokenCreateRequest struct {
	Name         string   `json:"name"`
	StatusPageID string   `json:"status_page_id"`
	Scopes       []string `json:"scopes"`
}

type tokenCreatedResponse struct {
	ID           string   `json:"id"`
	StatusPageID string   `json:"status_page_id"`
	Name         string   `json:"name"`
	Token        string   `json:"token"`
	Scopes       []string `json:"scopes"`
}

type tokenResponse struct {
	ID           string   `json:"id"`
	StatusPageID string   `json:"status_page_id"`
	Name         string   `json:"name"`
	Scopes       []string `json:"scopes"`
	LastUsedAt   *string  `json:"last_used_at"`
	CreatedAt    string   `json:"created_at"`
}

func toTokenResponse(t domain.APIToken) tokenResponse {
	var lastUsed *string
	if t.LastUsedAt != nil {
		v := t.LastUsedAt.UTC().Format(time.RFC3339)
		lastUsed = &v
	}
	return tokenResponse{
		ID:           t.ID.String(),
		StatusPageID: t.StatusPageID.String(),
		Name:         t.Name,
		Scopes:       scopeStrings(t.Scopes),
		LastUsedAt:   lastUsed,
		CreatedAt:    t.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func scopeStrings(scopes []domain.TokenScope) []string {
	out := make([]string, len(scopes))
	for i, s := range scopes {
		out[i] = string(s)
	}
	return out
}

// handleCreateToken создаёт API-токен страницы. Только оператор (не сам токен).
// Значение токена возвращается единожды.
func (s *server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireOperator(w, r); !ok {
		return
	}
	var req tokenCreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "name обязателен")
		return
	}
	pageID, err := uuid.Parse(req.StatusPageID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "требуется status_page_id (uuid)")
		return
	}
	if _, ok := s.authorizePage(w, r, pageID); !ok {
		return
	}
	scopes, ok := domain.NormalizeScopes(req.Scopes)
	if !ok {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый scope (допустимо: read, write)")
		return
	}
	// Пустой набор → полный доступ (как у Статусмейт; токен создаёт владелец страницы).
	if len(scopes) == 0 {
		scopes = []domain.TokenScope{domain.ScopeRead, domain.ScopeWrite}
	}

	plain, hash, err := security.GenerateAPIToken()
	if err != nil {
		writeServerError(w, err)
		return
	}
	tok, err := s.store.CreateAPIToken(r.Context(), pageID, hash, req.Name, scopes)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tokenCreatedResponse{
		ID:           tok.ID.String(),
		StatusPageID: tok.StatusPageID.String(),
		Name:         tok.Name,
		Token:        plain,
		Scopes:       scopeStrings(tok.Scopes),
	})
}

// handleListTokens возвращает токены страницы (без значений). Требует ?status_page_id. Только оператор.
func (s *server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireOperator(w, r); !ok {
		return
	}
	pageID, err := uuid.Parse(r.URL.Query().Get("status_page_id"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "требуется status_page_id (uuid)")
		return
	}
	if _, ok := s.authorizePage(w, r, pageID); !ok {
		return
	}
	tokens, err := s.store.ListAPITokensByPage(r.Context(), pageID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	out := make([]tokenResponse, len(tokens))
	for i, t := range tokens {
		out[i] = toTokenResponse(t)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleDeleteToken отзывает токен по id. Только оператор, авторизация по владению страницей.
func (s *server) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireOperator(w, r); !ok {
		return
	}
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return
	}
	tok, err := s.store.APITokenByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "токен не найден")
		} else {
			writeServerError(w, err)
		}
		return
	}
	if _, ok := s.authorizePage(w, r, tok.StatusPageID); !ok {
		return
	}
	if err := s.store.DeleteAPIToken(r.Context(), id); err != nil {
		writeServerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
