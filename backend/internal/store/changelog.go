package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store/db"
)

// CreateChangelogEntry добавляет запись changelog.
func (s *Store) CreateChangelogEntry(ctx context.Context, pageID uuid.UUID, title, body string, published bool, publishedAt *time.Time) (domain.ChangelogEntry, error) {
	row, err := s.q.CreateChangelogEntry(ctx, db.CreateChangelogEntryParams{
		StatusPageID: pageID,
		Title:        title,
		Body:         body,
		Published:    published,
		PublishedAt:  publishedAt,
	})
	if err != nil {
		return domain.ChangelogEntry{}, fmt.Errorf("store: create changelog: %w", err)
	}
	return mapChangelog(row), nil
}

// ChangelogEntryByID возвращает запись по id. ErrNotFound если нет.
func (s *Store) ChangelogEntryByID(ctx context.Context, id uuid.UUID) (domain.ChangelogEntry, error) {
	row, err := s.q.GetChangelogEntry(ctx, id)
	if err != nil {
		return domain.ChangelogEntry{}, wrapNotFound(err)
	}
	return mapChangelog(row), nil
}

// ListChangelogByPage возвращает все записи страницы (админ, включая черновики), новые сверху.
func (s *Store) ListChangelogByPage(ctx context.Context, pageID uuid.UUID) ([]domain.ChangelogEntry, error) {
	rows, err := s.q.ListChangelogByPage(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("store: list changelog: %w", err)
	}
	return mapChangelogs(rows), nil
}

// ListPublishedChangelog возвращает опубликованные записи страницы (публично, пагинация).
func (s *Store) ListPublishedChangelog(ctx context.Context, pageID uuid.UUID, limit, offset int) ([]domain.ChangelogEntry, error) {
	rows, err := s.q.ListPublishedChangelog(ctx, db.ListPublishedChangelogParams{
		StatusPageID: pageID,
		Limit:        int32(limit),
		Offset:       int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list published changelog: %w", err)
	}
	return mapChangelogs(rows), nil
}

// UpdateChangelogEntry сохраняет изменения записи.
func (s *Store) UpdateChangelogEntry(ctx context.Context, id uuid.UUID, title, body string, published bool, publishedAt *time.Time) (domain.ChangelogEntry, error) {
	row, err := s.q.UpdateChangelogEntry(ctx, db.UpdateChangelogEntryParams{
		ID:          id,
		Title:       title,
		Body:        body,
		Published:   published,
		PublishedAt: publishedAt,
	})
	if err != nil {
		return domain.ChangelogEntry{}, fmt.Errorf("store: update changelog: %w", err)
	}
	return mapChangelog(row), nil
}

// DeleteChangelogEntry удаляет запись (идемпотентно).
func (s *Store) DeleteChangelogEntry(ctx context.Context, id uuid.UUID) error {
	if err := s.q.DeleteChangelogEntry(ctx, id); err != nil {
		return fmt.Errorf("store: delete changelog: %w", err)
	}
	return nil
}

func mapChangelog(c db.ChangelogEntry) domain.ChangelogEntry {
	return domain.ChangelogEntry{
		ID:           c.ID,
		StatusPageID: c.StatusPageID,
		Title:        c.Title,
		Body:         c.Body,
		Published:    c.Published,
		PublishedAt:  c.PublishedAt,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
}

func mapChangelogs(rows []db.ChangelogEntry) []domain.ChangelogEntry {
	out := make([]domain.ChangelogEntry, len(rows))
	for i, r := range rows {
		out[i] = mapChangelog(r)
	}
	return out
}
