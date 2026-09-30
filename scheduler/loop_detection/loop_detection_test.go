// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package loop_detection

import (
	"errors"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func newTestDetector(t *testing.T) *loopDetector {
	t.Helper()
	return New(hclog.NewNullLogger())
}

func requireEdge(t *testing.T, s *loopDetector, from, to string) {
	t.Helper()

	_, ok := s.deps[from][to]
	must.True(t, ok)

	_, ok = s.dependents[to][from]
	must.True(t, ok)
}

func requireNoEdge(t *testing.T, s *loopDetector, from, to string) {
	t.Helper()

	if deps, ok := s.deps[from]; ok {
		_, exists := deps[to]
		must.False(t, exists)
	}

	if dependents, ok := s.dependents[to]; ok {
		_, exists := dependents[from]
		must.False(t, exists)
	}
}

func requireNode(t *testing.T, s *loopDetector, nodeID string) {
	t.Helper()

	_, depsOK := s.deps[nodeID]
	_, dependentsOK := s.dependents[nodeID]

	must.True(t, depsOK)
	must.True(t, dependentsOK)
}

func requireNoNode(t *testing.T, s *loopDetector, nodeID string) {
	t.Helper()

	_, depsOK := s.deps[nodeID]
	_, dependentsOK := s.dependents[nodeID]

	must.False(t, depsOK)
	must.False(t, dependentsOK)
}

// Verify every dependency edge has an equivalent reverse edge and vice versa.
func requireConsistentGraph(t *testing.T, s *loopDetector) {
	t.Helper()

	for node, deps := range s.deps {
		_, ok := s.dependents[node]
		must.True(t, ok)

		for dep := range deps {
			_, ok := s.deps[dep]
			must.True(t, ok)

			_, ok = s.dependents[dep][node]
			must.True(t, ok)
		}
	}

	for node, dependents := range s.dependents {
		_, ok := s.deps[node]
		must.True(t, ok)

		for dependent := range dependents {
			_, ok = s.deps[dependent][node]
			must.True(t, ok)
		}
	}
}

func TestNew(t *testing.T) {
	s := newTestDetector(t)

	must.NotNil(t, s)
	must.NotNil(t, s.deps)
	must.NotNil(t, s.dependents)
	must.Zero(t, len(s.deps))
	must.Zero(t, len(s.dependents))
}

func TestLoopDetector_AddNodes(t *testing.T) {
	tests := []struct {
		name    string
		ops     func(*loopDetector) error
		verify  func(*testing.T, *loopDetector, error)
	}{
		{
			name: "empty node ID",
			ops: func(s *loopDetector) error {
				return s.AddNodes("")
			},
			verify: func(t *testing.T, s *loopDetector, err error) {
				must.ErrorIs(t, err, ErrEmptyNodeID)
				must.Zero(t, len(s.deps))
				must.Zero(t, len(s.dependents))
			},
		},
		{
			name: "node without dependencies",
			ops: func(s *loopDetector) error {
				return s.AddNodes("main")
			},
			verify: func(t *testing.T, s *loopDetector, err error) {
				must.NoError(t, err)
				requireNode(t, s, "main")
				must.Zero(t, len(s.deps["main"]))
				must.Zero(t, len(s.dependents["main"]))
				requireConsistentGraph(t, s)
			},
		},
		{
			name: "empty dependency ID",
			ops: func(s *loopDetector) error {
				return s.AddNodes("main", "")
			},
			verify: func(t *testing.T, s *loopDetector, err error) {
				must.ErrorIs(t, err, ErrEmptyNodeID)
				requireNoNode(t, s, "main")
				must.Zero(t, len(s.deps))
				must.Zero(t, len(s.dependents))
			},
		},
		{
			name: "self dependency",
			ops: func(s *loopDetector) error {
				return s.AddNodes("main", "main")
			},
			verify: func(t *testing.T, s *loopDetector, err error) {
				must.ErrorIs(t, err, ErrSelfDependency)
				requireNoNode(t, s, "main")
				must.Zero(t, len(s.deps))
				must.Zero(t, len(s.dependents))
			},
		},
		{
			name: "single dependency",
			ops: func(s *loopDetector) error {
				return s.AddNodes("main", "dep")
			},
			verify: func(t *testing.T, s *loopDetector, err error) {
				must.NoError(t, err)

				requireNode(t, s, "main")
				requireNode(t, s, "dep")
				requireEdge(t, s, "main", "dep")

				require.Len(t, s.deps["main"], 1)
				must.Zero(t, len(s.deps["dep"]))

				must.Zero(t, len(s.dependents["main"]))
				must.One(t, len(s.dependents["dep"]))

				requireConsistentGraph(t, s)
			},
		},
		{
			name: "multiple dependencies",
			ops: func(s *loopDetector) error {
				return s.AddNodes("main", "database", "migration", "setup")
			},
			verify: func(t *testing.T, s *loopDetector, err error) {
				must.NoError(t, err)

				requireEdge(t, s, "main", "database")
				requireEdge(t, s, "main", "migration")
				requireEdge(t, s, "main", "setup")

				require.Len(t, s.deps["main"], 3)
				requireConsistentGraph(t, s)
			},
		},
		{
			name: "duplicate dependency in same call",
			ops: func(s *loopDetector) error {
				return s.AddNodes("main", "dep", "dep", "dep")
			},
			verify: func(t *testing.T, s *loopDetector, err error) {
				must.NoError(t, err)

				require.Len(t, s.deps["main"], 1)
				require.Len(t, s.dependents["dep"], 1)

				requireEdge(t, s, "main", "dep")
				requireConsistentGraph(t, s)
			},
		},
		{
			name: "existing edge is idempotent",
			ops: func(s *loopDetector) error {
				must.NoError(t, s.AddNodes("main", "dep"))
				return s.AddNodes("main", "dep")
			},
			verify: func(t *testing.T, s *loopDetector, err error) {
				must.NoError(t, err)

				require.Len(t, s.deps["main"], 1)
				require.Len(t, s.dependents["dep"], 1)

				requireEdge(t, s, "main", "dep")
				requireConsistentGraph(t, s)
			},
		},
		{
			name: "existing node can gain another dependency",
			ops: func(s *loopDetector) error {
				must.NoError(t, s.AddNodes("main", "dep1"))
				return s.AddNodes("main", "dep2")
			},
			verify: func(t *testing.T, s *loopDetector, err error) {
				must.NoError(t, err)

				requireEdge(t, s, "main", "dep1")
				requireEdge(t, s, "main", "dep2")

				require.Len(t, s.deps["main"], 2)
				requireConsistentGraph(t, s)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newTestDetector(t)
			err := tc.ops(s)
			tc.verify(t, s, err)
		})
	}
}

func TestLoopDetector_AddNodes_CycleDetection(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*loopDetector)
		add    func(*loopDetector) error
		verify func(*testing.T, *loopDetector, error)
	}{
		{
			name: "two node cycle",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("main", "dep"))
			},
			add: func(s *loopDetector) error {
				return s.AddNodes("dep", "main")
			},
			verify: func(t *testing.T, s *loopDetector, err error) {
				require.Error(t, err)
				must.ErrorIs(t, err, ErrCircularDependency)
				require.Contains(t, err.Error(), "circular dependency detected: dep -> main would create a loop")
				requireEdge(t, s, "main", "dep")
				requireNoEdge(t, s, "dep", "main")
				requireConsistentGraph(t, s)
			},
		},
		{
			name: "three node cycle",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "B"))
				must.NoError(t, s.AddNodes("B", "C"))
			},
			add: func(s *loopDetector) error {
				return s.AddNodes("C", "A")
			},
			verify: func(t *testing.T, s *loopDetector, err error) {
				require.Error(t, err)
				must.ErrorIs(t, err, ErrCircularDependency)
				require.Contains(t, err.Error(), "circular dependency detected")
				requireEdge(t, s, "A", "B")
				requireEdge(t, s, "B", "C")
				requireNoEdge(t, s, "C", "A")
				requireConsistentGraph(t, s)
			},
		},
		{
			name: "long cycle",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "B"))
				must.NoError(t, s.AddNodes("B", "C"))
				must.NoError(t, s.AddNodes("C", "D"))
				must.NoError(t, s.AddNodes("D", "E"))
			},
			add: func(s *loopDetector) error {
				return s.AddNodes("E", "A")
			},
			verify: func(t *testing.T, s *loopDetector, err error) {
				require.Error(t, err)
				must.ErrorIs(t, err, ErrCircularDependency)
				require.Contains(t, err.Error(), "circular dependency detected")
				requireNoEdge(t, s, "E", "A")
				requireConsistentGraph(t, s)
			},
		},
		{
			name: "cycle through branch",
			setup: func(s *loopDetector) {
				// A -> B -> D
				//  \
				//   -> C -> E
				must.NoError(t, s.AddNodes("A", "B", "C"))
				must.NoError(t, s.AddNodes("B", "D"))
				must.NoError(t, s.AddNodes("C", "E"))
			},
			add: func(s *loopDetector) error {
				return s.AddNodes("E", "A")
			},
			verify: func(t *testing.T, s *loopDetector, err error) {
				require.Error(t, err)
				must.ErrorIs(t, err, ErrCircularDependency)
				require.Contains(t, err.Error(), "circular dependency detected")
				requireNoEdge(t, s, "E", "A")
				requireConsistentGraph(t, s)
			},
		},
		{
			name: "diamond is not a cycle",
			setup: func(s *loopDetector) {
				//      A
				//     / \
				//    B   C
				//     \ /
				//      D
				must.NoError(t, s.AddNodes("A", "B", "C"))
				must.NoError(t, s.AddNodes("B", "D"))
				must.NoError(t, s.AddNodes("C", "D"))
			},
			add: func(s *loopDetector) error {
				return nil
			},
			verify: func(t *testing.T, s *loopDetector, err error) {
				must.NoError(t, err)
				requireEdge(t, s, "A", "B")
				requireEdge(t, s, "A", "C")
				requireEdge(t, s, "B", "D")
				requireEdge(t, s, "C", "D")
				requireConsistentGraph(t, s)
			},
		},
		{
			name: "shared dependency is not a cycle",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "C"))
				must.NoError(t, s.AddNodes("B", "C"))
			},
			add: func(s *loopDetector) error {
				return nil
			},
			verify: func(t *testing.T, s *loopDetector, err error) {
				must.NoError(t, err)
				requireEdge(t, s, "A", "C")
				requireEdge(t, s, "B", "C")
				require.Len(t, s.dependents["C"], 2)
				requireConsistentGraph(t, s)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newTestDetector(t)
			if tc.setup != nil {
				tc.setup(s)
			}
			err := tc.add(s)
			tc.verify(t, s, err)
		})
	}
}

