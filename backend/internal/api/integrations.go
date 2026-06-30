package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
	"github.com/healthpage/backend/internal/webhook"
)

// maxWebhookBody — лимит тела входящего webhook'а (защита от больших payload'ов).
const maxWebhookBody = 1 << 20 // 1 MiB

// handleGrafanaWebhook / handlePrometheusWebhook — входящие алерты (этап 5.3). Аутентификация по
// HMAC-подписи (X-Signature) секретом интеграции; создание/закрытие инцидентов идемпотентно по
// dedup-ключу из payload. generic/pagerduty отложены (501).
func (s *server) handleGrafanaWebhook(w http.ResponseWriter, r *http.Request) {
	s.handleInboundWebhook(w, r, domain.SourceGrafana, webhook.ParseGrafana)
}

func (s *server) handlePrometheusWebhook(w http.ResponseWriter, r *http.Request) {
	s.handleInboundWebhook(w, r, domain.SourcePrometheus, webhook.ParsePrometheus)
}

func (s *server) handleGenericWebhook(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "generic webhook отложен (этап 5.3)")
}

func (s *server) handlePagerDutyWebhook(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "pagerduty webhook отложен (этап 5.3)")
}

type alertParser func([]byte) ([]webhook.Alert, error)

func (s *server) handleInboundWebhook(w http.ResponseWriter, r *http.Request, source domain.WebhookSource, parse alertParser) {
	id, err := uuid.Parse(chi.URLParam(r, "integration_id"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "недействительная интеграция")
		return
	}
	integration, err := s.store.WebhookIntegrationByID(r.Context(), id)
	if err != nil {
		// Не раскрываем существование интеграции — всегда 401 при проблемах аутентификации.
		writeError(w, http.StatusUnauthorized, "unauthorized", "недействительная интеграция")
		return
	}
	// Источник интеграции должен совпадать с роутом.
	if integration.Source != source {
		writeError(w, http.StatusUnauthorized, "unauthorized", "источник интеграции не совпадает")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "не удалось прочитать тело")
		return
	}
	if !webhook.VerifySignature(integration.Secret, body, r.Header.Get("X-Signature")) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "неверная подпись")
		return
	}

	alerts, err := parse(body)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "не удалось разобрать payload")
		return
	}
	mapping, err := webhook.ParseMapping(integration.ComponentMapping)
	if err != nil {
		writeServerError(w, err)
		return
	}
	pageComponents, err := s.pageComponentSet(r.Context(), integration.StatusPageID)
	if err != nil {
		writeServerError(w, err)
		return
	}

	for _, a := range alerts {
		if err := s.ingestAlert(r.Context(), integration, mapping, pageComponents, a); err != nil {
			writeServerError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

// ingestAlert идемпотентно применяет один алерт: firing → создать инцидент (если открытого с этим
// dedup-ключом ещё нет), resolved → закрыть открытый. Повторные доставки не плодят дубли.
func (s *server) ingestAlert(
	ctx context.Context, integration domain.WebhookIntegration, mapping webhook.Mapping,
	pageComponents map[uuid.UUID]bool, a webhook.Alert,
) error {
	if a.DedupKey == "" {
		return nil // без ключа идемпотентность невозможна — игнорируем
	}
	existing, err := s.store.OpenIncidentByDedup(ctx, integration.StatusPageID, a.DedupKey)
	switch {
	case err == nil:
		// Открытый инцидент уже есть.
		if a.Firing {
			return nil // повторный firing — no-op
		}
		// resolved → закрыть.
		now := time.Now().UTC()
		upd, updated, uerr := s.store.AddIncidentUpdate(ctx, existing.ID, domain.IncidentResolved, a.Body, true, now)
		if uerr != nil {
			return uerr
		}
		s.emitNotify(func() error { return s.notifier.IncidentUpdated(ctx, updated, upd) })
		return nil
	case errors.Is(err, store.ErrNotFound):
		if !a.Firing {
			return nil // resolved без открытого инцидента — no-op
		}
		return s.createWebhookIncident(ctx, integration, mapping, pageComponents, a)
	default:
		return err
	}
}

func (s *server) createWebhookIncident(
	ctx context.Context, integration domain.WebhookIntegration, mapping webhook.Mapping,
	pageComponents map[uuid.UUID]bool, a webhook.Alert,
) error {
	impact := mapping.Impact()
	compStatus := componentStatusForImpact(impact)
	var comps []domain.IncidentComponent
	for _, cid := range mapping.Resolve(a) {
		if pageComponents[cid] {
			comps = append(comps, domain.IncidentComponent{ComponentID: cid, ComponentStatusInIncident: compStatus})
		}
	}

	now := time.Now().UTC()
	id := integration.ID
	dedup := a.DedupKey
	inc := domain.Incident{
		StatusPageID:     integration.StatusPageID,
		Title:            a.Title,
		Impact:           impact,
		StartedAt:        now,
		IsVisible:        true,
		IntegrationID:    &id,
		ExternalDedupKey: &dedup,
		Components:       comps,
	}
	if err := inc.ApplyStatusChange(domain.IncidentInvestigating, now); err != nil {
		return err
	}
	created, err := s.store.CreateIncident(ctx, inc, a.Body, true)
	if err != nil {
		if errors.Is(err, store.ErrDedupConflict) {
			return nil // гонка: открытый инцидент уже создан параллельно — no-op
		}
		return err
	}
	s.emitNotify(func() error { return s.notifier.IncidentCreated(ctx, created, a.Body) })
	return nil
}

// pageComponentSet возвращает множество id компонентов страницы (для фильтрации маппинга).
func (s *server) pageComponentSet(ctx context.Context, pageID uuid.UUID) (map[uuid.UUID]bool, error) {
	comps, err := s.store.ListComponentsByPage(ctx, pageID)
	if err != nil {
		return nil, err
	}
	set := make(map[uuid.UUID]bool, len(comps))
	for _, c := range comps {
		set[c.ID] = true
	}
	return set, nil
}

// componentStatusForImpact выбирает статус затронутого компонента по impact инцидента.
func componentStatusForImpact(impact domain.IncidentImpact) domain.ComponentStatus {
	switch impact {
	case domain.ImpactCritical:
		return domain.StatusMajorOutage
	case domain.ImpactMajor:
		return domain.StatusPartialOutage
	default:
		return domain.StatusDegradedPerformance
	}
}
