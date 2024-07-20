// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package drainer

import (
	"context"
	"testing"
	"time"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

func testNodes(t *testing.T, state *state.StateStore) (drainingNode, runningNode *structs.Node) {
	n1 := mock.Node()
	n1.Name = "draining"
	n1.DrainStrategy = &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: time.Minute,
		},
		ForceDeadline: time.Now().Add(time.Minute),
	}
	require.Nil(t, state.UpsertNode(structs.MsgTypeTestSetup, 100, n1))

	// Create a non-draining node
	n2 := mock.Node()
	n2.Name = "running"
	require.Nil(t, state.UpsertNode(structs.MsgTypeTestSetup, 101, n2))
	return n1, n2
}

func testDrainingJobWatcher(t *testing.T, state *state.StateStore) (*drainingJobWatcher, context.CancelFunc) {
	t.Helper()

	limiter := rate.NewLimiter(100.0, 100)
	logger := testlog.HCLogger(t)
	ctx, cancel := context.WithCancel(context.Background())
	w := NewDrainingJobWatcher(ctx, limiter, state, logger)
	return w, cancel
}

// TestDrainingJobWatcher_Interface is a compile-time assertion that we
// implement the intended interface.
func TestDrainingJobWatcher_Interface(t *testing.T) {
	ci.Parallel(t)

	w, cancel := testDrainingJobWatcher(t, state.TestStateStore(t))
	cancel()
	var _ DrainingJobWatcher = w
}

// asertJobWatcherOps asserts a certain number of allocs are drained and/or
// migrated by the job watcher.
func assertJobWatcherOps(t *testing.T, jw DrainingJobWatcher, drained, migrated int) (
	*DrainRequest, []*structs.Allocation) {
	t.Helper()
	var (
		drains                           *DrainRequest
		migrations                       []*structs.Allocation
		drainsChecked, migrationsChecked bool
	)
	for {
		select {
		case drains = <-jw.Drain():
			ids := make([]string, len(drains.Allocs))
			for i, a := range drains.Allocs {
				ids[i] = a.JobID[:6] + ":" + a.ID[:6]
			}
			t.Logf("draining %d allocs: %v", len(ids), ids)
			require.False(t, drainsChecked, "drains already received")
			drainsChecked = true
			require.Lenf(t, drains.Allocs, drained,
				"expected %d drains but found %d", drained, len(drains.Allocs))
		case migrations = <-jw.Migrated():
			ids := make([]string, len(migrations))
			for i, a := range migrations {
				ids[i] = a.JobID[:6] + ":" + a.ID[:6]
			}
			t.Logf("migrating %d allocs: %v", len(ids), ids)
			require.False(t, migrationsChecked, "migrations already received")
			migrationsChecked = true
			require.Lenf(t, migrations, migrated,
				"expected %d migrations but found %d", migrated, len(migrations))
		case <-time.After(10 * time.Millisecond):
			if !drainsChecked && drained > 0 {
				t.Fatalf("expected %d drains but none happened", drained)
			}
			if !migrationsChecked && migrated > 0 {
				t.Fatalf("expected %d migrations but none happened", migrated)
			}
			return drains, migrations
		}
	}
}

