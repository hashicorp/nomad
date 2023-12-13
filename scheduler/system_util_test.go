// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"testing"
	"time"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

// diffResultCount is a test helper struct that makes it easier to specify an
// expected diff
type diffResultCount struct {
	place, update, migrate, stop, ignore, lost, disconnecting, reconnecting int
}

// assertDiffCount is a test helper that compares against a diffResult
func assertDiffCount(t *testing.T, expected diffResultCount, diff *diffResult) {
	t.Helper()
	test.Len(t, expected.update, diff.update, test.Sprintf("expected update"))
	test.Len(t, expected.ignore, diff.ignore, test.Sprintf("expected ignore"))
	test.Len(t, expected.stop, diff.stop, test.Sprintf("expected stop"))
	test.Len(t, expected.migrate, diff.migrate, test.Sprintf("expected migrate"))
	test.Len(t, expected.lost, diff.lost, test.Sprintf("expected lost"))
	test.Len(t, expected.place, diff.place, test.Sprintf("expected place"))
}

func TestDiffSystemAllocsForNode_Sysbatch_terminal(t *testing.T) {
	ci.Parallel(t)

	// For a sysbatch job, the scheduler should not re-place an allocation
	// that has become terminal, unless the job has been updated.

	job := mock.SystemBatchJob()
	job.TaskGroups[0].Count = 2
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

		assertDiffCount(t, diffResultCount{ignore: 1, place: 1}, diff)
		if len(diff.ignore) > 0 {
			must.Eq(t, terminal["node1"]["my-sysbatch.pinger[0]"], diff.ignore[0].Alloc)
		}
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

		diff := diffSystemAllocsForNode(job, "node1", eligible, nil, tainted, required, live, terminal, true)
		assertDiffCount(t, diffResultCount{update: 1, place: 1}, diff)
	})

}

// TestDiffSystemAllocsForNode_Placements verifies we only place on nodes that
// need placements
func TestDiffSystemAllocsForNode_Placements(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()
	required := materializeSystemTaskGroups(job)

	goodNode := mock.Node()
	unusedNode := mock.Node()
	drainNode := mock.DrainNode()
	deadNode := mock.Node()
	deadNode.Status = structs.NodeStatusDown

	tainted := map[string]*structs.Node{
		deadNode.ID:  deadNode,
		drainNode.ID: drainNode,
	}

	eligible := map[string]*structs.Node{
		goodNode.ID: goodNode,
	}

	terminal := structs.TerminalByNodeByName{}
	allocsForNode := []*structs.Allocation{}

	testCases := []struct {
		name     string
		nodeID   string
		expected diffResultCount
	}{
		{
			name:     "expect placement on good node",
			nodeID:   goodNode.ID,
			expected: diffResultCount{place: 1},
		},
		{ // "unused" here means outside of the eligible set
			name:     "expect no placement on unused node",
			nodeID:   unusedNode.ID,
			expected: diffResultCount{},
		},
		{
			name:     "expect no placement on dead node",
			nodeID:   deadNode.ID,
			expected: diffResultCount{},
		},
		{
			name:     "expect no placement on draining node",
			nodeID:   drainNode.ID,
			expected: diffResultCount{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			diff := diffSystemAllocsForNode(
				job, tc.nodeID, eligible, nil,
				tainted, required, allocsForNode, terminal, true)

			assertDiffCount(t, tc.expected, diff)
		})
	}
}

// TestDiffSystemAllocsForNodes_Stops verifies we stop allocs we no longer need
func TestDiffSystemAllocsForNode_Stops(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()
	required := materializeSystemTaskGroups(job)

	// The "old" job has a previous modify index but is otherwise unchanged, so
	// existing non-terminal allocs for this version should be updated in-place

	// TODO(tgross): *unless* there's another alloc for the same job already on
	// the node. See https://github.com/hashicorp/nomad/pull/16097
	oldJob := new(structs.Job)
	*oldJob = *job
	oldJob.JobModifyIndex -= 1

	node := mock.Node()

	eligible := map[string]*structs.Node{
		node.ID: node,
	}

	allocs := []*structs.Allocation{
		{
			// extraneous alloc for old version of job should be updated
			// TODO(tgross): this should actually be stopped.
			// See https://github.com/hashicorp/nomad/pull/16097
			ID:     uuid.Generate(),
			NodeID: node.ID,
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},
		{ // most recent alloc for current version of job should be ignored
			ID:     uuid.Generate(),
			NodeID: node.ID,
			Name:   "my-job.web[0]",
			Job:    job,
		},
		{ // task group not required, should be stopped
			ID:     uuid.Generate(),
			NodeID: node.ID,
			Name:   "my-job.something-else[0]",
			Job:    job,
		},
	}

	tainted := map[string]*structs.Node{}
	terminal := structs.TerminalByNodeByName{}

	diff := diffSystemAllocsForNode(
		job, node.ID, eligible, nil, tainted, required, allocs, terminal, true)

	assertDiffCount(t, diffResultCount{ignore: 1, stop: 1, update: 1}, diff)
	if len(diff.update) > 0 {
		test.Eq(t, allocs[0], diff.update[0].Alloc)
	}
	if len(diff.ignore) > 0 {
		test.Eq(t, allocs[1], diff.ignore[0].Alloc)
	}
	if len(diff.stop) > 0 {
		test.Eq(t, allocs[2], diff.stop[0].Alloc)
	}
}

