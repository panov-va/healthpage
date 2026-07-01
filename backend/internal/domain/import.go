package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ── Миграция данных из внешних сервисов (DESIGN §4.3, §7; этап 7.5) ──

// ImportSource — источник импорта (openapi ImportSource). MVP: реализован только statuspal.
type ImportSource string

const (
	SourceStatusPal  ImportSource = "statuspal"
	SourceStatusPage ImportSource = "statuspage"
	SourceInstatus   ImportSource = "instatus"
	SourceStatusmate ImportSource = "statusmate"
)

// IsValid сообщает, входит ли значение в нормативный набор.
func (s ImportSource) IsValid() bool {
	switch s {
	case SourceStatusPal, SourceStatusPage, SourceInstatus, SourceStatusmate:
		return true
	default:
		return false
	}
}

// ImportRegion — регион источника (openapi ImportRegion).
type ImportRegion string

const (
	RegionUS ImportRegion = "us"
	RegionEU ImportRegion = "eu"
)

// IsValid допускает пустое значение (регион опционален).
func (r ImportRegion) IsValid() bool { return r == "" || r == RegionUS || r == RegionEU }

// ImportMode — режим повторного импорта (openapi ImportMode).
type ImportMode string

const (
	ModeSkip   ImportMode = "skip"   // пропускать уже перенесённые сущности
	ModeUpdate ImportMode = "update" // обновлять уже перенесённые сущности
)

// IsValid сообщает, входит ли значение в нормативный набор.
func (m ImportMode) IsValid() bool { return m == ModeSkip || m == ModeUpdate }

// ImportStatus — статус задачи импорта (openapi ImportStatus).
type ImportStatus string

const (
	ImportPending   ImportStatus = "pending"
	ImportRunning   ImportStatus = "running"
	ImportCompleted ImportStatus = "completed"
	ImportFailed    ImportStatus = "failed"
)

// IsValid сообщает, входит ли значение в нормативный набор.
func (s ImportStatus) IsValid() bool {
	switch s {
	case ImportPending, ImportRunning, ImportCompleted, ImportFailed:
		return true
	default:
		return false
	}
}

// ImportJob — задача импорта. api_key НЕ хранится (передаётся воркеру в сообщении очереди).
type ImportJob struct {
	ID           uuid.UUID
	StatusPageID uuid.UUID
	AccountID    uuid.UUID
	Source       ImportSource
	Region       ImportRegion
	Subdomain    string
	Mode         ImportMode
	Status       ImportStatus
	Report       ImportReport
	Error        string
	CreatedAt    time.Time
	FinishedAt   *time.Time
	UpdatedAt    time.Time
}

// ImportReport — итог импорта (перенесено/пропущено/ошибки), сериализуется в report jsonb.
type ImportReport struct {
	ComponentsCreated   int      `json:"components_created"`
	ComponentsSkipped   int      `json:"components_skipped"`
	IncidentsCreated    int      `json:"incidents_created"`
	IncidentsSkipped    int      `json:"incidents_skipped"`
	MaintenancesCreated int      `json:"maintenances_created"`
	MaintenancesSkipped int      `json:"maintenances_skipped"`
	SubscribersImported int      `json:"subscribers_imported"`
	SubscribersSkipped  int      `json:"subscribers_skipped"`
	Errors              []string `json:"errors,omitempty"`
}

// ── Промежуточные (нормализованные) сущности источника (DESIGN §4.3) ──

// ImportedComponent — компонент источника. ParentExternalID — для дерева подкомпонентов.
type ImportedComponent struct {
	ExternalID       string
	Name             string
	Description      string
	Status           ComponentStatus
	ParentExternalID string
	GroupName        string
	IsPrivate        bool
}

// ImportedIncidentUpdate — одно обновление инцидента источника.
type ImportedIncidentUpdate struct {
	Status    IncidentStatus
	Body      string
	CreatedAt time.Time
}

// ImportedIncident — инцидент источника с хроникой.
type ImportedIncident struct {
	ExternalID string
	Title      string
	Impact     IncidentImpact
	Status     IncidentStatus
	StartedAt  time.Time
	ResolvedAt *time.Time
	Updates    []ImportedIncidentUpdate
	Components []string // external component ids
}

// ImportedMaintenance — плановая работа источника.
type ImportedMaintenance struct {
	ExternalID  string
	Title       string
	Description string
	Status      MaintenanceStatus
	StartAt     time.Time
	EndAt       time.Time
	Components  []string // external component ids
}

// ImportedSubscriber — подписчик источника. Импортируются только email (у нас нет SMS).
type ImportedSubscriber struct {
	ExternalID string
	Email      string
}

// ImportPreview — предпросмотр объёма импорта (Importer.Probe).
type ImportPreview struct {
	Components   int
	Incidents    int
	Maintenances int
	Subscribers  int
}

// ImportCreds — учётные данные источника (не хранятся дольше задачи).
type ImportCreds struct {
	APIKey    string
	Region    ImportRegion
	Subdomain string
}

// Importer — адаптер внешнего источника (DESIGN §4.3). Каждый источник реализует единый интерфейс;
// доменный код импорта не зависит от конкретного источника.
type Importer interface {
	Source() ImportSource
	Probe(ctx context.Context, creds ImportCreds) (ImportPreview, error)
	FetchComponents(ctx context.Context, creds ImportCreds) ([]ImportedComponent, error)
	FetchIncidents(ctx context.Context, creds ImportCreds) ([]ImportedIncident, error)
	FetchMaintenances(ctx context.Context, creds ImportCreds) ([]ImportedMaintenance, error)
	FetchSubscribers(ctx context.Context, creds ImportCreds) ([]ImportedSubscriber, error)
}
