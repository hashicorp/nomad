// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package drainer

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testDrainingNode creates a *drainingNode with a 1h deadline but no allocs
func testDrainingNode(t *testing.T) *drainingNode {
	t.Helper()
	state := state.TestStateStore(t)
	node := mock.Node()
	node.DrainStrategy = &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: time.Hour,
		},
		ForceDeadline: time.Now().Add(time.Hour),
	}

	require.Nil(t, state.UpsertNode(structs.MsgTypeTestSetup, 100, node))
	return NewDrainingNode(node, state)
}

func assertDrainingNode(t *testing.T, dn *drainingNode, isDone bool, remaining, running int) {
	t.Helper()

	done, err := dn.IsDone()
	require.Nil(t, err)
	assert.Equal(t, isDone, done, "IsDone mismatch")

	allocs, err := dn.RemainingAllocs()
	require.Nil(t, err)
	assert.Len(t, allocs, remaining, "RemainingAllocs mismatch")

	jobs, err := dn.DrainingJobs()
	require.Nil(t, err)
	assert.Len(t, jobs, running, "DrainingJobs mismatch")
}

func TestDrainingNode_Table(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name      string
		isDone    bool
		remaining int
		running   int
		setup     func(*testing.T, *drainingNode)
	}{
		{
			name:      "Empty",
			isDone:    true,
			remaining: 0,
			running:   0,
			setup:     func(*testing.T, *drainingNode) {},
		},
		{
			name:      "Batch",
			isDone:    false,
			remaining: 1,
			running:   1,
			setup: func(t *testing.T, dn *drainingNode) {
				alloc := mock.BatchAlloc()
				alloc.NodeID = dn.node.ID
				require.Nil(t, dn.state.UpsertJob(structs.MsgTypeTestSetup, 101, nil, alloc.Job))
				require.Nil(t, dn.state.UpsertAllocs(structs.MsgTypeTestSetup, 102, []*structs.Allocation{alloc}))
			},
		},
		{
			name:      "Service",
			isDone:    false,
			remaining: 1,
			running:   1,
			setup: func(t *testing.T, dn *drainingNode) {
				alloc := mock.Alloc()
				alloc.NodeID = dn.node.ID
				require.Nil(t, dn.state.UpsertJob(structs.MsgTypeTestSetup, 101, nil, alloc.Job))
				require.Nil(t, dn.state.UpsertAllocs(structs.MsgTypeTestSetup, 102, []*structs.Allocation{alloc}))
			},
		},
		{
			name:      "System",
			isDone:    true,
			remaining: 1,
			running:   0,
			setup: func(t *testing.T, dn *drainingNode) {
				alloc := mock.SystemAlloc()
				alloc.NodeID = dn.node.ID
				require.Nil(t, dn.state.UpsertJob(structs.MsgTypeTestSetup, 101, nil, alloc.Job))
				require.Nil(t, dn.state.UpsertAllocs(structs.MsgTypeTestSetup, 102, []*structs.Allocation{alloc}))
			},
		},
		{
			name:      "AllTerminal",
			isDone:    true,
			remaining: 0,
			running:   0,
			setup: func(t *testing.T, dn *drainingNode) {
				allocs := []*structs.Allocation{mock.Alloc(), mock.BatchAlloc(), mock.SystemAlloc()}
				for _, a := range allocs {
					a.NodeID = dn.node.ID
					require.Nil(t, dn.state.UpsertJob(structs.MsgTypeTestSetup, 101, nil, a.Job))
				}
				require.Nil(t, dn.state.UpsertAllocs(structs.MsgTypeTestSetup, 102, allocs))

				// StateStore doesn't like inserting new allocs
				// with a terminal status, so set the status in
				// a second pass
				for _, a := range allocs {
					a.ClientStatus = structs.AllocClientStatusComplete
				}
				require.Nil(t, dn.state.UpsertAllocs(structs.MsgTypeTestSetup, 103, allocs))
			},
		},
		{
			name:      "ServiceTerminal",
			isDone:    false,
			remaining: 2,
			running:   1,
			setup: func(t *testing.T, dn *drainingNode) {
				allocs := []*structs.Allocation{mock.Alloc(), mock.BatchAlloc(), mock.SystemAlloc()}
				for _, a := range allocs {
					a.NodeID = dn.node.ID
					require.Nil(t, dn.state.UpsertJob(structs.MsgTypeTestSetup, 101, nil, a.Job))
				}
				require.Nil(t, dn.state.UpsertAllocs(structs.MsgTypeTestSetup, 102, allocs))

				// Set only the service job as terminal
				allocs[0].ClientStatus = structs.AllocClientStatusComplete
				require.Nil(t, dn.state.UpsertAllocs(structs.MsgTypeTestSetup, 103, allocs))
			},
		},
		{
			name:      "AllTerminalButBatch",
			isDone:    false,
			remaining: 1,
			running:   1,
			setup: func(t *testing.T, dn *drainingNode) {
				allocs := []*structs.Allocation{mock.Alloc(), mock.BatchAlloc(), mock.SystemAlloc()}
				for _, a := range allocs {
					a.NodeID = dn.node.ID
					require.Nil(t, dn.state.UpsertJob(structs.MsgTypeTestSetup, 101, nil, a.Job))
				}
				require.Nil(t, dn.state.UpsertAllocs(structs.MsgTypeTestSetup, 102, allocs))

				// Set only the service and batch jobs as terminal
				allocs[0].ClientStatus = structs.AllocClientStatusComplete
				allocs[2].ClientStatus = structs.AllocClientStatusComplete
				require.Nil(t, dn.state.UpsertAllocs(structs.MsgTypeTestSetup, 103, allocs))
			},
		},
		{
			name:      "AllTerminalButSystem",
			isDone:    true,
			remaining: 1,
			running:   0,
			setup: func(t *testing.T, dn *drainingNode) {
				allocs := []*structs.Allocation{mock.Alloc(), mock.BatchAlloc(), mock.SystemAlloc()}
				for _, a := range allocs {
					a.NodeID = dn.node.ID
					require.Nil(t, dn.state.UpsertJob(structs.MsgTypeTestSetup, 101, nil, a.Job))
				}
				require.Nil(t, dn.state.UpsertAllocs(structs.MsgTypeTestSetup, 102, allocs))

				// Set only the service and batch jobs as terminal
				allocs[0].ClientStatus = structs.AllocClientStatusComplete
				allocs[1].ClientStatus = structs.AllocClientStatusComplete
				require.Nil(t, dn.state.UpsertAllocs(structs.MsgTypeTestSetup, 103, allocs))
			},
		},
		{
			name:      "HalfTerminal",
			isDone:    false,
			remaining: 3,
			running:   2,
			setup: func(t *testing.T, dn *drainingNode) {
				allocs := []*structs.Allocation{
					mock.Alloc(),
					mock.BatchAlloc(),
					mock.SystemAlloc(),
					mock.Alloc(),
					mock.BatchAlloc(),
					mock.SystemAlloc(),
				}
				for _, a := range allocs {
					a.NodeID = dn.node.ID
					require.Nil(t, dn.state.UpsertJob(structs.MsgTypeTestSetup, 101, nil, a.Job))
				}
				require.Nil(t, dn.state.UpsertAllocs(structs.MsgTypeTestSetup, 102, allocs))

				// Set only the service and batch jobs as terminal
				allocs[0].ClientStatus = structs.AllocClientStatusComplete
				allocs[1].ClientStatus = structs.AllocClientStatusComplete
				allocs[2].ClientStatus = structs.AllocClientStatusComplete
				require.Nil(t, dn.state.UpsertAllocs(structs.MsgTypeTestSetup, 103, allocs))
			},
		},
	}

	// Default test drainingNode has no allocs, so it should be done and
	// have no remaining allocs.
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			dn := testDrainingNode(t)
			tc.setup(t, dn)
			assertDrainingNode(t, dn, tc.isDone, tc.remaining, tc.running)
		})
	}
}