// Test the desired diff for an updated system job running on a ineligible node
func TestDiffSystemAllocsForNode_IneligibleNode(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()
	required := materializeSystemTaskGroups(job)

	ineligibleNode := mock.Node()
	ineligibleNode.SchedulingEligibility = structs.NodeSchedulingIneligible
	ineligible := map[string]struct{}{
		ineligibleNode.ID: {},
	}

	eligible := map[string]*structs.Node{}
	tainted := map[string]*structs.Node{}

	terminal := structs.TerminalByNodeByName{
		ineligibleNode.ID: map[string]*structs.Allocation{
			"my-job.web[0]": { // terminal allocs should not appear in diff
				ID:           uuid.Generate(),
				NodeID:       ineligibleNode.ID,
				Name:         "my-job.web[0]",
				Job:          job,
				ClientStatus: structs.AllocClientStatusComplete,
			},
		},
	}

	testCases := []struct {
		name   string
		nodeID string
		expect diffResultCount
	}{
		{
			name:   "non-terminal alloc on ineligible node should be ignored",
			nodeID: ineligibleNode.ID,
			expect: diffResultCount{ignore: 1},
		},
		{
			name:   "non-terminal alloc on node not in eligible set should be stopped",
			nodeID: uuid.Generate(),
			expect: diffResultCount{stop: 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			alloc := &structs.Allocation{
				ID:     uuid.Generate(),
				NodeID: tc.nodeID,
				Name:   "my-job.web[0]",
				Job:    job,
			}

			diff := diffSystemAllocsForNode(
				job, tc.nodeID, eligible, ineligible, tainted,
				required, []*structs.Allocation{alloc}, terminal, true,
			)
			assertDiffCount(t, tc.expect, diff)
		})
	}
}

func TestDiffSystemAllocsForNode_DrainingNode(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()
	required := materializeSystemTaskGroups(job)

	// The "old" job has a previous modify index but is otherwise unchanged, so
	// existing non-terminal allocs for this version should be updated in-place
	oldJob := job.Copy()
	oldJob.JobModifyIndex -= 1

	drainNode := mock.DrainNode()
	tainted := map[string]*structs.Node{
		drainNode.ID: drainNode,
	}

	// Terminal allocs don't get touched
	terminal := structs.TerminalByNodeByName{
		drainNode.ID: map[string]*structs.Allocation{
			"my-job.web[0]": {
				ID:           uuid.Generate(),
				NodeID:       drainNode.ID,
				Name:         "my-job.web[0]",
				Job:          job,
				ClientStatus: structs.AllocClientStatusComplete,
			},
		},
	}

	allocs := []*structs.Allocation{
		{ // allocs for draining node should be migrated
			ID:     uuid.Generate(),
			NodeID: drainNode.ID,
			Name:   "my-job.web[0]",
			Job:    oldJob,
			DesiredTransition: structs.DesiredTransition{
				Migrate: pointer.Of(true),
			},
		},
		{ // allocs not marked for drain should be ignored
			ID:     uuid.Generate(),
			NodeID: drainNode.ID,
			Name:   "my-job.web[0]",
			Job:    job,
		},
	}

	diff := diffSystemAllocsForNode(
		job, drainNode.ID, map[string]*structs.Node{}, nil,
		tainted, required, allocs, terminal, true)

	assertDiffCount(t, diffResultCount{migrate: 1, ignore: 1}, diff)
	if len(diff.migrate) > 0 {
		test.Eq(t, allocs[0], diff.migrate[0].Alloc)
	}
}

