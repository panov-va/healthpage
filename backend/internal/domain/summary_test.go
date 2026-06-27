package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestBuildPublicSummary(t *testing.T) {
	g1 := uuid.New()
	g2 := uuid.New()
	groups := []ComponentGroup{
		{ID: g1, Name: "Core", Position: 0},
		{ID: g2, Name: "Aux", Position: 1},
	}
	comp := func(group *uuid.UUID, status ComponentStatus, private bool) Component {
		return Component{ID: uuid.New(), GroupID: group, CurrentStatus: status, IsPrivate: private, DisplayState: true}
	}
	components := []Component{
		comp(&g1, StatusDegradedPerformance, false),
		comp(&g1, StatusMajorOutage, true), // приватный — скрыт и не влияет
		comp(&g2, StatusOperational, false),
		comp(nil, StatusPartialOutage, false), // ungrouped, влияет на overall
	}

	s := BuildPublicSummary(groups, components)

	// overall: худший среди публичных (partial_outage > degraded). major приватный не считается.
	if s.OverallStatus != StatusPartialOutage {
		t.Fatalf("overall = %q, want partial_outage", s.OverallStatus)
	}
	if len(s.Groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(s.Groups))
	}
	// группа Core: публичный компонент только degraded (приватный major скрыт).
	if s.Groups[0].AggregatedStatus != StatusDegradedPerformance {
		t.Fatalf("group Core aggregated = %q, want degraded_performance", s.Groups[0].AggregatedStatus)
	}
	if len(s.Groups[0].Components) != 1 {
		t.Fatalf("group Core visible components = %d, want 1 (private excluded)", len(s.Groups[0].Components))
	}
	if s.Groups[1].AggregatedStatus != StatusOperational {
		t.Fatalf("group Aux aggregated = %q, want operational", s.Groups[1].AggregatedStatus)
	}
	if len(s.Ungrouped) != 1 {
		t.Fatalf("ungrouped = %d, want 1", len(s.Ungrouped))
	}
}
