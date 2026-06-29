package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ── Статус инцидента ──

// IncidentStatus — стадия жизненного цикла инцидента
// (нормативный enum, DESIGN §3.3 / openapi IncidentStatus).
type IncidentStatus string

const (
	IncidentInvestigating IncidentStatus = "investigating" // идёт расследование
	IncidentIdentified    IncidentStatus = "identified"    // причина обнаружена
	IncidentMonitoring    IncidentStatus = "monitoring"    // наблюдаем за исправлением
	IncidentResolved      IncidentStatus = "resolved"      // устранено
)

// AllIncidentStatuses — все допустимые значения (для валидации/перебора).
var AllIncidentStatuses = []IncidentStatus{
	IncidentInvestigating,
	IncidentIdentified,
	IncidentMonitoring,
	IncidentResolved,
}

// IsValid сообщает, входит ли значение в нормативный enum.
func (s IncidentStatus) IsValid() bool {
	switch s {
	case IncidentInvestigating, IncidentIdentified, IncidentMonitoring, IncidentResolved:
		return true
	default:
		return false
	}
}

// IsTerminal сообщает, что статус закрывает инцидент (resolved). Из него инцидент можно
// повторно открыть переводом в активный статус (см. Incident.ApplyStatusChange).
func (s IncidentStatus) IsTerminal() bool { return s == IncidentResolved }

// ── Уровень влияния ──

// IncidentImpact — уровень влияния инцидента (нормативный enum, DESIGN §3.3 / openapi IncidentImpact).
type IncidentImpact string

const (
	ImpactNone     IncidentImpact = "none"
	ImpactMinor    IncidentImpact = "minor"
	ImpactMajor    IncidentImpact = "major"
	ImpactCritical IncidentImpact = "critical"
)

// AllIncidentImpacts — все допустимые значения (для валидации/перебора).
var AllIncidentImpacts = []IncidentImpact{
	ImpactNone,
	ImpactMinor,
	ImpactMajor,
	ImpactCritical,
}

// IsValid сообщает, входит ли значение в нормативный enum.
func (i IncidentImpact) IsValid() bool {
	switch i {
	case ImpactNone, ImpactMinor, ImpactMajor, ImpactCritical:
		return true
	default:
		return false
	}
}

// impactSeverity — порядок тяжести влияния: none(0) < minor(1) < major(2) < critical(3).
// Неизвестное значение получает -1 и при агрегации игнорируется в пользу валидных.
func impactSeverity(i IncidentImpact) int {
	switch i {
	case ImpactNone:
		return 0
	case ImpactMinor:
		return 1
	case ImpactMajor:
		return 2
	case ImpactCritical:
		return 3
	default:
		return -1
	}
}

// WorstImpact возвращает наибольший по тяжести impact из набора. Пустой набор → none.
func WorstImpact(impacts ...IncidentImpact) IncidentImpact {
	worst := ImpactNone
	worstSev := 0
	for _, im := range impacts {
		if sev := impactSeverity(im); sev > worstSev {
			worst, worstSev = im, sev
		}
	}
	return worst
}

// ── Сущности ──

// IncidentComponent — затронутый инцидентом компонент и его состояние в рамках инцидента
// (DESIGN §3.3 / openapi IncidentComponent).
type IncidentComponent struct {
	ID                        uuid.UUID
	IncidentID                uuid.UUID
	ComponentID               uuid.UUID
	ComponentStatusInIncident ComponentStatus
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

// IncidentUpdate — запись ленты обновлений инцидента: статус на момент обновления, текст
// (Markdown) и флаг «уведомить подписчиков» (DESIGN §3.3 / openapi IncidentUpdate).
type IncidentUpdate struct {
	ID                uuid.UUID
	IncidentID        uuid.UUID
	Status            IncidentStatus
	Body              string
	NotifySubscribers bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Incident — публичный отчёт о сбое с хронологией обновлений (DESIGN §3.3).
type Incident struct {
	ID            uuid.UUID
	StatusPageID  uuid.UUID
	Title         string
	CurrentStatus IncidentStatus
	Impact        IncidentImpact
	StartedAt     time.Time
	ResolvedAt    *time.Time
	Postmortem    *string
	IsVisible     bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time

	// Связи (заполняются store при чтении агрегата; в самой строке incidents их нет).
	Components []IncidentComponent
	Updates    []IncidentUpdate
}

// IsResolved сообщает, что инцидент устранён.
func (i Incident) IsResolved() bool { return i.CurrentStatus == IncidentResolved }

// IsActive сообщает, что инцидент ещё не устранён (влияет на статус компонентов, DESIGN §6).
func (i Incident) IsActive() bool { return !i.IsResolved() }

// ── Ошибки жизненного цикла ──

var (
	// ErrInvalidIncidentStatus — попытка перевести инцидент в значение вне нормативного enum.
	ErrInvalidIncidentStatus = errors.New("invalid incident status")
	// ErrInvalidIncidentImpact — impact вне нормативного enum.
	ErrInvalidIncidentImpact = errors.New("invalid incident impact")
	// ErrPostmortemBeforeResolved — постмортем можно прикрепить только к устранённому инциденту
	// (DESIGN §3.3: «после resolved можно прикрепить пост-мортем»).
	ErrPostmortemBeforeResolved = errors.New("postmortem allowed only after incident is resolved")
)

// ApplyStatusChange переводит инцидент в новый статус на момент at и поддерживает инвариант
// ResolvedAt (DESIGN §3.3): переход в resolved фиксирует время устранения (если ещё не задано),
// а любой переход из resolved в активный статус (повторное открытие инцидента) сбрасывает его.
//
// Переходы между валидными статусами не ограничиваются: оператор может вернуться, например, из
// monitoring в investigating или повторно открыть устранённый инцидент. Ошибка — только если
// статус вне нормативного enum.
func (i *Incident) ApplyStatusChange(status IncidentStatus, at time.Time) error {
	if !status.IsValid() {
		return fmt.Errorf("%w: %q", ErrInvalidIncidentStatus, status)
	}
	i.CurrentStatus = status
	if status == IncidentResolved {
		if i.ResolvedAt == nil {
			t := at
			i.ResolvedAt = &t
		}
		return nil
	}
	i.ResolvedAt = nil
	return nil
}

// CanSetPostmortem сообщает, допустимо ли сейчас прикреплять постмортем (только к resolved).
func (i Incident) CanSetPostmortem() bool { return i.IsResolved() }

// SetPostmortem прикрепляет текст постмортема к устранённому инциденту. Пустая строка снимает
// постмортем. Возвращает ErrPostmortemBeforeResolved, если инцидент ещё не устранён.
func (i *Incident) SetPostmortem(text string) error {
	if !i.CanSetPostmortem() {
		return ErrPostmortemBeforeResolved
	}
	if text == "" {
		i.Postmortem = nil
		return nil
	}
	t := text
	i.Postmortem = &t
	return nil
}
