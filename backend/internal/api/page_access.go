package api

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

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

	writeAccessGrant(w, s.subSecret, page.ID)
}

// writeAccessGrant выдаёт токен доступа к приватной странице (общий для пароля и magic-link).
func writeAccessGrant(w http.ResponseWriter, secret string, pageID uuid.UUID) {
	expires := time.Now().Add(subscription.PageAccessTTL)
	token := subscription.PageAccessToken(secret, pageID, expires.Unix())
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": token,
		"expires_in":   int(subscription.PageAccessTTL.Seconds()),
	})
}

// handleRequestAccessLink (4.2.1): если email в списке доступа приватной страницы — отправляет
// письмо magic-link. Всегда 202 (не раскрывает, разрешён ли адрес). Публичная страница → 404.
func (s *server) handleRequestAccessLink(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, http.StatusNotFound, "not_found", "страница не требует доступа")
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "email обязателен")
		return
	}

	// Адрес разрешён + рассылка доступна → шлём ссылку. Иначе молча 202 (анти-энумерация).
	if allowed, err := s.store.IsEmailAllowed(r.Context(), page.ID, email); err == nil && allowed && s.notifier != nil {
		expires := time.Now().Add(subscription.AccessLinkTTL)
		token := subscription.AccessLinkToken(s.subSecret, page.ID, email, expires.Unix())
		if err := s.notifier.SendAccessLink(r.Context(), page.ID, email, token); err != nil {
			// Не раскрываем сбой клиенту — лог на сервере, ответ всё равно 202.
			log.Printf("access link: send failed: %v", err)
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

// handleVerifyAccessLink (4.2.1): обменивает токен из письма magic-link на токен доступа.
func (s *server) handleVerifyAccessLink(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "page")
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusUnauthorized, "invalid_token", "токен не указан")
		return
	}
	page, err := s.store.StatusPageBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "страница не найдена")
		} else {
			writeServerError(w, err)
		}
		return
	}
	pageID, email, err := subscription.ParseAccessLinkToken(s.subSecret, token, time.Now())
	if err != nil || pageID != page.ID {
		writeError(w, http.StatusUnauthorized, "invalid_token", "ссылка недействительна или истекла")
		return
	}
	// Адрес мог быть удалён из списка после выпуска ссылки — проверяем заново.
	allowed, err := s.store.IsEmailAllowed(r.Context(), page.ID, email)
	if err != nil {
		writeServerError(w, err)
		return
	}
	if !allowed {
		writeError(w, http.StatusUnauthorized, "invalid_token", "доступ для адреса отозван")
		return
	}
	writeAccessGrant(w, s.subSecret, page.ID)
}
