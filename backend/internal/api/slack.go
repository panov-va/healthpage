package api

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
	"github.com/healthpage/backend/internal/subscription"
)

// handleSlackStart инициирует OAuth «Add to Slack» (DESIGN §4.4): редиректит пользователя на Slack
// для выбора рабочего пространства и канала. Состояние (signed state) привязывает callback к этой
// странице. Если Slack OAuth не сконфигурирован — фича выключена (404).
func (s *server) handleSlackStart(w http.ResponseWriter, r *http.Request) {
	page, ok := s.loadPublicPage(w, r)
	if !ok {
		return
	}
	if s.slackOAuth == nil {
		writeError(w, http.StatusNotFound, "not_found", "Slack-подписка не настроена")
		return
	}
	state := subscription.SignSlackState(s.subSecret, page.ID, time.Now().Unix())
	http.Redirect(w, r, s.slackOAuth.AuthorizeURL(state), http.StatusFound)
}

// handleSlackCallback обрабатывает возврат от Slack OAuth: проверяет state, обменивает code на
// incoming-webhook URL канала и создаёт Subscriber{channel=slack, address=webhook_url}.
func (s *server) handleSlackCallback(w http.ResponseWriter, r *http.Request) {
	if s.slackOAuth == nil {
		writeError(w, http.StatusNotFound, "not_found", "Slack-подписка не настроена")
		return
	}
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "требуются code и state")
		return
	}
	pageID, err := subscription.ParseSlackState(s.subSecret, state, time.Now())
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_state", "state недействителен или истёк")
		return
	}
	// Страница должна существовать (могла быть удалена за время OAuth).
	page, err := s.store.StatusPageByID(r.Context(), pageID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_state", "страница недоступна")
		return
	}

	grant, err := s.slackOAuth.Exchange(r.Context(), code)
	if err != nil {
		log.Printf("slack callback: exchange failed: %v", err)
		writeError(w, http.StatusBadRequest, "oauth_failed", "не удалось заверш: OAuth Slack")
		return
	}

	// Идемпотентность по (page, channel, webhook_url): повторный коллбэк с тем же URL не дублирует.
	_, err = s.store.SubscriberByPageChannelAddress(r.Context(), page.ID, domain.ChannelSlack, grant.WebhookURL)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]string{"status": "subscribed", "channel": grant.Channel})
		return
	case !errors.Is(err, store.ErrNotFound):
		writeServerError(w, err)
		return
	}

	if _, err := s.store.CreateSubscriber(r.Context(), domain.Subscriber{
		StatusPageID: page.ID,
		Channel:      domain.ChannelSlack,
		Address:      grant.WebhookURL,
		Confirmed:    true, // прохождение OAuth = явное согласие, double opt-in не нужен
		Scope:        domain.ScopePage,
	}); err != nil {
		writeServerError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "subscribed", "channel": grant.Channel})
}
