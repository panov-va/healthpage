package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
)

type ctxKey int

const principalCtxKey ctxKey = iota

// principal — аутентифицированный субъект управляющего запроса. Ровно одно из полей ненулевое:
// либо operator (операторский JWT, доступ ко всем страницам своего аккаунта), либо token
// (page-токен ApiToken, доступ только к своей странице в пределах своих scope'ов).
type principal struct {
	operator *domain.User
	token    *domain.APIToken
}

func (p principal) isToken() bool { return p.token != nil }

// requireAuth аутентифицирует управляющий запрос: операторский JWT (Authorization: Bearer <jwt>)
// ЛИБО page-токен (Authorization: <token>, без префикса Bearer — Статусмейт-совместимо).
// Для page-токена дополнительно проверяет scope по HTTP-методу (мутации требуют write).
// При отсутствии/невалидности — 401, при нехватке scope — 403.
func (s *server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := strings.TrimSpace(r.Header.Get("Authorization"))
		if h == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "требуется авторизация")
			return
		}

		// Операторский JWT — заголовок с префиксом Bearer.
		if jwt, ok := bearerToken(r); ok {
			user, err := s.auth.Authenticate(r.Context(), jwt)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized", "недействительный токен")
				return
			}
			next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), principal{operator: &user})))
			return
		}

		// Page-токен — сырое значение Authorization.
		tok, err := s.store.APITokenByHash(r.Context(), security.HashAPIToken(h))
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				log.Printf("auth: api token lookup: %v", err)
			}
			writeError(w, http.StatusUnauthorized, "unauthorized", "недействительный токен")
			return
		}
		if !tokenScopeAllows(tok, r.Method) {
			writeError(w, http.StatusForbidden, "forbidden", "токену не хватает scope для операции")
			return
		}
		// last_used_at — best-effort, ошибки не влияют на запрос.
		if err := s.store.TouchAPIToken(r.Context(), tok.ID); err != nil {
			log.Printf("auth: touch api token: %v", err)
		}
		next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), principal{token: &tok})))
	})
}

// tokenScopeAllows проверяет, что scope токена покрывает HTTP-метод: безопасные методы требуют
// read (write его подразумевает), мутации — write.
func tokenScopeAllows(t domain.APIToken, method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return t.HasScope(domain.ScopeRead)
	default:
		return t.HasScope(domain.ScopeWrite)
	}
}

func withPrincipal(ctx context.Context, p principal) context.Context {
	return context.WithValue(ctx, principalCtxKey, p)
}

// principalFromContext возвращает аутентифицированный субъект из контекста.
func principalFromContext(ctx context.Context) (principal, bool) {
	p, ok := ctx.Value(principalCtxKey).(principal)
	return p, ok
}

// userFromContext возвращает оператора из контекста (для account-level эндпоинтов).
// Для page-токена ok=false (у токена нет пользователя).
func userFromContext(ctx context.Context) (domain.User, bool) {
	p, ok := principalFromContext(ctx)
	if !ok || p.operator == nil {
		return domain.User{}, false
	}
	return *p.operator, true
}

// requireOperator извлекает оператора и для page-токена пишет 403 (операция доступна только
// оператору: управление страницами/токенами, профиль). Возвращает false, если ответ уже записан.
func requireOperator(w http.ResponseWriter, r *http.Request) (domain.User, bool) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusForbidden, "forbidden", "операция доступна только оператору, не API-токену")
		return domain.User{}, false
	}
	return user, true
}

// bearerToken извлекает токен из заголовка Authorization: Bearer <token>.
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(h[len(prefix):])
	return token, token != ""
}
