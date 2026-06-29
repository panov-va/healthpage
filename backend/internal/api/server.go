package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/healthpage/backend/internal/auth"
	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
)

const (
	refreshCookieName = "hp_refresh"
	authBasePath      = "/api/v1/auth"
)

// Deps — зависимости HTTP-слоя.
type Deps struct {
	Auth       *auth.Service
	Store      *store.Store
	Prod       bool          // влияет на флаг Secure у refresh-cookie
	RefreshTTL time.Duration // срок жизни refresh-cookie
}

type server struct {
	auth       *auth.Service
	store      *store.Store
	prod       bool
	refreshTTL time.Duration
}

// NewRouter собирает корневой роутер: служебный /healthz и /api/v1/* (auth, управление страницами/компонентами).
func NewRouter(d Deps) http.Handler {
	s := &server{auth: d.Auth, store: d.Store, prod: d.Prod, refreshTTL: d.RefreshTTL}

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

		// Публичные read-only эндпоинты (без авторизации). Параметр сегмента страницы — {page}
		// (единое имя для chi); здесь трактуется как slug.
		r.Get("/pages/{page}/summary", s.handlePublicSummary)
		r.Get("/pages/{page}/components", s.handlePublicComponents)
		r.Get("/pages/{page}/incidents", s.handlePublicIncidents)
		r.Get("/pages/{page}/incidents/{id}", s.handlePublicIncidentDetail)
		r.Get("/pages/{page}/maintenances", s.handlePublicMaintenances)

		// Управляющие эндпоинты — только по операторскому JWT (ApiToken — этап 5).
		// {page} здесь трактуется как uuid.
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)

			r.Get("/pages", s.handleListPages)
			r.Post("/pages", s.handleCreatePage)
			r.Get("/pages/{page}", s.handleGetPage)
			r.Patch("/pages/{page}", s.handlePatchPage)
			r.Delete("/pages/{page}", s.handleDeletePage)

			r.Get("/pages/{page}/component-groups", s.handleListGroups)
			r.Post("/pages/{page}/component-groups", s.handleCreateGroup)
			r.Patch("/component-groups/{id}", s.handlePatchGroup)
			r.Delete("/component-groups/{id}", s.handleDeleteGroup)

			r.Get("/components", s.handleListComponents)
			r.Post("/components", s.handleCreateComponent)
			r.Patch("/components/{id}", s.handlePatchComponent)
			r.Delete("/components/{id}", s.handleDeleteComponent)

			r.Get("/incidents", s.handleListIncidents)
			r.Post("/incidents", s.handleCreateIncident)
			r.Get("/incidents/{id}", s.handleGetIncident)
			r.Patch("/incidents/{id}", s.handlePatchIncident)
			r.Delete("/incidents/{id}", s.handleDeleteIncident)
			r.Post("/incidents/{id}/updates", s.handleAddIncidentUpdate)

			r.Get("/incident-templates", s.handleListIncidentTemplates)
			r.Post("/incident-templates", s.handleCreateIncidentTemplate)
			r.Get("/incident-templates/{id}", s.handleGetIncidentTemplate)
			r.Patch("/incident-templates/{id}", s.handlePatchIncidentTemplate)
			r.Delete("/incident-templates/{id}", s.handleDeleteIncidentTemplate)

			r.Get("/maintenances", s.handleListMaintenances)
			r.Post("/maintenances", s.handleCreateMaintenance)
			r.Get("/maintenances/{id}", s.handleGetMaintenance)
			r.Patch("/maintenances/{id}", s.handlePatchMaintenance)
			r.Delete("/maintenances/{id}", s.handleDeleteMaintenance)
			r.Post("/maintenances/{id}/updates", s.handleAddMaintenanceUpdate)
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
