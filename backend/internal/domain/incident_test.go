package domain

import (
	"errors"
	"testing"
	"time"
)

func TestIncidentStatus_IsValid(t *testing.T) {
	for _, s := range AllIncidentStatuses {
		if !s.IsValid() {
			t.Errorf("%q should be valid", s)
		}
	}
	if IncidentStatus("nope").IsValid() {
		t.Error("invalid incident status reported valid")
	}
}

func TestIncidentStatus_IsTerminal(t *testing.T) {
	if !IncidentResolved.IsTerminal() {
		t.Error("resolved must be terminal")
	}
	for _, s := range []IncidentStatus{IncidentInvestigating, IncidentIdentified, IncidentMonitoring} {
		if s.IsTerminal() {
			t.Errorf("%q must not be terminal", s)
		}
	}
}

func TestIncidentImpact_IsValid(t *testing.T) {
	for _, im := range AllIncidentImpacts {
		if !im.IsValid() {
			t.Errorf("%q should be valid", im)
		}
	}
	if IncidentImpact("nope").IsValid() {
		t.Error("invalid impact reported valid")
	}
}

func TestWorstImpact(t *testing.T) {
	cases := []struct {
		name    string
		impacts []IncidentImpact
		want    IncidentImpact
	}{
		{"empty -> none", nil, ImpactNone},
		{"minor beats none", []IncidentImpact{ImpactNone, ImpactMinor}, ImpactMinor},
		{"major beats minor", []IncidentImpact{ImpactMinor, ImpactMajor}, ImpactMajor},
		{"critical beats major", []IncidentImpact{ImpactMajor, ImpactCritical}, ImpactCritical},
		{"unknown ignored", []IncidentImpact{IncidentImpact("bogus"), ImpactMinor}, ImpactMinor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := WorstImpact(tc.impacts...); got != tc.want {
				t.Fatalf("WorstImpact(%v) = %q, want %q", tc.impacts, got, tc.want)
			}
		})
	}
}

func TestIncident_IsResolvedActive(t *testing.T) {
	open := Incident{CurrentStatus: IncidentMonitoring}
	if open.IsResolved() || !open.IsActive() {
		t.Error("monitoring incident must be active, not resolved")
	}
	done := Incident{CurrentStatus: IncidentResolved}
	if !done.IsResolved() || done.IsActive() {
		t.Error("resolved incident must be resolved, not active")
	}
}

func TestIncident_ApplyStatusChange_resolveSetsTimestamp(t *testing.T) {
	at := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	inc := Incident{CurrentStatus: IncidentInvestigating}

	if err := inc.ApplyStatusChange(IncidentResolved, at); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inc.CurrentStatus != IncidentResolved {
		t.Fatalf("status = %q, want resolved", inc.CurrentStatus)
	}
	if inc.ResolvedAt == nil || !inc.ResolvedAt.Equal(at) {
		t.Fatalf("ResolvedAt = %v, want %v", inc.ResolvedAt, at)
	}
}

func TestIncident_ApplyStatusChange_resolveTwicePreservesFirstTimestamp(t *testing.T) {
	first := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)
	inc := Incident{CurrentStatus: IncidentInvestigating}

	_ = inc.ApplyStatusChange(IncidentResolved, first)
	// Повторный resolved (например, правка) не должен сдвигать момент устранения.
	_ = inc.ApplyStatusChange(IncidentResolved, second)
	if inc.ResolvedAt == nil || !inc.ResolvedAt.Equal(first) {
		t.Fatalf("ResolvedAt = %v, want first %v", inc.ResolvedAt, first)
	}
}

func TestIncident_ApplyStatusChange_reopenClearsTimestamp(t *testing.T) {
	at := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	inc := Incident{CurrentStatus: IncidentInvestigating}
	_ = inc.ApplyStatusChange(IncidentResolved, at)

	if err := inc.ApplyStatusChange(IncidentInvestigating, at.Add(time.Hour)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inc.ResolvedAt != nil {
		t.Fatalf("reopen must clear ResolvedAt, got %v", inc.ResolvedAt)
	}
	if inc.IsResolved() {
		t.Error("reopened incident must be active")
	}
}

func TestIncident_ApplyStatusChange_invalid(t *testing.T) {
	inc := Incident{CurrentStatus: IncidentInvestigating}
	err := inc.ApplyStatusChange(IncidentStatus("bogus"), time.Now())
	if !errors.Is(err, ErrInvalidIncidentStatus) {
		t.Fatalf("err = %v, want ErrInvalidIncidentStatus", err)
	}
	if inc.CurrentStatus != IncidentInvestigating {
		t.Error("invalid transition must not mutate status")
	}
}

func TestIncident_SetPostmortem(t *testing.T) {
	at := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)

	// До resolved постмортем запрещён.
	open := Incident{CurrentStatus: IncidentMonitoring}
	if err := open.SetPostmortem("root cause"); !errors.Is(err, ErrPostmortemBeforeResolved) {
		t.Fatalf("err = %v, want ErrPostmortemBeforeResolved", err)
	}
	if open.Postmortem != nil {
		t.Error("postmortem must stay nil before resolved")
	}

	// После resolved — можно.
	done := Incident{CurrentStatus: IncidentInvestigating}
	_ = done.ApplyStatusChange(IncidentResolved, at)
	if err := done.SetPostmortem("root cause"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done.Postmortem == nil || *done.Postmortem != "root cause" {
		t.Fatalf("postmortem = %v, want \"root cause\"", done.Postmortem)
	}

	// Пустая строка снимает постмортем.
	if err := done.SetPostmortem(""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done.Postmortem != nil {
		t.Error("empty postmortem must clear the field")
	}
}