// TestDrainingJobWatcher_DrainJobs asserts DrainingJobWatcher batches
// allocation changes from multiple jobs.
func TestDrainingJobWatcher_DrainJobs(t *testing.T) {
	ci.Parallel(t)

	store := state.TestStateStore(t)
	jobWatcher, cancelWatcher := testDrainingJobWatcher(t, store)
	defer cancelWatcher()
	drainingNode, runningNode := testNodes(t, store)

	var index uint64 = 101
	count := 8

	newAlloc := func(node *structs.Node, job *structs.Job) *structs.Allocation {
		a := mock.Alloc()
		a.JobID = job.ID
		a.Job = job
		a.TaskGroup = job.TaskGroups[0].Name
		a.NodeID = node.ID
		return a
	}

	// 2 jobs with count 10, max parallel 3
	jnss := make([]structs.NamespacedID, 2)
	jobs := make([]*structs.Job, 2)
	for i := 0; i < 2; i++ {
		job := mock.Job()
		jobs[i] = job
		jnss[i] = structs.NamespacedID{Namespace: job.Namespace, ID: job.ID}
		job.TaskGroups[0].Migrate.MaxParallel = 3
		job.TaskGroups[0].Count = count
		must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, index, nil, job))
		index++

		var allocs []*structs.Allocation
		for i := 0; i < count; i++ {
			a := newAlloc(drainingNode, job)
			a.DeploymentStatus = &structs.AllocDeploymentStatus{
				Healthy: pointer.Of(true),
			}
			allocs = append(allocs, a)
		}

		must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, index, allocs))
		index++

	}

	// Only register jobs with watcher after creating all data models as
	// once the watcher starts we need to track the index carefully for
	// updating the batch future
	jobWatcher.RegisterJobs(jnss)

	// Expect a first batch of MaxParallel allocs from each job
	drains, _ := assertJobWatcherOps(t, jobWatcher, 6, 0)

	// Fake migrating the drained allocs by starting new ones and stopping
	// the old ones
	drainedAllocs := make([]*structs.Allocation, len(drains.Allocs))
	for i, a := range drains.Allocs {
		a.DesiredTransition.Migrate = pointer.Of(true)

		// create a copy so we can reuse this slice
		drainedAllocs[i] = a.Copy()
	}
	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, index, drainedAllocs))
	drains.Resp.Respond(index, nil)
	index++

	// Just setting ShouldMigrate should not cause any further drains
	assertJobWatcherOps(t, jobWatcher, 0, 0)

	// Proceed our fake migration along by creating new allocs and stopping
	// old ones
	replacements := make([]*structs.Allocation, len(drainedAllocs))
	updates := make([]*structs.Allocation, 0, len(drainedAllocs)*2)
	for i, a := range drainedAllocs {
		// Stop drained allocs
		a.DesiredTransition.Migrate = nil
		a.DesiredStatus = structs.AllocDesiredStatusStop

		// Create a replacement
		replacement := mock.Alloc()
		replacement.JobID = a.Job.ID
		replacement.Job = a.Job
		replacement.TaskGroup = a.TaskGroup
		replacement.NodeID = runningNode.ID
		// start in pending state with no health status

		updates = append(updates, a, replacement)
		replacements[i] = replacement.Copy()
	}
	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, index, updates))
	index++

	// The drained allocs stopping cause migrations but no new drains
	// because the replacements have not started
	assertJobWatcherOps(t, jobWatcher, 0, 0)

	// Client sends stop on these allocs
	completeAllocs := make([]*structs.Allocation, len(drainedAllocs))
	for i, a := range drainedAllocs {
		a = a.Copy()
		a.ClientStatus = structs.AllocClientStatusComplete
		completeAllocs[i] = a
	}
	must.NoError(t, store.UpdateAllocsFromClient(structs.MsgTypeTestSetup, index, completeAllocs))
	index++

	// The drained allocs stopping cause migrations but no new drains
	// because the replacements have not started
	assertJobWatcherOps(t, jobWatcher, 0, 6)

	// Finally kickoff further drain activity by "starting" replacements
	for _, a := range replacements {
		a.ClientStatus = structs.AllocClientStatusRunning
		a.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: pointer.Of(true),
		}
	}
	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, index, replacements))
	index++

	must.MapNotEmpty(t, jobWatcher.drainingJobs())

	// 6 new drains
	drains, _ = assertJobWatcherOps(t, jobWatcher, 6, 0)

	// Fake migrations once more to finish the drain
	drainedAllocs = make([]*structs.Allocation, len(drains.Allocs))
	for i, a := range drains.Allocs {
		a.DesiredTransition.Migrate = pointer.Of(true)

		// create a copy so we can reuse this slice
		drainedAllocs[i] = a.Copy()
	}
	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, index, drainedAllocs))
	drains.Resp.Respond(index, nil)
	index++

	assertJobWatcherOps(t, jobWatcher, 0, 0)

	replacements = make([]*structs.Allocation, len(drainedAllocs))
	updates = make([]*structs.Allocation, 0, len(drainedAllocs)*2)
	for i, a := range drainedAllocs {
		a.DesiredTransition.Migrate = nil
		a.DesiredStatus = structs.AllocDesiredStatusStop
		a.ClientStatus = structs.AllocClientStatusComplete

		replacement := newAlloc(runningNode, a.Job)
		updates = append(updates, a, replacement)
		replacements[i] = replacement.Copy()
	}
	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, index, updates))
	index++

	assertJobWatcherOps(t, jobWatcher, 0, 6)

	for _, a := range replacements {
		a.ClientStatus = structs.AllocClientStatusRunning
		a.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: pointer.Of(true),
		}
	}
	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, index, replacements))
	index++

	must.MapNotEmpty(t, jobWatcher.drainingJobs())

	// Final 4 new drains
	drains, _ = assertJobWatcherOps(t, jobWatcher, 4, 0)

	// Fake migrations once more to finish the drain
	drainedAllocs = make([]*structs.Allocation, len(drains.Allocs))
	for i, a := range drains.Allocs {
		a.DesiredTransition.Migrate = pointer.Of(true)

		// create a copy so we can reuse this slice
		drainedAllocs[i] = a.Copy()
	}
	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, index, drainedAllocs))
	drains.Resp.Respond(index, nil)
	index++

	assertJobWatcherOps(t, jobWatcher, 0, 0)

	replacements = make([]*structs.Allocation, len(drainedAllocs))
	updates = make([]*structs.Allocation, 0, len(drainedAllocs)*2)
	for i, a := range drainedAllocs {
		a.DesiredTransition.Migrate = nil
		a.DesiredStatus = structs.AllocDesiredStatusStop
		a.ClientStatus = structs.AllocClientStatusComplete

		replacement := newAlloc(runningNode, a.Job)
		updates = append(updates, a, replacement)
		replacements[i] = replacement.Copy()
	}
	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, index, updates))
	index++

	assertJobWatcherOps(t, jobWatcher, 0, 4)

	for _, a := range replacements {
		a.ClientStatus = structs.AllocClientStatusRunning
		a.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: pointer.Of(true),
		}
	}
	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, index, replacements))

	// No jobs should be left!
	must.MapEmpty(t, jobWatcher.drainingJobs())
}

