package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store/db"
)

// ── Группы компонентов ──

// CreateComponentGroup создаёт группу.
func (s *Store) CreateComponentGroup(ctx context.Context, pageID uuid.UUID, name string, position int) (domain.ComponentGroup, error) {
	g, err := s.q.CreateComponentGroup(ctx, db.CreateComponentGroupParams{
		StatusPageID: pageID, Name: name, Position: int32(position),
	})
	if err != nil {
		return domain.ComponentGroup{}, fmt.Errorf("store: create group: %w", err)
	}
	return mapComponentGroup(g), nil
}

// ComponentGroupByID возвращает группу по id. ErrNotFound если нет.
func (s *Store) ComponentGroupByID(ctx context.Context, id uuid.UUID) (domain.ComponentGroup, error) {
	g, err := s.q.GetComponentGroupByID(ctx, id)
	if err != nil {
		return domain.ComponentGroup{}, wrapNotFound(err)
	}
	return mapComponentGroup(g), nil
}

// ListComponentGroupsByPage возвращает группы страницы.
func (s *Store) ListComponentGroupsByPage(ctx context.Context, pageID uuid.UUID) ([]domain.ComponentGroup, error) {
	rows, err := s.q.ListComponentGroupsByPage(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("store: list groups: %w", err)
	}
	out := make([]domain.ComponentGroup, len(rows))
	for i, g := range rows {
		out[i] = mapComponentGroup(g)
	}
	return out, nil
}

// UpdateComponentGroup обновляет имя/позицию группы. ErrNotFound если нет.
func (s *Store) UpdateComponentGroup(ctx context.Context, id uuid.UUID, name string, position int) (domain.ComponentGroup, error) {
	g, err := s.q.UpdateComponentGroup(ctx, db.UpdateComponentGroupParams{ID: id, Name: name, Position: int32(position)})
	if err != nil {
		return domain.ComponentGroup{}, wrapNotFound(err)
	}
	return mapComponentGroup(g), nil
}

// SoftDeleteComponentGroup помечает группу удалённой.
func (s *Store) SoftDeleteComponentGroup(ctx context.Context, id uuid.UUID) error {
	if err := s.q.SoftDeleteComponentGroup(ctx, id); err != nil {
		return fmt.Errorf("store: delete group: %w", err)
	}
	return nil
}

// ── Компоненты ──

// CreateComponent создаёт компонент. Пустой current_status трактуется как operational.
func (s *Store) CreateComponent(ctx context.Context, c domain.Component) (domain.Component, error) {
	if c.CurrentStatus == "" {
		c.CurrentStatus = domain.StatusOperational
	}
	created, err := s.q.CreateComponent(ctx, db.CreateComponentParams{
		StatusPageID:  c.StatusPageID,
		GroupID:       c.GroupID,
		ParentID:      c.ParentID,
		Name:          c.Name,
		Description:   c.Description,
		Position:      int32(c.Position),
		CurrentStatus: db.ComponentStatus(c.CurrentStatus),
		IsPrivate:     c.IsPrivate,
		ShowUptime:    c.ShowUptime,
		DisplayState:  c.DisplayState,
	})
	if err != nil {
		return domain.Component{}, fmt.Errorf("store: create component: %w", err)
	}
	return mapComponent(created), nil
}

// ComponentByID возвращает компонент по id. ErrNotFound если нет.
func (s *Store) ComponentByID(ctx context.Context, id uuid.UUID) (domain.Component, error) {
	c, err := s.q.GetComponentByID(ctx, id)
	if err != nil {
		return domain.Component{}, wrapNotFound(err)
	}
	return mapComponent(c), nil
}

// ListComponentsByPage возвращает компоненты страницы (для дерева и сводки).
func (s *Store) ListComponentsByPage(ctx context.Context, pageID uuid.UUID) ([]domain.Component, error) {
	rows, err := s.q.ListComponentsByPage(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("store: list components: %w", err)
	}
	out := make([]domain.Component, len(rows))
	for i, c := range rows {
		out[i] = mapComponent(c)
	}
	return out, nil
}

// UpdateComponent обновляет атрибуты компонента (кроме статуса — см. ChangeComponentStatus).
func (s *Store) UpdateComponent(ctx context.Context, c domain.Component) (domain.Component, error) {
	updated, err := s.q.UpdateComponent(ctx, db.UpdateComponentParams{
		ID:           c.ID,
		GroupID:      c.GroupID,
		ParentID:     c.ParentID,
		Name:         c.Name,
		Description:  c.Description,
		Position:     int32(c.Position),
		IsPrivate:    c.IsPrivate,
		ShowUptime:   c.ShowUptime,
		DisplayState: c.DisplayState,
	})
	if err != nil {
		return domain.Component{}, wrapNotFound(err)
	}
	return mapComponent(updated), nil
}

