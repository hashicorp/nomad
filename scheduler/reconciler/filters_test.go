// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestReconciler_filterServerTerminalAllocs(t *testing.T) {
	makeSet := func() allocSet {
		set := make(allocSet)
		for _ = range 5 {
			alloc := mock.Alloc()
			set[alloc.ID] = alloc
		}

		return set
	}

	t.Run("none", func(t *testing.T) {
		set := makeSet()
		filtered := set.filterServerTerminalAllocs()
		must.Eq(t, filtered, set)
	})

	t.Run("with stop", func(t *testing.T) {
		set := makeSet()
		alloc := mock.Alloc()
		alloc.DesiredStatus = structs.AllocDesiredStatusStop
		set[alloc.ID] = alloc

		filtered := set.filterServerTerminalAllocs()
		must.NotEq(t, filtered, set)
		must.MapLen(t, 5, filtered)
	})

	t.Run("with evict", func(t *testing.T) {
		set := makeSet()
		alloc := mock.Alloc()
		alloc.DesiredStatus = structs.AllocDesiredStatusEvict
		set[alloc.ID] = alloc

		filtered := set.filterServerTerminalAllocs()
		must.NotEq(t, filtered, set)
		must.MapLen(t, 5, filtered)
	})

	t.Run("with stop batch", func(t *testing.T) {
		set := makeSet()
		alloc := mock.BatchAlloc()
		alloc.DesiredStatus = structs.AllocDesiredStatusStop
		set[alloc.ID] = alloc

		filtered := set.filterServerTerminalAllocs()
		must.Eq(t, filtered, set)
		must.MapLen(t, 6, filtered)
	})

	t.Run("with evict batch", func(t *testing.T) {
		set := makeSet()
		alloc := mock.BatchAlloc()
		alloc.DesiredStatus = structs.AllocDesiredStatusEvict
		set[alloc.ID] = alloc

		filtered := set.filterServerTerminalAllocs()
		must.Eq(t, filtered, set)
		must.MapLen(t, 6, filtered)
	})
}

