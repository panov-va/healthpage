// Package domain содержит сущности ядра HealthPage и бизнес-правила, не зависящие от БД,
// HTTP и внешних библиотек. Наружу из backend не экспортируется (CLAUDE.md §7).
package domain

import (
	"time"

	"github.com/google/uuid"
)

// ── Аккаунты, пользователи, доступ ──

// BillingPlan — тариф аккаунта (нормативный enum, DESIGN §5).
type BillingPlan string

const (
	PlanFree    BillingPlan = "free"
	PlanPremium BillingPlan = "premium"
)

// IsValid сообщает, входит ли значение в нормативный enum.
func (p BillingPlan) IsValid() bool { return p == PlanFree || p == PlanPremium }

// Role — роль пользователя на странице статуса (DESIGN §5, Membership.role).
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

// IsValid сообщает, входит ли роль в допустимый набор.
func (r Role) IsValid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleEditor, RoleViewer:
		return true
	default:
		return false
	}
}

// CanEdit сообщает, может ли роль изменять контент (статусы, инциденты, работы).
// viewer — только чтение; owner/admin/editor — запись.
func (r Role) CanEdit() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleEditor:
		return true
	default:
		return false
	}
}

// User — учётная запись оператора.
type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Name         string
	Locale       string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Account — владелец одной или нескольких страниц статуса.
type Account struct {
	ID          uuid.UUID
	Name        string
	BillingPlan BillingPlan
	OwnerUserID uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Membership — связь пользователя со страницей статуса и его роль на ней.
type Membership struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	StatusPageID uuid.UUID
	Role         Role
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ── Страница статуса ──

// Visibility — публичность страницы (DESIGN §3.1).
type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

// IsValid сообщает, входит ли значение в допустимый набор.
func (v Visibility) IsValid() bool { return v == VisibilityPublic || v == VisibilityPrivate }

// StatusPage — публичная страница статуса продукта.
type StatusPage struct {
	ID             uuid.UUID
	AccountID      uuid.UUID
	Name           string
	Description    string
	Slug           string
	Timezone       string
	DefaultLocale  string
	Visibility     Visibility
	PasswordHash   *string
	CustomDomain   *string
	DomainVerified bool
	Theme          []byte // jsonb: colors, layout, dark_mode
	LogoURL        *string
	FaviconURL     *string
	HidePoweredBy  bool
	SMTPConfig     []byte // jsonb, nullable
	FromEmail      *string
	RedirectURL    *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// AllowedEmail — адрес из списка доступа приватной страницы (этап 4.2.1, magic-link).
type AllowedEmail struct {
	ID           uuid.UUID
	StatusPageID uuid.UUID
	Email        string
	CreatedAt    time.Time
}

// IsPrivate сообщает, закрыта ли страница от анонимного доступа.
func (p StatusPage) IsPrivate() bool { return p.Visibility == VisibilityPrivate }

// ── Компоненты ──

// ComponentGroup — группа компонентов на странице.
type ComponentGroup struct {
	ID           uuid.UUID
	StatusPageID uuid.UUID
	Name         string
	Position     int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}

// Component — единица сервиса, чьё состояние видит клиент. Может входить в группу и/или
// быть узлом дерева через ParentID (DESIGN §3.2).
type Component struct {
	ID            uuid.UUID
	StatusPageID  uuid.UUID
	GroupID       *uuid.UUID
	ParentID      *uuid.UUID
	Name          string
	Description   string
	Position      int
	CurrentStatus ComponentStatus
	IsPrivate     bool
	ShowUptime    bool
	DisplayState  bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
}

// CountsTowardStatus сообщает, влияет ли компонент на общий/групповой статус публичной страницы:
// приватные и помеченные «не показывать состояние» (информационные метки) — не влияют (DESIGN §3.2, §6).
func (c Component) CountsTowardStatus() bool {
	return !c.IsPrivate && c.DisplayState
}

// ── История статусов ──

// HistorySource — источник изменения статуса компонента (DESIGN §5).
type HistorySource string

const (
	SourceManual      HistorySource = "manual"
	SourceIncident    HistorySource = "incident"
	SourceMaintenance HistorySource = "maintenance"
	SourceAPI         HistorySource = "api"
)

// IsValid сообщает, входит ли значение в допустимый набор.
func (s HistorySource) IsValid() bool {
	switch s {
	case SourceManual, SourceIncident, SourceMaintenance, SourceAPI:
		return true
	default:
		return false
	}
}

// ComponentStatusHistory — период нахождения компонента в определённом статусе
// (для расчёта uptime и графика, DESIGN §5). Открытый период — EndedAt == nil.
type ComponentStatusHistory struct {
	ID          uuid.UUID
	ComponentID uuid.UUID
	Status      ComponentStatus
	StartedAt   time.Time
	EndedAt     *time.Time
	Source      HistorySource
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
