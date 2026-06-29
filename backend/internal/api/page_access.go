package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
	"github.com/healthpage/backend/internal/subscription"
)

// handlePageAccess проверяет пароль приватной страницы и выдаёт токен доступа (этап 4.2).
// Токен передаётся посетителем в заголовке X-Page-Access на публичных read-эндпоинтах.
// Публичная (не приватная) страница пароля не требует → 404. Неверный пароль / страница без
// заданного пароля → 401 (без раскрытия деталей конфигурации).
func (s *server) handlePageAccess(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "page")
	page, err := s.store.StatusPageBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "страница не найдена")
		} else {
			writeServerError(w, err)
		}
		return
	}
	if !page.IsPrivate() {
		writeError(w, http.StatusNotFound, "not_found", "страница не требует пароля")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if page.PasswordHash == nil || req.Password == "" {
		writeError(w, http.StatusUnauthorized, "invalid_password", "неверный пароль")
		return
	}
	ok, err := security.VerifyPassword(req.Password, *page.PasswordHash)
	if err != nil || !ok {
		writeError(w, http.StatusUnauthorized, "invalid_password", "неверный пароль")
		return
	}

	expires := time.Now().Add(subscription.PageAccessTTL)
	token := subscription.PageAccessToken(s.subSecret, page.ID, expires.Unix())
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": token,
		"expires_in":   int(subscription.PageAccessTTL.Seconds()),
	})
}
