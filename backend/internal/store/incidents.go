package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store/db"
)

// CreateIncident создаёт инцидент со стартовым обновлением ленты и затронутыми компонентами,
// затем авто-выводит статус затронутых компонентов от активных инцидентов (DESIGN §3.3, §6).
// Всё в одной транзакции. `inc` уже должен нести согласованные CurrentStatus/ResolvedAt
// (вызывающий применяет domain-логику жизненного цикла).
func (s *Store) CreateIncident(
	ctx context.Context, inc domain.Incident, initialBody string, notify bool,
) (domain.Incident, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Incident{}, fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	created, err := q.CreateIncident(ctx, db.CreateIncidentParams{
		StatusPageID:     inc.StatusPageID,
		Title:            inc.Title,
		CurrentStatus:    db.IncidentStatus(inc.CurrentStatus),
		Impact:           db.IncidentImpact(inc.Impact),
		StartedAt:        inc.StartedAt,
		ResolvedAt:       inc.ResolvedAt,
		Postmortem:       inc.Postmortem,
		IsVisible:        inc.IsVisible,
		IntegrationID:    inc.IntegrationID,
		ExternalDedupKey: inc.ExternalDedupKey,
	})
	if err != nil {
		// Гонка двух firing-webhook'ов с одним dedup-ключом: partial-unique индекс отбивает второй.
		if isUniqueViolation(err) {
			return domain.Incident{}, ErrDedupConflict
		}
		return domain.Incident{}, fmt.Errorf("store: create incident: %w", err)
	}

	for _, ic := range inc.Components {
		if _, err := q.AddIncidentComponent(ctx, db.AddIncidentComponentParams{
			IncidentID:                created.ID,
			ComponentID:               ic.ComponentID,
			ComponentStatusInIncident: db.ComponentStatus(ic.ComponentStatusInIncident),
		}); err != nil {
			return domain.Incident{}, fmt.Errorf("store: add incident component: %w", err)
		}
	}

	if _, err := q.AddIncidentUpdate(ctx, db.AddIncidentUpdateParams{
		IncidentID:        created.ID,
		Status:            created.CurrentStatus,
		Body:              initialBody,
		NotifySubscribers: notify,
	}); err != nil {
		return domain.Incident{}, fmt.Errorf("store: add initial update: %w", err)
	}

	if err := recomputeComponentStatusesTx(ctx, q, inc.StatusPageID, incidentComponentIDs(inc.Components)); err != nil {
		return domain.Incident{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Incident{}, fmt.Errorf("store: commit: %w", err)
	}
	return s.IncidentByID(ctx, created.ID)
}

// OpenIncidentByDedup возвращает открытый (не resolved, не удалённый) инцидент с данным
// dedup-ключом на странице. ErrNotFound если такого нет.
func (s *Store) OpenIncidentByDedup(ctx context.Context, pageID uuid.UUID, dedupKey string) (domain.Incident, error) {
	row, err := s.q.GetOpenIncidentByDedup(ctx, db.GetOpenIncidentByDedupParams{
		StatusPageID:     pageID,
		ExternalDedupKey: &dedupKey,
	})
	if err != nil {
		return domain.Incident{}, wrapNotFound(err)
	}
	return s.IncidentByID(ctx, row.ID)
}

// IncidentByID загружает агрегат инцидента (строка + компоненты + лента обновлений).
// ErrNotFound если инцидента нет или он удалён.
func (s *Store) IncidentByID(ctx context.Context, id uuid.UUID) (domain.Incident, error) {
	row, err := s.q.GetIncidentByID(ctx, id)
	if err != nil {
		return domain.Incident{}, wrapNotFound(err)
	}
	return s.hydrateIncident(ctx, mapIncident(row))
}

// hydrateIncident догружает в инцидент его компоненты и ленту обновлений (по inc.ID).
func (s *Store) hydrateIncident(ctx context.Context, inc domain.Incident) (domain.Incident, error) {
	comps, err := s.q.ListIncidentComponents(ctx, inc.ID)
	if err != nil {
		return domain.Incident{}, fmt.Errorf("store: list incident components: %w", err)
	}
	inc.Components = make([]domain.IncidentComponent, len(comps))
	for i, c := range comps {
		inc.Components[i] = mapIncidentComponent(c)
	}

	updates, err := s.q.ListIncidentUpdates(ctx, inc.ID)
	if err != nil {
		return domain.Incident{}, fmt.Errorf("store: list incident updates: %w", err)
	}
	inc.Updates = make([]domain.IncidentUpdate, len(updates))
	for i, u := range updates {
		inc.Updates[i] = mapIncidentUpdate(u)
	}
	return inc, nil
}

// IncidentFilter — опциональные фильтры публичной истории инцидентов (DESIGN §3.3).
type IncidentFilter struct {
	Status      *domain.IncidentStatus
	Impact      *domain.IncidentImpact
	ComponentID *uuid.UUID
}

// ListPublicIncidents возвращает страницу публичной истории инцидентов (видимые, не удалённые)
// с фильтрами и пагинацией, а также общее число подходящих записей. limit/offset — нормализованы
// вызывающим. Каждый инцидент — полный агрегат (компоненты + лента).
func (s *Store) ListPublicIncidents(
	ctx context.Context, pageID uuid.UUID, f IncidentFilter, limit, offset int,
) ([]domain.Incident, int, error) {
	var status db.NullIncidentStatus
	if f.Status != nil {
		status = db.NullIncidentStatus{IncidentStatus: db.IncidentStatus(*f.Status), Valid: true}
	}
	var impact db.NullIncidentImpact
	if f.Impact != nil {
		impact = db.NullIncidentImpact{IncidentImpact: db.IncidentImpact(*f.Impact), Valid: true}
	}

	rows, err := s.q.ListPublicIncidents(ctx, db.ListPublicIncidentsParams{
		StatusPageID: pageID,
		Status:       status,
		Impact:       impact,
		ComponentID:  f.ComponentID,
		Lim:          int32(limit),
		Off:          int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("store: list public incidents: %w", err)
	}
	total, err := s.q.CountPublicIncidents(ctx, db.CountPublicIncidentsParams{
		StatusPageID: pageID,
		Status:       status,
		Impact:       impact,
		ComponentID:  f.ComponentID,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("store: count public incidents: %w", err)
	}

	out := make([]domain.Incident, len(rows))
	for i, row := range rows {
		hydrated, err := s.hydrateIncident(ctx, mapIncident(row))
		if err != nil {
			return nil, 0, err
		}
		out[i] = hydrated
	}
	return out, int(total), nil
}

// ListIncidents — админский список инцидентов страницы (не удалённые, **включая скрытые**) с теми
// же фильтрами/пагинацией, что и публичная история. Для управления оператором. Возвращает страницу
// агрегатов и общее число подходящих записей.
func (s *Store) ListIncidents(
	ctx context.Context, pageID uuid.UUID, f IncidentFilter, limit, offset int,
) ([]domain.Incident, int, error) {
	var status db.NullIncidentStatus
	if f.Status != nil {
		status = db.NullIncidentStatus{IncidentStatus: db.IncidentStatus(*f.Status), Valid: true}
	}
	var impact db.NullIncidentImpact
	if f.Impact != nil {
		impact = db.NullIncidentImpact{IncidentImpact: db.IncidentImpact(*f.Impact), Valid: true}
	}

	rows, err := s.q.ListIncidents(ctx, db.ListIncidentsParams{
		StatusPageID: pageID,
		Status:       status,
		Impact:       impact,
		ComponentID:  f.ComponentID,
		Lim:          int32(limit),
		Off:          int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("store: list incidents: %w", err)
	}
	total, err := s.q.CountIncidents(ctx, db.CountIncidentsParams{
		StatusPageID: pageID,
		Status:       status,
		Impact:       impact,
		ComponentID:  f.ComponentID,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("store: count incidents: %w", err)
	}

	out := make([]domain.Incident, len(rows))
	for i, row := range rows {
		hydrated, err := s.hydrateIncident(ctx, mapIncident(row))
		if err != nil {
			return nil, 0, err
		}
		out[i] = hydrated
	}
	return out, int(total), nil
}

// ListActiveIncidents возвращает активные (не resolved, видимые) инциденты страницы для публичной
// сводки — полными агрегатами.
func (s *Store) ListActiveIncidents(ctx context.Context, pageID uuid.UUID) ([]domain.Incident, error) {
	rows, err := s.q.ListActivePublicIncidents(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("store: list active incidents: %w", err)
	}
	out := make([]domain.Incident, len(rows))
	for i, row := range rows {
		hydrated, err := s.hydrateIncident(ctx, mapIncident(row))
		if err != nil {
			return nil, err
		}
		out[i] = hydrated
	}
	return out, nil
}

// UpdateIncident сохраняет изменённые поля инцидента (вызывающий уже применил domain-логику в
// `inc`). При replaceComponents набор затронутых компонентов заменяется на inc.Components.
// После изменения авто-выводится статус всех затронутых компонентов (прежних и новых).
func (s *Store) UpdateIncident(
	ctx context.Context, inc domain.Incident, replaceComponents bool,
) (domain.Incident, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Incident{}, fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	// Прежние компоненты — чтобы при замене вернуть осиротевшие к авто-статусу.
	prev, err := q.ListIncidentComponents(ctx, inc.ID)
	if err != nil {
		return domain.Incident{}, fmt.Errorf("store: list incident components: %w", err)
	}

	updated, err := q.UpdateIncident(ctx, db.UpdateIncidentParams{
		ID:            inc.ID,
		Title:         inc.Title,
		CurrentStatus: db.IncidentStatus(inc.CurrentStatus),
		Impact:        db.IncidentImpact(inc.Impact),
		ResolvedAt:    inc.ResolvedAt,
		Postmortem:    inc.Postmortem,
		IsVisible:     inc.IsVisible,
	})
	if err != nil {
		return domain.Incident{}, wrapNotFound(err)
	}

	affected := make(map[uuid.UUID]struct{})
	for _, c := range prev {
		affected[c.ComponentID] = struct{}{}
	}
	if replaceComponents {
		if err := q.DeleteIncidentComponents(ctx, inc.ID); err != nil {
			return domain.Incident{}, fmt.Errorf("store: clear incident components: %w", err)
		}
		for _, ic := range inc.Components {
			if _, err := q.AddIncidentComponent(ctx, db.AddIncidentComponentParams{
				IncidentID:                inc.ID,
				ComponentID:               ic.ComponentID,
				ComponentStatusInIncident: db.ComponentStatus(ic.ComponentStatusInIncident),
			}); err != nil {
				return domain.Incident{}, fmt.Errorf("store: add incident component: %w", err)
			}
			affected[ic.ComponentID] = struct{}{}
		}
	}

	if err := recomputeComponentStatusesTx(ctx, q, updated.StatusPageID, mapKeys(affected)); err != nil {
		return domain.Incident{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Incident{}, fmt.Errorf("store: commit: %w", err)
	}
	return s.IncidentByID(ctx, inc.ID)
}

// AddIncidentUpdate добавляет запись в ленту инцидента, применяет смену статуса инцидента
// (current_status + ResolvedAt по domain-логике) и авто-выводит статус затронутых компонентов.
// Возвращает созданное обновление и обновлённый агрегат инцидента.
func (s *Store) AddIncidentUpdate(
	ctx context.Context, incidentID uuid.UUID, status domain.IncidentStatus, body string, notify bool, at time.Time,
) (domain.IncidentUpdate, domain.Incident, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.IncidentUpdate{}, domain.Incident{}, fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	row, err := q.GetIncidentByID(ctx, incidentID)
	if err != nil {
		return domain.IncidentUpdate{}, domain.Incident{}, wrapNotFound(err)
	}
	inc := mapIncident(row)
	if err := inc.ApplyStatusChange(status, at); err != nil {
		return domain.IncidentUpdate{}, domain.Incident{}, err
	}

	created, err := q.AddIncidentUpdate(ctx, db.AddIncidentUpdateParams{
		IncidentID:        incidentID,
		Status:            db.IncidentStatus(status),
		Body:              body,
		NotifySubscribers: notify,
	})
	if err != nil {
		return domain.IncidentUpdate{}, domain.Incident{}, fmt.Errorf("store: add incident update: %w", err)
	}

	if _, err := q.UpdateIncident(ctx, db.UpdateIncidentParams{
		ID:            inc.ID,
		Title:         inc.Title,
		CurrentStatus: db.IncidentStatus(inc.CurrentStatus),
		Impact:        db.IncidentImpact(inc.Impact),
		ResolvedAt:    inc.ResolvedAt,
		Postmortem:    inc.Postmortem,
		IsVisible:     inc.IsVisible,
	}); err != nil {
		return domain.IncidentUpdate{}, domain.Incident{}, fmt.Errorf("store: update incident status: %w", err)
	}

	comps, err := q.ListIncidentComponents(ctx, incidentID)
	if err != nil {
		return domain.IncidentUpdate{}, domain.Incident{}, fmt.Errorf("store: list incident components: %w", err)
	}
	ids := make([]uuid.UUID, len(comps))
	for i, c := range comps {
		ids[i] = c.ComponentID
	}
	if err := recomputeComponentStatusesTx(ctx, q, inc.StatusPageID, ids); err != nil {
		return domain.IncidentUpdate{}, domain.Incident{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.IncidentUpdate{}, domain.Incident{}, fmt.Errorf("store: commit: %w", err)
	}
	full, err := s.IncidentByID(ctx, incidentID)
	if err != nil {
		return domain.IncidentUpdate{}, domain.Incident{}, err
	}
	return mapIncidentUpdate(created), full, nil
}

// SoftDeleteIncident помечает инцидент удалённым и возвращает затронутые компоненты к
// авто-статусу (инцидент перестаёт быть активным). ErrNotFound если инцидента нет.
func (s *Store) SoftDeleteIncident(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	row, err := q.GetIncidentByID(ctx, id)
	if err != nil {
		return wrapNotFound(err)
	}
	comps, err := q.ListIncidentComponents(ctx, id)
	if err != nil {
		return fmt.Errorf("store: list incident components: %w", err)
	}
	if err := q.SoftDeleteIncident(ctx, id); err != nil {
		return fmt.Errorf("store: delete incident: %w", err)
	}
	ids := make([]uuid.UUID, len(comps))
	for i, c := range comps {
		ids[i] = c.ComponentID
	}
	if err := recomputeComponentStatusesTx(ctx, q, row.StatusPageID, ids); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// recomputeComponentStatusesTx пересчитывает current_status каждого из componentIDs по
// авто-деривации (DESIGN §3.3, §3.4, §6): худший статус среди активных инцидентов и активных
// (in_progress) работ страницы; если ни инцидент, ни работа компонент не затрагивают — компонент
// возвращается в operational. Изменение пишется в историю с source=incident/maintenance
// (по природе навязанного статуса); компоненты без изменения не трогаются.
func recomputeComponentStatusesTx(
	ctx context.Context, q *db.Queries, pageID uuid.UUID, componentIDs []uuid.UUID,
) error {
	if len(componentIDs) == 0 {
		return nil
	}

	rows, err := q.ListActiveIncidentComponentStatuses(ctx, pageID)
	if err != nil {
		return fmt.Errorf("store: list active incident components: %w", err)
	}
	// Все строки — из активных инцидентов, поэтому собираем их в один «активный» агрегат:
	// domain.DerivedComponentStatus отфильтрует по нужному компоненту.
	activeInc := domain.Incident{CurrentStatus: domain.IncidentInvestigating}
	for _, r := range rows {
		activeInc.Components = append(activeInc.Components, domain.IncidentComponent{
			ComponentID:               r.ComponentID,
			ComponentStatusInIncident: domain.ComponentStatus(r.ComponentStatusInIncident),
		})
	}

	mcIDs, err := q.ListActiveMaintenanceComponentIDs(ctx, pageID)
	if err != nil {
		return fmt.Errorf("store: list active maintenance components: %w", err)
	}
	// Все строки — из активных (in_progress) работ; один «активный» агрегат навязывает
	// under_maintenance своим компонентам (Maintenance.ImposedComponentStatus).
	activeM := domain.Maintenance{Status: domain.MaintenanceInProgress, ComponentIDs: mcIDs}

	for _, cid := range componentIDs {
		comp, err := q.GetComponentByID(ctx, cid)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue // компонент удалён — пропускаем
			}
			return fmt.Errorf("store: load component for recompute: %w", err)
		}
		derived, _ := domain.DerivedComponentStatus(
			cid, []domain.Incident{activeInc}, []domain.Maintenance{activeM},
		)
		if domain.ComponentStatus(comp.CurrentStatus) == derived {
			continue
		}
		// Источник истории отражает природу навязанного статуса: under_maintenance — от работ,
		// прочее (включая возврат в operational) — от инцидентов.
		source := domain.SourceIncident
		if derived == domain.StatusUnderMaintenance {
			source = domain.SourceMaintenance
		}
		if _, err := changeComponentStatusTx(ctx, q, cid, derived, source); err != nil {
			return err
		}
	}
	return nil
}

func incidentComponentIDs(comps []domain.IncidentComponent) []uuid.UUID {
	ids := make([]uuid.UUID, len(comps))
	for i, c := range comps {
		ids[i] = c.ComponentID
	}
	return ids
}

func mapKeys(m map[uuid.UUID]struct{}) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func mapIncident(i db.Incident) domain.Incident {
	return domain.Incident{
		ID:               i.ID,
		StatusPageID:     i.StatusPageID,
		Title:            i.Title,
		CurrentStatus:    domain.IncidentStatus(i.CurrentStatus),
		Impact:           domain.IncidentImpact(i.Impact),
		StartedAt:        i.StartedAt,
		ResolvedAt:       i.ResolvedAt,
		Postmortem:       i.Postmortem,
		IsVisible:        i.IsVisible,
		CreatedAt:        i.CreatedAt,
		UpdatedAt:        i.UpdatedAt,
		DeletedAt:        i.DeletedAt,
		IntegrationID:    i.IntegrationID,
		ExternalDedupKey: i.ExternalDedupKey,
	}
}

func mapIncidentComponent(c db.IncidentComponent) domain.IncidentComponent {
	return domain.IncidentComponent{
		ID:                        c.ID,
		IncidentID:                c.IncidentID,
		ComponentID:               c.ComponentID,
		ComponentStatusInIncident: domain.ComponentStatus(c.ComponentStatusInIncident),
		CreatedAt:                 c.CreatedAt,
		UpdatedAt:                 c.UpdatedAt,
	}
}

func mapIncidentUpdate(u db.IncidentUpdate) domain.IncidentUpdate {
	return domain.IncidentUpdate{
		ID:                u.ID,
		IncidentID:        u.IncidentID,
		Status:            domain.IncidentStatus(u.Status),
		Body:              u.Body,
		NotifySubscribers: u.NotifySubscribers,
		CreatedAt:         u.CreatedAt,
		UpdatedAt:         u.UpdatedAt,
	}
}
