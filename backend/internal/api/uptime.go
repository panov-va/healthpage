package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
)

// ── DTO uptime (синхронно с openapi UptimeReport) ──

type uptimeDailyResponse struct {
	Date          string  `json:"date"`
	UptimePercent float64 `json:"uptime_percent"`
}

type uptimeReportResponse struct {
	ComponentID   string                `json:"component_id"`
	Days          int                   `json:"days"`
	UptimePercent float64               `json:"uptime_percent"`
	Daily         []uptimeDailyResponse `json:"daily"`
}

// handleUptime — публичный отчёт доступности компонента за период (этап 7.1).
// Приватная страница гейтится через loadPublicPage (X-Page-Access). Приватные/чужие/удалённые
// компоненты → 404 (не раскрываются публично).
func (s *server) handleUptime(w http.ResponseWriter, r *http.Request) {
	page, ok := s.loadPublicPage(w, r)
	if !ok {
		return
	}
	componentID, err := uuid.Parse(r.URL.Query().Get("component_id"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", "требуется component_id (uuid)")
		return
	}
	days := atoiDefault(r.URL.Query().Get("days"), 90)
	if days < 1 {
		days = 1
	}
	if days > 365 {
		days = 365
	}

	comp, err := s.store.ComponentByID(r.Context(), componentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "компонент не найден")
			return
		}
		writeServerError(w, err)
		return
	}
	// Компонент должен принадлежать этой странице и быть публичным.
	if comp.StatusPageID != page.ID || comp.IsPrivate {
		writeError(w, http.StatusNotFound, "not_found", "компонент не найден")
		return
	}

	now := time.Now().UTC()
	since := now.Truncate(24*time.Hour).AddDate(0, 0, -(days - 1))
	history, err := s.store.StatusHistorySince(r.Context(), componentID, since)
	if err != nil {
		writeServerError(w, err)
		return
	}

	report := domain.ComputeUptime(componentID, history, comp.CreatedAt, now, days)
	writeJSON(w, http.StatusOK, toUptimeResponse(report))
}

func toUptimeResponse(r domain.UptimeReport) uptimeReportResponse {
	daily := make([]uptimeDailyResponse, len(r.Daily))
	for i, d := range r.Daily {
		daily[i] = uptimeDailyResponse{
			Date:          d.Date.Format("2006-01-02"),
			UptimePercent: d.Percent,
		}
	}
	return uptimeReportResponse{
		ComponentID:   r.ComponentID.String(),
		Days:          r.Days,
		UptimePercent: r.UptimePercent,
		Daily:         daily,
	}
}