func TestDiffSystemAllocsForNode_LostNode(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()
	required := materializeSystemTaskGroups(job)

	// The "old" job has a previous modify index but is otherwise unchanged, so
	// existing non-terminal allocs for this version should be updated in-place
	oldJob := new(structs.Job)
	*oldJob = *job
	oldJob.JobModifyIndex -= 1

	deadNode := mock.Node()
	deadNode.Status = structs.NodeStatusDown

	tainted := map[string]*structs.Node{
		deadNode.ID: deadNode,
	}

	allocs := []*structs.Allocation{
		{ // current allocs on a lost node are lost, even if terminal
			ID:     uuid.Generate(),
			NodeID: deadNode.ID,
			Name:   "my-job.web[0]",
			Job:    job,
		},
		{ // old allocs on a lost node are also lost
			ID:     uuid.Generate(),
			NodeID: deadNode.ID,
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},
	}

	// Terminal allocs don't get touched
	terminal := structs.TerminalByNodeByName{
		deadNode.ID: map[string]*structs.Allocation{
			"my-job.web[0]": allocs[0],
		},
	}

	diff := diffSystemAllocsForNode(
		job, deadNode.ID, map[string]*structs.Node{}, nil,
		tainted, required, allocs, terminal, true)

	assertDiffCount(t, diffResultCount{lost: 2}, diff)
	if len(diff.migrate) > 0 {
		test.Eq(t, allocs[0], diff.migrate[0].Alloc)
	}
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
			expect: diffResultCount{disconnecting: 1},
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
			expect: diffResultCount{reconnecting: 1},
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
			expect: diffResultCount{ignore: 1},
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
			expect: diffResultCount{lost: 1},
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
			expect: diffResultCount{ignore: 1},
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
			assertDiffCount(t, tc.expect, got)
		})
	}
}

// TestDiffSystemAllocs is a higher-level test of interactions of diffs across
// multiple nodes.
func TestDiffSystemAllocs(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()
	tg := job.TaskGroups[0].Copy()
	tg.Name = "other"
	job.TaskGroups = append(job.TaskGroups, tg)

	drainNode := mock.DrainNode()
	drainNode.ID = "drain"

	deadNode := mock.Node()
	deadNode.ID = "dead"
	deadNode.Status = structs.NodeStatusDown

	tainted := map[string]*structs.Node{
		deadNode.ID:  deadNode,
		drainNode.ID: drainNode,
	}

	// Create four alive nodes.
	nodes := []*structs.Node{{ID: "foo"}, {ID: "bar"}, {ID: "baz"},
		{ID: "has-term"}, {ID: drainNode.ID}, {ID: deadNode.ID}}

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

	// Have one terminal allocs
	terminal := structs.TerminalByNodeByName{
		"has-term": map[string]*structs.Allocation{
			"my-job.web[0]": {
				ID:     uuid.Generate(),
				NodeID: "has-term",
				Name:   "my-job.web[0]",
				Job:    job,
			},
		},
	}

	diff := diffSystemAllocs(job, nodes, nil, tainted, allocs, terminal, true)

	assertDiffCount(t, diffResultCount{
		update: 1, ignore: 1, migrate: 1, lost: 1, place: 6}, diff)

	if len(diff.update) > 0 {
		must.Eq(t, allocs[0], diff.update[0].Alloc) // first alloc should be updated
	}
	if len(diff.ignore) > 0 {
		must.Eq(t, allocs[1], diff.ignore[0].Alloc) // We should ignore the second alloc
	}
	if len(diff.migrate) > 0 {
		must.Eq(t, allocs[2], diff.migrate[0].Alloc)
	}
	if len(diff.lost) > 0 {
		must.Eq(t, allocs[3], diff.lost[0].Alloc) // We should mark the 5th alloc as lost
	}

	// Ensure that the allocations which are replacements of terminal allocs are
	// annotated.
	for _, m := range terminal {
		for _, alloc := range m {
			for _, tuple := range diff.place {
				if alloc.NodeID == tuple.Alloc.NodeID && alloc.TaskGroup == "web" {
					must.Eq(t, alloc, tuple.Alloc)
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
	must.True(t, evictAndPlace(ctx, diff, allocs, "", &limit),
		must.Sprintf("evictAndReplace() should have returned true"))
	must.Zero(t, limit,
		must.Sprint("evictAndReplace() should decrement limit"))
	must.Len(t, 2, diff.place,
		must.Sprintf("evictAndReplace() didn't insert into diffResult properly: %v", diff.place))
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
	must.False(t, evictAndPlace(ctx, diff, allocs, "", &limit),
		must.Sprint("evictAndReplace() should have returned false"))
	must.Zero(t, limit, must.Sprint("evictAndReplace() should decrement limit"))
	must.Len(t, 4, diff.place,
		must.Sprintf("evictAndReplace() didn't insert into diffResult properly: %v", diff.place))
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
	must.False(t, evictAndPlace(ctx, diff, allocs, "", &limit))
	must.Eq(t, 2, limit, must.Sprint("evictAndReplace() should decrement limit"))
	must.Len(t, 4, diff.place, must.Sprintf("evictAndReplace() didn't insert into diffResult properly: %v", diff.place))
}
