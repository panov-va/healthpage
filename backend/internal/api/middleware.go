package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/healthpage/backend/internal/domain"
)

type ctxKey int

const userCtxKey ctxKey = iota

// requireAuth проверяет операторский access-токен (Authorization: Bearer <jwt>) и кладёт
// пользователя в контекст. При отсутствии/невалидности — 401.
func (s *server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "требуется Bearer-токен")
			return
		}
		user, err := s.auth.Authenticate(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "недействительный токен")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// userFromContext возвращает аутентифицированного пользователя из контекста.
func userFromContext(ctx context.Context) (domain.User, bool) {
	u, ok := ctx.Value(userCtxKey).(domain.User)
	return u, ok
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
