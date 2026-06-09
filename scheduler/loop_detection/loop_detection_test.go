package depgraph

import (
	"errors"
	"testing"
)

// Test case diagram:
// jobA --> jobB
//   |
//   +----> jobC
//
// Expected:
// - Add succeeds
// - No cycle error
func TestAddNodesNoCycle(t *testing.T) {
	g := New()

	if err := g.AddNodes("jobA", "jobB", "jobC"); err != nil {
		t.Fatalf("unexpected error adding non-cyclic deps: %v", err)
	}
}

// Test case diagram:
// jobA --> jobB --> jobC
//   ^                |
//   +----------------+
//
// Operation:
// - add edge jobC -> jobA
//
// Expected:
// - cycle detected (error)
func TestAddNodesDetectsCycle(t *testing.T) {
	g := New()

	if err := g.AddNodes("jobA", "jobB"); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := g.AddNodes("jobB", "jobC"); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	err := g.AddNodes("jobC", "jobA")
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

// Test case diagram:
// jobA --> jobB
//
// Operation:
// - remove jobB
//
// Expected:
// - blocked, because jobA depends on jobB
// - returns ErrNodeIsDependency
func TestRemoveNodeBlockedIfDependedUpon(t *testing.T) {
	g := New()

	if err := g.AddNodes("jobA", "jobB"); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	err := g.RemoveNode("jobB")
	if !errors.Is(err, ErrNodeIsDependency) {
		t.Fatalf("expected ErrNodeIsDependency, got: %v", err)
	}
}

// Test case diagram:
// jobA --> jobB --> jobC
//
// Operation:
// - remove jobA
//
// Expected:
// - jobA removed
// - jobB and jobC pruned as orphans (no other dependents)
func TestRemoveNodePrunesOrphanChain(t *testing.T) {
	g := New()

	if err := g.AddNodes("jobA", "jobB"); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := g.AddNodes("jobB", "jobC"); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if err := g.RemoveNode("jobA"); err != nil {
		t.Fatalf("unexpected remove error: %v", err)
	}

	if err := g.RemoveNode("jobB"); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("expected ErrNodeNotFound for pruned jobB, got: %v", err)
	}
	if err := g.RemoveNode("jobC"); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("expected ErrNodeNotFound for pruned jobC, got: %v", err)
	}
}

// Test case diagram:
// jobX --> jobY
//   \\----> jobY (duplicate edge in same call)
//
// Expected:
// - add succeeds
// - duplicate dependency ignored (no error)
func TestAddNodesDuplicateDependenciesIgnored(t *testing.T) {
	g := New()

	if err := g.AddNodes("jobX", "jobY", "jobY"); err != nil {
		t.Fatalf("unexpected error for duplicate dependency: %v", err)
	}
}

// Test case diagram:
// jobZ --> jobZ (self loop)
//
// Expected:
// - returns ErrSelfDependency
func TestAddNodesSelfDependencyRejected(t *testing.T) {
	g := New()

	err := g.AddNodes("jobZ", "jobZ")
	if !errors.Is(err, ErrSelfDependency) {
		t.Fatalf("expected ErrSelfDependency, got: %v", err)
	}
}
