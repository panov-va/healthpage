package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ── Статус плановых работ ──

// MaintenanceStatus — стадия жизненного цикла плановых работ
// (нормативный enum, DESIGN §3.4 / openapi MaintenanceStatus).
type MaintenanceStatus string

const (
	MaintenanceScheduled  MaintenanceStatus = "scheduled"   // запланировано
	MaintenanceInProgress MaintenanceStatus = "in_progress" // выполняется
	MaintenanceCompleted  MaintenanceStatus = "completed"   // завершено
)

// AllMaintenanceStatuses — все допустимые значения (для валидации/перебора).
var AllMaintenanceStatuses = []MaintenanceStatus{
	MaintenanceScheduled,
	MaintenanceInProgress,
	MaintenanceCompleted,
}

// IsValid сообщает, входит ли значение в нормативный enum.
func (s MaintenanceStatus) IsValid() bool {
	switch s {
	case MaintenanceScheduled, MaintenanceInProgress, MaintenanceCompleted:
		return true
	default:
		return false
	}
}

// ── Сущности ──

// MaintenanceUpdate — запись ленты обновлений работ: текст (Markdown) и флаг «уведомить
// подписчиков». В отличие от инцидента, у обновления работ нет своего статуса
// (DESIGN §3.4 / openapi MaintenanceUpdate).
type MaintenanceUpdate struct {
	ID                uuid.UUID
	MaintenanceID     uuid.UUID
	Body              string
	NotifySubscribers bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Maintenance — запланированные работы с окном проведения и хронологией обновлений (DESIGN §3.4).
type Maintenance struct {
	ID             uuid.UUID
	StatusPageID   uuid.UUID
	Title          string
	Description    string
	Status         MaintenanceStatus
	ScheduledStart time.Time
	ScheduledEnd   time.Time
	StartedAt      *time.Time // фактический старт (перевод в in_progress)
	CompletedAt    *time.Time // фактическое завершение (перевод в completed)
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time

	// Связи (заполняются store при чтении агрегата; в самой строке maintenances их нет).
	ComponentIDs []uuid.UUID // затронутые компоненты (maintenance_components)
	Updates      []MaintenanceUpdate
}

// IsScheduled / IsInProgress / IsCompleted — предикаты текущей стадии.
func (m Maintenance) IsScheduled() bool  { return m.Status == MaintenanceScheduled }
func (m Maintenance) IsInProgress() bool { return m.Status == MaintenanceInProgress }
func (m Maintenance) IsCompleted() bool  { return m.Status == MaintenanceCompleted }

// IsActive сообщает, что работы идут прямо сейчас (in_progress) — именно тогда затронутые
// компоненты переводятся в under_maintenance (DESIGN §3.4, §6).
func (m Maintenance) IsActive() bool { return m.IsInProgress() }

// ImposedComponentStatus возвращает статус, который активные работы навязывают своим
// компонентам: under_maintenance во время in_progress. Вне in_progress работы статус не
// навязывают (ok=false) — это основа авто-перевода компонентов (DESIGN §3.4; деривация — 2.4).
func (m Maintenance) ImposedComponentStatus() (status ComponentStatus, ok bool) {
	if m.IsActive() {
		return StatusUnderMaintenance, true
	}
	return "", false
}

// ── Ошибки жизненного цикла ──

var (
	// ErrInvalidMaintenanceStatus — попытка перевести работы в значение вне нормативного enum.
	ErrInvalidMaintenanceStatus = errors.New("invalid maintenance status")
	// ErrInvalidSchedule — окно работ задано некорректно (конец не позже начала).
	ErrInvalidSchedule = errors.New("scheduled_end must be after scheduled_start")
)

// ValidateSchedule проверяет, что окно работ непусто: scheduled_end строго позже scheduled_start.
func (m Maintenance) ValidateSchedule() error {
	if !m.ScheduledEnd.After(m.ScheduledStart) {
		return ErrInvalidSchedule
	}
	return nil
}

// ApplyStatusChange переводит работы в новый статус на момент at и поддерживает фактические
// метки StartedAt/CompletedAt (DESIGN §3.4):
//   - in_progress фиксирует StartedAt (если ещё не задан) и сбрасывает CompletedAt
//     (на случай повторного запуска завершённых работ);
//   - completed фиксирует CompletedAt (если ещё не задан), StartedAt не трогает;
//   - возврат в scheduled сбрасывает обе метки.
//
// Переходы между валидными статусами не ограничиваются (оператор может вернуть работы назад).
// Ошибка — только если статус вне нормативного enum.
func (m *Maintenance) ApplyStatusChange(status MaintenanceStatus, at time.Time) error {
	if !status.IsValid() {
		return fmt.Errorf("%w: %q", ErrInvalidMaintenanceStatus, status)
	}
	m.Status = status
	switch status {
	case MaintenanceScheduled:
		m.StartedAt = nil
		m.CompletedAt = nil
	case MaintenanceInProgress:
		if m.StartedAt == nil {
			t := at
			m.StartedAt = &t
		}
		m.CompletedAt = nil
	case MaintenanceCompleted:
		if m.CompletedAt == nil {
			t := at
			m.CompletedAt = &t
		}
	}
	return nil
}
