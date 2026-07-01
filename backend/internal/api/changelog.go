package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
)

// ── DTO changelog (синхронно с openapi ChangelogEntry / Create / Patch) ──

type changelogResponse struct {
	ID           string  `json:"id"`
	StatusPageID string  `json:"status_page_id"`
	Title        string  `json:"title"`
	Body         string  `json:"body"`
	Published    bool    `json:"published"`
	PublishedAt  *string `json:"published_at"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

func toChangelogResponse(e domain.ChangelogEntry) changelogResponse {
	return changelogResponse{
		ID:           e.ID.String(),
		StatusPageID: e.StatusPageID.String(),
		Title:        e.Title,
		Body:         e.Body,
		Published:    e.Published,
		PublishedAt:  rfc3339Ptr(e.PublishedAt),
		CreatedAt:    e.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    e.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

type changelogCreateRequest struct {
	StatusPageID string `json:"status_page_id"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	Published    bool   `json:"published"`
}

// changelogPatchRequest — частичное обновление; nil-поля не трогаем.
type changelogPatchRequest struct {
	Title     *string `json:"title"`
	Body      *string `json:"body"`
	Published *bool   `json:"published"`
}

// publishedAtFor вычисляет published_at по флагу публикации: при публикации проставляет now
// (если ещё не было), при снятии — очищает.
func publishedAtFor(existing *time.Time, published bool, now time.Time) *time.Time {
	if !published {
		return nil
	}
	if existing != nil {
		return existing
	}
	return &now
}

// handleListChangelog возвращает записи changelog страницы (админ, включая черновики).
func (s *server) handleListChangelog(w http.ResponseWriter, r *http.Request) {
	page, ok := s.resolveManagedPage(w, r, r.URL.Query().Get("status_page_id"))
	if !ok {
		return
	}
	entries, err := s.store.ListChangelogByPage(r.Context(), page.ID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	out := make([]changelogResponse, len(entries))
	for i, e := range entries {
		out[i] = toChangelogResponse(e)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleCreateChangelog создаёт запись changelog.
func (s *server) handleCreateChangelog(w http.ResponseWriter, r *http.Request) {
	var req changelogCreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	page, ok := s.resolveManagedPage(w, r, req.StatusPageID)
	if !ok {
		return
	}
	if !domain.ValidateChangelogTitle(req.Title) {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "title обязателен")
		return
	}
	publishedAt := publishedAtFor(nil, req.Published, time.Now().UTC())
	entry, err := s.store.CreateChangelogEntry(r.Context(), page.ID, strings.TrimSpace(req.Title), req.Body, req.Published, publishedAt)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toChangelogResponse(entry))
}

// loadAuthorizedChangelog загружает запись и проверяет доступ субъекта к её странице.
func (s *server) loadAuthorizedChangelog(w http.ResponseWriter, r *http.Request) (domain.ChangelogEntry, bool) {
	id, ok := pathUUID(w, r, "id")
	if !ok {
		return domain.ChangelogEntry{}, false
	}
	entry, err := s.store.ChangelogEntryByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "запись не найдена")
		} else {
			writeServerError(w, err)
		}
		return domain.ChangelogEntry{}, false
	}
	if _, ok := s.authorizePage(w, r, entry.StatusPageID); !ok {
		return domain.ChangelogEntry{}, false
	}
	return entry, true
}

// handleGetChangelog возвращает запись по id.
func (s *server) handleGetChangelog(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.loadAuthorizedChangelog(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toChangelogResponse(entry))
}

// handlePatchChangelog частично обновляет запись (в т.ч. публикует/снимает с публикации).
func (s *server) handlePatchChangelog(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.loadAuthorizedChangelog(w, r)
	if !ok {
		return
	}
	var req changelogPatchRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Title != nil {
		if !domain.ValidateChangelogTitle(*req.Title) {
			writeError(w, http.StatusUnprocessableEntity, "invalid_request", "title не может быть пустым")
			return
		}
		entry.Title = strings.TrimSpace(*req.Title)
	}
	if req.Body != nil {
		entry.Body = *req.Body
	}
	if req.Published != nil {
		entry.Published = *req.Published
	}
	publishedAt := publishedAtFor(entry.PublishedAt, entry.Published, time.Now().UTC())
	updated, err := s.store.UpdateChangelogEntry(r.Context(), entry.ID, entry.Title, entry.Body, entry.Published, publishedAt)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toChangelogResponse(updated))
}

// handleDeleteChangelog удаляет запись.
func (s *server) handleDeleteChangelog(w http.ResponseWriter, r *http.Request) {
	entry, ok := s.loadAuthorizedChangelog(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteChangelogEntry(r.Context(), entry.ID); err != nil {
		writeServerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handlePublicChangelog — публичная лента релизов (только опубликованные).
func (s *server) handlePublicChangelog(w http.ResponseWriter, r *http.Request) {
	page, ok := s.loadPublicPage(w, r)
	if !ok {
		return
	}
	_, perPage, offset := parsePagination(r)
	entries, err := s.store.ListPublishedChangelog(r.Context(), page.ID, perPage, offset)
	if err != nil {
		writeServerError(w, err)
		return
	}
	out := make([]changelogResponse, len(entries))
	for i, e := range entries {
		out[i] = toChangelogResponse(e)
	}
	writeJSON(w, http.StatusOK, out)
}
