package domain

import (
	"testing"

	"github.com/google/uuid"
)

func node(id uuid.UUID, parent *uuid.UUID, status ComponentStatus, pos int, name string) Component {
	return Component{ID: id, ParentID: parent, CurrentStatus: status, Position: pos, Name: name, DisplayState: true}
}

func TestBuildComponentTree_nestingAndOrder(t *testing.T) {
	root1 := uuid.New()
	root2 := uuid.New()
	child := uuid.New()
	grandchild := uuid.New()

	components := []Component{
		node(root2, nil, StatusOperational, 2, "B-root"),
		node(root1, nil, StatusOperational, 1, "A-root"),
		node(grandchild, &child, StatusMajorOutage, 1, "grandchild"),
		node(child, &root1, StatusOperational, 1, "child"),
	}

	roots := BuildComponentTree(components)
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}
	// отсортировано по Position: root1 (pos 1) перед root2 (pos 2).
	if roots[0].ID != root1 || roots[1].ID != root2 {
		t.Fatalf("roots out of order: %v, %v", roots[0].Name, roots[1].Name)
	}
	if len(roots[0].Children) != 1 || roots[0].Children[0].ID != child {
		t.Fatalf("root1 should have child")
	}
	if len(roots[0].Children[0].Children) != 1 || roots[0].Children[0].Children[0].ID != grandchild {
		t.Fatalf("child should have grandchild")
	}
}

func TestEffectiveStatus_worstOfSubtree(t *testing.T) {
	root := uuid.New()
	child := uuid.New()
	components := []Component{
		node(root, nil, StatusOperational, 1, "root"),
		node(child, &root, StatusMajorOutage, 1, "child"),
	}
	roots := BuildComponentTree(components)
	if got := roots[0].EffectiveStatus(); got != StatusMajorOutage {
		t.Fatalf("EffectiveStatus = %q, want major_outage (from child)", got)
	}
}

func TestBuildComponentTree_missingParentBecomesRoot(t *testing.T) {
	orphan := uuid.New()
	missing := uuid.New()
	roots := BuildComponentTree([]Component{node(orphan, &missing, StatusOperational, 1, "orphan")})
	if len(roots) != 1 || roots[0].ID != orphan {
		t.Fatalf("orphan with missing parent should be a root")
	}
}

func TestComputeGroupStatus(t *testing.T) {
	g := uuid.New()
	other := uuid.New()
	withGroup := func(gid *uuid.UUID, status ComponentStatus, private bool) Component {
		return Component{GroupID: gid, CurrentStatus: status, DisplayState: true, IsPrivate: private}
	}
	components := []Component{
		withGroup(&g, StatusDegradedPerformance, false),
		withGroup(&g, StatusMajorOutage, true), // приватный — не учитывается
		withGroup(&other, StatusMajorOutage, false),
		withGroup(nil, StatusMajorOutage, false),
	}
	if got := ComputeGroupStatus(g, components); got != StatusDegradedPerformance {
		t.Fatalf("ComputeGroupStatus = %q, want degraded_performance", got)
	}
}
