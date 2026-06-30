package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store/db"
)

// ErrEmailAlreadyAllowed — email уже в списке доступа страницы.
var ErrEmailAlreadyAllowed = errors.New("store: email already allowed")

// ListAllowedEmails возвращает список email с доступом к приватной странице (этап 4.2.1).
func (s *Store) ListAllowedEmails(ctx context.Context, pageID uuid.UUID) ([]domain.AllowedEmail, error) {
	rows, err := s.q.ListAllowedEmails(ctx, pageID)
	if err != nil {
		return nil, fmt.Errorf("store: list allowed emails: %w", err)
	}
	out := make([]domain.AllowedEmail, len(rows))
	for i, r := range rows {
		out[i] = mapAllowedEmail(r)
	}
	return out, nil
}

// AddAllowedEmail добавляет email в список доступа. ErrEmailAlreadyAllowed при дубле.
func (s *Store) AddAllowedEmail(ctx context.Context, pageID uuid.UUID, email string) (domain.AllowedEmail, error) {
	row, err := s.q.AddAllowedEmail(ctx, db.AddAllowedEmailParams{StatusPageID: pageID, Email: email})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.AllowedEmail{}, ErrEmailAlreadyAllowed
		}
		return domain.AllowedEmail{}, fmt.Errorf("store: add allowed email: %w", err)
	}
	return mapAllowedEmail(row), nil
}

// AllowedEmailByID возвращает запись по id (для авторизации удаления). ErrNotFound если нет.
func (s *Store) AllowedEmailByID(ctx context.Context, id uuid.UUID) (domain.AllowedEmail, error) {
	row, err := s.q.AllowedEmailByID(ctx, id)
	if err != nil {
		return domain.AllowedEmail{}, wrapNotFound(err)
	}
	return mapAllowedEmail(row), nil
}

// DeleteAllowedEmail удаляет запись по id.
func (s *Store) DeleteAllowedEmail(ctx context.Context, id uuid.UUID) error {
	if err := s.q.DeleteAllowedEmail(ctx, id); err != nil {
		return fmt.Errorf("store: delete allowed email: %w", err)
	}
	return nil
}

// IsEmailAllowed сообщает, разрешён ли email для страницы (без учёта регистра).
func (s *Store) IsEmailAllowed(ctx context.Context, pageID uuid.UUID, email string) (bool, error) {
	ok, err := s.q.IsEmailAllowed(ctx, db.IsEmailAllowedParams{StatusPageID: pageID, Lower: email})
	if err != nil {
		return false, fmt.Errorf("store: is email allowed: %w", err)
	}
	return ok, nil
}

func mapAllowedEmail(r db.PageAllowedEmail) domain.AllowedEmail {
	return domain.AllowedEmail{
		ID:           r.ID,
		StatusPageID: r.StatusPageID,
		Email:        r.Email,
		CreatedAt:    r.CreatedAt,
	}
}
