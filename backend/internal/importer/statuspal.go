package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/healthpage/backend/internal/domain"
)

// StatusPal — адаптер импорта из StatusPal (API v2, DESIGN §4.3.1). Регион us/eu определяет
// базовый хост. Аутентификация — заголовок Authorization с api_key (без префикса).
//
// Схема ниже сверена на живом read-only ключе клиента (2026-07-22), не только по документации:
//   - Все list-эндпоинты оборачивают массив в объект: {"services": [...]}, {"incidents": [...],
//     "links": {...}, "meta": {...}}, {"subscriptions": [...], ...} — а не голый JSON-массив.
//   - GET /services возвращает ТОЛЬКO корневые сервисы (parent_id=null) с полем children_ids
//     (id дочерних, без вложенных объектов) — полное дерево строится дозапросом
//     GET /services/{id} для каждого id рекурсивно (у потомков parent_id уже проставлен верно).
//   - Отдельного эндпоинта /maintenances НЕТ (404) — плановые работы это тот же ресурс
//     /incidents с фильтром ?type=scheduled (подтверждено: type=major+type=minor в сумме дают
//     total_count обычных инцидентов, type=scheduled возвращает 200 с отдельной выборкой).
//   - Пагинация — курсорная, links.next отдаёт ГОТОВЫЙ абсолютный URL (даже если в нём
//     встречается "/api/v1/" — подтверждено, что это опечатка версии на стороне StatusPal,
//     ответ по факту в формате v2; переходить по нему как есть).
//   - Subscriptions.id — строка (UUID), не число. Поле "type" различает канал (email/sms/slack/
//     webhook/...); в ImportedSubscriber попадают только type=="email" (SMS/Slack не тянем — у нас
//     нет соответствующих подписных каналов при импорте, DESIGN §4.3).
//   - Update.Type реально встречающиеся значения в истории клиента: "issue", "update",
//     "deescalate", "resolved" (в документации API это не расписано явно). "investigating"/
//     "identified"/"monitoring" в реальных данных ни разу не встретились — маппинг ниже это
//     решение агента (комментарий у spIncidentStatus), не подтверждённый факт API.
//
// [ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ ИМПОРТА]: плановые работы (type=scheduled) не удалось увидеть на живых
// данных проверочного аккаунта (там их 0) — схема по аналогии с обычными инцидентами того же
// ресурса, поля starts_at/ends_at/service_ids предполагаются идентичными, но не воочию сверены.
// Сверить на первом реальном импорте клиента с хотя бы одной плановой работой.
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

// ── DTO StatusPal (сверено на живом ключе, см. комментарий типа выше) ──

type spLinks struct {
	Next *string `json:"next"`
}

type spService struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	Description         string `json:"description"`
	ParentID            *int   `json:"parent_id"`
	Private             bool   `json:"private"`
	CurrentIncidentType string `json:"current_incident_type"`
	ChildrenIDs         []int  `json:"children_ids"`
}

type spServicesResponse struct {
	Services []spService `json:"services"`
}

type spServiceResponse struct {
	Service spService `json:"service"`
}

type spIncidentUpdate struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	InsertedAt  time.Time `json:"inserted_at"`
}

type spIncident struct {
	ID         int                `json:"id"`
	Title      string             `json:"title"`
	Type       string             `json:"type"` // major | minor | scheduled
	StartsAt   time.Time          `json:"starts_at"`
	EndsAt     *time.Time         `json:"ends_at"`
	ServiceIDs []int              `json:"service_ids"`
	Updates    []spIncidentUpdate `json:"updates"`
}

type spIncidentsResponse struct {
	Incidents []spIncident `json:"incidents"`
	Links     spLinks      `json:"links"`
}

type spSubscription struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Type  string `json:"type"` // email | sms | slack | webhook | ...
}

type spSubscriptionsResponse struct {
	Subscriptions []spSubscription `json:"subscriptions"`
	Links         spLinks          `json:"links"`
}

// get выполняет GET к ресурсу status page (path — относительный, например "services?limit=100").
func (s *StatusPal) get(ctx context.Context, creds domain.ImportCreds, path string, out any) error {
	base := s.BaseFor(creds.Region)
	u := fmt.Sprintf("%s/status_pages/%s/%s", base, url.PathEscape(creds.Subdomain), path)
	return s.getAbsolute(ctx, creds, u, out)
}

