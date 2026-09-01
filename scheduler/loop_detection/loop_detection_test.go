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

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return New(hclog.NewNullLogger())
}

func requireEdge(t *testing.T, s *Store, from, to string) {
	t.Helper()

	_, ok := s.deps[from][to]
	must.True(t, ok)

	_, ok = s.dependents[to][from]
	must.True(t, ok)
}

func requireNoEdge(t *testing.T, s *Store, from, to string) {
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

func requireNode(t *testing.T, s *Store, nodeID string) {
	t.Helper()

	_, depsOK := s.deps[nodeID]
	_, dependentsOK := s.dependents[nodeID]

	must.True(t, depsOK)
	must.True(t, dependentsOK)
}

func requireNoNode(t *testing.T, s *Store, nodeID string) {
	t.Helper()

	_, depsOK := s.deps[nodeID]
	_, dependentsOK := s.dependents[nodeID]

	must.False(t, depsOK)
	must.False(t, dependentsOK)
}

// Verify every dependency edge has an equivalent reverse edge and vice versa.
func requireConsistentGraph(t *testing.T, s *Store) {
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
	s := newTestStore(t)

	must.NotNil(t, s)
	must.NotNil(t, s.deps)
	must.NotNil(t, s.dependents)
	must.Zero(t, len(s.deps))
	must.Zero(t, len(s.dependents))
}

func TestStore_AddNodes(t *testing.T) {
	t.Run("empty node ID", func(t *testing.T) {
		s := newTestStore(t)

		err := s.AddNodes("")

		must.ErrorIs(t, err, ErrEmptyNodeID)
		must.Zero(t, len(s.deps))
		must.Zero(t, len(s.dependents))
	})

	t.Run("node without dependencies", func(t *testing.T) {
		s := newTestStore(t)

		err := s.AddNodes("main")

		must.NoError(t, err)
		requireNode(t, s, "main")
		must.Zero(t, len(s.deps["main"]))
		must.Zero(t, len(s.dependents["main"]))
		requireConsistentGraph(t, s)
	})

	t.Run("empty dependency ID", func(t *testing.T) {
		s := newTestStore(t)

		err := s.AddNodes("main", "")

		must.ErrorIs(t, err, ErrEmptyNodeID)

		// The owner is created before dependencies are validated.
		requireNode(t, s, "main")
		must.Zero(t, len(s.deps["main"]))
		requireConsistentGraph(t, s)
	})

	t.Run("self dependency", func(t *testing.T) {
		s := newTestStore(t)

		err := s.AddNodes("main", "main")

		must.ErrorIs(t, err, ErrSelfDependency)
		requireNode(t, s, "main")
		must.Zero(t, len(s.deps["main"]))
		requireConsistentGraph(t, s)
	})

	t.Run("single dependency", func(t *testing.T) {
		s := newTestStore(t)

		err := s.AddNodes("main", "dep")

		must.NoError(t, err)

		requireNode(t, s, "main")
		requireNode(t, s, "dep")

		requireEdge(t, s, "main", "dep")

		require.Len(t, s.deps["main"], 1)
		must.Zero(t, len(s.deps["dep"]))

		must.Zero(t, len(s.dependents["main"]))
		must.One(t, len(s.dependents["dep"]))

		requireConsistentGraph(t, s)
	})

	t.Run("multiple dependencies", func(t *testing.T) {
		s := newTestStore(t)

		err := s.AddNodes(
			"main",
			"database",
			"migration",
			"setup",
		)

		must.NoError(t, err)

		requireEdge(t, s, "main", "database")
		requireEdge(t, s, "main", "migration")
		requireEdge(t, s, "main", "setup")

		require.Len(t, s.deps["main"], 3)

		requireConsistentGraph(t, s)
	})

	t.Run("duplicate dependency in same call", func(t *testing.T) {
		s := newTestStore(t)

		err := s.AddNodes(
			"main",
			"dep",
			"dep",
			"dep",
		)

		must.NoError(t, err)

		require.Len(t, s.deps["main"], 1)
		require.Len(t, s.dependents["dep"], 1)

		requireEdge(t, s, "main", "dep")
		requireConsistentGraph(t, s)
	})

	t.Run("existing edge is idempotent", func(t *testing.T) {
		s := newTestStore(t)

		must.NoError(t, s.AddNodes("main", "dep"))
		must.NoError(t, s.AddNodes("main", "dep"))

		require.Len(t, s.deps["main"], 1)
		require.Len(t, s.dependents["dep"], 1)

		requireEdge(t, s, "main", "dep")
		requireConsistentGraph(t, s)
	})

	t.Run("existing node can gain another dependency", func(t *testing.T) {
		s := newTestStore(t)

		must.NoError(t, s.AddNodes("main", "dep1"))
		must.NoError(t, s.AddNodes("main", "dep2"))

		requireEdge(t, s, "main", "dep1")
		requireEdge(t, s, "main", "dep2")

		require.Len(t, s.deps["main"], 2)
		requireConsistentGraph(t, s)
	})
}

func TestStore_AddNodes_CycleDetection(t *testing.T) {
	t.Run("two node cycle", func(t *testing.T) {
		s := newTestStore(t)

		must.NoError(t, s.AddNodes("main", "dep"))

		err := s.AddNodes("dep", "main")

		require.Error(t, err)
		require.Contains(
			t,
			err.Error(),
			"circular dependency detected: dep -> main would create a loop",
		)

		requireEdge(t, s, "main", "dep")
		requireNoEdge(t, s, "dep", "main")

		requireConsistentGraph(t, s)
	})

	t.Run("three node cycle", func(t *testing.T) {
		s := newTestStore(t)

		must.NoError(t, s.AddNodes("A", "B"))
		must.NoError(t, s.AddNodes("B", "C"))

		err := s.AddNodes("C", "A")

		require.Error(t, err)
		require.Contains(t, err.Error(), "circular dependency detected")

		requireEdge(t, s, "A", "B")
		requireEdge(t, s, "B", "C")
		requireNoEdge(t, s, "C", "A")

		requireConsistentGraph(t, s)
	})

	t.Run("long cycle", func(t *testing.T) {
		s := newTestStore(t)

		must.NoError(t, s.AddNodes("A", "B"))
		must.NoError(t, s.AddNodes("B", "C"))
		must.NoError(t, s.AddNodes("C", "D"))
		must.NoError(t, s.AddNodes("D", "E"))

		err := s.AddNodes("E", "A")

		require.Error(t, err)
		require.Contains(t, err.Error(), "circular dependency detected")

		requireNoEdge(t, s, "E", "A")
		requireConsistentGraph(t, s)
	})

	t.Run("cycle through branch", func(t *testing.T) {
		s := newTestStore(t)

		// A -> B -> D
		//  \
		//   -> C -> E
		must.NoError(t, s.AddNodes("A", "B", "C"))
		must.NoError(t, s.AddNodes("B", "D"))
		must.NoError(t, s.AddNodes("C", "E"))

		// E -> A would produce:
		//
		// A -> C -> E -> A
		err := s.AddNodes("E", "A")

		require.Error(t, err)
		require.Contains(t, err.Error(), "circular dependency detected")

		requireNoEdge(t, s, "E", "A")
		requireConsistentGraph(t, s)
	})

	t.Run("diamond is not a cycle", func(t *testing.T) {
		s := newTestStore(t)

		//      A
		//     / \
		//    B   C
		//     \ /
		//      D
		must.NoError(t, s.AddNodes("A", "B", "C"))
		must.NoError(t, s.AddNodes("B", "D"))
		must.NoError(t, s.AddNodes("C", "D"))

		requireEdge(t, s, "A", "B")
		requireEdge(t, s, "A", "C")
		requireEdge(t, s, "B", "D")
		requireEdge(t, s, "C", "D")

		requireConsistentGraph(t, s)
	})

	t.Run("shared dependency is not a cycle", func(t *testing.T) {
		s := newTestStore(t)

		must.NoError(t, s.AddNodes("A", "C"))
		must.NoError(t, s.AddNodes("B", "C"))

		requireEdge(t, s, "A", "C")
		requireEdge(t, s, "B", "C")

		require.Len(t, s.dependents["C"], 2)
		requireConsistentGraph(t, s)
	})
}

func TestStore_Reaches(t *testing.T) {
	t.Run("same node", func(t *testing.T) {
		s := newTestStore(t)

		must.True(t, s.reaches("A", "A"))
	})

	t.Run("direct dependency", func(t *testing.T) {
		s := newTestStore(t)

		must.NoError(t, s.AddNodes("A", "B"))

		must.True(t, s.reaches("A", "B"))
		must.False(t, s.reaches("B", "A"))
	})

	t.Run("indirect dependency", func(t *testing.T) {
		s := newTestStore(t)

		must.NoError(t, s.AddNodes("A", "B"))
		must.NoError(t, s.AddNodes("B", "C"))
		must.NoError(t, s.AddNodes("C", "D"))

		must.True(t, s.reaches("A", "D"))
		must.False(t, s.reaches("D", "A"))
	})

	t.Run("unreachable node", func(t *testing.T) {
		s := newTestStore(t)

		must.NoError(t, s.AddNodes("A", "B"))
		must.NoError(t, s.AddNodes("C", "D"))

		must.False(t, s.reaches("A", "D"))
	})

	t.Run("branch traversal", func(t *testing.T) {
		s := newTestStore(t)

		must.NoError(t, s.AddNodes("A", "B", "C"))
		must.NoError(t, s.AddNodes("B", "D"))
		must.NoError(t, s.AddNodes("C", "E"))

		must.True(t, s.reaches("A", "D"))
		must.True(t, s.reaches("A", "E"))

		must.False(t, s.reaches("D", "E"))
		must.False(t, s.reaches("E", "D"))
	})
}

func TestStore_RemoveNode(t *testing.T) {
	t.Run("empty node ID", func(t *testing.T) {
		s := newTestStore(t)

		err := s.RemoveNode("")

		must.ErrorIs(t, err, ErrEmptyNodeID)
	})

	t.Run("node not found", func(t *testing.T) {
		s := newTestStore(t)

		err := s.RemoveNode("does-not-exist")

		must.ErrorIs(t, err, ErrNodeNotFound)
	})

	t.Run("node with no dependencies or dependents", func(t *testing.T) {
		s := newTestStore(t)

		must.NoError(t, s.AddNodes("A"))

		err := s.RemoveNode("A")

		must.NoError(t, err)
		requireNoNode(t, s, "A")
		must.Zero(t, len(s.deps))
		must.Zero(t, len(s.dependents))
	})

	t.Run("cannot remove node another node depends on", func(t *testing.T) {
		s := newTestStore(t)

		must.NoError(t, s.AddNodes("A", "B"))

		err := s.RemoveNode("B")

		must.ErrorIs(t, err, ErrNodeIsDependency)

		requireNode(t, s, "A")
		requireNode(t, s, "B")
		requireEdge(t, s, "A", "B")

		requireConsistentGraph(t, s)
	})

	t.Run("remove root prunes single dependency", func(t *testing.T) {
		s := newTestStore(t)

		must.NoError(t, s.AddNodes("A", "B"))

		err := s.RemoveNode("A")

		must.NoError(t, err)

		requireNoNode(t, s, "A")
		requireNoNode(t, s, "B")

		must.Zero(t, len(s.deps))
		must.Zero(t, len(s.dependents))
	})

	t.Run("remove root recursively prunes dependency chain", func(t *testing.T) {
		s := newTestStore(t)

		// A -> B -> C -> D
		must.NoError(t, s.AddNodes("A", "B"))
		must.NoError(t, s.AddNodes("B", "C"))
		must.NoError(t, s.AddNodes("C", "D"))

		err := s.RemoveNode("A")

		must.NoError(t, err)

		requireNoNode(t, s, "A")
		requireNoNode(t, s, "B")
		requireNoNode(t, s, "C")
		requireNoNode(t, s, "D")

		must.Zero(t, len(s.deps))
		must.Zero(t, len(s.dependents))
	})

	t.Run("shared dependency is not pruned", func(t *testing.T) {
		s := newTestStore(t)

		// A -> C
		// B -> C
		must.NoError(t, s.AddNodes("A", "C"))
		must.NoError(t, s.AddNodes("B", "C"))

		err := s.RemoveNode("A")

		must.NoError(t, err)

		requireNoNode(t, s, "A")

		requireNode(t, s, "B")
		requireNode(t, s, "C")

		requireNoEdge(t, s, "A", "C")
		requireEdge(t, s, "B", "C")

		require.Len(t, s.dependents["C"], 1)

		requireConsistentGraph(t, s)
	})

	t.Run("shared dependency is pruned after last parent is removed", func(t *testing.T) {
		s := newTestStore(t)

		must.NoError(t, s.AddNodes("A", "C"))
		must.NoError(t, s.AddNodes("B", "C"))

		must.NoError(t, s.RemoveNode("A"))

		requireNode(t, s, "B")
		requireNode(t, s, "C")

		must.NoError(t, s.RemoveNode("B"))

		requireNoNode(t, s, "B")
		requireNoNode(t, s, "C")

		must.Zero(t, len(s.deps))
		must.Zero(t, len(s.dependents))
	})

	t.Run("pruning stops at shared descendant", func(t *testing.T) {
		s := newTestStore(t)

		// A -> B -> D
		// C ------> D
		must.NoError(t, s.AddNodes("A", "B"))
		must.NoError(t, s.AddNodes("B", "D"))
		must.NoError(t, s.AddNodes("C", "D"))

		must.NoError(t, s.RemoveNode("A"))

		requireNoNode(t, s, "A")
		requireNoNode(t, s, "B")

		requireNode(t, s, "C")
		requireNode(t, s, "D")

		requireEdge(t, s, "C", "D")
		requireConsistentGraph(t, s)
	})

	t.Run("remove branch recursively", func(t *testing.T) {
		s := newTestStore(t)

		//       A
		//      / \
		//     B   C
		//    /     \
		//   D       E
		must.NoError(t, s.AddNodes("A", "B", "C"))
		must.NoError(t, s.AddNodes("B", "D"))
		must.NoError(t, s.AddNodes("C", "E"))

		must.NoError(t, s.RemoveNode("A"))

		for _, node := range []string{"A", "B", "C", "D", "E"} {
			requireNoNode(t, s, node)
		}

		must.Zero(t, len(s.deps))
		must.Zero(t, len(s.dependents))
	})
}

func TestStore_PruneOrphan(t *testing.T) {
	t.Run("missing node", func(t *testing.T) {
		s := newTestStore(t)

		s.pruneOrphan("missing")

		must.Zero(t, len(s.deps))
		must.Zero(t, len(s.dependents))
	})

	t.Run("node with dependent is preserved", func(t *testing.T) {
		s := newTestStore(t)

		must.NoError(t, s.AddNodes("A", "B"))

		s.pruneOrphan("B")

		requireNode(t, s, "B")
		requireEdge(t, s, "A", "B")

		requireConsistentGraph(t, s)
	})

	t.Run("orphan is recursively removed", func(t *testing.T) {
		s := newTestStore(t)

		must.NoError(t, s.AddNodes("A", "B"))
		must.NoError(t, s.AddNodes("B", "C"))

		// Manually make A no longer reference B, leaving B orphaned.
		delete(s.deps["A"], "B")
		delete(s.dependents["B"], "A")

		s.pruneOrphan("B")

		requireNoNode(t, s, "B")
		requireNoNode(t, s, "C")

		requireNode(t, s, "A")
		must.Zero(t, len(s.deps["A"]))

		requireConsistentGraph(t, s)
	})
}

func TestStore_ErrorSentinels(t *testing.T) {
	must.True(t, errors.Is(ErrEmptyNodeID, ErrEmptyNodeID))
	must.True(t, errors.Is(ErrSelfDependency, ErrSelfDependency))
	must.True(t, errors.Is(ErrNodeNotFound, ErrNodeNotFound))
	must.True(t, errors.Is(ErrNodeIsDependency, ErrNodeIsDependency))
}

func TestStore_AddNodes_CycleErrorDoesNotPartiallyModifyGraph(t *testing.T) {
	s := newTestStore(t)

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
	must.StrContains(t, err.Error(), "circular dependency detected")

	requireNoEdge(t, s, "C", "D")
	requireNoEdge(t, s, "C", "A")

	requireEdge(t, s, "A", "C")
	requireConsistentGraph(t, s)
}