func TestLoopDetector_Reaches(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*loopDetector)
		check func(*testing.T, *loopDetector)
	}{
		{
			name: "same node",
			check: func(t *testing.T, s *loopDetector) {
				must.True(t, s.reaches("A", "A"))
			},
		},
		{
			name: "direct dependency",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "B"))
			},
			check: func(t *testing.T, s *loopDetector) {
				must.True(t, s.reaches("A", "B"))
				must.False(t, s.reaches("B", "A"))
			},
		},
		{
			name: "indirect dependency",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "B"))
				must.NoError(t, s.AddNodes("B", "C"))
				must.NoError(t, s.AddNodes("C", "D"))
			},
			check: func(t *testing.T, s *loopDetector) {
				must.True(t, s.reaches("A", "D"))
				must.False(t, s.reaches("D", "A"))
			},
		},
		{
			name: "unreachable node",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "B"))
				must.NoError(t, s.AddNodes("C", "D"))
			},
			check: func(t *testing.T, s *loopDetector) {
				must.False(t, s.reaches("A", "D"))
			},
		},
		{
			name: "branch traversal",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "B", "C"))
				must.NoError(t, s.AddNodes("B", "D"))
				must.NoError(t, s.AddNodes("C", "E"))
			},
			check: func(t *testing.T, s *loopDetector) {
				must.True(t, s.reaches("A", "D"))
				must.True(t, s.reaches("A", "E"))
				must.False(t, s.reaches("D", "E"))
				must.False(t, s.reaches("E", "D"))
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newTestDetector(t)
			if tc.setup != nil {
				tc.setup(s)
			}
			tc.check(t, s)
		})
	}
}

