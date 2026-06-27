package domain

import "testing"

func TestWorstStatus_severityOrder(t *testing.T) {
	cases := []struct {
		name     string
		statuses []ComponentStatus
		want     ComponentStatus
	}{
		{"empty -> operational", nil, StatusOperational},
		{"all operational", []ComponentStatus{StatusOperational, StatusOperational}, StatusOperational},
		{"degraded beats operational",
			[]ComponentStatus{StatusOperational, StatusDegradedPerformance}, StatusDegradedPerformance},
		// нормативно (DESIGN §6): under_maintenance показывается ВЫШЕ деградации.
		{"maintenance beats degraded",
			[]ComponentStatus{StatusDegradedPerformance, StatusUnderMaintenance}, StatusUnderMaintenance},
		// ...но НИЖЕ реальных сбоев: partial/major перекрывают maintenance.
		{"partial beats maintenance",
			[]ComponentStatus{StatusUnderMaintenance, StatusPartialOutage}, StatusPartialOutage},
		{"major beats partial",
			[]ComponentStatus{StatusPartialOutage, StatusMajorOutage}, StatusMajorOutage},
		{"major beats maintenance",
			[]ComponentStatus{StatusUnderMaintenance, StatusMajorOutage}, StatusMajorOutage},
		{"unknown ignored",
			[]ComponentStatus{ComponentStatus("bogus"), StatusDegradedPerformance}, StatusDegradedPerformance},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := WorstStatus(tc.statuses...); got != tc.want {
				t.Fatalf("WorstStatus(%v) = %q, want %q", tc.statuses, got, tc.want)
			}
		})
	}
}

func TestComponentStatus_IsValid(t *testing.T) {
	for _, s := range AllComponentStatuses {
		if !s.IsValid() {
			t.Errorf("%q should be valid", s)
		}
	}
	if ComponentStatus("nope").IsValid() {
		t.Error("invalid status reported valid")
	}
}

func comp(status ComponentStatus, private, displayState bool) Component {
	return Component{CurrentStatus: status, IsPrivate: private, DisplayState: displayState}
}

func TestComputeOverallStatus(t *testing.T) {
	cases := []struct {
		name       string
		components []Component
		want       ComponentStatus
	}{
		{"no components -> operational", nil, StatusOperational},
		{"private excluded",
			[]Component{comp(StatusOperational, false, true), comp(StatusMajorOutage, true, true)},
			StatusOperational},
		{"hidden state excluded",
			[]Component{comp(StatusOperational, false, true), comp(StatusMajorOutage, false, false)},
			StatusOperational},
		{"maintenance over degraded (public)",
			[]Component{comp(StatusDegradedPerformance, false, true), comp(StatusUnderMaintenance, false, true)},
			StatusUnderMaintenance},
		{"outage over maintenance",
			[]Component{comp(StatusUnderMaintenance, false, true), comp(StatusPartialOutage, false, true)},
			StatusPartialOutage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ComputeOverallStatus(tc.components); got != tc.want {
				t.Fatalf("ComputeOverallStatus = %q, want %q", got, tc.want)
			}
		})
	}
}
