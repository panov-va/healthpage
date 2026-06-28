package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func activeIncidentOn(componentID uuid.UUID, status ComponentStatus) Incident {
	return Incident{
		CurrentStatus: IncidentInvestigating,
		Components: []IncidentComponent{
			{ComponentID: componentID, ComponentStatusInIncident: status},
		},
	}
}

func TestDerivedComponentStatus_noneActive(t *testing.T) {
	cid := uuid.New()
	st, driven := DerivedComponentStatus(cid, nil, nil)
	if driven || st != StatusOperational {
		t.Fatalf("got (%q,%v), want (operational,false)", st, driven)
	}
}

func TestDerivedComponentStatus_activeIncident(t *testing.T) {
	cid := uuid.New()
	st, driven := DerivedComponentStatus(cid,
		[]Incident{activeIncidentOn(cid, StatusPartialOutage)}, nil)
	if !driven || st != StatusPartialOutage {
		t.Fatalf("got (%q,%v), want (partial_outage,true)", st, driven)
	}
}

func TestDerivedComponentStatus_resolvedIncidentIgnored(t *testing.T) {
	cid := uuid.New()
	inc := activeIncidentOn(cid, StatusMajorOutage)
	inc.CurrentStatus = IncidentResolved // устранён → не навязывает
	st, driven := DerivedComponentStatus(cid, []Incident{inc}, nil)
	if driven || st != StatusOperational {
		t.Fatalf("resolved incident must not drive status: got (%q,%v)", st, driven)
	}
}

func TestDerivedComponentStatus_deletedIgnored(t *testing.T) {
	cid := uuid.New()
	now := time.Now()
	inc := activeIncidentOn(cid, StatusMajorOutage)
	inc.DeletedAt = &now
	st, driven := DerivedComponentStatus(cid, []Incident{inc}, nil)
	if driven || st != StatusOperational {
		t.Fatalf("deleted incident must be ignored: got (%q,%v)", st, driven)
	}
}

func TestDerivedComponentStatus_activeMaintenance(t *testing.T) {
	cid := uuid.New()
	m := Maintenance{Status: MaintenanceInProgress, ComponentIDs: []uuid.UUID{cid}}
	st, driven := DerivedComponentStatus(cid, nil, []Maintenance{m})
	if !driven || st != StatusUnderMaintenance {
		t.Fatalf("got (%q,%v), want (under_maintenance,true)", st, driven)
	}
}

func TestDerivedComponentStatus_scheduledMaintenanceIgnored(t *testing.T) {
	cid := uuid.New()
	m := Maintenance{Status: MaintenanceScheduled, ComponentIDs: []uuid.UUID{cid}}
	st, driven := DerivedComponentStatus(cid, nil, []Maintenance{m})
	if driven || st != StatusOperational {
		t.Fatalf("scheduled maintenance must not drive status: got (%q,%v)", st, driven)
	}
}

func TestDerivedComponentStatus_priorityAcrossSources(t *testing.T) {
	cid := uuid.New()
	m := Maintenance{Status: MaintenanceInProgress, ComponentIDs: []uuid.UUID{cid}}

	// Реальный сбой (major) перекрывает under_maintenance (§6).
	st, _ := DerivedComponentStatus(cid,
		[]Incident{activeIncidentOn(cid, StatusMajorOutage)}, []Maintenance{m})
	if st != StatusMajorOutage {
		t.Fatalf("outage must win over maintenance: got %q", st)
	}

	// ...а under_maintenance перекрывает деградацию (§6).
	st, _ = DerivedComponentStatus(cid,
		[]Incident{activeIncidentOn(cid, StatusDegradedPerformance)}, []Maintenance{m})
	if st != StatusUnderMaintenance {
		t.Fatalf("maintenance must win over degraded: got %q", st)
	}
}

func TestDerivedComponentStatus_worstAmongIncidents(t *testing.T) {
	cid := uuid.New()
	st, driven := DerivedComponentStatus(cid, []Incident{
		activeIncidentOn(cid, StatusDegradedPerformance),
		activeIncidentOn(cid, StatusPartialOutage),
	}, nil)
	if !driven || st != StatusPartialOutage {
		t.Fatalf("got (%q,%v), want worst (partial_outage,true)", st, driven)
	}
}

func TestDerivedComponentStatus_unaffectedComponent(t *testing.T) {
	target := uuid.New()
	other := uuid.New()
	st, driven := DerivedComponentStatus(target,
		[]Incident{activeIncidentOn(other, StatusMajorOutage)},
		[]Maintenance{{Status: MaintenanceInProgress, ComponentIDs: []uuid.UUID{other}}})
	if driven || st != StatusOperational {
		t.Fatalf("unaffected component must be operational: got (%q,%v)", st, driven)
	}
}
