package drainer

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func testDrainingJobWatcher(t *testing.T) (*drainingJobWatcher, *state.StateStore) {
	t.Helper()

	state := state.TestStateStore(t)
	limiter := rate.NewLimiter(100.0, 100)
	logger := testlog.Logger(t)
	w := NewDrainingJobWatcher(context.Background(), limiter, state, logger)
	return w, state
}

func TestDrainingJobWatcher_Interface(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	w, _ := testDrainingJobWatcher(t)
	require.Implements((*DrainingJobWatcher)(nil), w)
}

// DrainingJobWatcher tests:
// TODO Test that several jobs allocation changes get batched
// TODO Test that jobs are deregistered when they have no more to migrate
// TODO Test that the watcher gets triggered on alloc changes
// TODO Test that the watcher cancels its query when a new job is registered

func TestHandleTaskGroup_AllDone(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Create a non-draining node
	state := state.TestStateStore(t)
	n := mock.Node()
	require.Nil(state.UpsertNode(100, n))

	job := mock.Job()
	require.Nil(state.UpsertJob(101, job))

	// Create 10 running allocs on the healthy node
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		a := mock.Alloc()
		a.Job = job
		a.TaskGroup = job.TaskGroups[0].Name
		a.NodeID = n.ID
		a.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: helper.BoolToPtr(false),
		}
		allocs = append(allocs, a)
	}
	require.Nil(state.UpsertAllocs(102, allocs))

	snap, err := state.Snapshot()
	require.Nil(err)

	res := &jobResult{}
	require.Nil(handleTaskGroup(snap, job.TaskGroups[0], allocs, 101, res))
	require.Empty(res.drain)
	require.Empty(res.migrated)
	require.True(res.done)
}

func TestHandleTaskGroup_AllOnDrainingNodes(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// The loop value sets the max parallel for the drain strategy
	for i := 1; i < 8; i++ {
		// Create a draining node
		state := state.TestStateStore(t)
		n := mock.Node()
		n.DrainStrategy = &structs.DrainStrategy{
			DrainSpec: structs.DrainSpec{
				Deadline: 5 * time.Minute,
			},
			ForceDeadline: time.Now().Add(1 * time.Minute),
		}
		require.Nil(state.UpsertNode(100, n))

		job := mock.Job()
		job.TaskGroups[0].Migrate.MaxParallel = i
		require.Nil(state.UpsertJob(101, job))

		// Create 10 running allocs on the draining node
		var allocs []*structs.Allocation
		for i := 0; i < 10; i++ {
			a := mock.Alloc()
			a.Job = job
			a.TaskGroup = job.TaskGroups[0].Name
			a.NodeID = n.ID
			a.DeploymentStatus = &structs.AllocDeploymentStatus{
				Healthy: helper.BoolToPtr(false),
			}
			allocs = append(allocs, a)
		}
		require.Nil(state.UpsertAllocs(102, allocs))

		snap, err := state.Snapshot()
		require.Nil(err)

		res := &jobResult{}
		require.Nil(handleTaskGroup(snap, job.TaskGroups[0], allocs, 101, res))
		require.Len(res.drain, i)
		require.Empty(res.migrated)
		require.False(res.done)
	}
}