// TestDrainingJobWatcher_HandleTaskGroup tests that the watcher handles
// allocation updates as expected.
func TestDrainingJobWatcher_HandleTaskGroup(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name        string
		batch       bool // use a batch job
		allocCount  int  // number of allocs in test (defaults to 10)
		maxParallel int  // max_parallel (defaults to 1)

		// addAllocFn will be called allocCount times to create test allocs,
		// and the allocs default to be healthy on the draining node
		addAllocFn func(idx int, a *structs.Allocation, drainingID, runningID string)

		expectDrained  int
		expectMigrated int
		expectDone     bool
	}{
		{
			// all allocs on draining node, should respect max_parallel=1
			name:           "drain-respects-max-parallel-1",
			expectDrained:  1,
			expectMigrated: 0,
			expectDone:     false,
		},
		{
			// allocs on a non-draining node, should not be drained
			name:           "allocs-on-non-draining-node-should-not-drain",
			expectDrained:  0,
			expectMigrated: 0,
			expectDone:     true,
			addAllocFn: func(i int, a *structs.Allocation, drainingID, runningID string) {
				a.NodeID = runningID
			},
		},
		{
			// even unhealthy allocs on a non-draining node should not be drained
			name:           "unhealthy-allocs-on-non-draining-node-should-not-drain",
			expectDrained:  0,
			expectMigrated: 0,
			expectDone:     false,
			addAllocFn: func(i int, a *structs.Allocation, drainingID, runningID string) {
				if i%2 == 0 {
					a.NodeID = runningID
					a.DeploymentStatus = nil
				}
			},
		},
		{
			// only the alloc on draining node should be drained
			name:           "healthy-alloc-draining-node-should-drain",
			expectDrained:  1,
			expectMigrated: 0,
			expectDone:     false,
			addAllocFn: func(i int, a *structs.Allocation, drainingID, runningID string) {
				if i != 0 {
					a.NodeID = runningID
				}
			},
		},
		{
			// alloc that's still draining doesn't produce more result updates
			name:           "still-draining-alloc-no-new-updates",
			expectDrained:  0,
			expectMigrated: 0,
			expectDone:     false,
			addAllocFn: func(i int, a *structs.Allocation, drainingID, runningID string) {
				if i == 0 {
					a.DesiredTransition.Migrate = pointer.Of(true)
					return
				}
				a.NodeID = runningID
			},
		},
		{
			// alloc that's finished draining gets marked as migrated
			name:           "client-terminal-alloc-drain-should-be-finished",
			expectDrained:  0,
			expectMigrated: 1,
			expectDone:     true,
			addAllocFn: func(i int, a *structs.Allocation, drainingID, runningID string) {
				if i == 0 {
					a.DesiredStatus = structs.AllocDesiredStatusStop
					a.ClientStatus = structs.AllocClientStatusComplete
					return
				}
				a.NodeID = runningID
			},
		},
		{
			// batch alloc that's finished draining gets marked as migrated
			name:           "client-terminal-batch-alloc-drain-should-be-finished",
			batch:          true,
			expectDrained:  0,
			expectMigrated: 1,
			expectDone:     true,
			addAllocFn: func(i int, a *structs.Allocation, drainingID, runningID string) {
				if i == 0 {
					a.DesiredStatus = structs.AllocDesiredStatusStop
					a.ClientStatus = structs.AllocClientStatusComplete
					return
				}
				a.NodeID = runningID
			},
		},
		{
			// all allocs are client-terminal, so nothing left to drain
			name:           "all-client-terminal-drain-should-be-finished",
			expectDrained:  0,
			expectMigrated: 10,
			expectDone:     true,
			addAllocFn: func(i int, a *structs.Allocation, drainingID, runningID string) {
				a.DesiredStatus = structs.AllocDesiredStatusStop
				a.ClientStatus = structs.AllocClientStatusComplete
			},
		},
		{
			// all allocs are terminal, but only half are client-terminal
			name:           "half-client-terminal-drain-should-not-be-finished",
			expectDrained:  0,
			expectMigrated: 5,
			expectDone:     false,
			addAllocFn: func(i int, a *structs.Allocation, drainingID, runningID string) {
				a.DesiredStatus = structs.AllocDesiredStatusStop
				if i%2 == 0 {
					a.ClientStatus = structs.AllocClientStatusComplete
				}
			},
		},
		{
			// All allocs are terminal, nothing to be drained
			name:           "all-terminal-batch",
			batch:          true,
			expectDrained:  0,
			expectMigrated: 10,
			expectDone:     true,
			addAllocFn: func(i int, a *structs.Allocation, drainingID, runningID string) {
				a.DesiredStatus = structs.AllocDesiredStatusStop
				a.ClientStatus = structs.AllocClientStatusComplete
			},
		},
		{
			// with max_parallel=10, all allocs can be drained at once
			name:           "drain-respects-max-parallel-all-at-once",
			expectDrained:  10,
			expectMigrated: 0,
			expectDone:     false,
			maxParallel:    10,
		},
		{
			// with max_parallel=2, up to 2 allocs can be drained at a time
			name:           "drain-respects-max-parallel-2",
			expectDrained:  2,
			expectMigrated: 0,
			expectDone:     false,
			maxParallel:    2,
		},
		{
			// with max_parallel=2, up to 2 allocs can be drained at a time but
			// we haven't yet informed the drainer that 1 has completed
			// migrating
			name:           "notify-migrated-1-on-new-1-drained-1-draining",
			expectDrained:  1,
			expectMigrated: 1,
			maxParallel:    2,
			addAllocFn: func(i int, a *structs.Allocation, drainingID, runningID string) {
				switch i {
				case 0:
					// One alloc on running node
					a.NodeID = runningID
				case 1:
					// One alloc already migrated
					a.DesiredStatus = structs.AllocDesiredStatusStop
					a.ClientStatus = structs.AllocClientStatusComplete
				}
			},
		},
		{
			// with max_parallel=2, up to 2 allocs can be drained at a time but
			// we haven't yet informed the drainer that 1 has completed
			// migrating
			name:           "notify-migrated-8-on-new-1-drained-1-draining",
			expectDrained:  1,
			expectMigrated: 1,
			maxParallel:    2,
			addAllocFn: func(i int, a *structs.Allocation, drainingID, runningID string) {
				switch i {
				case 0, 1, 2, 3, 4, 5, 6, 7:
					a.NodeID = runningID
				case 8:
					a.DesiredStatus = structs.AllocDesiredStatusStop
					a.ClientStatus = structs.AllocClientStatusComplete
				}
			},
		},
		{
			// 5 on new node, two drained, and three draining
			// with max_parallel=5, up to 5 allocs can be drained at a time but
			// we haven't yet informed the drainer that 2 have completed
			// migrating
			name:           "notify-migrated-5-on-new-2-drained-3-draining",
			expectDrained:  3,
			expectMigrated: 2,
			maxParallel:    5,
			addAllocFn: func(i int, a *structs.Allocation, drainingID, runningID string) {
				switch i {
				case 0, 1, 2, 3, 4:
					a.NodeID = runningID
				case 8, 9:
					a.DesiredStatus = structs.AllocDesiredStatusStop
					a.ClientStatus = structs.AllocClientStatusComplete
				}
			},
		},
		{
			// half the allocs have been moved to the new node but 1 doesn't
			// have health set yet, so we should have MaxParallel - 1 in flight
			name:           "pending-health-blocks",
			expectDrained:  1,
			expectMigrated: 1,
			maxParallel:    3,
			addAllocFn: func(i int, a *structs.Allocation, drainingID, runningID string) {
				switch i {
				case 0:
					// Deployment status UNset for 1 on new node
					a.NodeID = runningID
					a.DeploymentStatus = nil
				case 1, 2, 3, 4:
					// Deployment status set for 4 on new node
					a.NodeID = runningID
				case 9:
					a.DesiredStatus = structs.AllocDesiredStatusStop
					a.ClientStatus = structs.AllocClientStatusComplete
				}
			},
		},
		{
			// half the allocs have been moved to the new node but 2 don't have
			// health set yet, so we should have MaxParallel - 2 in flight
			name:           "pending-health-blocks-higher-max",
			expectDrained:  2,
			expectMigrated: 1,
			maxParallel:    5,
			addAllocFn: func(i int, a *structs.Allocation, drainingID, runningID string) {
				switch i {
				case 0, 1:
					// Deployment status UNset for 2 on new node
					a.NodeID = runningID
					a.DeploymentStatus = nil
				case 2, 3, 4:
					// Deployment status set for 3 on new node
					a.NodeID = runningID
				case 9:
					a.DesiredStatus = structs.AllocDesiredStatusStop
					a.ClientStatus = structs.AllocClientStatusComplete
				}
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)

			// Create nodes
			store := state.TestStateStore(t)
			drainingNode, runningNode := testNodes(t, store)

			job := mock.Job()
			if tc.batch {
				job = mock.BatchJob()
			}
			job.TaskGroups[0].Count = 10
			if tc.allocCount > 0 {
				job.TaskGroups[0].Count = tc.allocCount
			}
			if tc.maxParallel > 0 {
				job.TaskGroups[0].Migrate.MaxParallel = tc.maxParallel
			}
			must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 102, nil, job))

			var allocs []*structs.Allocation
			for i := 0; i < 10; i++ {
				a := mock.Alloc()
				if tc.batch {
					a = mock.BatchAlloc()
				}
				a.JobID = job.ID
				a.Job = job
				a.TaskGroup = job.TaskGroups[0].Name

				// Default to being healthy on the draining node
				a.NodeID = drainingNode.ID
				a.DeploymentStatus = &structs.AllocDeploymentStatus{
					Healthy: pointer.Of(true),
				}
				if tc.addAllocFn != nil {
					tc.addAllocFn(i, a, drainingNode.ID, runningNode.ID)
				}
				allocs = append(allocs, a)
			}

			must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, 103, allocs))
			snap, err := store.Snapshot()
			must.NoError(t, err)

			res := newJobResult()
			must.NoError(t, handleTaskGroup(snap, tc.batch, job.TaskGroups[0], allocs, 102, res))
			test.Len(t, tc.expectDrained, res.drain, test.Sprint("expected drained allocs"))
			test.Len(t, tc.expectMigrated, res.migrated, test.Sprint("expected migrated allocs"))
			test.Eq(t, tc.expectDone, res.done)
		})
	}
}

