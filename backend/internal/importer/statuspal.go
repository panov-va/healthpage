package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/healthpage/backend/internal/domain"
)

// StatusPal — адаптер импорта из StatusPal (API v2, DESIGN §4.3.1). Регион us/eu определяет
// базовый хост. Аутентификация — заголовок Authorization с api_key.
//
// [ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ ИМПОРТА]: точная схема JSON и пути StatusPal API v2 сверяются на
// живом ключе (структуры ниже — по документации; поля могут отличаться). Маппинг статусов/impact
// проверить на реальных данных. Реальный прогон — на прод-деплое.
type StatusPal struct {
	// BaseFor возвращает базовый URL API для региона (инъекция для тестов).
	BaseFor func(region domain.ImportRegion) string
	HTTP    *http.Client
}

// NewStatusPal создаёт адаптер с дефолтными хостами StatusPal.
func NewStatusPal() *StatusPal {
	return &StatusPal{
		BaseFor: func(region domain.ImportRegion) string {
			if region == domain.RegionEU {
				return "https://statuspal.eu/api/v2"
			}
			return "https://statuspal.io/api/v2"
		},
		HTTP: &http.Client{Timeout: 30 * time.Second},
	}
}

func (*StatusPal) Source() domain.ImportSource { return domain.SourceStatusPal }

// ── DTO StatusPal (по документации API v2; сверить на живом ключе) ──

type spService struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	Description         string `json:"description"`
	ParentID            *int   `json:"parent_id"`
	Private             bool   `json:"private"`
	CurrentIncidentType string `json:"current_incident_type"` // major | minor | scheduled | ""
}

type spIncidentUpdate struct {
	Type        string    `json:"type"` // investigating|identified|monitoring|resolved
	Description string    `json:"description"`
	InsertedAt  time.Time `json:"inserted_at"`
}

type spIncident struct {
	ID         int                `json:"id"`
	Title      string             `json:"title"`
	Type       string             `json:"type"` // major | minor
	StartsAt   time.Time          `json:"starts_at"`
	EndsAt     *time.Time         `json:"ends_at"`
	ServiceIDs []int              `json:"service_ids"`
	Updates    []spIncidentUpdate `json:"updates"`
}

