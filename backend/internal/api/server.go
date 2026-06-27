package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/healthpage/backend/internal/auth"
	"github.com/healthpage/backend/internal/domain"
)

const (
	refreshCookieName = "hp_refresh"
	authBasePath      = "/api/v1/auth"
)

// Deps — зависимости HTTP-слоя.
type Deps struct {
	Auth       *auth.Service
	Prod       bool          // влияет на флаг Secure у refresh-cookie
	RefreshTTL time.Duration // срок жизни refresh-cookie
}

type server struct {
	auth       *auth.Service
	prod       bool
	refreshTTL time.Duration
}

// NewRouter собирает корневой роутер: служебный /healthz и /api/v1/* (auth на этапе 1.3).
func NewRouter(d Deps) http.Handler {
	s := &server{auth: d.Auth, prod: d.Prod, refreshTTL: d.RefreshTTL}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", healthz)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", s.handleRegister)
			r.Post("/login", s.handleLogin)
			r.Post("/refresh", s.handleRefresh)
			r.Post("/logout", s.handleLogout)
			r.With(s.requireAuth).Get("/me", s.handleMe)
		})
	})

	return r
}

// healthz — liveness-проба: отвечает 200, если процесс жив.
func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ── DTO (синхронны с openapi; конформность закрывается контрактными тестами) ──

type authUser struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Locale string `json:"locale"`
}

type authResultResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken *string  `json:"refresh_token"` // null: refresh отдаётся в httpOnly-cookie
	TokenType    string   `json:"token_type"`
	ExpiresIn    int      `json:"expires_in"`
	User         authUser `json:"user"`
}

func toAuthUser(u domain.User) authUser {
	return authUser{ID: u.ID.String(), Email: u.Email, Name: u.Name, Locale: u.Locale}
}

// writeAuthResult ставит refresh-cookie и отдаёт тело AuthResult (без refresh в теле).
func (s *server) writeAuthResult(w http.ResponseWriter, status int, res auth.Result) {
	s.setRefreshCookie(w, res.RefreshToken)
	writeJSON(w, status, authResultResponse{
		AccessToken: res.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   res.ExpiresIn,
		User:        toAuthUser(res.User),
	})
}

func (s *server) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     authBasePath,
		HttpOnly: true,
		Secure:   s.prod,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(s.refreshTTL.Seconds()),
	})
}

func (s *server) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     authBasePath,
		HttpOnly: true,
		Secure:   s.prod,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
