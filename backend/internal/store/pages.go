package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store/db"
)

// ErrSlugTaken — slug страницы уже занят (среди не-удалённых).
var ErrSlugTaken = errors.New("store: slug already taken")

// CreateStatusPage создаёт страницу статуса и owner-membership для создателя в одной транзакции.
// ErrSlugTaken при конфликте slug.
func (s *Store) CreateStatusPage(
	ctx context.Context, accountID, ownerUserID uuid.UUID, name, description, slug, timezone, locale, visibility string,
) (domain.StatusPage, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.StatusPage{}, fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	p, err := q.CreateStatusPage(ctx, db.CreateStatusPageParams{
		AccountID:     accountID,
		Name:          name,
		Description:   description,
		Slug:          slug,
		Timezone:      timezone,
		DefaultLocale: locale,
		Visibility:    visibility,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.StatusPage{}, ErrSlugTaken
		}
		return domain.StatusPage{}, fmt.Errorf("store: create status page: %w", err)
	}

	if _, err := q.CreateMembership(ctx, db.CreateMembershipParams{
		UserID: ownerUserID, StatusPageID: p.ID, Role: string(domain.RoleOwner),
	}); err != nil {
		return domain.StatusPage{}, fmt.Errorf("store: create owner membership: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.StatusPage{}, fmt.Errorf("store: commit: %w", err)
	}
	return mapStatusPage(p), nil
}

// StatusPageByID возвращает страницу по id (не soft-deleted). ErrNotFound если нет.
func (s *Store) StatusPageByID(ctx context.Context, id uuid.UUID) (domain.StatusPage, error) {
	p, err := s.q.GetStatusPageByID(ctx, id)
	if err != nil {
		return domain.StatusPage{}, wrapNotFound(err)
	}
	return mapStatusPage(p), nil
}

// StatusPageBySlug возвращает страницу по slug (не soft-deleted). ErrNotFound если нет.
func (s *Store) StatusPageBySlug(ctx context.Context, slug string) (domain.StatusPage, error) {
	p, err := s.q.GetStatusPageBySlug(ctx, slug)
	if err != nil {
		return domain.StatusPage{}, wrapNotFound(err)
	}
	return mapStatusPage(p), nil
}

// ListStatusPagesByAccount возвращает страницы аккаунта.
func (s *Store) ListStatusPagesByAccount(ctx context.Context, accountID uuid.UUID) ([]domain.StatusPage, error) {
	rows, err := s.q.ListStatusPagesByAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("store: list status pages: %w", err)
	}
	out := make([]domain.StatusPage, len(rows))
	for i, p := range rows {
		out[i] = mapStatusPage(p)
	}
	return out, nil
}

// UpdateStatusPage обновляет редактируемые поля страницы. ErrNotFound если страницы нет.
func (s *Store) UpdateStatusPage(ctx context.Context, p domain.StatusPage) (domain.StatusPage, error) {
	updated, err := s.q.UpdateStatusPage(ctx, db.UpdateStatusPageParams{
		ID:            p.ID,
		Name:          p.Name,
		Description:   p.Description,
		Timezone:      p.Timezone,
		DefaultLocale: p.DefaultLocale,
		Visibility:    string(p.Visibility),
		Theme:         p.Theme,
		LogoUrl:       p.LogoURL,
		FaviconUrl:    p.FaviconURL,
		HidePoweredBy: p.HidePoweredBy,
		RedirectUrl:   p.RedirectURL,
	})
	if err != nil {
		return domain.StatusPage{}, wrapNotFound(err)
	}
	return mapStatusPage(updated), nil
}

// SetStatusPagePassword задаёт хэш пароля приватной страницы (этап 4.2); nil снимает пароль.
func (s *Store) SetStatusPagePassword(ctx context.Context, id uuid.UUID, passwordHash *string) error {
	if err := s.q.SetStatusPagePassword(ctx, db.SetStatusPagePasswordParams{
		ID: id, PasswordHash: passwordHash,
	}); err != nil {
		return fmt.Errorf("store: set page password: %w", err)
	}
	return nil
}

// SoftDeleteStatusPage помечает страницу удалённой.
func (s *Store) SoftDeleteStatusPage(ctx context.Context, id uuid.UUID) error {
	if err := s.q.SoftDeleteStatusPage(ctx, id); err != nil {
		return fmt.Errorf("store: delete status page: %w", err)
	}
	return nil
}

func mapStatusPage(p db.StatusPage) domain.StatusPage {
	return domain.StatusPage{
		ID:             p.ID,
		AccountID:      p.AccountID,
		Name:           p.Name,
		Description:    p.Description,
		Slug:           p.Slug,
		Timezone:       p.Timezone,
		DefaultLocale:  p.DefaultLocale,
		Visibility:     domain.Visibility(p.Visibility),
		PasswordHash:   p.PasswordHash,
		CustomDomain:   p.CustomDomain,
		DomainVerified: p.DomainVerified,
		Theme:          p.Theme,
		LogoURL:        p.LogoUrl,
		FaviconURL:     p.FaviconUrl,
		HidePoweredBy:  p.HidePoweredBy,
		SMTPConfig:     p.SmtpConfig,
		FromEmail:      p.FromEmail,
		RedirectURL:    p.RedirectUrl,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
		DeletedAt:      p.DeletedAt,
	}
}
