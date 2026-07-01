// Package api реализует HTTP-слой HealthPage: маршрутизацию, хендлеры и middleware
// поверх доменной логики и store (CLAUDE.md §7 — наружу из backend не экспортируется).
package api

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/healthpage/backend/internal/auth"
	"github.com/healthpage/backend/internal/billing"
	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/metrics"
	"github.com/healthpage/backend/internal/notify"
	"github.com/healthpage/backend/internal/slack"
	"github.com/healthpage/backend/internal/store"
)

const (
	refreshCookieName = "hp_refresh"
	authBasePath      = "/api/v1/auth"
)

// Deps — зависимости HTTP-слоя.
type Deps struct {
	Auth            *auth.Service
	Store           *store.Store
	Notifier        *notify.Engine   // движок уведомлений; nil — рассылка отключена (RabbitMQ недоступен)
	SubSecret       string           // секрет HMAC-токенов отписки (должен совпадать с worker-email)
	BaseURL         string           // базовый URL для ссылок в фидах/письмах
	SlackOAuth      *slack.OAuth     // OAuth-клиент Slack; nil — подписка Slack выключена
	Billing         *billing.Service // сервис биллинга (этап 6); nil — эндпоинты /billing/* отвечают 503
	ImportPublisher ImportPublisher  // публикатор задач импорта (этап 7.5); nil — /import отвечает 503
	Prod            bool             // влияет на флаг Secure у refresh-cookie
	RefreshTTL      time.Duration    // срок жизни refresh-cookie

	// Кастомные домены (этап 4.3): целевой хост для CNAME и резолвер для верификации.
	// CNAMEResolver nil → используется net.DefaultResolver.LookupCNAME (тесты инъектируют фейк).
	CNAMETarget   string
	CNAMEResolver func(ctx context.Context, host string) (string, error)
}

type server struct {
	auth            *auth.Service
	store           *store.Store
	notifier        *notify.Engine
	subSecret       string
	baseURL         string
	slackOAuth      *slack.OAuth
	billing         *billing.Service
	importPublisher ImportPublisher
	prod            bool
	refreshTTL      time.Duration
	cnameTarget     string
	cnameResolver   func(ctx context.Context, host string) (string, error)
}