func TestLoopDetector_RemoveNode(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*loopDetector)
		id    string
		check func(*testing.T, *loopDetector, error)
	}{
		{
			name: "empty node ID",
			id:   "",
			check: func(t *testing.T, s *loopDetector, err error) {
				must.ErrorIs(t, err, ErrEmptyNodeID)
			},
		},
		{
			name: "node not found",
			id:   "does-not-exist",
			check: func(t *testing.T, s *loopDetector, err error) {
				must.ErrorIs(t, err, ErrNodeNotFound)
			},
		},
		{
			name: "node with no dependencies or dependents",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A"))
			},
			id: "A",
			check: func(t *testing.T, s *loopDetector, err error) {
				must.NoError(t, err)
				requireNoNode(t, s, "A")
				must.Zero(t, len(s.deps))
				must.Zero(t, len(s.dependents))
			},
		},
		{
			name: "cannot remove node another node depends on",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "B"))
			},
			id: "B",
			check: func(t *testing.T, s *loopDetector, err error) {
				must.ErrorIs(t, err, ErrNodeIsDependency)
				requireNode(t, s, "A")
				requireNode(t, s, "B")
				requireEdge(t, s, "A", "B")
				requireConsistentGraph(t, s)
			},
		},
		{
			name: "remove root prunes single dependency",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "B"))
			},
			id: "A",
			check: func(t *testing.T, s *loopDetector, err error) {
				must.NoError(t, err)
				requireNoNode(t, s, "A")
				requireNoNode(t, s, "B")
				must.Zero(t, len(s.deps))
				must.Zero(t, len(s.dependents))
			},
		},
		{
			name: "remove root recursively prunes dependency chain",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "B"))
				must.NoError(t, s.AddNodes("B", "C"))
				must.NoError(t, s.AddNodes("C", "D"))
			},
			id: "A",
			check: func(t *testing.T, s *loopDetector, err error) {
				must.NoError(t, err)
				requireNoNode(t, s, "A")
				requireNoNode(t, s, "B")
				requireNoNode(t, s, "C")
				requireNoNode(t, s, "D")
				must.Zero(t, len(s.deps))
				must.Zero(t, len(s.dependents))
			},
		},
		{
			name: "shared dependency is not pruned",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "C"))
				must.NoError(t, s.AddNodes("B", "C"))
			},
			id: "A",
			check: func(t *testing.T, s *loopDetector, err error) {
				must.NoError(t, err)
				requireNoNode(t, s, "A")
				requireNode(t, s, "B")
				requireNode(t, s, "C")
				requireNoEdge(t, s, "A", "C")
				requireEdge(t, s, "B", "C")
				require.Len(t, s.dependents["C"], 1)
				requireConsistentGraph(t, s)
			},
		},
		{
			name: "shared dependency is pruned after last parent is removed",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "C"))
				must.NoError(t, s.AddNodes("B", "C"))
			},
			id: "A",
			check: func(t *testing.T, s *loopDetector, err error) {
				must.NoError(t, err)
				requireNode(t, s, "B")
				requireNode(t, s, "C")
				must.NoError(t, s.RemoveNode("B"))
				requireNoNode(t, s, "B")
				requireNoNode(t, s, "C")
				must.Zero(t, len(s.deps))
				must.Zero(t, len(s.dependents))
			},
		},
		{
			name: "pruning stops at shared descendant",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "B"))
				must.NoError(t, s.AddNodes("B", "D"))
				must.NoError(t, s.AddNodes("C", "D"))
			},
			id: "A",
			check: func(t *testing.T, s *loopDetector, err error) {
				must.NoError(t, err)
				requireNoNode(t, s, "A")
				requireNoNode(t, s, "B")
				requireNode(t, s, "C")
				requireNode(t, s, "D")
				requireEdge(t, s, "C", "D")
				requireConsistentGraph(t, s)
			},
		},
		{
			name: "remove branch recursively",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "B", "C"))
				must.NoError(t, s.AddNodes("B", "D"))
				must.NoError(t, s.AddNodes("C", "E"))
			},
			id: "A",
			check: func(t *testing.T, s *loopDetector, err error) {
				must.NoError(t, err)
				for _, node := range []string{"A", "B", "C", "D", "E"} {
					requireNoNode(t, s, node)
				}
				must.Zero(t, len(s.deps))
				must.Zero(t, len(s.dependents))
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newTestDetector(t)
			if tc.setup != nil {
				tc.setup(s)
			}
			err := s.RemoveNode(tc.id)
			tc.check(t, s, err)
		})
	}
}

