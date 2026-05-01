// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
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

type fakeAlloc struct {
	ID             string
	NodeID         string
	ClientStatus   string
	DesiredStatus  string
	terminal       bool
	serverTerminal bool
	canary         bool
	shouldMigrate  bool
	expired        bool
	reconnect      bool
	disconnectNow  bool
	replaceOnDisc  bool
}

func (a fakeAlloc) NeedsToReconnect() bool { return a.reconnect }
func (a fakeAlloc) TerminalStatus() bool   { return a.terminal }
func (a fakeAlloc) ServerTerminalStatus() bool {
	return a.serverTerminal
}

func (a fakeAlloc) Expired(now time.Time) bool { return a.expired }
func (a fakeAlloc) DisconnectTimeout(now time.Time) time.Time {
	if a.disconnectNow {
		return now
	}
	return now.Add(time.Hour)
}

func (a fakeAlloc) ReplaceOnDisconnect() bool { return a.replaceOnDisc }

/*
func (a fakeAlloc) DesiredTransition() fakeTransition {
	return fakeTransition{a.shouldMigrate}
}
*/

func TestAllocSet_FilterByTainted(t *testing.T) {
	now := time.Now()

	nodes := map[string]*structs.Node{
		"draining": {
			ID:            "draining",
			DrainStrategy: mock.DrainNode().DrainStrategy,
		},
		"down": {
			ID:     "down",
			Status: structs.NodeStatusDown,
		},
		"nil": nil,
		"normal": {
			ID:     "normal",
			Status: structs.NodeStatusReady,
		},
		"disconnected": {
			ID:     "disconnected",
			Status: structs.NodeStatusDisconnected,
		},
	}

	type testCase struct {
		name     string
		alloc    *structs.Allocation
		expected string
	}

	tests := []testCase{
		{
			name: "failed reconnecting",
			alloc: &structs.Allocation{
				ID:            "a1",
				NodeID:        "normal",
				ClientStatus:  structs.AllocClientStatusFailed,
				DesiredStatus: structs.AllocDesiredStatusRun,
			},
			expected: "reconnecting",
		},
		/* {
			name: "terminal server -> ignore",
			alloc: &structs.Allocation{
				ID:           "a2",
				NodeID:       "normal",
				ClientStatus: structs.AllocClientStatusComplete,
			},
			expected: "ignore",
		},
		{
			name: "terminal canary migrate",
			alloc: &structs.Allocation{
				ID:     "a3",
				NodeID: "normal",
				DeploymentStatus: &structs.AllocDeploymentStatus{
					Canary: true,
				},
				DesiredTransition: structs.DesiredTransition{
					Migrate: pointer.Of(true),
				},
				ClientStatus: structs.AllocClientStatusComplete,
			},
			expected: "migrate",
		},
		{
			name: "expired -> expiring",
			alloc: &structs.Allocation{
				ID:     "a4",
				NodeID: "normal",
			},
			expected: "expiring",
		},
		{
			name: "disconnected pending -> lost",
			alloc: &structs.Allocation{
				ID:           "a5",
				NodeID:       "disconnected",
				ClientStatus: structs.AllocClientStatusPending,
			},
			expected: "lost",
		},
		{
			name: "disconnected -> disconnecting",
			alloc: &structs.Allocation{
				ID:           "a6",
				NodeID:       "disconnected",
				ClientStatus: structs.AllocClientStatusRunning,
			},
			expected: "disconnecting",
		},
		{
			name: "should migrate",
			alloc: &structs.Allocation{
				ID:     "a7",
				NodeID: "normal",
				DesiredTransition: structs.DesiredTransition{
					Migrate: pointer.Of(true),
				},
			},
			expected: "migrate",
		},
		{
			name: "untainted normal",
			alloc: &structs.Allocation{
				ID:     "a8",
				NodeID: "normal",
			},
			expected: "untainted",
		},
		{
			name: "nil node -> lost",
			alloc: &structs.Allocation{
				ID:     "a9",
				NodeID: "nil",
			},
			expected: "lost",
		},
		{
			name: "down node -> lost",
			alloc: &structs.Allocation{
				ID:     "a10",
				NodeID: "down",
			},
			expected: "lost",
		}, */
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := allocSet{
				tt.alloc.ID: tt.alloc,
			}

			state := ClusterState{
				Now:          now,
				TaintedNodes: nodes,
			}

			u, m, l, d, r, i, e := set.filterByTainted(state)

			buckets := map[string]allocSet{
				"untainted":     u,
				"migrate":       m,
				"lost":          l,
				"disconnecting": d,
				"reconnecting":  r,
				"ignore":        i,
				"expiring":      e,
			}

			for name, bucket := range buckets {
				if name == tt.expected {
					require.Contains(t, bucket, tt.alloc.ID)
				} else {
					require.NotContains(t, bucket, tt.alloc.ID)
				}
			}
		})
	}
}
