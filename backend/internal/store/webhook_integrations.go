package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store/db"
)

// CreateWebhookIntegration сохраняет интеграцию и возвращает доменную сущность.
func (s *Store) CreateWebhookIntegration(ctx context.Context, wi domain.WebhookIntegration) (domain.WebhookIntegration, error) {
	row, err := s.q.CreateWebhookIntegration(ctx, db.CreateWebhookIntegrationParams{
		StatusPageID:     wi.StatusPageID,
		Source:           string(wi.Source),
		Name:             wi.Name,
		Secret:           wi.Secret,
		ComponentMapping: mappingBytes(wi.ComponentMapping),
	})
	if err != nil {
		return domain.WebhookIntegration{}, fmt.Errorf("store: create webhook integration: %w", err)
	}
	return mapWebhookIntegration(row), nil
}

// WebhookIntegrationByID находит интеграцию по id. ErrNotFound если нет.
func (s *Store) WebhookIntegrationByID(ctx context.Context, id uuid.UUID) (domain.WebhookIntegration, error) {
	row, err := s.q.GetWebhookIntegration(ctx, id)
	if err != nil {
		return domain.WebhookIntegration{}, wrapNotFound(err)
	}
	return mapWebhookIntegration(row), nil
}

// ListWebhookIntegrationsByPage возвращает интеграции страницы (новые сверху).
func (s *Store) ListWebhookIntegrationsByPage(ctx context.Context, pageID uuid.UUID) ([]domain.WebhookIntegration, error) {
	rows, err := s.q.ListWebhookIntegrationsByPage(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("store: list webhook integrations: %w", err)
	}
	out := make([]domain.WebhookIntegration, len(rows))
	for i, row := range rows {
		out[i] = mapWebhookIntegration(row)
	}
	return out, nil
}

// UpdateWebhookIntegration обновляет name/secret/component_mapping. ErrNotFound если нет.
func (s *Store) UpdateWebhookIntegration(ctx context.Context, wi domain.WebhookIntegration) (domain.WebhookIntegration, error) {
	row, err := s.q.UpdateWebhookIntegration(ctx, db.UpdateWebhookIntegrationParams{
		ID:               wi.ID,
		Name:             wi.Name,
		Secret:           wi.Secret,
		ComponentMapping: mappingBytes(wi.ComponentMapping),
	})
	if err != nil {
		return domain.WebhookIntegration{}, wrapNotFound(err)
	}
	return mapWebhookIntegration(row), nil
}

// DeleteWebhookIntegration удаляет интеграцию. Идемпотентно: несуществующая — без ошибки.
func (s *Store) DeleteWebhookIntegration(ctx context.Context, id uuid.UUID) error {
	if err := s.q.DeleteWebhookIntegration(ctx, id); err != nil {
		return fmt.Errorf("store: delete webhook integration: %w", err)
	}
	return nil
}

func mapWebhookIntegration(w db.WebhookIntegration) domain.WebhookIntegration {
	return domain.WebhookIntegration{
		ID:               w.ID,
		StatusPageID:     w.StatusPageID,
		Source:           domain.WebhookSource(w.Source),
		Name:             w.Name,
		Secret:           w.Secret,
		ComponentMapping: w.ComponentMapping,
		CreatedAt:        w.CreatedAt,
		UpdatedAt:        w.UpdatedAt,
	}
}

// mappingBytes нормализует пустой/нулевой component_mapping в "{}" (колонка NOT NULL).
func mappingBytes(raw []byte) []byte {
	if len(raw) == 0 || string(raw) == "null" {
		return []byte("{}")
	}
	return raw
}