// NewRouter собирает корневой роутер: служебный /healthz и /api/v1/* (auth, управление страницами/компонентами).
func NewRouter(d Deps) http.Handler {
	s := &server{auth: d.Auth, store: d.Store, notifier: d.Notifier, subSecret: d.SubSecret, baseURL: d.BaseURL, slackOAuth: d.SlackOAuth, billing: d.Billing, importPublisher: d.ImportPublisher, prod: d.Prod, refreshTTL: d.RefreshTTL, cnameTarget: d.CNAMETarget, cnameResolver: d.CNAMEResolver}
	if s.cnameResolver == nil {
		s.cnameResolver = net.DefaultResolver.LookupCNAME
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(metrics.Middleware) // учёт HTTP-метрик (этап 7.3)

	r.Get("/healthz", healthz)
	r.Handle("/metrics", metrics.Handler()) // Prometheus-скрейпинг (этап 7.3)

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
		r.Post("/pages/{page}/access", s.handlePageAccess)
		r.Post("/pages/{page}/access/request-link", s.handleRequestAccessLink)
		r.Get("/pages/{page}/access/verify", s.handleVerifyAccessLink)
		r.Get("/pages/{page}/summary", s.handlePublicSummary)
		r.Get("/pages/{page}/components", s.handlePublicComponents)
		r.Get("/pages/{page}/incidents", s.handlePublicIncidents)
		r.Get("/pages/{page}/incidents/{id}", s.handlePublicIncidentDetail)
		r.Get("/pages/{page}/maintenances", s.handlePublicMaintenances)
		r.Get("/pages/{page}/uptime", s.handleUptime)
		r.Get("/pages/{page}/changelog", s.handlePublicChangelog)

		// Подписки (этап 3.5): публичные, без авторизации.
		r.Post("/pages/{page}/subscribe", s.handleSubscribe)
		r.Get("/subscribe/confirm", s.handleConfirmSubscribe)
		r.Get("/unsubscribe", s.handleUnsubscribe)

		// Подписка Slack через OAuth (этап 3.9): публичные.
		r.Get("/pages/{page}/subscribe/slack/start", s.handleSlackStart)
		r.Get("/subscribe/slack/callback", s.handleSlackCallback)

		// Публичные фиды (этап 3.6).
		r.Get("/pages/{page}/badge.svg", s.handleBadge)
		r.Get("/pages/{page}/rss", s.handleRSS)
		r.Get("/pages/{page}/calendar.ics", s.handleICal)

		// Входящие webhook'и интеграций (этап 5.3): аутентификация по HMAC-подписи (X-Signature),
		// не по JWT/ApiToken. generic/pagerduty отложены (501).
		r.Post("/integrations/{integration_id}/grafana", s.handleGrafanaWebhook)
		r.Post("/integrations/{integration_id}/prometheus", s.handlePrometheusWebhook)
		r.Post("/integrations/{integration_id}/generic", s.handleGenericWebhook)
		r.Post("/integrations/{integration_id}/pagerduty", s.handlePagerDutyWebhook)

		// Webhook платёжного провайдера (этап 6): публичный, подлинность проверяет провайдер.
		r.Post("/billing/webhook/{provider}", s.handleBillingWebhook)

		// Управляющие эндпоинты — только по операторскому JWT (ApiToken — этап 5).
		// {page} здесь трактуется как uuid.
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)

			r.Get("/pages", s.handleListPages)
			r.Post("/pages", s.handleCreatePage)
			r.Get("/pages/{page}", s.handleGetPage)
			r.Patch("/pages/{page}", s.handlePatchPage)
			r.Delete("/pages/{page}", s.handleDeletePage)
			r.Post("/pages/{page}/domain/verify", s.handleVerifyDomain)
			r.Get("/pages/{page}/allowed-emails", s.handleListAllowedEmails)
			r.Post("/pages/{page}/allowed-emails", s.handleAddAllowedEmail)
			r.Delete("/allowed-emails/{id}", s.handleDeleteAllowedEmail)

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

			r.Get("/changelog", s.handleListChangelog)
			r.Post("/changelog", s.handleCreateChangelog)
			r.Get("/changelog/{id}", s.handleGetChangelog)
			r.Patch("/changelog/{id}", s.handlePatchChangelog)
			r.Delete("/changelog/{id}", s.handleDeleteChangelog)

			r.Get("/maintenances", s.handleListMaintenances)
			r.Post("/maintenances", s.handleCreateMaintenance)
			r.Get("/maintenances/{id}", s.handleGetMaintenance)
			r.Patch("/maintenances/{id}", s.handlePatchMaintenance)
			r.Delete("/maintenances/{id}", s.handleDeleteMaintenance)
			r.Post("/maintenances/{id}/updates", s.handleAddMaintenanceUpdate)

			r.Get("/subscribers", s.handleListSubscribers)
			r.Post("/subscribers", s.handleCreateSubscriber)
			r.Delete("/subscribers/{id}", s.handleDeleteSubscriber)

			// API-токены страницы (этап 5.1). Управление — только оператор (JWT), не сам токен.
			r.Get("/tokens", s.handleListTokens)
			r.Post("/tokens", s.handleCreateToken)
			r.Delete("/tokens/{id}", s.handleDeleteToken)

			// Биллинг (этап 6). Управление — только оператор (account-level).
			r.Get("/billing/subscription", s.handleGetSubscription)
			r.Post("/billing/checkout", s.handleCheckout)
			r.Post("/billing/cancel", s.handleCancelSubscription)
			r.Get("/billing/payments", s.handleListPayments)

			// Импорт из внешних сервисов (этап 7.5). Только оператор (account-level).
			r.Post("/import", s.handleStartImport)
			r.Get("/import/{job_id}", s.handleGetImportJob)

			// Webhook-интеграции (этап 5.3). Управление — только оператор (минтит секрет).
			r.Get("/webhook-integrations", s.handleListWebhookIntegrations)
			r.Post("/webhook-integrations", s.handleCreateWebhookIntegration)
			r.Get("/webhook-integrations/{id}", s.handleGetWebhookIntegration)
			r.Patch("/webhook-integrations/{id}", s.handlePatchWebhookIntegration)
			r.Delete("/webhook-integrations/{id}", s.handleDeleteWebhookIntegration)
		})
	})

	return r
}

// healthz — liveness-проба: отвечает 200, если процесс жив.
func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// emitNotify выполняет рассылку события f, если движок настроен (nil-safe). Ошибки логируются, но
// не влияют на ответ API — операция над инцидентом/работой уже зафиксирована, а записи журнала
// остаются pending и восстановимы. Синхронно: для объёмов MVP публикация в брокер дёшева.
func (s *server) emitNotify(f func() error) {
	if s.notifier == nil {
		return
	}
	if err := f(); err != nil {
		log.Printf("notify: dispatch failed: %v", err)
	}
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
