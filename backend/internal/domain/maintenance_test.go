package domain

import (
	"errors"
	"testing"
	"time"
)

func TestMaintenanceStatus_IsValid(t *testing.T) {
	for _, s := range AllMaintenanceStatuses {
		if !s.IsValid() {
			t.Errorf("%q should be valid", s)
		}
	}
	if MaintenanceStatus("nope").IsValid() {
		t.Error("invalid maintenance status reported valid")
	}
}

func TestMaintenance_StagePredicates(t *testing.T) {
	if !(Maintenance{Status: MaintenanceScheduled}).IsScheduled() {
		t.Error("scheduled predicate")
	}
	inProg := Maintenance{Status: MaintenanceInProgress}
	if !inProg.IsInProgress() || !inProg.IsActive() {
		t.Error("in_progress must be in-progress and active")
	}
	done := Maintenance{Status: MaintenanceCompleted}
	if !done.IsCompleted() || done.IsActive() {
		t.Error("completed must be completed and not active")
	}
}

func TestMaintenance_ImposedComponentStatus(t *testing.T) {
	// Только in_progress навязывает under_maintenance.
	if st, ok := (Maintenance{Status: MaintenanceInProgress}).ImposedComponentStatus(); !ok || st != StatusUnderMaintenance {
		t.Fatalf("in_progress: got (%q,%v), want (under_maintenance,true)", st, ok)
	}
	for _, s := range []MaintenanceStatus{MaintenanceScheduled, MaintenanceCompleted} {
		if _, ok := (Maintenance{Status: s}).ImposedComponentStatus(); ok {
			t.Errorf("%q must not impose a component status", s)
		}
	}
}

func TestMaintenance_ValidateSchedule(t *testing.T) {
	start := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	if err := (Maintenance{ScheduledStart: start, ScheduledEnd: start.Add(time.Hour)}).ValidateSchedule(); err != nil {
		t.Fatalf("valid window rejected: %v", err)
	}
	// Конец не позже начала — ошибка (включая равенство).
	for _, end := range []time.Time{start, start.Add(-time.Hour)} {
		if err := (Maintenance{ScheduledStart: start, ScheduledEnd: end}).ValidateSchedule(); !errors.Is(err, ErrInvalidSchedule) {
			t.Errorf("end=%v: err=%v, want ErrInvalidSchedule", end, err)
		}
	}
}

func TestMaintenance_ApplyStatusChange_lifecycle(t *testing.T) {
	t0 := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	m := Maintenance{Status: MaintenanceScheduled}

	// scheduled -> in_progress: фиксирует StartedAt, CompletedAt пуст.
	if err := m.ApplyStatusChange(MaintenanceInProgress, t0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.StartedAt == nil || !m.StartedAt.Equal(t0) {
		t.Fatalf("StartedAt = %v, want %v", m.StartedAt, t0)
	}
	if m.CompletedAt != nil {
		t.Fatalf("CompletedAt must stay nil while in_progress, got %v", m.CompletedAt)
	}

	// in_progress -> completed: фиксирует CompletedAt, StartedAt сохраняется.
	t1 := t0.Add(time.Hour)
	_ = m.ApplyStatusChange(MaintenanceCompleted, t1)
	if m.CompletedAt == nil || !m.CompletedAt.Equal(t1) {
		t.Fatalf("CompletedAt = %v, want %v", m.CompletedAt, t1)
	}
	if m.StartedAt == nil || !m.StartedAt.Equal(t0) {
		t.Fatalf("StartedAt changed on completion: %v", m.StartedAt)
	}
}

func TestMaintenance_ApplyStatusChange_reopenAndReset(t *testing.T) {
	t0 := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	m := Maintenance{Status: MaintenanceScheduled}
	_ = m.ApplyStatusChange(MaintenanceInProgress, t0)
	started := *m.StartedAt
	_ = m.ApplyStatusChange(MaintenanceCompleted, t0.Add(time.Hour))

	// completed -> in_progress (повторный запуск): сбрасывает CompletedAt, StartedAt сохраняется.
	_ = m.ApplyStatusChange(MaintenanceInProgress, t0.Add(2*time.Hour))
	if m.CompletedAt != nil {
		t.Fatalf("reopen must clear CompletedAt, got %v", m.CompletedAt)
	}
	if m.StartedAt == nil || !m.StartedAt.Equal(started) {
		t.Fatalf("StartedAt must be preserved on reopen, got %v", m.StartedAt)
	}

	// -> scheduled: сбрасывает обе метки.
	_ = m.ApplyStatusChange(MaintenanceScheduled, t0.Add(3*time.Hour))
	if m.StartedAt != nil || m.CompletedAt != nil {
		t.Fatalf("scheduled must clear both marks, got start=%v end=%v", m.StartedAt, m.CompletedAt)
	}
}

func TestMaintenance_ApplyStatusChange_invalid(t *testing.T) {
	m := Maintenance{Status: MaintenanceScheduled}
	err := m.ApplyStatusChange(MaintenanceStatus("bogus"), time.Now())
	if !errors.Is(err, ErrInvalidMaintenanceStatus) {
		t.Fatalf("err = %v, want ErrInvalidMaintenanceStatus", err)
	}
	if m.Status != MaintenanceScheduled {
		t.Error("invalid transition must not mutate status")
	}
}
