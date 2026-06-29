package api

import (
	"errors"
	"log"
	"net/http"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
	"github.com/healthpage/backend/internal/subscription"
)

// subscribeRequest — тело POST /pages/{slug}/subscribe (openapi SubscribeRequest).
type subscribeRequest struct {
	Channel      string   `json:"channel"`
	Address      string   `json:"address"`
	Scope        string   `json:"scope"`
	ComponentIDs []string `json:"component_ids"`
}

// handleSubscribe запускает double opt-in email-подписки (DESIGN §3.5): создаёт/перевыпускает
// неподтверждённого подписчика со случайным confirm-токеном (в БД — его хэш) и публикует письмо
// подтверждения. Идемпотентно: уже подтверждённая подписка тем же адресом не трогается.
// На этапе 3.5 поддерживается только email; telegram/MAX/Slack подключаются своими флоу (3.7–3.9).
func (s *server) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	page, ok := s.loadPublicPage(w, r)
	if !ok {
		return
	}
	var req subscribeRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	channel := domain.SubscriberChannel(req.Channel)
	if channel != domain.ChannelEmail {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request",
			"на этом этапе поддерживается только channel=email")
		return
	}
	if req.Address == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "address обязателен")
		return
	}
	scope := domain.SubscriberScope(req.Scope)
	if req.Scope == "" {
		scope = domain.ScopePage
	}
	if !scope.IsValid() {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "недопустимый scope")
		return
	}
	var componentIDs []uuid.UUID
	if scope == domain.ScopeComponents {
		ids, ok := s.parseMaintenanceComponents(w, r, page.ID, req.ComponentIDs)
		if !ok {
			return
		}
		componentIDs = ids
	}

	token, hash, err := subscription.GenerateConfirmToken()
	if err != nil {
		writeServerError(w, err)
		return
	}

	// Идемпотентность по (page, channel, address): создать / перевыпустить / не трогать.
	sub, err := s.store.SubscriberByPageChannelAddress(r.Context(), page.ID, channel, req.Address)
	switch {
	case errors.Is(err, store.ErrNotFound):
		sub, err = s.store.CreateSubscriber(r.Context(), domain.Subscriber{
			StatusPageID: page.ID, Channel: channel, Address: req.Address,
			Confirmed: false, ConfirmToken: &hash, Scope: scope, ComponentIDs: componentIDs,
		})
		if err != nil {
			writeServerError(w, err)
			return
		}
	case err != nil:
		writeServerError(w, err)
		return
	case sub.Confirmed:
		// Уже подписан и подтверждён — повторное письмо не шлём, отвечаем как принято.
		w.WriteHeader(http.StatusAccepted)
		return
	default:
		// Существует, но не подтверждён — перевыпускаем токен и шлём письмо заново.
		sub, err = s.store.ReissueConfirmToken(r.Context(), sub.ID, hash, scope, componentIDs)
		if err != nil {
			writeServerError(w, err)
			return
		}
	}

	if s.notifier != nil {
		if err := s.notifier.SendConfirmation(r.Context(), sub, token); err != nil {
			// Подписчик создан (pending); письмо не ушло — логируем, но не валим запрос.
			log.Printf("subscribe: send confirmation failed: %v", err)
		}
	} else {
		log.Printf("subscribe: notifier disabled — confirmation email not sent for %s", sub.ID)
	}

	w.WriteHeader(http.StatusAccepted)
}

// handleConfirmSubscribe подтверждает подписку по одноразовому токену из письма (GET /subscribe/confirm).
func (s *server) handleConfirmSubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "требуется token")
		return
	}
	sub, err := s.store.SubscriberByConfirmTokenHash(r.Context(), subscription.HashConfirmToken(token))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "invalid_token", "токен недействителен или уже использован")
			return
		}
		writeServerError(w, err)
		return
	}
	if err := s.store.ConfirmSubscriber(r.Context(), sub.ID); err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "confirmed"})
}

// handleUnsubscribe отписывает по HMAC-токену из письма (GET /unsubscribe). Идемпотентно:
// повторная отписка по валидному токену безвредна (строки уже нет).
func (s *server) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "требуется token")
		return
	}
	id, err := subscription.ParseUnsubscribeToken(s.subSecret, token)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_token", "токен недействителен")
		return
	}
	if err := s.store.DeleteSubscriber(r.Context(), id); err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unsubscribed"})
}
