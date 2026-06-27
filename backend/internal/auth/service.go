// Package auth оркеструет аутентификацию оператора: регистрация, вход, ротация refresh,
// выход и проверка access-токена. Криптография — в internal/security, хранение — в internal/store.
package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/healthpage/backend/internal/domain"
	"github.com/healthpage/backend/internal/security"
	"github.com/healthpage/backend/internal/store"
)

// Ошибки уровня auth (хендлеры мапят их в HTTP-коды).
var (
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	ErrEmailTaken         = errors.New("auth: email already taken")
	ErrInvalidRefresh     = errors.New("auth: invalid refresh token")
	ErrInvalidToken       = errors.New("auth: invalid access token")
	ErrWeakPassword       = errors.New("auth: password too short")
)

// minPasswordLen — минимальная длина пароля (синхронно с openapi RegisterRequest.minLength).
const minPasswordLen = 8

// Repo — нужный auth срез store (для тестируемости). Реализуется *store.Store.
type Repo interface {
	CreateUserWithAccount(ctx context.Context, email, passwordHash, name, accountName, locale string) (domain.User, domain.Account, error)
	UserByEmail(ctx context.Context, email string) (domain.User, error)
	UserByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	CreateRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error
	RefreshTokenByHash(ctx context.Context, tokenHash string) (store.RefreshTokenRecord, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
}

// Service — сервис аутентификации.
type Service struct {
	repo   Repo
	tokens *security.TokenManager
	now    func() time.Time
}

// NewService создаёт сервис. now по умолчанию — time.Now.
func NewService(repo Repo, tokens *security.TokenManager) *Service {
	return &Service{repo: repo, tokens: tokens, now: time.Now}
}

// Result — результат успешной аутентификации.
type Result struct {
	AccessToken  string
	RefreshToken string // непрозрачный токен, отдаётся клиенту (в cookie/теле)
	ExpiresIn    int    // TTL access-токена в секундах
	User         domain.User
}

// RegisterParams — данные регистрации.
type RegisterParams struct {
	Email       string
	Password    string
	Name        string
	AccountName string
	Locale      string
}

// Register создаёт пользователя и аккаунт, возвращает пару токенов.
func (s *Service) Register(ctx context.Context, p RegisterParams) (Result, error) {
	email := normalizeEmail(p.Email)
	if email == "" {
		return Result{}, ErrInvalidCredentials
	}
	if len(p.Password) < minPasswordLen {
		return Result{}, ErrWeakPassword
	}

	hash, err := security.HashPassword(p.Password)
	if err != nil {
		return Result{}, err
	}

	locale := p.Locale
	if locale == "" {
		locale = "ru"
	}
	accountName := p.AccountName
	if accountName == "" {
		accountName = p.Name
	}

	user, _, err := s.repo.CreateUserWithAccount(ctx, email, hash, p.Name, accountName, locale)
	if err != nil {
		if errors.Is(err, store.ErrEmailTaken) {
			return Result{}, ErrEmailTaken
		}
		return Result{}, err
	}
	return s.issueTokens(ctx, user)
}

// Login проверяет пароль и выдаёт пару токенов.
func (s *Service) Login(ctx context.Context, email, password string) (Result, error) {
	user, err := s.repo.UserByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return Result{}, ErrInvalidCredentials
		}
		return Result{}, err
	}
	if !user.IsActive {
		return Result{}, ErrInvalidCredentials
	}
	ok, err := security.VerifyPassword(password, user.PasswordHash)
	if err != nil || !ok {
		return Result{}, ErrInvalidCredentials
	}
	return s.issueTokens(ctx, user)
}

// Refresh проверяет refresh-токен, ротаирует его (старый отзывается) и выдаёт новую пару.
func (s *Service) Refresh(ctx context.Context, rawRefresh string) (Result, error) {
	if rawRefresh == "" {
		return Result{}, ErrInvalidRefresh
	}
	hash := security.HashRefreshToken(rawRefresh)
	rec, err := s.repo.RefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return Result{}, ErrInvalidRefresh
		}
		return Result{}, err
	}
	if !rec.IsActive(s.now()) {
		return Result{}, ErrInvalidRefresh
	}
	// ротация: старый refresh инвалидируется.
	if err := s.repo.RevokeRefreshToken(ctx, hash); err != nil {
		return Result{}, err
	}
	user, err := s.repo.UserByID(ctx, rec.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return Result{}, ErrInvalidRefresh
		}
		return Result{}, err
	}
	if !user.IsActive {
		return Result{}, ErrInvalidRefresh
	}
	return s.issueTokens(ctx, user)
}

// Logout отзывает refresh-токен. Идемпотентен: пустой/неизвестный токен — без ошибки.
func (s *Service) Logout(ctx context.Context, rawRefresh string) error {
	if rawRefresh == "" {
		return nil
	}
	return s.repo.RevokeRefreshToken(ctx, security.HashRefreshToken(rawRefresh))
}

// Authenticate проверяет access-токен и возвращает пользователя (для middleware и /auth/me).
func (s *Service) Authenticate(ctx context.Context, accessToken string) (domain.User, error) {
	id, err := s.tokens.ParseAccess(accessToken)
	if err != nil {
		return domain.User{}, ErrInvalidToken
	}
	user, err := s.repo.UserByID(ctx, id)
	if err != nil {
		return domain.User{}, ErrInvalidToken
	}
	if !user.IsActive {
		return domain.User{}, ErrInvalidToken
	}
	return user, nil
}

func (s *Service) issueTokens(ctx context.Context, user domain.User) (Result, error) {
	now := s.now()
	access, err := s.tokens.IssueAccess(user.ID, now)
	if err != nil {
		return Result{}, err
	}
	raw, hash, err := security.GenerateRefreshToken()
	if err != nil {
		return Result{}, err
	}
	if err := s.repo.CreateRefreshToken(ctx, user.ID, hash, now.Add(s.tokens.RefreshTTL())); err != nil {
		return Result{}, err
	}
	return Result{
		AccessToken:  access,
		RefreshToken: raw,
		ExpiresIn:    int(s.tokens.AccessTTL().Seconds()),
		User:         user,
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
