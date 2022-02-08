package scheduler

import (
	"testing"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// Test that we properly create the bitmap even when the alloc set includes an
// allocation with a higher count than the current min count and it is byte
// aligned.
// Ensure no regression from: https://github.com/hashicorp/nomad/issues/3008
func TestBitmapFrom(t *testing.T) {
	input := map[string]*structs.Allocation{
		"8": {
			JobID:     "foo",
			TaskGroup: "bar",
			Name:      "foo.bar[8]",
		},
	}
	b := bitmapFrom(input, 1)
	exp := uint(16)
	if act := b.Size(); act != exp {
		t.Fatalf("got %d; want %d", act, exp)
	}

	b = bitmapFrom(input, 8)
	if act := b.Size(); act != exp {
		t.Fatalf("got %d; want %d", act, exp)
	}
}

func TestAllocSet_filterByTainted(t *testing.T) {
	nodes := map[string]*structs.Node{
		"draining": {
			ID:            "draining",
			DrainStrategy: mock.DrainNode().DrainStrategy,
		},
		"lost": {
			ID:     "lost",
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

	batchJob := &structs.Job{
		Type: structs.JobTypeBatch,
	}

	allocs := allocSet{
		// Non-terminal alloc with migrate=true should migrate on a draining node
		"migrating1": {
			ID:                "migrating1",
			ClientStatus:      structs.AllocClientStatusRunning,
			DesiredTransition: structs.DesiredTransition{Migrate: helper.BoolToPtr(true)},
			Job:               batchJob,
			NodeID:            "draining",
		},
		// Non-terminal alloc with migrate=true should migrate on an unknown node
		"migrating2": {
			ID:                "migrating2",
			ClientStatus:      structs.AllocClientStatusRunning,
			DesiredTransition: structs.DesiredTransition{Migrate: helper.BoolToPtr(true)},
			Job:               batchJob,
			NodeID:            "nil",
		},
		"untainted1": {
			ID:           "untainted1",
			ClientStatus: structs.AllocClientStatusRunning,
			Job:          batchJob,
			NodeID:       "normal",
		},
		// Terminal allocs are always untainted
		"untainted2": {
			ID:           "untainted2",
			ClientStatus: structs.AllocClientStatusComplete,
			Job:          batchJob,
			NodeID:       "normal",
		},
		// Terminal allocs are always untainted, even on draining nodes
		"untainted3": {
			ID:           "untainted3",
			ClientStatus: structs.AllocClientStatusComplete,
			Job:          batchJob,
			NodeID:       "draining",
		},
		// Terminal allocs are always untainted, even on lost nodes
		"untainted4": {
			ID:           "untainted4",
			ClientStatus: structs.AllocClientStatusComplete,
			Job:          batchJob,
			NodeID:       "lost",
		},
		// Non-terminal allocs on lost nodes are lost
		"lost1": {
			ID:           "lost1",
			ClientStatus: structs.AllocClientStatusPending,
			Job:          batchJob,
			NodeID:       "lost",
		},
		// Non-terminal allocs on lost nodes are lost
		"lost2": {
			ID:           "lost2",
			ClientStatus: structs.AllocClientStatusRunning,
			Job:          batchJob,
			NodeID:       "lost",
		},
		// Non-terminal allocs on disconnected nodes are disconnecting
		"disconnecting1": {
			ID:           "disconnecting1",
			ClientStatus: structs.AllocClientStatusRunning,
			Job:          batchJob,
			NodeID:       "disconnected",
		},
		// Non-terminal allocs on disconnected nodes are disconnecting
		"disconnecting2": {
			ID:           "disconnecting2",
			ClientStatus: structs.AllocClientStatusRunning,
			Job:          batchJob,
			NodeID:       "disconnected",
		},
		// Non-terminal allocs on disconnected nodes are disconnecting
		"disconnecting3": {
			ID:           "disconnecting3",
			ClientStatus: structs.AllocClientStatusRunning,
			Job:          batchJob,
			NodeID:       "disconnected",
		},
		// Unknown allocs on re-connected nodes are reconnecting
		"reconnecting1": {
			ID:           "reconnecting1",
			ClientStatus: structs.AllocClientStatusUnknown,
			Job:          batchJob,
			NodeID:       "normal",
		},
		// Unknown allocs on re-connected nodes are reconnecting
		"reconnecting2": {
			ID:           "reconnecting2",
			ClientStatus: structs.AllocClientStatusUnknown,
			Job:          batchJob,
			NodeID:       "normal",
		},
	}

	untainted, migrate, lost, disconnecting, reconnecting := allocs.filterByTainted(nodes)
	require.Len(t, untainted, 4)
	require.Contains(t, untainted, "untainted1")
	require.Contains(t, untainted, "untainted2")
	require.Contains(t, untainted, "untainted3")
	require.Contains(t, untainted, "untainted4")
	require.Len(t, migrate, 2)
	require.Contains(t, migrate, "migrating1")
	require.Contains(t, migrate, "migrating2")
	require.Len(t, lost, 2)
	require.Contains(t, lost, "lost1")
	require.Contains(t, lost, "lost2")
	require.Len(t, disconnecting, 3)
	require.Contains(t, disconnecting, "disconnecting1")
	require.Contains(t, disconnecting, "disconnecting2")
	require.Contains(t, disconnecting, "disconnecting3")
	require.Len(t, reconnecting, 2)
	require.Contains(t, reconnecting, "reconnecting1")
	require.Contains(t, reconnecting, "reconnecting2")
}
