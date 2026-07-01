// Package importer выполняет миграцию данных из внешних status-page-сервисов (DESIGN §4.3, §7).
// Движок не зависит от источника — источники реализуют domain.Importer (паттерн «адаптер»).
// Идемпотентность — через external_id_map; уведомления по историческим данным НЕ рассылаются
// (пишем в модель напрямую через store, минуя движок notify).
package importer

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store"
)

const (
	entComponent   = "component"
	entIncident    = "incident"
	entMaintenance = "maintenance"
	entSubscriber  = "subscriber"
)

// Engine оркеструет импорт одной задачи. Держит реестр адаптеров источников.
type Engine struct {
	store     *store.Store
	importers map[domain.ImportSource]domain.Importer
}

// NewEngine собирает движок с набором адаптеров.
func NewEngine(st *store.Store, importers ...domain.Importer) *Engine {
	m := make(map[domain.ImportSource]domain.Importer, len(importers))
	for _, imp := range importers {
		m[imp.Source()] = imp
	}
	return &Engine{store: st, importers: m}
}

// Supports сообщает, зарегистрирован ли адаптер источника.
func (e *Engine) Supports(source domain.ImportSource) bool {
	_, ok := e.importers[source]
	return ok
}

// Run выполняет импорт задачи: тянет сущности источника и пишет их в модель страницы
// идемпотентно. Возвращает отчёт (перенесено/пропущено/ошибки).
func (e *Engine) Run(ctx context.Context, job domain.ImportJob, creds domain.ImportCreds) (domain.ImportReport, error) {
	var report domain.ImportReport
	imp, ok := e.importers[job.Source]
	if !ok {
		return report, fmt.Errorf("importer: источник %s не поддерживается", job.Source)
	}

	// Внешний id компонента → наш internal id (для связей инцидентов/работ).
	compMap := map[string]uuid.UUID{}

	// 1) Компоненты (с группами и деревом).
	comps, err := imp.FetchComponents(ctx, creds)
	if err != nil {
		return report, fmt.Errorf("importer: fetch components: %w", err)
	}
	groupCache := map[string]*uuid.UUID{}
	pending := make([]domain.ImportedComponent, 0, len(comps))
	for _, sc := range comps {
		internal, exists, err := e.store.ExternalMapping(ctx, job.StatusPageID, job.Source, entComponent, sc.ExternalID)
		if err != nil {
			return report, err
		}
		if exists {
			compMap[sc.ExternalID] = internal
			if job.Mode == domain.ModeSkip {
				report.ComponentsSkipped++
				pending = append(pending, sc) // для 2-го прохода (parent) всё равно учитываем
				continue
			}
		}
		groupID, err := e.resolveGroup(ctx, job.StatusPageID, sc.GroupName, groupCache)
		if err != nil {
			report.Errors = append(report.Errors, "component "+sc.Name+": "+err.Error())
			continue
		}
		status := sc.Status
		if !status.IsValid() {
			status = domain.StatusOperational
		}
		created, err := e.store.CreateComponent(ctx, domain.Component{
			StatusPageID:  job.StatusPageID,
			GroupID:       groupID,
			Name:          sc.Name,
			Description:   sc.Description,
			CurrentStatus: status,
			IsPrivate:     sc.IsPrivate,
			ShowUptime:    true,
			DisplayState:  true,
		})
		if err != nil {
			report.Errors = append(report.Errors, "component "+sc.Name+": "+err.Error())
			continue
		}
		if err := e.store.SetExternalMapping(ctx, job.StatusPageID, job.Source, entComponent, sc.ExternalID, created.ID); err != nil {
			return report, err
		}
		compMap[sc.ExternalID] = created.ID
		report.ComponentsCreated++
		pending = append(pending, sc)
	}
	// 2-й проход: проставляем parent_id (родитель мог создаться позже ребёнка).
	for _, sc := range pending {
		if sc.ParentExternalID == "" {
			continue
		}
		childID, ok := compMap[sc.ExternalID]
		parentID, okp := compMap[sc.ParentExternalID]
		if !ok || !okp {
			continue
		}
		comp, err := e.store.ComponentByID(ctx, childID)
		if err != nil {
			continue
		}
		if comp.ParentID != nil && *comp.ParentID == parentID {
			continue
		}
		comp.ParentID = &parentID
		_, _ = e.store.UpdateComponent(ctx, comp)
	}

	// 2) Инциденты (с хроникой обновлений).
	incidents, err := imp.FetchIncidents(ctx, creds)
	if err != nil {
		report.Errors = append(report.Errors, "fetch incidents: "+err.Error())
	}
	for _, si := range incidents {
		_, exists, err := e.store.ExternalMapping(ctx, job.StatusPageID, job.Source, entIncident, si.ExternalID)
		if err != nil {
			return report, err
		}
		if exists {
			report.IncidentsSkipped++
			continue
		}
		if err := e.importIncident(ctx, job, si, compMap); err != nil {
			report.Errors = append(report.Errors, "incident "+si.Title+": "+err.Error())
			continue
		}
		report.IncidentsCreated++
	}

	// 3) Плановые работы.
	maints, err := imp.FetchMaintenances(ctx, creds)
	if err != nil {
		report.Errors = append(report.Errors, "fetch maintenances: "+err.Error())
	}
	for _, sm := range maints {
		_, exists, err := e.store.ExternalMapping(ctx, job.StatusPageID, job.Source, entMaintenance, sm.ExternalID)
		if err != nil {
			return report, err
		}
		if exists {
			report.MaintenancesSkipped++
			continue
		}
		if err := e.importMaintenance(ctx, job, sm, compMap); err != nil {
			report.Errors = append(report.Errors, "maintenance "+sm.Title+": "+err.Error())
			continue
		}
		report.MaintenancesCreated++
	}

	// 4) Подписчики (только email; НЕ подтверждены автоматически — 152-ФЗ, opt-in при первом контакте).
	subs, err := imp.FetchSubscribers(ctx, creds)
	if err != nil {
		report.Errors = append(report.Errors, "fetch subscribers: "+err.Error())
	}
	for _, ss := range subs {
		if ss.Email == "" {
			report.SubscribersSkipped++
			continue
		}
		_, exists, err := e.store.ExternalMapping(ctx, job.StatusPageID, job.Source, entSubscriber, ss.ExternalID)
		if err != nil {
			return report, err
		}
		if exists {
			report.SubscribersSkipped++
			continue
		}
		created, err := e.store.CreateSubscriber(ctx, domain.Subscriber{
			StatusPageID: job.StatusPageID,
			Channel:      domain.ChannelEmail,
			Address:      ss.Email,
			Confirmed:    false, // [ВЕРНУТЬСЯ ПЕРЕД ЗАПУСКОМ]: импортированные email не подтверждены (152-ФЗ)
			Scope:        domain.ScopePage,
		})
		if err != nil {
			report.SubscribersSkipped++
			continue
		}
		if err := e.store.SetExternalMapping(ctx, job.StatusPageID, job.Source, entSubscriber, ss.ExternalID, created.ID); err != nil {
			return report, err
		}
		report.SubscribersImported++
	}

	return report, nil
}

