package api

import (
	"errors"
	"net/http"

	"github.com/healthpage/backend/internal/auth"
)

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	Name        string `json:"name"`
	AccountName string `json:"account_name"`
	Locale      string `json:"locale"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (s *server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := s.auth.Register(r.Context(), auth.RegisterParams{
		Email:       req.Email,
		Password:    req.Password,
		Name:        req.Name,
		AccountName: req.AccountName,
		Locale:      req.Locale,
	})
	switch {
	case errors.Is(err, auth.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", "email уже зарегистрирован")
		return
	case errors.Is(err, auth.ErrWeakPassword):
		writeError(w, http.StatusUnprocessableEntity, "weak_password", "пароль слишком короткий (мин. 8)")
		return
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "некорректный email")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", "внутренняя ошибка")
		return
	}
	s.writeAuthResult(w, http.StatusCreated, res)
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := s.auth.Login(r.Context(), req.Email, req.Password)
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "неверный email или пароль")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", "внутренняя ошибка")
		return
	}
	s.writeAuthResult(w, http.StatusOK, res)
}

func (s *server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	token := s.readRefreshToken(r)
	res, err := s.auth.Refresh(r.Context(), token)
	switch {
	case errors.Is(err, auth.ErrInvalidRefresh):
		s.clearRefreshCookie(w)
		writeError(w, http.StatusUnauthorized, "invalid_refresh", "недействительный refresh-токен")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", "внутренняя ошибка")
		return
	}
	s.writeAuthResult(w, http.StatusOK, res)
}

func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := s.readRefreshToken(r)
	if err := s.auth.Logout(r.Context(), token); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "внутренняя ошибка")
		return
	}
	s.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "не аутентифицирован")
		return
	}
	writeJSON(w, http.StatusOK, toAuthUser(user))
}

// readRefreshToken берёт refresh-токен из httpOnly-cookie, затем из тела запроса.
func (s *server) readRefreshToken(r *http.Request) string {
	if c, err := r.Cookie(refreshCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	var req refreshRequest
	if r.Body != nil {
		_ = decodeBodyQuiet(r, &req)
	}
	return req.RefreshToken
}
