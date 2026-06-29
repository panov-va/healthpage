package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
)

// subscriberResponse — DTO подписчика (openapi Subscriber).
type subscriberResponse struct {
	ID           string    `json:"id"`
	StatusPageID string    `json:"status_page_id"`
	Channel      string    `json:"channel"`
	Address      string    `json:"address"`
	Confirmed    bool      `json:"confirmed"`
	Scope        string    `json:"scope"`
	ComponentIDs []string  `json:"component_ids"`
	CreatedAt    time.Time `json:"created_at"`
}

func toSubscriberResponse(s domain.Subscriber) subscriberResponse {
	ids := make([]string, len(s.ComponentIDs))
	for i, id := range s.ComponentIDs {
		ids[i] = id.String()
	}
	return subscriberResponse{
		ID:           s.ID.String(),
		StatusPageID: s.StatusPageID.String(),
		Channel:      string(s.Channel),
		Address:      s.Address,
		Confirmed:    s.Confirmed,
		Scope:        string(s.Scope),
		ComponentIDs: ids,
		CreatedAt:    s.CreatedAt,
	}
}

// subscriberCreateRequest — тело POST /subscribers (openapi SubscriberCreate).
type subscriberCreateRequest struct {
	StatusPageID string   `json:"status_page_id"`
	Channel      string   `json:"channel"`
	Address      string   `json:"address"`
	Scope        string   `json:"scope"`
	ComponentIDs []string `json:"component_ids"`
}

// handleListSubscribers возвращает подписчиков страницы (вкл. неподтверждённых), новые сверху,
// с пагинацией. Требует status_page_id и владение страницей.
func (s *server) handleListSubscribers(w http.ResponseWriter, r *http.Request) {
	pageID, err := uuid.Parse(r.URL.Query().Get("status_page_id"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "требуется status_page_id (uuid)")
		return
	}
	if _, ok := s.authorizePage(w, r, pageID); !ok {
		return
	}
	_, perPage, offset := parsePagination(r)
	subs, err := s.store.ListSubscribersByPage(r.Context(), pageID, perPage, offset)
	if err != nil {
		writeServerError(w, err)
		return
	}
	out := make([]subscriberResponse, len(subs))
	for i, sub := range subs {
		out[i] = toSubscriberResponse(sub)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleCreateSubscriber добавляет подписчика вручную оператором (этап 3.10). Подписчик создаётся
// сразу подтверждённым (confirmed=true) — оператор отвечает за наличие согласия (152-ФЗ; см.
// DESIGN §4.3, §9). Поддерживаются только push-каналы (email/telegram/max/slack); rss/ical/webhook
// добавляются иными путями.
func (s *server) handleCreateSubscriber(w http.ResponseWriter, r *http.Request) {
	var req subscriberCreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	pageID, err := uuid.Parse(req.StatusPageID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "требуется status_page_id (uuid)")
		return
	}
	if _, ok := s.authorizePage(w, r, pageID); !ok {
		return
	}

	channel := domain.SubscriberChannel(req.Channel)
	if !channel.IsPush() {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request",
			"ручное добавление поддерживает только каналы email/telegram/max/slack")
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
		ids, ok := s.parseMaintenanceComponents(w, r, pageID, req.ComponentIDs)
		if !ok {
			return
		}
		componentIDs = ids
	}

	// Идемпотентность по (page, channel, address): дубль — 422.
	if _, err := s.store.SubscriberByPageChannelAddress(r.Context(), pageID, channel, req.Address); err == nil {
		writeError(w, http.StatusUnprocessableEntity, "already_exists", "подписчик с таким каналом и адресом уже есть")
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		writeServerError(w, err)
		return
	}

	sub, err := s.store.CreateSubscriber(r.Context(), domain.Subscriber{
		StatusPageID: pageID, Channel: channel, Address: req.Address,
		Confirmed: true, Scope: scope, ComponentIDs: componentIDs,
	})
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toSubscriberResponse(sub))
}

// handleDeleteSubscriber удаляет подписчика (отписка оператором). Авторизация — по владению
// страницей подписчика.
func (s *server) handleDeleteSubscriber(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "подписчик не найден")
		return
	}
	sub, err := s.store.SubscriberByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "подписчик не найден")
			return
		}
		writeServerError(w, err)
		return
	}
	if _, ok := s.authorizePage(w, r, sub.StatusPageID); !ok {
		return
	}
	if err := s.store.DeleteSubscriber(r.Context(), id); err != nil {
		writeServerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