// componentStatusForImpact — статус компонента в инциденте по impact источника.
func componentStatusForImpact(impact domain.IncidentImpact) domain.ComponentStatus {
	switch impact {
	case domain.ImpactCritical, domain.ImpactMajor:
		return domain.StatusMajorOutage
	case domain.ImpactMinor:
		return domain.StatusPartialOutage
	default:
		return domain.StatusDegradedPerformance
	}
}

func (e *Engine) resolveGroup(ctx context.Context, pageID uuid.UUID, name string, cache map[string]*uuid.UUID) (*uuid.UUID, error) {
	if name == "" {
		return nil, nil
	}
	if id, ok := cache[name]; ok {
		return id, nil
	}
	g, err := e.store.CreateComponentGroup(ctx, pageID, name, 0)
	if err != nil {
		return nil, err
	}
	cache[name] = &g.ID
	return &g.ID, nil
}

func (e *Engine) importIncident(ctx context.Context, job domain.ImportJob, si domain.ImportedIncident, compMap map[string]uuid.UUID) error {
	inc := domain.Incident{
		StatusPageID:  job.StatusPageID,
		Title:         si.Title,
		CurrentStatus: si.Status,
		Impact:        si.Impact,
		StartedAt:     si.StartedAt,
		ResolvedAt:    si.ResolvedAt,
		IsVisible:     true,
	}
	compStatus := componentStatusForImpact(si.Impact)
	for _, ext := range si.Components {
		if cid, ok := compMap[ext]; ok {
			inc.Components = append(inc.Components, domain.IncidentComponent{
				ComponentID:               cid,
				ComponentStatusInIncident: compStatus,
			})
		}
	}
	updates := si.Updates
	initialBody := ""
	if len(updates) > 0 {
		initialBody = updates[0].Body
	}
	created, err := e.store.CreateIncident(ctx, inc, initialBody, false)
	if err != nil {
		return err
	}
	// Остальные обновления хроники — без уведомлений.
	for _, u := range updates[min(1, len(updates)):] {
		if _, _, err := e.store.AddIncidentUpdate(ctx, created.ID, u.Status, u.Body, false, u.CreatedAt); err != nil {
			return err
		}
	}
	return e.store.SetExternalMapping(ctx, job.StatusPageID, job.Source, entIncident, si.ExternalID, created.ID)
}

func (e *Engine) importMaintenance(ctx context.Context, job domain.ImportJob, sm domain.ImportedMaintenance, compMap map[string]uuid.UUID) error {
	m := domain.Maintenance{
		StatusPageID:   job.StatusPageID,
		Title:          sm.Title,
		Description:    sm.Description,
		Status:         sm.Status,
		ScheduledStart: sm.StartAt,
		ScheduledEnd:   sm.EndAt,
	}
	for _, ext := range sm.Components {
		if cid, ok := compMap[ext]; ok {
			m.ComponentIDs = append(m.ComponentIDs, cid)
		}
	}
	created, err := e.store.CreateMaintenance(ctx, m)
	if err != nil {
		return err
	}
	return e.store.SetExternalMapping(ctx, job.StatusPageID, job.Source, entMaintenance, sm.ExternalID, created.ID)
}
