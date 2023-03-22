package scheduler

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestDiffSystemAllocsForNode_Sysbatch_terminal(t *testing.T) {
	ci.Parallel(t)

	// For a sysbatch job, the scheduler should not re-place an allocation
	// that has become terminal, unless the job has been updated.

	job := mock.SystemBatchJob()
	required := materializeSystemTaskGroups(job)

	eligible := map[string]*structs.Node{
		"node1": newNode("node1"),
	}

	var live []*structs.Allocation // empty

	tainted := map[string]*structs.Node(nil)

	t.Run("current job", func(t *testing.T) {
		terminal := structs.TerminalByNodeByName{
			"node1": map[string]*structs.Allocation{
				"my-sysbatch.pinger[0]": {
					ID:           uuid.Generate(),
					NodeID:       "node1",
					Name:         "my-sysbatch.pinger[0]",
					Job:          job,
					ClientStatus: structs.AllocClientStatusComplete,
				},
			},
		}

		diff := diffSystemAllocsForNode(job, "node1", eligible, nil, tainted, required, live, terminal, true)
		require.Empty(t, diff.place)
		require.Empty(t, diff.update)
		require.Empty(t, diff.stop)
		require.Empty(t, diff.migrate)
		require.Empty(t, diff.lost)
		require.True(t, len(diff.ignore) == 1 && diff.ignore[0].Alloc == terminal["node1"]["my-sysbatch.pinger[0]"])
	})

	t.Run("outdated job", func(t *testing.T) {
		previousJob := job.Copy()
		previousJob.JobModifyIndex -= 1
		terminal := structs.TerminalByNodeByName{
			"node1": map[string]*structs.Allocation{
				"my-sysbatch.pinger[0]": {
					ID:     uuid.Generate(),
					NodeID: "node1",
					Name:   "my-sysbatch.pinger[0]",
					Job:    previousJob,
				},
			},
		}

		expAlloc := terminal["node1"]["my-sysbatch.pinger[0]"]
		expAlloc.NodeID = "node1"

		diff := diffSystemAllocsForNode(job, "node1", eligible, nil, tainted, required, live, terminal, true)
		require.Empty(t, diff.place)
		require.Len(t, diff.update, 1)
		require.Empty(t, diff.stop)
		require.Empty(t, diff.migrate)
		require.Empty(t, diff.lost)
		require.Empty(t, diff.ignore)
	})
}