// SoftDeleteComponent помечает компонент удалённым (потомки каскадно удаляются на уровне БД
// только при физическом удалении; soft-delete потомков выполняет вышележащий слой при надобности).
func (s *Store) SoftDeleteComponent(ctx context.Context, id uuid.UUID) error {
	if err := s.q.SoftDeleteComponent(ctx, id); err != nil {
		return fmt.Errorf("store: delete component: %w", err)
	}
	return nil
}

// ChangeComponentStatus атомарно меняет статус компонента: закрывает открытый период истории,
// проставляет новый current_status и открывает новый период (DESIGN §5, §6).
func (s *Store) ChangeComponentStatus(
	ctx context.Context, componentID uuid.UUID, status domain.ComponentStatus, source domain.HistorySource,
) (domain.Component, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Component{}, fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	updated, err := changeComponentStatusTx(ctx, q, componentID, status, source)
	if err != nil {
		return domain.Component{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Component{}, fmt.Errorf("store: commit: %w", err)
	}
	return updated, nil
}

// changeComponentStatusTx выполняет смену статуса в рамках переданной транзакции (close period →
// set status → open period). Используется как ручной сменой (ChangeComponentStatus), так и
// авто-деривацией от инцидентов/работ (recomputeComponentStatusesTx).
func changeComponentStatusTx(
	ctx context.Context, q *db.Queries, componentID uuid.UUID,
	status domain.ComponentStatus, source domain.HistorySource,
) (domain.Component, error) {
	if err := q.CloseOpenStatusPeriod(ctx, componentID); err != nil {
		return domain.Component{}, fmt.Errorf("store: close period: %w", err)
	}
	updated, err := q.SetComponentStatus(ctx, db.SetComponentStatusParams{
		ID: componentID, CurrentStatus: db.ComponentStatus(status),
	})
	if err != nil {
		return domain.Component{}, wrapNotFound(err)
	}
	if _, err := q.OpenStatusPeriod(ctx, db.OpenStatusPeriodParams{
		ComponentID: componentID, Status: db.ComponentStatus(status), Source: string(source),
	}); err != nil {
		return domain.Component{}, fmt.Errorf("store: open period: %w", err)
	}
	return mapComponent(updated), nil
}

// ListStatusHistory возвращает историю статусов компонента (для uptime — этап 7).
func (s *Store) ListStatusHistory(ctx context.Context, componentID uuid.UUID) ([]domain.ComponentStatusHistory, error) {
	rows, err := s.q.ListStatusHistory(ctx, componentID)
	if err != nil {
		return nil, fmt.Errorf("store: list history: %w", err)
	}
	out := make([]domain.ComponentStatusHistory, len(rows))
	for i, h := range rows {
		out[i] = mapStatusHistory(h)
	}
	return out, nil
}

func mapComponentGroup(g db.ComponentGroup) domain.ComponentGroup {
	return domain.ComponentGroup{
		ID: g.ID, StatusPageID: g.StatusPageID, Name: g.Name, Position: int(g.Position),
		CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt, DeletedAt: g.DeletedAt,
	}
}

func mapComponent(c db.Component) domain.Component {
	return domain.Component{
		ID: c.ID, StatusPageID: c.StatusPageID, GroupID: c.GroupID, ParentID: c.ParentID,
		Name: c.Name, Description: c.Description, Position: int(c.Position),
		CurrentStatus: domain.ComponentStatus(c.CurrentStatus),
		IsPrivate:     c.IsPrivate, ShowUptime: c.ShowUptime, DisplayState: c.DisplayState,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, DeletedAt: c.DeletedAt,
	}
}

func mapStatusHistory(h db.ComponentStatusHistory) domain.ComponentStatusHistory {
	return domain.ComponentStatusHistory{
		ID: h.ID, ComponentID: h.ComponentID, Status: domain.ComponentStatus(h.Status),
		StartedAt: h.StartedAt, EndedAt: h.EndedAt, Source: domain.HistorySource(h.Source),
		CreatedAt: h.CreatedAt, UpdatedAt: h.UpdatedAt,
	}
}
