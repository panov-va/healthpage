package api

import (
	"net/http"
	"time"

	"github.com/healthpage/backend/internal/feed"
	"github.com/healthpage/backend/internal/store"
)

// feedLimit — сколько последних записей берём для фидов.
const feedLimit = 50

// handleRSS отдаёт RSS-фид инцидентов и работ страницы (DESIGN §3.5, openapi /pages/{slug}/rss).
func (s *server) handleRSS(w http.ResponseWriter, r *http.Request) {
	page, ok := s.loadPublicPage(w, r)
	if !ok {
		return
	}
	incidents, _, err := s.store.ListPublicIncidents(r.Context(), page.ID, store.IncidentFilter{}, feedLimit, 0)
	if err != nil {
		writeServerError(w, err)
		return
	}
	maintenances, _, err := s.store.ListPublicMaintenances(r.Context(), page.ID, nil, feedLimit, 0)
	if err != nil {
		writeServerError(w, err)
		return
	}
	body, err := feed.BuildRSS(page, incidents, maintenances, s.baseURL)
	if err != nil {
		writeServerError(w, err)
		return
	}
	writeRaw(w, "application/rss+xml; charset=utf-8", body)
}

// handleICal отдаёт iCal-фид плановых работ (openapi /pages/{slug}/calendar.ics).
func (s *server) handleICal(w http.ResponseWriter, r *http.Request) {
	page, ok := s.loadPublicPage(w, r)
	if !ok {
		return
	}
	maintenances, _, err := s.store.ListPublicMaintenances(r.Context(), page.ID, nil, feedLimit, 0)
	if err != nil {
		writeServerError(w, err)
		return
	}
	body := feed.BuildICal(page, maintenances, s.baseURL, time.Now())
	writeRaw(w, "text/calendar; charset=utf-8", body)
}

// writeRaw пишет тело с заданным Content-Type и статусом 200.
func writeRaw(w http.ResponseWriter, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body); err != nil {
		// Тело уже частично отправлено — только логируем.
		return
	}
}