func TestDiffSystemAllocsForNode(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	required := materializeSystemTaskGroups(job)

	// The "old" job has a previous modify index
	oldJob := new(structs.Job)
	*oldJob = *job
	oldJob.JobModifyIndex -= 1

	eligibleNode := mock.Node()
	eligibleNode.ID = "zip"

	drainNode := mock.DrainNode()

	deadNode := mock.Node()
	deadNode.Status = structs.NodeStatusDown

	tainted := map[string]*structs.Node{
		"dead":      deadNode,
		"drainNode": drainNode,
	}

	eligible := map[string]*structs.Node{
		eligibleNode.ID: eligibleNode,
	}

	allocs := []*structs.Allocation{
		// Update the 1st
		{
			ID:     uuid.Generate(),
			NodeID: "zip",
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},

		// Ignore the 2rd
		{
			ID:     uuid.Generate(),
			NodeID: "zip",
			Name:   "my-job.web[1]",
			Job:    job,
		},

		// Evict 11th
		{
			ID:     uuid.Generate(),
			NodeID: "zip",
			Name:   "my-job.web[10]",
			Job:    oldJob,
		},

		// Migrate the 3rd
		{
			ID:     uuid.Generate(),
			NodeID: "drainNode",
			Name:   "my-job.web[2]",
			Job:    oldJob,
			DesiredTransition: structs.DesiredTransition{
				Migrate: pointer.Of(true),
			},
		},
		// Mark the 4th lost
		{
			ID:     uuid.Generate(),
			NodeID: "dead",
			Name:   "my-job.web[3]",
			Job:    oldJob,
		},
	}

	// Have three terminal allocs
	terminal := structs.TerminalByNodeByName{
		"zip": map[string]*structs.Allocation{
			"my-job.web[4]": {
				ID:     uuid.Generate(),
				NodeID: "zip",
				Name:   "my-job.web[4]",
				Job:    job,
			},
			"my-job.web[5]": {
				ID:     uuid.Generate(),
				NodeID: "zip",
				Name:   "my-job.web[5]",
				Job:    job,
			},
			"my-job.web[6]": {
				ID:     uuid.Generate(),
				NodeID: "zip",
				Name:   "my-job.web[6]",
				Job:    job,
			},
		},
	}

	diff := diffSystemAllocsForNode(job, "zip", eligible, nil, tainted, required, allocs, terminal, true)

	// We should update the first alloc
	require.Len(t, diff.update, 1)
	require.Equal(t, allocs[0], diff.update[0].Alloc)

	// We should ignore the second alloc
	require.Len(t, diff.ignore, 1)
	require.Equal(t, allocs[1], diff.ignore[0].Alloc)

	// We should stop the 3rd alloc
	require.Len(t, diff.stop, 1)
	require.Equal(t, allocs[2], diff.stop[0].Alloc)

	// We should migrate the 4rd alloc
	require.Len(t, diff.migrate, 1)
	require.Equal(t, allocs[3], diff.migrate[0].Alloc)

	// We should mark the 5th alloc as lost
	require.Len(t, diff.lost, 1)
	require.Equal(t, allocs[4], diff.lost[0].Alloc)

	// We should place 6
	require.Len(t, diff.place, 6)

	// Ensure that the allocations which are replacements of terminal allocs are
	// annotated.
	for _, m := range terminal {
		for _, alloc := range m {
			for _, tuple := range diff.place {
				if alloc.Name == tuple.Name {
					require.Equal(t, alloc, tuple.Alloc)
				}
			}
		}
	}
}

// Test the desired diff for an updated system job running on a
// ineligible node
func TestDiffSystemAllocsForNode_ExistingAllocIneligibleNode(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()
	required := materializeSystemTaskGroups(job)

	// The "old" job has a previous modify index
	oldJob := new(structs.Job)
	*oldJob = *job
	oldJob.JobModifyIndex -= 1

	eligibleNode := mock.Node()
	ineligibleNode := mock.Node()
	ineligibleNode.SchedulingEligibility = structs.NodeSchedulingIneligible

	tainted := map[string]*structs.Node{}

	eligible := map[string]*structs.Node{
		eligibleNode.ID: eligibleNode,
	}

	allocs := []*structs.Allocation{
		// Update the TG alloc running on eligible node
		{
			ID:     uuid.Generate(),
			NodeID: eligibleNode.ID,
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},

		// Ignore the TG alloc running on ineligible node
		{
			ID:     uuid.Generate(),
			NodeID: ineligibleNode.ID,
			Name:   "my-job.web[0]",
			Job:    job,
		},
	}

	// No terminal allocs
	terminal := make(structs.TerminalByNodeByName)

	diff := diffSystemAllocsForNode(job, eligibleNode.ID, eligible, nil, tainted, required, allocs, terminal, true)

	require.Len(t, diff.place, 0)
	require.Len(t, diff.update, 1)
	require.Len(t, diff.migrate, 0)
	require.Len(t, diff.stop, 0)
	require.Len(t, diff.ignore, 1)
	require.Len(t, diff.lost, 0)
}