type spMaintenance struct {
	ID         int       `json:"id"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
	ServiceIDs []int     `json:"service_ids"`
}

type spSubscription struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
}

func (s *StatusPal) get(ctx context.Context, creds domain.ImportCreds, path string, out any) error {
	base := s.BaseFor(creds.Region)
	u := fmt.Sprintf("%s/status_pages/%s/%s", base, url.PathEscape(creds.Subdomain), path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", creds.APIKey)
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("statuspal: http %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("statuspal: доступ отклонён (проверьте api_key/subdomain)")
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("statuspal: %s статус %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("statuspal: decode %s: %w", path, err)
	}
	return nil
}

func (s *StatusPal) Probe(ctx context.Context, creds domain.ImportCreds) (domain.ImportPreview, error) {
	comps, err := s.FetchComponents(ctx, creds)
	if err != nil {
		return domain.ImportPreview{}, err
	}
	inc, _ := s.FetchIncidents(ctx, creds)
	mnt, _ := s.FetchMaintenances(ctx, creds)
	sub, _ := s.FetchSubscribers(ctx, creds)
	return domain.ImportPreview{
		Components:   len(comps),
		Incidents:    len(inc),
		Maintenances: len(mnt),
		Subscribers:  len(sub),
	}, nil
}

func (s *StatusPal) FetchComponents(ctx context.Context, creds domain.ImportCreds) ([]domain.ImportedComponent, error) {
	var services []spService
	if err := s.get(ctx, creds, "services", &services); err != nil {
		return nil, err
	}
	out := make([]domain.ImportedComponent, 0, len(services))
	for _, sv := range services {
		parent := ""
		if sv.ParentID != nil {
			parent = fmt.Sprintf("%d", *sv.ParentID)
		}
		out = append(out, domain.ImportedComponent{
			ExternalID:       fmt.Sprintf("%d", sv.ID),
			Name:             sv.Name,
			Description:      sv.Description,
			Status:           spServiceStatus(sv.CurrentIncidentType),
			ParentExternalID: parent,
			IsPrivate:        sv.Private,
		})
	}
	return out, nil
}

func (s *StatusPal) FetchIncidents(ctx context.Context, creds domain.ImportCreds) ([]domain.ImportedIncident, error) {
	var incidents []spIncident
	if err := s.get(ctx, creds, "incidents", &incidents); err != nil {
		return nil, err
	}
	out := make([]domain.ImportedIncident, 0, len(incidents))
	for _, in := range incidents {
		si := domain.ImportedIncident{
			ExternalID: fmt.Sprintf("%d", in.ID),
			Title:      in.Title,
			Impact:     spIncidentImpact(in.Type),
			Status:     domain.IncidentInvestigating,
			StartedAt:  in.StartsAt,
			ResolvedAt: in.EndsAt,
		}
		for _, sid := range in.ServiceIDs {
			si.Components = append(si.Components, fmt.Sprintf("%d", sid))
		}
		for _, u := range in.Updates {
			si.Updates = append(si.Updates, domain.ImportedIncidentUpdate{
				Status:    spIncidentStatus(u.Type),
				Body:      u.Description,
				CreatedAt: u.InsertedAt,
			})
		}
		// Итоговый статус инцидента — из последнего обновления или resolved, если закрыт.
		if in.EndsAt != nil {
			si.Status = domain.IncidentResolved
		} else if n := len(si.Updates); n > 0 {
			si.Status = si.Updates[n-1].Status
		}
		out = append(out, si)
	}
	return out, nil
}

func (s *StatusPal) FetchMaintenances(ctx context.Context, creds domain.ImportCreds) ([]domain.ImportedMaintenance, error) {
	var maints []spMaintenance
	if err := s.get(ctx, creds, "maintenances", &maints); err != nil {
		return nil, err
	}
	out := make([]domain.ImportedMaintenance, 0, len(maints))
	for _, m := range maints {
		sm := domain.ImportedMaintenance{
			ExternalID:  fmt.Sprintf("%d", m.ID),
			Title:       m.Title,
			Description: m.Body,
			Status:      spMaintenanceStatus(m.StartsAt, m.EndsAt),
			StartAt:     m.StartsAt,
			EndAt:       m.EndsAt,
		}
		for _, sid := range m.ServiceIDs {
			sm.Components = append(sm.Components, fmt.Sprintf("%d", sid))
		}
		out = append(out, sm)
	}
	return out, nil
}

func (s *StatusPal) FetchSubscribers(ctx context.Context, creds domain.ImportCreds) ([]domain.ImportedSubscriber, error) {
	var subs []spSubscription
	if err := s.get(ctx, creds, "subscriptions", &subs); err != nil {
		return nil, err
	}
	out := make([]domain.ImportedSubscriber, 0, len(subs))
	for _, sub := range subs {
		out = append(out, domain.ImportedSubscriber{
			ExternalID: fmt.Sprintf("%d", sub.ID),
			Email:      sub.Email,
		})
	}
	return out, nil
}

// ── маппинг статусов StatusPal → нормативные enum'ы ──

func spServiceStatus(incidentType string) domain.ComponentStatus {
	switch incidentType {
	case "major":
		return domain.StatusMajorOutage
	case "minor":
		return domain.StatusPartialOutage
	case "scheduled":
		return domain.StatusUnderMaintenance
	case "degraded":
		return domain.StatusDegradedPerformance
	default:
		return domain.StatusOperational
	}
}

func spIncidentImpact(t string) domain.IncidentImpact {
	switch t {
	case "major":
		return domain.ImpactMajor
	case "minor":
		return domain.ImpactMinor
	default:
		return domain.ImpactMinor
	}
}

func spIncidentStatus(t string) domain.IncidentStatus {
	switch t {
	case "identified":
		return domain.IncidentIdentified
	case "monitoring":
		return domain.IncidentMonitoring
	case "resolved":
		return domain.IncidentResolved
	default:
		return domain.IncidentInvestigating
	}
}

func spMaintenanceStatus(start, end time.Time) domain.MaintenanceStatus {
	now := time.Now()
	switch {
	case now.After(end):
		return domain.MaintenanceCompleted
	case now.After(start):
		return domain.MaintenanceInProgress
	default:
		return domain.MaintenanceScheduled
	}
}
