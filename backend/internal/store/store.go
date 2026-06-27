// Package store — доступ к PostgreSQL поверх sqlc-сгенерированного кода (internal/store/db).
// Наружу отдаёт доменные сущности, скрывая детали БД.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/store/db"
)

// Ошибки store, на которые реагируют вышележащие слои.
var (
	ErrNotFound   = errors.New("store: not found")
	ErrEmailTaken = errors.New("store: email already taken")
)

// Store держит пул соединений и sqlc-запросы.
type Store struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// New открывает пул соединений к PostgreSQL и проверяет связь.
func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &Store{pool: pool, q: db.New(pool)}, nil
}

// Close закрывает пул.
func (s *Store) Close() { s.pool.Close() }

// RefreshTokenRecord — представление хранимого refresh-токена для проверки в auth-слое.
type RefreshTokenRecord struct {
	UserID    uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// IsActive сообщает, что токен не отозван и не истёк на момент now.
func (r RefreshTokenRecord) IsActive(now time.Time) bool {
	return r.RevokedAt == nil && now.Before(r.ExpiresAt)
}

// CreateUserWithAccount создаёт пользователя и его аккаунт в одной транзакции.
// Возвращает ErrEmailTaken при конфликте уникальности email.
func (s *Store) CreateUserWithAccount(
	ctx context.Context, email, passwordHash, name, accountName, locale string,
) (domain.User, domain.Account, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, domain.Account{}, fmt.Errorf("store: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := s.q.WithTx(tx)
	u, err := q.CreateUser(ctx, db.CreateUserParams{
		Email: email, PasswordHash: passwordHash, Name: name, Locale: locale,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.User{}, domain.Account{}, ErrEmailTaken
		}
		return domain.User{}, domain.Account{}, fmt.Errorf("store: create user: %w", err)
	}

	a, err := q.CreateAccount(ctx, db.CreateAccountParams{Name: accountName, OwnerUserID: u.ID})
	if err != nil {
		return domain.User{}, domain.Account{}, fmt.Errorf("store: create account: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, domain.Account{}, fmt.Errorf("store: commit: %w", err)
	}
	return mapUser(u), mapAccount(a), nil
}

// UserByEmail находит пользователя по email (без учёта регистра). ErrNotFound если нет.
func (s *Store) UserByEmail(ctx context.Context, email string) (domain.User, error) {
	u, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		return domain.User{}, wrapNotFound(err)
	}
	return mapUser(u), nil
}

// UserByID находит пользователя по id. ErrNotFound если нет.
func (s *Store) UserByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	u, err := s.q.GetUserByID(ctx, id)
	if err != nil {
		return domain.User{}, wrapNotFound(err)
	}
	return mapUser(u), nil
}

// CreateRefreshToken сохраняет хэш refresh-токена.
func (s *Store) CreateRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	_, err := s.q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID: userID, TokenHash: tokenHash, ExpiresAt: expiresAt,
	})
	if err != nil {
		return fmt.Errorf("store: create refresh token: %w", err)
	}
	return nil
}

// RefreshTokenByHash возвращает запись по хэшу. ErrNotFound если нет.
func (s *Store) RefreshTokenByHash(ctx context.Context, tokenHash string) (RefreshTokenRecord, error) {
	t, err := s.q.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		return RefreshTokenRecord{}, wrapNotFound(err)
	}
	return RefreshTokenRecord{UserID: t.UserID, ExpiresAt: t.ExpiresAt, RevokedAt: t.RevokedAt}, nil
}

// RevokeRefreshToken помечает токен отозванным (идемпотентно).
func (s *Store) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	if err := s.q.RevokeRefreshToken(ctx, tokenHash); err != nil {
		return fmt.Errorf("store: revoke refresh token: %w", err)
	}
	return nil
}

func mapUser(u db.User) domain.User {
	return domain.User{
		ID: u.ID, Email: u.Email, PasswordHash: u.PasswordHash, Name: u.Name,
		Locale: u.Locale, IsActive: u.IsActive, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

func mapAccount(a db.Account) domain.Account {
	return domain.Account{
		ID: a.ID, Name: a.Name, BillingPlan: domain.BillingPlan(a.BillingPlan),
		OwnerUserID: a.OwnerUserID, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

func wrapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
