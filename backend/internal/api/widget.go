package api

import (
	"net/http"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/widget"
)

// handleBadge отдаёт встраиваемый SVG-бейдж с общим статусом страницы (этап 4.6).
// Приватные страницы гейтятся через loadPublicPage (нужен X-Page-Access).
func (s *server) handleBadge(w http.ResponseWriter, r *http.Request) {
	page, ok := s.loadPublicPage(w, r)
	if !ok {
		return
	}
	components, err := s.store.ListComponentsByPage(r.Context(), page.ID)
	if err != nil {
		writeServerError(w, err)
		return
	}
	overall := domain.ComputeOverallStatus(components)
	svg := widget.BuildBadge(string(overall), r.URL.Query().Get("lang"))

	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	// Короткий кэш: бейдж встраивается на сторонних сайтах, но статус должен обновляться оперативно.
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(svg)
}
