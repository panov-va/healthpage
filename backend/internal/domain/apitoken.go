package domain

import (
	"time"

	"github.com/google/uuid"
)

// ── API-токен страницы (DESIGN §5, этап 5.1) ──

// TokenScope — область действия API-токена (нормативный набор, openapi TokenCreate.scopes;
// решение человека: грубо read/write). read — управляющие GET-эндпоинты; write — все
// мутации (компоненты/инциденты/работы/подписчики). write подразумевает read.
type TokenScope string

const (
	ScopeRead  TokenScope = "read"
	ScopeWrite TokenScope = "write"
)

// AllTokenScopes — все допустимые значения (для валидации).
var AllTokenScopes = []TokenScope{ScopeRead, ScopeWrite}

// IsValid сообщает, входит ли значение в нормативный набор.
func (s TokenScope) IsValid() bool {
	switch s {
	case ScopeRead, ScopeWrite:
		return true
	default:
		return false
	}
}

// APIToken — токен страницы для аутентификации управляющих запросов.
// Сам токен в БД не хранится (только token_hash, DESIGN §9).
type APIToken struct {
	ID           uuid.UUID
	StatusPageID uuid.UUID
	Name         string
	Scopes       []TokenScope
	LastUsedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// HasScope сообщает, что токен обладает указанной областью. write подразумевает read.
func (t APIToken) HasScope(want TokenScope) bool {
	for _, s := range t.Scopes {
		if s == want {
			return true
		}
		if want == ScopeRead && s == ScopeWrite {
			return true
		}
	}
	return false
}

// CanWrite сообщает, может ли токен выполнять мутации.
func (t APIToken) CanWrite() bool { return t.HasScope(ScopeWrite) }

// NormalizeScopes валидирует и дедуплицирует набор scope'ов. Возвращает false, если встретился
// недопустимый scope. Пустой набор по умолчанию трактуется вызывающим (см. API-слой).
func NormalizeScopes(raw []string) ([]TokenScope, bool) {
	seen := make(map[TokenScope]bool, len(raw))
	out := make([]TokenScope, 0, len(raw))
	for _, r := range raw {
		s := TokenScope(r)
		if !s.IsValid() {
			return nil, false
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out, true
}
