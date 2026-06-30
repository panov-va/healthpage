package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ErrInvalidToken — access-токен недействителен (подпись/срок/формат).
var ErrInvalidToken = errors.New("security: invalid token")

const issuer = "healthpage"

// TokenManager выпускает и проверяет операторские access-JWT и генерирует refresh-токены.
type TokenManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewTokenManager создаёт менеджер токенов. secret должен быть непустым.
func NewTokenManager(secret string, accessTTL, refreshTTL time.Duration) (*TokenManager, error) {
	if secret == "" {
		return nil, errors.New("security: empty jwt secret")
	}
	return &TokenManager{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}, nil
}

// AccessTTL — срок жизни access-токена (для отдачи expires_in).
func (m *TokenManager) AccessTTL() time.Duration  { return m.accessTTL }
func (m *TokenManager) RefreshTTL() time.Duration { return m.refreshTTL }

// IssueAccess выпускает подписанный access-JWT с subject=userID.
// now передаётся явно для тестируемости.
func (m *TokenManager) IssueAccess(userID uuid.UUID, now time.Time) (string, error) {
	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("security: sign token: %w", err)
	}
	return signed, nil
}

// ParseAccess проверяет подпись/срок и возвращает userID из subject.
func (m *TokenManager) ParseAccess(tokenStr string) (uuid.UUID, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	}, jwt.WithIssuer(issuer), jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return uuid.Nil, errors.Join(ErrInvalidToken, err)
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	return id, nil
}

// GenerateRefreshToken возвращает непрозрачный refresh-токен (отдаётся клиенту) и его
// SHA-256 хэш в hex (хранится в БД). Сам токен в БД не хранится.
func GenerateRefreshToken() (token, hash string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("security: read refresh: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	hash = HashRefreshToken(token)
	return token, hash, nil
}

// HashRefreshToken возвращает hex SHA-256 от refresh-токена (для поиска/сравнения в БД).
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// apiTokenPrefix — узнаваемый префикс API-токена страницы (этап 5.1). Облегчает распознавание
// токена в логах/секрет-сканерах и отделяет page-токен от операторского Bearer-JWT.
const apiTokenPrefix = "hp_"

// GenerateAPIToken возвращает непрозрачный API-токен страницы (отдаётся клиенту единожды) и его
// SHA-256 хэш в hex (хранится в БД). Сам токен в БД не хранится (DESIGN §9).
func GenerateAPIToken() (token, hash string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("security: read api token: %w", err)
	}
	token = apiTokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	hash = HashAPIToken(token)
	return token, hash, nil
}

// HashAPIToken возвращает hex SHA-256 от API-токена (для поиска/сравнения в БД).
func HashAPIToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
