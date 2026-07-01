package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ── Changelog / страница релизов (DESIGN §5; этап 7.2) ──

// ChangelogEntry — запись ленты анонсов продукта (отдельно от инцидентов).
type ChangelogEntry struct {
	ID           uuid.UUID
	StatusPageID uuid.UUID
	Title        string
	Body         string
	Published    bool
	PublishedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ValidateChangelogTitle проверяет обязательный непустой заголовок.
func ValidateChangelogTitle(title string) bool {
	return strings.TrimSpace(title) != ""
}
