package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
)

// pathUUID парсит uuid из параметра пути. При ошибке пишет 400 и возвращает false.
func pathUUID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, key))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "некорректный идентификатор в пути")
		return uuid.Nil, false
	}
	return id, true
}

// authorizePage загружает страницу и проверяет, что текущий оператор владеет её аккаунтом.
// Чтобы не раскрывать существование чужих страниц, при отсутствии доступа возвращается 404.
func (s *server) authorizePage(w http.ResponseWriter, r *http.Request, pageID uuid.UUID) (domain.StatusPage, bool) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "не аутентифицирован")
		return domain.StatusPage{}, false
	}
	page, err := s.store.StatusPageByID(r.Context(), pageID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "страница не найдена")
		} else {
			writeServerError(w, err)
		}
		return domain.StatusPage{}, false
	}
	acc, err := s.store.AccountByOwner(r.Context(), user.ID)
	if err != nil || page.AccountID != acc.ID {
		writeError(w, http.StatusNotFound, "not_found", "страница не найдена")
		return domain.StatusPage{}, false
	}
	return page, true
}

// writeServerError логирует и отдаёт 500 в формате контракта.
func writeServerError(w http.ResponseWriter, _ error) {
	writeError(w, http.StatusInternalServerError, "internal", "внутренняя ошибка")
}
