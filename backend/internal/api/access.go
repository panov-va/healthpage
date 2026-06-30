package api

import (
	"context"
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

// authorizePage загружает страницу и проверяет, что текущий субъект имеет к ней доступ:
// оператор — владеет её аккаунтом; page-токен — привязан именно к этой странице.
// Чтобы не раскрывать существование чужих страниц, при отсутствии доступа возвращается 404.
func (s *server) authorizePage(w http.ResponseWriter, r *http.Request, pageID uuid.UUID) (domain.StatusPage, bool) {
	p, ok := principalFromContext(r.Context())
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
	if !s.principalOwnsPage(r.Context(), p, page) {
		writeError(w, http.StatusNotFound, "not_found", "страница не найдена")
		return domain.StatusPage{}, false
	}
	return page, true
}

// resolveManagedPage определяет и авторизует страницу управляющего запроса по значению
// status_page_id (из query или тела; "" — не передано).
//   - Оператор (JWT): status_page_id обязателен → 422 при отсутствии/невалидности.
//   - Page-токен (ApiToken): страница берётся из токена; переданный status_page_id должен
//     совпадать с ней, иначе 404 (чужая/несуществующая страница).
//
// При ошибке сам пишет ответ и возвращает ok=false. Возвращает загруженную страницу.
func (s *server) resolveManagedPage(w http.ResponseWriter, r *http.Request, raw string) (domain.StatusPage, bool) {
	p, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "не аутентифицирован")
		return domain.StatusPage{}, false
	}
	if p.isToken() {
		if raw != "" {
			id, err := uuid.Parse(raw)
			if err != nil || id != p.token.StatusPageID {
				writeError(w, http.StatusNotFound, "not_found", "страница не найдена")
				return domain.StatusPage{}, false
			}
		}
		return s.authorizePage(w, r, p.token.StatusPageID)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "требуется status_page_id (uuid)")
		return domain.StatusPage{}, false
	}
	return s.authorizePage(w, r, id)
}

// principalOwnsPage сообщает, имеет ли субъект доступ к странице.
func (s *server) principalOwnsPage(ctx context.Context, p principal, page domain.StatusPage) bool {
	if p.isToken() {
		return p.token.StatusPageID == page.ID
	}
	if p.operator == nil {
		return false
	}
	acc, err := s.store.AccountByOwner(ctx, p.operator.ID)
	return err == nil && page.AccountID == acc.ID
}

// writeServerError логирует и отдаёт 500 в формате контракта.
func writeServerError(w http.ResponseWriter, _ error) {
	writeError(w, http.StatusInternalServerError, "internal", "внутренняя ошибка")
}