func TestDiffSystemAllocsForNode_DisconnectedNode(t *testing.T) {
	ci.Parallel(t)

	// Create job.
	job := mock.SystemJob()
	job.TaskGroups[0].MaxClientDisconnect = pointer.Of(time.Hour)

	// Create nodes.
	readyNode := mock.Node()
	readyNode.Status = structs.NodeStatusReady

	disconnectedNode := mock.Node()
	disconnectedNode.Status = structs.NodeStatusDisconnected

	eligibleNodes := map[string]*structs.Node{
		readyNode.ID: readyNode,
	}

	taintedNodes := map[string]*structs.Node{
		disconnectedNode.ID: disconnectedNode,
	}

	// Create allocs.
	required := materializeSystemTaskGroups(job)
	terminal := make(structs.TerminalByNodeByName)

	type diffResultCount struct {
		place, update, migrate, stop, ignore, lost, disconnecting, reconnecting int
	}

	testCases := []struct {
		name    string
		node    *structs.Node
		allocFn func(*structs.Allocation)
		expect  diffResultCount
	}{
		{
			name: "alloc in disconnected client is marked as unknown",
			node: disconnectedNode,
			allocFn: func(alloc *structs.Allocation) {
				alloc.ClientStatus = structs.AllocClientStatusRunning
			},
			expect: diffResultCount{
				disconnecting: 1,
			},
		},
		{
			name: "disconnected alloc reconnects",
			node: readyNode,
			allocFn: func(alloc *structs.Allocation) {
				alloc.ClientStatus = structs.AllocClientStatusRunning

				alloc.AllocStates = []*structs.AllocState{{
					Field: structs.AllocStateFieldClientStatus,
					Value: structs.AllocClientStatusUnknown,
					Time:  time.Now().Add(-time.Minute),
				}}
			},
			expect: diffResultCount{
				reconnecting: 1,
			},
		},
		{
			name: "alloc not reconnecting after it reconnects",
			node: readyNode,
			allocFn: func(alloc *structs.Allocation) {
				alloc.ClientStatus = structs.AllocClientStatusRunning

				alloc.AllocStates = []*structs.AllocState{
					{
						Field: structs.AllocStateFieldClientStatus,
						Value: structs.AllocClientStatusUnknown,
						Time:  time.Now().Add(-time.Minute),
					},
					{
						Field: structs.AllocStateFieldClientStatus,
						Value: structs.AllocClientStatusRunning,
						Time:  time.Now(),
					},
				}
			},
			expect: diffResultCount{
				ignore: 1,
			},
		},
		{
			name: "disconnected alloc is lost after it expires",
			node: disconnectedNode,
			allocFn: func(alloc *structs.Allocation) {
				alloc.ClientStatus = structs.AllocClientStatusUnknown

				alloc.AllocStates = []*structs.AllocState{{
					Field: structs.AllocStateFieldClientStatus,
					Value: structs.AllocClientStatusUnknown,
					Time:  time.Now().Add(-10 * time.Hour),
				}}
			},
			expect: diffResultCount{
				lost: 1,
			},
		},
		{
			name: "disconnected allocs are ignored",
			node: disconnectedNode,
			allocFn: func(alloc *structs.Allocation) {
				alloc.ClientStatus = structs.AllocClientStatusUnknown

				alloc.AllocStates = []*structs.AllocState{{
					Field: structs.AllocStateFieldClientStatus,
					Value: structs.AllocClientStatusUnknown,
					Time:  time.Now(),
				}}
			},
			expect: diffResultCount{
				ignore: 1,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			alloc := mock.AllocForNode(tc.node)
			alloc.JobID = job.ID
			alloc.Job = job
			alloc.Name = fmt.Sprintf("%s.%s[0]", job.Name, job.TaskGroups[0].Name)

			if tc.allocFn != nil {
				tc.allocFn(alloc)
			}

			got := diffSystemAllocsForNode(
				job, tc.node.ID, eligibleNodes, nil, taintedNodes,
				required, []*structs.Allocation{alloc}, terminal, true,
			)

			assert.Len(t, got.place, tc.expect.place, "place")
			assert.Len(t, got.update, tc.expect.update, "update")
			assert.Len(t, got.migrate, tc.expect.migrate, "migrate")
			assert.Len(t, got.stop, tc.expect.stop, "stop")
			assert.Len(t, got.ignore, tc.expect.ignore, "ignore")
			assert.Len(t, got.lost, tc.expect.lost, "lost")
			assert.Len(t, got.disconnecting, tc.expect.disconnecting, "disconnecting")
			assert.Len(t, got.reconnecting, tc.expect.reconnecting, "reconnecting")
		})
	}
}