func TestAllocSet_classifyAllocs_ClassificationRules(t *testing.T) {
	now := time.Now()

	nodes := map[string]*structs.Node{
		"ready": {
			ID:     "ready",
			Status: structs.NodeStatusReady,
		},
		"disconnected": {
			ID:     "disconnected",
			Status: structs.NodeStatusDisconnected,
		},
		"down": {
			ID:     "down",
			Status: structs.NodeStatusDown,
		},
		"initializing": {
			ID:     "initializing",
			Status: structs.NodeStatusInit,
		},
		"gc": nil,
	}

	setDisconnect := func(alloc *structs.Allocation, lostAfter time.Duration, replace bool) {
		alloc.Job.TaskGroups[0].Disconnect = &structs.DisconnectStrategy{
			LostAfter: lostAfter,
			Replace:   &replace,
		}
	}

	makeAlloc := func(id, nodeID, clientStatus, desiredStatus string) *structs.Allocation {
		alloc := mock.Alloc()
		alloc.ID = id
		alloc.NodeID = nodeID
		alloc.ClientStatus = clientStatus
		alloc.DesiredStatus = desiredStatus
		alloc.AllocStates = nil
		alloc.DeploymentStatus = nil
		alloc.DesiredTransition = structs.DesiredTransition{}
		setDisconnect(alloc, 5*time.Minute, true)
		return alloc
	}

	unknownState := []*structs.AllocState{{
		Field: structs.AllocStateFieldClientStatus,
		Value: structs.AllocClientStatusUnknown,
		Time:  now.Add(-10 * time.Second),
	}}

	testCases := []struct {
		name     string
		alloc    *structs.Allocation
		expected string
	}{
		{
			name: "failed reconnect run",
			alloc: func() *structs.Allocation {
				a := makeAlloc("c1", "ready", structs.AllocClientStatusFailed, structs.AllocDesiredStatusRun)
				a.AllocStates = unknownState
				return a
			}(),
			expected: "reconnecting",
		},
		{
			name:     "terminal server status ignored",
			alloc:    makeAlloc("c2", "ready", structs.AllocClientStatusRunning, structs.AllocDesiredStatusStop),
			expected: "ignore",
		},
		{
			name: "terminal canary migrate",
			alloc: func() *structs.Allocation {
				a := makeAlloc("c3", "ready", structs.AllocClientStatusComplete, structs.AllocDesiredStatusRun)
				a.DeploymentStatus = &structs.AllocDeploymentStatus{Canary: true}
				a.DesiredTransition = structs.DesiredTransition{Migrate: pointer.Of(true)}
				return a
			}(),
			expected: "migrate",
		},
		{
			name:     "terminal untainted",
			alloc:    makeAlloc("c4", "ready", structs.AllocClientStatusComplete, structs.AllocDesiredStatusRun),
			expected: "untainted",
		},
		{
			name: "expired alloc",
			alloc: func() *structs.Allocation {
				a := makeAlloc("c5", "ready", structs.AllocClientStatusUnknown, structs.AllocDesiredStatusRun)
				setDisconnect(a, 1*time.Second, true)
				a.AllocStates = unknownState
				return a
			}(),
			expected: "expiring",
		},
		{
			name: "failed reconnect stop ignored",
			alloc: func() *structs.Allocation {
				a := makeAlloc("c6", "ready", structs.AllocClientStatusFailed, structs.AllocDesiredStatusStop)
				a.AllocStates = unknownState
				return a
			}(),
			expected: "ignore",
		},
		{
			name:     "disconnected unknown becomes untainted",
			alloc:    makeAlloc("c7", "disconnected", structs.AllocClientStatusUnknown, structs.AllocDesiredStatusRun),
			expected: "untainted",
		},
		{
			name:     "disconnected pending lost",
			alloc:    makeAlloc("c8", "disconnected", structs.AllocClientStatusPending, structs.AllocDesiredStatusRun),
			expected: "lost",
		},
		{
			name: "disconnected zero timeout lost",
			alloc: func() *structs.Allocation {
				a := makeAlloc("c9", "disconnected", structs.AllocClientStatusRunning, structs.AllocDesiredStatusRun)
				a.Job = nil
				return a
			}(),
			expected: "lost",
		},
		{
			name:     "disconnected grace period",
			alloc:    makeAlloc("c10", "disconnected", structs.AllocClientStatusRunning, structs.AllocDesiredStatusRun),
			expected: "disconnecting",
		},
		{
			name: "migrate flag",
			alloc: func() *structs.Allocation {
				a := makeAlloc("c11", "ready", structs.AllocClientStatusPending, structs.AllocDesiredStatusRun)
				a.DesiredTransition = structs.DesiredTransition{Migrate: pointer.Of(true)}
				return a
			}(),
			expected: "migrate",
		},
		{
			name: "untainted reconnecting via ready node",
			alloc: func() *structs.Allocation {
				a := makeAlloc("c12", "ready", structs.AllocClientStatusRunning, structs.AllocDesiredStatusRun)
				a.AllocStates = unknownState
				return a
			}(),
			expected: "reconnecting",
		},
		{
			name:     "untainted on non tainted node",
			alloc:    makeAlloc("c13", "missing", structs.AllocClientStatusRunning, structs.AllocDesiredStatusRun),
			expected: "untainted",
		},
		{
			name:     "gc node lost",
			alloc:    makeAlloc("c14", "gc", structs.AllocClientStatusRunning, structs.AllocDesiredStatusRun),
			expected: "lost",
		},
		{
			name: "terminal node unknown no replace",
			alloc: func() *structs.Allocation {
				a := makeAlloc("c15", "down", structs.AllocClientStatusUnknown, structs.AllocDesiredStatusRun)
				setDisconnect(a, 5*time.Minute, false)
				return a
			}(),
			expected: "untainted",
		},
		{
			name: "terminal node running no replace",
			alloc: func() *structs.Allocation {
				a := makeAlloc("c16", "down", structs.AllocClientStatusRunning, structs.AllocDesiredStatusRun)
				setDisconnect(a, 5*time.Minute, false)
				return a
			}(),
			expected: "disconnecting",
		},
		{
			name:     "terminal node default lost",
			alloc:    makeAlloc("c17", "down", structs.AllocClientStatusPending, structs.AllocDesiredStatusRun),
			expected: "lost",
		},
		{
			name:     "other tainted node defaults to untainted",
			alloc:    makeAlloc("c18", "initializing", structs.AllocClientStatusPending, structs.AllocDesiredStatusRun),
			expected: "untainted",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			all := allocSet{tc.alloc.ID: tc.alloc}
			state := ClusterState{Now: now, TaintedNodes: nodes}

			untainted, migrate, lost, disconnecting, reconnecting, ignore, expiring := all.classifyAllocs(state)

			buckets := map[string]allocSet{
				"untainted":     untainted,
				"migrate":       migrate,
				"lost":          lost,
				"disconnecting": disconnecting,
				"reconnecting":  reconnecting,
				"ignore":        ignore,
				"expiring":      expiring,
			}

			for category, bucket := range buckets {
				if category == tc.expected {
					must.MapContainsKey(t, bucket, tc.alloc.ID)
					must.MapLen(t, 1, bucket)
					continue
				}

				must.MapNotContainsKey(t, bucket, tc.alloc.ID)
				must.MapLen(t, 0, bucket)
			}
		})
	}
}
