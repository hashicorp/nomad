// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/allocrunner/state"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// fakeSnapshotAllocRunner builds on emptyAllocRunner but returns the raw
// allocation pointer and a settable destroyed/state pair, so tests of
// Client.Allocations() exercise its own copy and overlay logic rather than
// the fake's.
type fakeSnapshotAllocRunner struct {
	emptyAllocRunner
	rawAlloc  *structs.Allocation
	rawState  *state.State
	destroyed bool
}

func (ar *fakeSnapshotAllocRunner) Alloc() *structs.Allocation { return ar.rawAlloc }
func (ar *fakeSnapshotAllocRunner) AllocState() *state.State   { return ar.rawState }
func (ar *fakeSnapshotAllocRunner) IsDestroyed() bool          { return ar.destroyed }

func TestClient_Allocations(t *testing.T) {
	ci.Parallel(t)

	c, cleanup := TestClient(t, nil)
	defer cleanup()

	// Stale server-reported status; live state says running.
	allocOverlay := mock.Alloc()
	allocOverlay.ClientStatus = structs.AllocClientStatusPending

	// Live state has no status yet; the alloc's own status is kept.
	allocEmptyState := mock.Alloc()
	allocEmptyState.ClientStatus = structs.AllocClientStatusRunning

	// Destroyed alloc runners are excluded entirely.
	allocDestroyed := mock.Alloc()

	c.allocLock.Lock()
	c.allocs = map[string]interfaces.AllocRunner{
		allocOverlay.ID: &fakeSnapshotAllocRunner{
			rawAlloc: allocOverlay,
			rawState: &state.State{ClientStatus: structs.AllocClientStatusRunning},
		},
		allocEmptyState.ID: &fakeSnapshotAllocRunner{
			rawAlloc: allocEmptyState,
			rawState: &state.State{},
		},
		// Client background goroutines (e.g. emitStats) call AllocState()
		// on every alloc runner, destroyed or not, so the state must be non-nil.
		allocDestroyed.ID: &fakeSnapshotAllocRunner{
			rawAlloc:  allocDestroyed,
			rawState:  &state.State{ClientStatus: structs.AllocClientStatusComplete},
			destroyed: true,
		},
	}
	c.allocLock.Unlock()

	out := c.Allocations()
	require.Len(t, out, 2)

	byID := map[string]*structs.Allocation{}
	for _, alloc := range out {
		byID[alloc.ID] = alloc
	}

	// Live status overlays the stale server copy.
	require.Equal(t, structs.AllocClientStatusRunning, byID[allocOverlay.ID].ClientStatus)
	// An empty live status leaves the alloc's own status untouched.
	require.Equal(t, structs.AllocClientStatusRunning, byID[allocEmptyState.ID].ClientStatus)
	// Destroyed alloc runner is excluded.
	require.NotContains(t, byID, allocDestroyed.ID)

	// Returned allocations are copies: mutating them must not write
	// through to the alloc runner's allocation.
	byID[allocOverlay.ID].ClientStatus = "mutated"
	require.Equal(t, structs.AllocClientStatusPending, allocOverlay.ClientStatus)
}
