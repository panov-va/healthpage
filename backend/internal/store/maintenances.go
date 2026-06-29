package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store/db"
)

// CreateMaintenance создаёт плановые работы с затронутыми компонентами, затем авто-выводит
// статус затронутых компонентов (под under_maintenance, если работы создаются сразу как
// in_progress; DESIGN §3.4, §6). Всё в одной транзакции. `m` уже должен нести согласованные
// Status/StartedAt/CompletedAt (вызывающий применяет domain-логику жизненного цикла).
func (s *Store) CreateMaintenance(
	ctx context.Context, m domain.Maintenance,
) (domain.Maintenance, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Maintenance{}, fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	created, err := q.CreateMaintenance(ctx, db.CreateMaintenanceParams{
		StatusPageID:   m.StatusPageID,
		Title:          m.Title,
		Description:    m.Description,
		Status:         db.MaintenanceStatus(m.Status),
		ScheduledStart: m.ScheduledStart,
		ScheduledEnd:   m.ScheduledEnd,
		StartedAt:      m.StartedAt,
		CompletedAt:    m.CompletedAt,
	})
	if err != nil {
		return domain.Maintenance{}, fmt.Errorf("store: create maintenance: %w", err)
	}

	for _, cid := range m.ComponentIDs {
		if _, err := q.AddMaintenanceComponent(ctx, db.AddMaintenanceComponentParams{
			MaintenanceID: created.ID,
			ComponentID:   cid,
		}); err != nil {
			return domain.Maintenance{}, fmt.Errorf("store: add maintenance component: %w", err)
		}
	}

	if err := recomputeComponentStatusesTx(ctx, q, m.StatusPageID, m.ComponentIDs); err != nil {
		return domain.Maintenance{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Maintenance{}, fmt.Errorf("store: commit: %w", err)
	}
	return s.MaintenanceByID(ctx, created.ID)
}

// MaintenanceByID загружает агрегат работ (строка + компоненты + лента обновлений).
// ErrNotFound если работ нет или они удалены.
func (s *Store) MaintenanceByID(ctx context.Context, id uuid.UUID) (domain.Maintenance, error) {
	row, err := s.q.GetMaintenanceByID(ctx, id)
	if err != nil {
		return domain.Maintenance{}, wrapNotFound(err)
	}
	m := mapMaintenance(row)

	comps, err := s.q.ListMaintenanceComponents(ctx, id)
	if err != nil {
		return domain.Maintenance{}, fmt.Errorf("store: list maintenance components: %w", err)
	}
	m.ComponentIDs = make([]uuid.UUID, len(comps))
	for i, c := range comps {
		m.ComponentIDs[i] = c.ComponentID
	}

	updates, err := s.q.ListMaintenanceUpdates(ctx, id)
	if err != nil {
		return domain.Maintenance{}, fmt.Errorf("store: list maintenance updates: %w", err)
	}
	m.Updates = make([]domain.MaintenanceUpdate, len(updates))
	for i, u := range updates {
		m.Updates[i] = mapMaintenanceUpdate(u)
	}
	return m, nil
}

// UpdateMaintenance сохраняет изменённые поля работ (вызывающий уже применил domain-логику в `m`,
// включая ApplyStatusChange со StartedAt/CompletedAt). При replaceComponents набор затронутых
// компонентов заменяется на m.ComponentIDs. После изменения авто-выводится статус всех затронутых
// компонентов (прежних и новых) — перевод в/из under_maintenance при смене статуса работ.
func (s *Store) UpdateMaintenance(
	ctx context.Context, m domain.Maintenance, replaceComponents bool,
) (domain.Maintenance, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Maintenance{}, fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	// Прежние компоненты — чтобы при замене/смене статуса вернуть осиротевшие к авто-статусу.
	prev, err := q.ListMaintenanceComponents(ctx, m.ID)
	if err != nil {
		return domain.Maintenance{}, fmt.Errorf("store: list maintenance components: %w", err)
	}

	updated, err := q.UpdateMaintenance(ctx, db.UpdateMaintenanceParams{
		ID:             m.ID,
		Title:          m.Title,
		Description:    m.Description,
		Status:         db.MaintenanceStatus(m.Status),
		ScheduledStart: m.ScheduledStart,
		ScheduledEnd:   m.ScheduledEnd,
		StartedAt:      m.StartedAt,
		CompletedAt:    m.CompletedAt,
	})
	if err != nil {
		return domain.Maintenance{}, wrapNotFound(err)
	}

	affected := make(map[uuid.UUID]struct{})
	for _, c := range prev {
		affected[c.ComponentID] = struct{}{}
	}
	if replaceComponents {
		if err := q.DeleteMaintenanceComponents(ctx, m.ID); err != nil {
			return domain.Maintenance{}, fmt.Errorf("store: clear maintenance components: %w", err)
		}
		for _, cid := range m.ComponentIDs {
			if _, err := q.AddMaintenanceComponent(ctx, db.AddMaintenanceComponentParams{
				MaintenanceID: m.ID,
				ComponentID:   cid,
			}); err != nil {
				return domain.Maintenance{}, fmt.Errorf("store: add maintenance component: %w", err)
			}
			affected[cid] = struct{}{}
		}
	}
	// Прежние компоненты уже в affected — даже без замены смена статуса работ влияет на их
	// under_maintenance, поэтому они пересчитываются.

	if err := recomputeComponentStatusesTx(ctx, q, updated.StatusPageID, mapKeys(affected)); err != nil {
		return domain.Maintenance{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Maintenance{}, fmt.Errorf("store: commit: %w", err)
	}
	return s.MaintenanceByID(ctx, m.ID)
}

// AddMaintenanceUpdate добавляет запись в ленту работ (текст + флаг уведомления). У обновления
// работ нет своего статуса — смена статуса работ идёт через UpdateMaintenance (DESIGN §3.4),
// поэтому здесь авто-деривация компонентов не нужна. ErrNotFound если работ нет.
func (s *Store) AddMaintenanceUpdate(
	ctx context.Context, maintenanceID uuid.UUID, body string, notify bool,
) (domain.MaintenanceUpdate, domain.Maintenance, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.MaintenanceUpdate{}, domain.Maintenance{}, fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	if _, err := q.GetMaintenanceByID(ctx, maintenanceID); err != nil {
		return domain.MaintenanceUpdate{}, domain.Maintenance{}, wrapNotFound(err)
	}

	created, err := q.AddMaintenanceUpdate(ctx, db.AddMaintenanceUpdateParams{
		MaintenanceID:     maintenanceID,
		Body:              body,
		NotifySubscribers: notify,
	})
	if err != nil {
		return domain.MaintenanceUpdate{}, domain.Maintenance{}, fmt.Errorf("store: add maintenance update: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.MaintenanceUpdate{}, domain.Maintenance{}, fmt.Errorf("store: commit: %w", err)
	}
	full, err := s.MaintenanceByID(ctx, maintenanceID)
	if err != nil {
		return domain.MaintenanceUpdate{}, domain.Maintenance{}, err
	}
	return mapMaintenanceUpdate(created), full, nil
}

// SoftDeleteMaintenance помечает работы удалёнными и возвращает затронутые компоненты к
// авто-статусу (работы перестают навязывать under_maintenance). ErrNotFound если работ нет.
func (s *Store) SoftDeleteMaintenance(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	row, err := q.GetMaintenanceByID(ctx, id)
	if err != nil {
		return wrapNotFound(err)
	}
	comps, err := q.ListMaintenanceComponents(ctx, id)
	if err != nil {
		return fmt.Errorf("store: list maintenance components: %w", err)
	}
	if err := q.SoftDeleteMaintenance(ctx, id); err != nil {
		return fmt.Errorf("store: delete maintenance: %w", err)
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

func mapMaintenance(m db.Maintenance) domain.Maintenance {
	return domain.Maintenance{
		ID:             m.ID,
		StatusPageID:   m.StatusPageID,
		Title:          m.Title,
		Description:    m.Description,
		Status:         domain.MaintenanceStatus(m.Status),
		ScheduledStart: m.ScheduledStart,
		ScheduledEnd:   m.ScheduledEnd,
		StartedAt:      m.StartedAt,
		CompletedAt:    m.CompletedAt,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
		DeletedAt:      m.DeletedAt,
	}
}

func mapMaintenanceUpdate(u db.MaintenanceUpdate) domain.MaintenanceUpdate {
	return domain.MaintenanceUpdate{
		ID:                u.ID,
		MaintenanceID:     u.MaintenanceID,
		Body:              u.Body,
		NotifySubscribers: u.NotifySubscribers,
		CreatedAt:         u.CreatedAt,
		UpdatedAt:         u.UpdatedAt,
	}
}