func TestHandleTaskGroup_Migrations(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create a draining node
	state := state.TestStateStore(t)
	n := mock.Node()
	n.DrainStrategy = &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: 5 * time.Minute,
		},
		ForceDeadline: time.Now().Add(1 * time.Minute),
	}
	require.Nil(state.UpsertNode(structs.MsgTypeTestSetup, 100, n))

	job := mock.Job()
	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 101, nil, job))

	// Create 10 done allocs
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		a := mock.Alloc()
		a.Job = job
		a.TaskGroup = job.TaskGroups[0].Name
		a.NodeID = n.ID
		a.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: pointer.Of(false),
		}

		if i%2 == 0 {
			a.DesiredStatus = structs.AllocDesiredStatusStop
			a.ClientStatus = structs.AllocClientStatusComplete
		} else {
			a.ClientStatus = structs.AllocClientStatusFailed
		}
		allocs = append(allocs, a)
	}
	require.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 102, allocs))

	snap, err := state.Snapshot()
	require.Nil(err)

	// Handle before and after indexes as both service and batch
	res := newJobResult()
	require.Nil(handleTaskGroup(snap, false, job.TaskGroups[0], allocs, 101, res))
	require.Empty(res.drain)
	require.Len(res.migrated, 10)
	require.True(res.done)

	res = newJobResult()
	require.Nil(handleTaskGroup(snap, true, job.TaskGroups[0], allocs, 101, res))
	require.Empty(res.drain)
	require.Len(res.migrated, 10)
	require.True(res.done)

	res = newJobResult()
	require.Nil(handleTaskGroup(snap, false, job.TaskGroups[0], allocs, 103, res))
	require.Empty(res.drain)
	require.Empty(res.migrated)
	require.True(res.done)

	res = newJobResult()
	require.Nil(handleTaskGroup(snap, true, job.TaskGroups[0], allocs, 103, res))
	require.Empty(res.drain)
	require.Empty(res.migrated)
	require.True(res.done)
}