func TestHandleTaskGroup_MixedHealth(t *testing.T) {
	cases := []struct {
		maxParallel        int
		drainingNodeAllocs int
		healthSet          int
		healthUnset        int
		expectedDrain      int
		expectedMigrated   int
		expectedDone       bool
	}{
		{
			maxParallel:        2,
			drainingNodeAllocs: 10,
			healthSet:          0,
			healthUnset:        0,
			expectedDrain:      2,
			expectedMigrated:   0,
			expectedDone:       false,
		},
		{
			maxParallel:        2,
			drainingNodeAllocs: 9,
			healthSet:          0,
			healthUnset:        0,
			expectedDrain:      1,
			expectedMigrated:   1,
			expectedDone:       false,
		},
		{
			maxParallel:        5,
			drainingNodeAllocs: 9,
			healthSet:          0,
			healthUnset:        0,
			expectedDrain:      4,
			expectedMigrated:   1,
			expectedDone:       false,
		},
		{
			maxParallel:        2,
			drainingNodeAllocs: 5,
			healthSet:          2,
			healthUnset:        0,
			expectedDrain:      0,
			expectedMigrated:   5,
			expectedDone:       false,
		},
		{
			maxParallel:        2,
			drainingNodeAllocs: 5,
			healthSet:          3,
			healthUnset:        0,
			expectedDrain:      0,
			expectedMigrated:   5,
			expectedDone:       false,
		},
		{
			maxParallel:        2,
			drainingNodeAllocs: 5,
			healthSet:          4,
			healthUnset:        0,
			expectedDrain:      1,
			expectedMigrated:   5,
			expectedDone:       false,
		},
		{
			maxParallel:        2,
			drainingNodeAllocs: 5,
			healthSet:          4,
			healthUnset:        1,
			expectedDrain:      1,
			expectedMigrated:   5,
			expectedDone:       false,
		},
		{
			maxParallel:        1,
			drainingNodeAllocs: 5,
			healthSet:          4,
			healthUnset:        1,
			expectedDrain:      0,
			expectedMigrated:   5,
			expectedDone:       false,
		},
		{
			maxParallel:        3,
			drainingNodeAllocs: 5,
			healthSet:          3,
			healthUnset:        0,
			expectedDrain:      1,
			expectedMigrated:   5,
			expectedDone:       false,
		},
		{
			maxParallel:        3,
			drainingNodeAllocs: 0,
			healthSet:          10,
			healthUnset:        0,
			expectedDrain:      0,
			expectedMigrated:   10,
			expectedDone:       true,
		},
		{
			// Is the case where deadline is hit and all 10 are just marked
			// stopped. We should detect the job as done.
			maxParallel:        3,
			drainingNodeAllocs: 0,
			healthSet:          0,
			healthUnset:        0,
			expectedDrain:      0,
			expectedMigrated:   10,
			expectedDone:       true,
		},
	}

	for cnum, c := range cases {
		t.Run(fmt.Sprintf("%d", cnum), func(t *testing.T) {
			require := require.New(t)

			// Create a draining node
			state := state.TestStateStore(t)

			drainingNode := mock.Node()
			drainingNode.DrainStrategy = &structs.DrainStrategy{
				DrainSpec: structs.DrainSpec{
					Deadline: 5 * time.Minute,
				},
				ForceDeadline: time.Now().Add(1 * time.Minute),
			}
			require.Nil(state.UpsertNode(100, drainingNode))

			healthyNode := mock.Node()
			require.Nil(state.UpsertNode(101, healthyNode))

			job := mock.Job()
			job.TaskGroups[0].Migrate.MaxParallel = c.maxParallel
			require.Nil(state.UpsertJob(101, job))

			// Create running allocs on the draining node with health set
			var allocs []*structs.Allocation
			for i := 0; i < c.drainingNodeAllocs; i++ {
				a := mock.Alloc()
				a.Job = job
				a.TaskGroup = job.TaskGroups[0].Name
				a.NodeID = drainingNode.ID
				a.DeploymentStatus = &structs.AllocDeploymentStatus{
					Healthy: helper.BoolToPtr(false),
				}
				allocs = append(allocs, a)
			}

			// Create stopped allocs on the draining node
			for i := 10 - c.drainingNodeAllocs; i > 0; i-- {
				a := mock.Alloc()
				a.Job = job
				a.TaskGroup = job.TaskGroups[0].Name
				a.NodeID = drainingNode.ID
				a.DeploymentStatus = &structs.AllocDeploymentStatus{
					Healthy: helper.BoolToPtr(false),
				}
				a.DesiredStatus = structs.AllocDesiredStatusStop
				allocs = append(allocs, a)
			}

			// Create allocs on the healthy node with health set
			for i := 0; i < c.healthSet; i++ {
				a := mock.Alloc()
				a.Job = job
				a.TaskGroup = job.TaskGroups[0].Name
				a.NodeID = healthyNode.ID
				a.DeploymentStatus = &structs.AllocDeploymentStatus{
					Healthy: helper.BoolToPtr(false),
				}
				allocs = append(allocs, a)
			}

			// Create allocs on the healthy node with health not set
			for i := 0; i < c.healthUnset; i++ {
				a := mock.Alloc()
				a.Job = job
				a.TaskGroup = job.TaskGroups[0].Name
				a.NodeID = healthyNode.ID
				allocs = append(allocs, a)
			}
			require.Nil(state.UpsertAllocs(103, allocs))

			snap, err := state.Snapshot()
			require.Nil(err)

			res := &jobResult{}
			require.Nil(handleTaskGroup(snap, job.TaskGroups[0], allocs, 101, res))
			require.Len(res.drain, c.expectedDrain)
			require.Len(res.migrated, c.expectedMigrated)
			require.Equal(c.expectedDone, res.done)
		})
	}
}

func TestHandleTaskGroup_Migrations(t *testing.T) {
	t.Parallel()
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
	require.Nil(state.UpsertNode(100, n))

	job := mock.Job()
	require.Nil(state.UpsertJob(101, job))

	// Create 10 done allocs
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		a := mock.Alloc()
		a.Job = job
		a.TaskGroup = job.TaskGroups[0].Name
		a.NodeID = n.ID
		a.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: helper.BoolToPtr(false),
		}

		if i%2 == 0 {
			a.DesiredStatus = structs.AllocDesiredStatusStop
		} else {
			a.ClientStatus = structs.AllocClientStatusFailed
		}
		allocs = append(allocs, a)
	}
	require.Nil(state.UpsertAllocs(102, allocs))

	snap, err := state.Snapshot()
	require.Nil(err)

	// Handle before and after indexes
	res := &jobResult{}
	require.Nil(handleTaskGroup(snap, job.TaskGroups[0], allocs, 101, res))
	require.Empty(res.drain)
	require.Len(res.migrated, 10)
	require.True(res.done)

	res = &jobResult{}
	require.Nil(handleTaskGroup(snap, job.TaskGroups[0], allocs, 103, res))
	require.Empty(res.drain)
	require.Empty(res.migrated)
	require.True(res.done)
}
