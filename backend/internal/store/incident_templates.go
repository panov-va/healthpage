package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store/db"
)

// CreateIncidentTemplate создаёт шаблон инцидента с преднастроенными компонентами в одной
// транзакции. Шаблон не влияет на статус компонентов (это заготовка), поэтому рекомпьюта нет.
func (s *Store) CreateIncidentTemplate(
	ctx context.Context, t domain.IncidentTemplate,
) (domain.IncidentTemplate, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.IncidentTemplate{}, fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	created, err := q.CreateIncidentTemplate(ctx, db.CreateIncidentTemplateParams{
		StatusPageID:  t.StatusPageID,
		Name:          t.Name,
		TitleTmpl:     t.TitleTmpl,
		BodyTmpl:      t.BodyTmpl,
		DefaultImpact: db.IncidentImpact(t.DefaultImpact),
	})
	if err != nil {
		return domain.IncidentTemplate{}, fmt.Errorf("store: create incident template: %w", err)
	}

	for _, c := range t.DefaultComponents {
		if _, err := q.AddIncidentTemplateComponent(ctx, db.AddIncidentTemplateComponentParams{
			TemplateID:                created.ID,
			ComponentID:               c.ComponentID,
			ComponentStatusInIncident: db.ComponentStatus(c.ComponentStatusInIncident),
		}); err != nil {
			return domain.IncidentTemplate{}, fmt.Errorf("store: add template component: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.IncidentTemplate{}, fmt.Errorf("store: commit: %w", err)
	}
	return s.IncidentTemplateByID(ctx, created.ID)
}

// IncidentTemplateByID загружает агрегат шаблона (строка + преднастроенные компоненты).
// ErrNotFound если шаблона нет.
func (s *Store) IncidentTemplateByID(ctx context.Context, id uuid.UUID) (domain.IncidentTemplate, error) {
	row, err := s.q.GetIncidentTemplateByID(ctx, id)
	if err != nil {
		return domain.IncidentTemplate{}, wrapNotFound(err)
	}
	t := mapIncidentTemplate(row)
	comps, err := s.q.ListIncidentTemplateComponents(ctx, id)
	if err != nil {
		return domain.IncidentTemplate{}, fmt.Errorf("store: list template components: %w", err)
	}
	t.DefaultComponents = make([]domain.IncidentComponent, len(comps))
	for i, c := range comps {
		t.DefaultComponents[i] = mapIncidentTemplateComponent(c)
	}
	return t, nil
}

// ListIncidentTemplates возвращает шаблоны страницы (по имени), каждый с преднастроенными
// компонентами.
func (s *Store) ListIncidentTemplates(ctx context.Context, pageID uuid.UUID) ([]domain.IncidentTemplate, error) {
	rows, err := s.q.ListIncidentTemplatesByPage(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("store: list incident templates: %w", err)
	}
	out := make([]domain.IncidentTemplate, len(rows))
	for i, row := range rows {
		t := mapIncidentTemplate(row)
		comps, err := s.q.ListIncidentTemplateComponents(ctx, row.ID)
		if err != nil {
			return nil, fmt.Errorf("store: list template components: %w", err)
		}
		t.DefaultComponents = make([]domain.IncidentComponent, len(comps))
		for j, c := range comps {
			t.DefaultComponents[j] = mapIncidentTemplateComponent(c)
		}
		out[i] = t
	}
	return out, nil
}

// UpdateIncidentTemplate сохраняет изменённые поля шаблона (вызывающий уже применил domain-
// валидацию). При replaceComponents набор преднастроенных компонентов заменяется на
// t.DefaultComponents.
func (s *Store) UpdateIncidentTemplate(
	ctx context.Context, t domain.IncidentTemplate, replaceComponents bool,
) (domain.IncidentTemplate, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.IncidentTemplate{}, fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	if _, err := q.UpdateIncidentTemplate(ctx, db.UpdateIncidentTemplateParams{
		ID:            t.ID,
		Name:          t.Name,
		TitleTmpl:     t.TitleTmpl,
		BodyTmpl:      t.BodyTmpl,
		DefaultImpact: db.IncidentImpact(t.DefaultImpact),
	}); err != nil {
		return domain.IncidentTemplate{}, wrapNotFound(err)
	}

	if replaceComponents {
		if err := q.DeleteIncidentTemplateComponents(ctx, t.ID); err != nil {
			return domain.IncidentTemplate{}, fmt.Errorf("store: clear template components: %w", err)
		}
		for _, c := range t.DefaultComponents {
			if _, err := q.AddIncidentTemplateComponent(ctx, db.AddIncidentTemplateComponentParams{
				TemplateID:                t.ID,
				ComponentID:               c.ComponentID,
				ComponentStatusInIncident: db.ComponentStatus(c.ComponentStatusInIncident),
			}); err != nil {
				return domain.IncidentTemplate{}, fmt.Errorf("store: add template component: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.IncidentTemplate{}, fmt.Errorf("store: commit: %w", err)
	}
	return s.IncidentTemplateByID(ctx, t.ID)
}

// DeleteIncidentTemplate физически удаляет шаблон (и каскадом его компоненты). Шаблон — операторская
// конфигурация без истории, поэтому удаление не soft-delete. ErrNotFound если шаблона нет.
func (s *Store) DeleteIncidentTemplate(ctx context.Context, id uuid.UUID) error {
	if _, err := s.q.GetIncidentTemplateByID(ctx, id); err != nil {
		return wrapNotFound(err)
	}
	if err := s.q.DeleteIncidentTemplate(ctx, id); err != nil {
		return fmt.Errorf("store: delete incident template: %w", err)
	}
	return nil
}

func mapIncidentTemplate(t db.IncidentTemplate) domain.IncidentTemplate {
	return domain.IncidentTemplate{
		ID:            t.ID,
		StatusPageID:  t.StatusPageID,
		Name:          t.Name,
		TitleTmpl:     t.TitleTmpl,
		BodyTmpl:      t.BodyTmpl,
		DefaultImpact: domain.IncidentImpact(t.DefaultImpact),
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
	}
}

func mapIncidentTemplateComponent(c db.IncidentTemplateComponent) domain.IncidentComponent {
	return domain.IncidentComponent{
		ID:                        c.ID,
		ComponentID:               c.ComponentID,
		ComponentStatusInIncident: domain.ComponentStatus(c.ComponentStatusInIncident),
		CreatedAt:                 c.CreatedAt,
		UpdatedAt:                 c.UpdatedAt,
	}
}