// This test asserts that handle task group works when an allocation is on a
// garbage collected node
func TestHandleTaskGroup_GarbageCollectedNode(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Create a draining node
	state := state.TestStateStore(t)
	n := mock.Node()
	n.DrainStrategy = &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: 5 * time.Minute,
		},
		ForceDeadline: time.Now().Add(1 * time.Minute),
	}
	require.Nil(state.UpsertNode(structs.MsgTypeTestSetup, 100, n))

	job := mock.Job()
	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 101, nil, job))

	// Create 10 done allocs
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		a := mock.Alloc()
		a.Job = job
		a.TaskGroup = job.TaskGroups[0].Name
		a.NodeID = n.ID
		a.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: pointer.Of(false),
		}

		if i%2 == 0 {
			a.DesiredStatus = structs.AllocDesiredStatusStop
			a.ClientStatus = structs.AllocClientStatusComplete
		} else {
			a.ClientStatus = structs.AllocClientStatusFailed
		}
		allocs = append(allocs, a)
	}

	// Make the first one be on a GC'd node
	allocs[0].NodeID = uuid.Generate()
	require.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 102, allocs))

	snap, err := state.Snapshot()
	require.Nil(err)

	// Handle before and after indexes as both service and batch
	res := newJobResult()
	require.Nil(handleTaskGroup(snap, false, job.TaskGroups[0], allocs, 101, res))
	require.Empty(res.drain)
	require.Len(res.migrated, 9)
	require.True(res.done)

	res = newJobResult()
	require.Nil(handleTaskGroup(snap, true, job.TaskGroups[0], allocs, 101, res))
	require.Empty(res.drain)
	require.Len(res.migrated, 9)
	require.True(res.done)

	res = newJobResult()
	require.Nil(handleTaskGroup(snap, false, job.TaskGroups[0], allocs, 103, res))
	require.Empty(res.drain)
	require.Empty(res.migrated)
	require.True(res.done)

	res = newJobResult()
	require.Nil(handleTaskGroup(snap, true, job.TaskGroups[0], allocs, 103, res))
	require.Empty(res.drain)
	require.Empty(res.migrated)
	require.True(res.done)
}