func TestDiffSystemAllocs(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()

	drainNode := mock.DrainNode()

	deadNode := mock.Node()
	deadNode.Status = structs.NodeStatusDown

	tainted := map[string]*structs.Node{
		deadNode.ID:  deadNode,
		drainNode.ID: drainNode,
	}

	// Create three alive nodes.
	nodes := []*structs.Node{{ID: "foo"}, {ID: "bar"}, {ID: "baz"},
		{ID: "pipe"}, {ID: drainNode.ID}, {ID: deadNode.ID}}

	// The "old" job has a previous modify index
	oldJob := new(structs.Job)
	*oldJob = *job
	oldJob.JobModifyIndex -= 1

	allocs := []*structs.Allocation{
		// Update allocation on baz
		{
			ID:     uuid.Generate(),
			NodeID: "baz",
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},

		// Ignore allocation on bar
		{
			ID:     uuid.Generate(),
			NodeID: "bar",
			Name:   "my-job.web[0]",
			Job:    job,
		},

		// Stop allocation on draining node.
		{
			ID:     uuid.Generate(),
			NodeID: drainNode.ID,
			Name:   "my-job.web[0]",
			Job:    oldJob,
			DesiredTransition: structs.DesiredTransition{
				Migrate: pointer.Of(true),
			},
		},
		// Mark as lost on a dead node
		{
			ID:     uuid.Generate(),
			NodeID: deadNode.ID,
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},
	}

	// Have three (?) terminal allocs
	terminal := structs.TerminalByNodeByName{
		"pipe": map[string]*structs.Allocation{
			"my-job.web[0]": {
				ID:     uuid.Generate(),
				NodeID: "pipe",
				Name:   "my-job.web[0]",
				Job:    job,
			},
		},
	}

	diff := diffSystemAllocs(job, nodes, nil, tainted, allocs, terminal, true)

	// We should update the first alloc
	require.Len(t, diff.update, 1)
	require.Equal(t, allocs[0], diff.update[0].Alloc)

	// We should ignore the second alloc
	require.Len(t, diff.ignore, 1)
	require.Equal(t, allocs[1], diff.ignore[0].Alloc)

	// We should stop the third alloc
	require.Empty(t, diff.stop)

	// There should be no migrates.
	require.Len(t, diff.migrate, 1)
	require.Equal(t, allocs[2], diff.migrate[0].Alloc)

	// We should mark the 5th alloc as lost
	require.Len(t, diff.lost, 1)
	require.Equal(t, allocs[3], diff.lost[0].Alloc)

	// We should place 2
	require.Len(t, diff.place, 2)

	// Ensure that the allocations which are replacements of terminal allocs are
	// annotated.
	for _, m := range terminal {
		for _, alloc := range m {
			for _, tuple := range diff.place {
				if alloc.NodeID == tuple.Alloc.NodeID {
					require.Equal(t, alloc, tuple.Alloc)
				}
			}
		}
	}
}

func TestEvictAndPlace_LimitLessThanAllocs(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	allocs := []allocTuple{
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
	}
	diff := &diffResult{}

	limit := 2
	require.True(t, evictAndPlace(ctx, diff, allocs, "", &limit), "evictAndReplace() should have returned true")
	require.Zero(t, limit, "evictAndReplace() should decremented limit; got %v; want 0", limit)
	require.Equal(t, 2, len(diff.place), "evictAndReplace() didn't insert into diffResult properly: %v", diff.place)
}

func TestEvictAndPlace_LimitEqualToAllocs(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	allocs := []allocTuple{
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
	}
	diff := &diffResult{}

	limit := 4
	require.False(t, evictAndPlace(ctx, diff, allocs, "", &limit), "evictAndReplace() should have returned false")
	require.Zero(t, limit, "evictAndReplace() should decremented limit; got %v; want 0", limit)
	require.Equal(t, 4, len(diff.place), "evictAndReplace() didn't insert into diffResult properly: %v", diff.place)
}

func TestEvictAndPlace_LimitGreaterThanAllocs(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	allocs := []allocTuple{
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
	}
	diff := &diffResult{}

	limit := 6
	require.False(t, evictAndPlace(ctx, diff, allocs, "", &limit))
	require.Equal(t, 2, limit, "evictAndReplace() should decremented limit")
	require.Equal(t, 4, len(diff.place), "evictAndReplace() didn't insert into diffResult properly: %v", diff.place)
}