func TestLoopDetector_PruneOrphan(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*loopDetector)
		id    string
		check func(*testing.T, *loopDetector)
	}{
		{
			name: "missing node",
			id:   "missing",
			check: func(t *testing.T, s *loopDetector) {
				must.Zero(t, len(s.deps))
				must.Zero(t, len(s.dependents))
			},
		},
		{
			name: "node with dependent is preserved",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "B"))
			},
			id: "B",
			check: func(t *testing.T, s *loopDetector) {
				requireNode(t, s, "B")
				requireEdge(t, s, "A", "B")
				requireConsistentGraph(t, s)
			},
		},
		{
			name: "orphan is recursively removed",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "B"))
				must.NoError(t, s.AddNodes("B", "C"))

				// Manually make A no longer reference B, leaving B orphaned.
				delete(s.deps["A"], "B")
				delete(s.dependents["B"], "A")
			},
			id: "B",
			check: func(t *testing.T, s *loopDetector) {
				requireNoNode(t, s, "B")
				requireNoNode(t, s, "C")
				requireNode(t, s, "A")
				must.Zero(t, len(s.deps["A"]))
				requireConsistentGraph(t, s)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newTestDetector(t)
			if tc.setup != nil {
				tc.setup(s)
			}
			s.pruneOrphan(tc.id)
			tc.check(t, s)
		})
	}
}