// getAbsolute выполняет GET по готовому абсолютному URL (для перехода по links.next пагинации).
func (s *StatusPal) getAbsolute(ctx context.Context, creds domain.ImportCreds, absURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, absURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", creds.APIKey)
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("statuspal: http %s: %w", absURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("statuspal: доступ отклонён (проверьте api_key/subdomain)")
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("statuspal: %s статус %d", absURL, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("statuspal: decode %s: %w", absURL, err)
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

// FetchComponents отдаёт полное дерево сервисов. /services возвращает только корневые узлы
// (parent_id=null) со списком children_ids — потомки дотягиваются рекурсивно через
// GET /services/{id}, у каждого потомка parent_id уже указывает на реального родителя.
func (s *StatusPal) FetchComponents(ctx context.Context, creds domain.ImportCreds) ([]domain.ImportedComponent, error) {
	var roots spServicesResponse
	if err := s.get(ctx, creds, "services", &roots); err != nil {
		return nil, err
	}
	all := make(map[int]spService, len(roots.Services))
	queue := make([]int, 0)
	for _, sv := range roots.Services {
		all[sv.ID] = sv
		queue = append(queue, sv.ChildrenIDs...)
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if _, seen := all[id]; seen {
			continue
		}
		var child spServiceResponse
		if err := s.get(ctx, creds, fmt.Sprintf("services/%d", id), &child); err != nil {
			return nil, err
		}
		all[child.Service.ID] = child.Service
		queue = append(queue, child.Service.ChildrenIDs...)
	}

	ids := make([]int, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	out := make([]domain.ImportedComponent, 0, len(all))
	for _, id := range ids {
		sv := all[id]
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

// fetchIncidentsPaged тянет /incidents с фильтром по типу, проходя всю курсорную пагинацию.
func (s *StatusPal) fetchIncidentsPaged(ctx context.Context, creds domain.ImportCreds, incidentType string) ([]spIncident, error) {
	var out []spIncident
	path := fmt.Sprintf("incidents?type=%s&limit=100", url.QueryEscape(incidentType))
	for {
		var resp spIncidentsResponse
		var err error
		if isAbsoluteURL(path) {
			err = s.getAbsolute(ctx, creds, path, &resp)
		} else {
			err = s.get(ctx, creds, path, &resp)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, resp.Incidents...)
		if resp.Links.Next == nil {
			break
		}
		path = *resp.Links.Next
	}
	return out, nil
}

func isAbsoluteURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.IsAbs()
}

func (s *StatusPal) FetchIncidents(ctx context.Context, creds domain.ImportCreds) ([]domain.ImportedIncident, error) {
	var raw []spIncident
	for _, t := range []string{"major", "minor"} {
		page, err := s.fetchIncidentsPaged(ctx, creds, t)
		if err != nil {
			return nil, err
		}
		raw = append(raw, page...)
	}

	out := make([]domain.ImportedIncident, 0, len(raw))
	for _, in := range raw {
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

// FetchMaintenances — плановые работы это /incidents?type=scheduled (отдельного ресурса нет,
// см. комментарий у типа StatusPal). Описание берём из первого обновления (у инцидентов нет
// отдельного поля "body" — оно только у устаревшего /maintenances, которого больше нет в API v2).
func (s *StatusPal) FetchMaintenances(ctx context.Context, creds domain.ImportCreds) ([]domain.ImportedMaintenance, error) {
	raw, err := s.fetchIncidentsPaged(ctx, creds, "scheduled")
	if err != nil {
		return nil, err
	}
	out := make([]domain.ImportedMaintenance, 0, len(raw))
	for _, m := range raw {
		var end time.Time
		if m.EndsAt != nil {
			end = *m.EndsAt
		}
		desc := ""
		if len(m.Updates) > 0 {
			desc = m.Updates[0].Description
		}
		sm := domain.ImportedMaintenance{
			ExternalID:  fmt.Sprintf("%d", m.ID),
			Title:       m.Title,
			Description: desc,
			Status:      spMaintenanceStatus(m.StartsAt, m.EndsAt),
			StartAt:     m.StartsAt,
			EndAt:       end,
		}
		for _, sid := range m.ServiceIDs {
			sm.Components = append(sm.Components, fmt.Sprintf("%d", sid))
		}
		out = append(out, sm)
	}
	return out, nil
}

// FetchSubscribers импортирует только email-подписчиков (у нас нет SMS/Slack/webhook-каналов
// подписки при импорте — DESIGN §4.3, ImportedSubscriber). Subscriptions.id — строка (UUID).
func (s *StatusPal) FetchSubscribers(ctx context.Context, creds domain.ImportCreds) ([]domain.ImportedSubscriber, error) {
	var out []domain.ImportedSubscriber
	path := "subscriptions?limit=100"
	for {
		var resp spSubscriptionsResponse
		var err error
		if isAbsoluteURL(path) {
			err = s.getAbsolute(ctx, creds, path, &resp)
		} else {
			err = s.get(ctx, creds, path, &resp)
		}
		if err != nil {
			return nil, err
		}
		for _, sub := range resp.Subscriptions {
			if sub.Type != "email" || sub.Email == "" {
				continue
			}
			out = append(out, domain.ImportedSubscriber{
				ExternalID: sub.ID,
				Email:      sub.Email,
			})
		}
		if resp.Links.Next == nil {
			break
		}
		path = *resp.Links.Next
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

// spIncidentStatus — маппинг типа обновления StatusPal → наш IncidentStatus. Документация API
// не расписывает полный enum; на живом ключе клиента реально встретились только "issue"/"update"/
// "deescalate"/"resolved" (см. комментарий у типа StatusPal) — у StatusPal нет прямых аналогов
// "identified"/"monitoring", это решение агента, а не факт API:
//   - "resolved"   → закрывает инцидент.
//   - "issue"      → первое объявление проблемы → investigating.
//   - "escalate"   → серьёзность выросла, причина не до конца ясна → identified.
//   - "deescalate" → серьёзность снизилась, но всё ещё под наблюдением → monitoring.
//   - остальное (в т.ч. "update") → identified (типовое "мы знаем, в процессе").
func spIncidentStatus(t string) domain.IncidentStatus {
	switch t {
	case "resolved":
		return domain.IncidentResolved
	case "issue":
		return domain.IncidentInvestigating
	case "deescalate":
		return domain.IncidentMonitoring
	case "escalate":
		return domain.IncidentIdentified
	default:
		return domain.IncidentIdentified
	}
}

func spMaintenanceStatus(start time.Time, end *time.Time) domain.MaintenanceStatus {
	now := time.Now()
	switch {
	case end != nil && now.After(*end):
		return domain.MaintenanceCompleted
	case now.After(start):
		return domain.MaintenanceInProgress
	default:
		return domain.MaintenanceScheduled
	}
}
