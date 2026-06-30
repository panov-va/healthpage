package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store/db"
)

// CreateAPIToken сохраняет токен (хэш + scopes) для страницы и возвращает доменную сущность.
func (s *Store) CreateAPIToken(ctx context.Context, pageID uuid.UUID, tokenHash, name string, scopes []domain.TokenScope) (domain.APIToken, error) {
	t, err := s.q.CreateAPIToken(ctx, db.CreateAPITokenParams{
		StatusPageID: pageID,
		TokenHash:    tokenHash,
		Name:         name,
		Scopes:       scopeStrings(scopes),
	})
	if err != nil {
		return domain.APIToken{}, fmt.Errorf("store: create api token: %w", err)
	}
	return mapAPIToken(t), nil
}

// APITokenByHash находит токен по хэшу (для аутентификации). ErrNotFound если нет.
func (s *Store) APITokenByHash(ctx context.Context, tokenHash string) (domain.APIToken, error) {
	t, err := s.q.GetAPITokenByHash(ctx, tokenHash)
	if err != nil {
		return domain.APIToken{}, wrapNotFound(err)
	}
	return mapAPIToken(t), nil
}

// APITokenByID находит токен по id. ErrNotFound если нет.
func (s *Store) APITokenByID(ctx context.Context, id uuid.UUID) (domain.APIToken, error) {
	t, err := s.q.GetAPIToken(ctx, id)
	if err != nil {
		return domain.APIToken{}, wrapNotFound(err)
	}
	return mapAPIToken(t), nil
}

// ListAPITokensByPage возвращает токены страницы (новые сверху, без значений/хэшей).
func (s *Store) ListAPITokensByPage(ctx context.Context, pageID uuid.UUID) ([]domain.APIToken, error) {
	rows, err := s.q.ListAPITokensByPage(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("store: list api tokens: %w", err)
	}
	out := make([]domain.APIToken, len(rows))
	for i, t := range rows {
		out[i] = mapAPIToken(t)
	}
	return out, nil
}

// TouchAPIToken обновляет last_used_at (best-effort, вызывается при аутентификации).
func (s *Store) TouchAPIToken(ctx context.Context, id uuid.UUID) error {
	if err := s.q.TouchAPIToken(ctx, id); err != nil {
		return fmt.Errorf("store: touch api token: %w", err)
	}
	return nil
}

// DeleteAPIToken удаляет токен (отзыв). Идемпотентно: несуществующий — без ошибки.
func (s *Store) DeleteAPIToken(ctx context.Context, id uuid.UUID) error {
	if err := s.q.DeleteAPIToken(ctx, id); err != nil {
		return fmt.Errorf("store: delete api token: %w", err)
	}
	return nil
}

func mapAPIToken(t db.ApiToken) domain.APIToken {
	scopes := make([]domain.TokenScope, len(t.Scopes))
	for i, s := range t.Scopes {
		scopes[i] = domain.TokenScope(s)
	}
	return domain.APIToken{
		ID:           t.ID,
		StatusPageID: t.StatusPageID,
		Name:         t.Name,
		Scopes:       scopes,
		LastUsedAt:   t.LastUsedAt,
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
	}
}

func scopeStrings(scopes []domain.TokenScope) []string {
	out := make([]string, len(scopes))
	for i, s := range scopes {
		out[i] = string(s)
	}
	return out
}