func TestLoopDetector_ErrorSentinels(t *testing.T) {
	tests := []error{
		ErrEmptyNodeID,
		ErrSelfDependency,
		ErrNodeNotFound,
		ErrNodeIsDependency,
		ErrCircularDependency,
	}

	for _, err := range tests {
		must.True(t, errors.Is(err, err))
	}
}

func TestLoopDetector_AddNodes_CycleErrorDoesNotPartiallyModifyGraph(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "cycle error does not partially modify graph"},
	}

	for range tests {
		s := newTestDetector(t)

		// Existing graph:
		//
		// A -> C
		must.NoError(t, s.AddNodes("A", "C"))

		// Proposed:
		//
		// C -> D   valid
		// C -> A   invalid because A -> C already exists
		//
		// The overall AddNodes call should fail without installing C -> D.
		err := s.AddNodes("C", "D", "A")

		must.Error(t, err)
		must.ErrorIs(t, err, ErrCircularDependency)
		must.StrContains(t, err.Error(), "circular dependency detected")

		requireNoEdge(t, s, "C", "D")
		requireNoEdge(t, s, "C", "A")
		requireEdge(t, s, "A", "C")
		requireConsistentGraph(t, s)
	}
}

func TestLoopDetector_CreatesCircularDependency(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*loopDetector)
		dep   string
		nodes []string
		want  bool
	}{
		{
			name: "empty dependency is ignored",
			dep:  "",
			nodes: []string{"A"},
			want: false,
		},
		{
			name: "direct cycle",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "B"))
			},
			dep:  "B",
			nodes: []string{"A"},
			want: true,
		},
		{
			name: "indirect cycle",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "B"))
				must.NoError(t, s.AddNodes("B", "C"))
			},
			dep:  "C",
			nodes: []string{"A"},
			want: true,
		},
		{
			name: "multiple candidates",
			setup: func(s *loopDetector) {
				must.NoError(t, s.AddNodes("A", "B"))
				must.NoError(t, s.AddNodes("B", "C"))
			},
			dep:  "C",
			nodes: []string{"X", "B"},
			want: true,
		},
		{
			name: "no cycle among candidates",
			dep:  "X",
			nodes: []string{"Y"},
			want: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newTestDetector(t)
			if tc.setup != nil {
				tc.setup(s)
			}
			require.Equal(t, tc.want, s.CreatesCircularDependency(tc.dep, tc.nodes...))
		})
	}
}
