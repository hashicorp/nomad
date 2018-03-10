package drainer

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
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
	require.Nil(t, state.UpsertNode(100, n1))

	// Create a non-draining node
	n2 := mock.Node()
	n2.Name = "running"
	require.Nil(t, state.UpsertNode(101, n2))
	return n1, n2
}

func testDrainingJobWatcher(t *testing.T, state *state.StateStore) *drainingJobWatcher {
	t.Helper()

	limiter := rate.NewLimiter(100.0, 100)
	logger := testlog.Logger(t)
	w := NewDrainingJobWatcher(context.Background(), limiter, state, logger)
	return w
}

// TestDrainingJobWatcher_Interface is a compile-time assertion that we
// implement the intended interface.
func TestDrainingJobWatcher_Interface(t *testing.T) {
	var _ DrainingJobWatcher = testDrainingJobWatcher(t, state.TestStateStore(t))
}

	t.Parallel()
	require := require.New(t)

	state := state.TestStateStore(t)


		job := mock.Job()

		var allocs []*structs.Allocation
		for i := 0; i < 10; i++ {
			a := mock.Alloc()
			a.Job = job
			a.TaskGroup = job.TaskGroups[0].Name
			a.DeploymentStatus = &structs.AllocDeploymentStatus{
			}
			allocs = append(allocs, a)
		}


	}
// handleTaskGroupTestCase is the test case struct for TestHandleTaskGroup
//
// Two nodes will be initialized: one draining and one running.
type handleTaskGroupTestCase struct {
	// Name of test
	Name string

	// Expectations
	ExpectedDrained  int
	ExpectedMigrated int
	ExpectedDone     bool

	// Count overrides the default count of 10 if set
	Count int

	// MaxParallel overrides the default max_parallel of 1 if set
	MaxParallel int

	// AddAlloc will be called 10 times to create test allocs
	//
	// Allocs default to be healthy on the draining node
	AddAlloc func(i int, a *structs.Allocation, drainingID, runningID string)
}

func TestHandeTaskGroup_Table(t *testing.T) {
	cases := []handleTaskGroupTestCase{
		{
			// All allocs on draining node
			Name:             "AllDraining",
			ExpectedDrained:  1,
			ExpectedMigrated: 0,
			ExpectedDone:     false,
		},
		{
			// All allocs on non-draining node
			Name:             "AllNonDraining",
			ExpectedDrained:  0,
			ExpectedMigrated: 0,
			ExpectedDone:     true,
			AddAlloc: func(i int, a *structs.Allocation, drainingID, runningID string) {
				a.NodeID = runningID
			},
		},
		{
			// Some allocs on non-draining node but not healthy
			Name:             "SomeNonDrainingUnhealthy",
			ExpectedDrained:  0,
			ExpectedMigrated: 0,
			ExpectedDone:     false,
			AddAlloc: func(i int, a *structs.Allocation, drainingID, runningID string) {
				if i%2 == 0 {
					a.NodeID = runningID
					a.DeploymentStatus = nil
				}
			},
		},
		{
			// One draining, other allocs on non-draining node and healthy
			Name:             "OneDraining",
			ExpectedDrained:  1,
			ExpectedMigrated: 0,
			ExpectedDone:     false,
			AddAlloc: func(i int, a *structs.Allocation, drainingID, runningID string) {
				if i != 0 {
					a.NodeID = runningID
				}
			},
		},
		{
			// One already draining, other allocs on non-draining node and healthy
			Name:             "OneAlreadyDraining",
			ExpectedDrained:  0,
			ExpectedMigrated: 0,
			ExpectedDone:     false,
			AddAlloc: func(i int, a *structs.Allocation, drainingID, runningID string) {
				if i == 0 {
					a.DesiredTransition.Migrate = helper.BoolToPtr(true)
					return
				}
				a.NodeID = runningID
			},
		},
		{
			// One already drained, other allocs on non-draining node and healthy
			Name:             "OneAlreadyDrained",
			ExpectedDrained:  0,
			ExpectedMigrated: 1,
			ExpectedDone:     true,
			AddAlloc: func(i int, a *structs.Allocation, drainingID, runningID string) {
				if i == 0 {
					a.DesiredStatus = structs.AllocDesiredStatusStop
					return
				}
				a.NodeID = runningID
			},
		},
		{
			// All allocs are terminl, nothing to be drained
			Name:             "AllMigrating",
			ExpectedDrained:  0,
			ExpectedMigrated: 10,
			ExpectedDone:     true,
			AddAlloc: func(i int, a *structs.Allocation, drainingID, runningID string) {
				a.DesiredStatus = structs.AllocDesiredStatusStop
			},
		},
		{
			// All allocs may be drained at once
			Name:             "AllAtOnce",
			ExpectedDrained:  10,
			ExpectedMigrated: 0,
			ExpectedDone:     false,
			MaxParallel:      10,
		},
		{
			// Drain 2
			Name:             "Drain2",
			ExpectedDrained:  2,
			ExpectedMigrated: 0,
			ExpectedDone:     false,
			MaxParallel:      2,
		},
		{
			// One on new node, one drained, and one draining
			ExpectedDrained:  1,
			ExpectedMigrated: 1,
			MaxParallel:      2,
			AddAlloc: func(i int, a *structs.Allocation, drainingID, runningID string) {
				switch i {
				case 0:
					// One alloc on running node
					a.NodeID = runningID
				case 1:
					// One alloc already migrated
					a.DesiredStatus = structs.AllocDesiredStatusStop
				}
			},
		},
		{
			// 8 on new node, one drained, and one draining
			ExpectedDrained:  1,
			ExpectedMigrated: 1,
			MaxParallel:      2,
			AddAlloc: func(i int, a *structs.Allocation, drainingID, runningID string) {
				switch i {
				case 0, 1, 2, 3, 4, 5, 6, 7:
					a.NodeID = runningID
				case 8:
					a.DesiredStatus = structs.AllocDesiredStatusStop
				}
			},
		},
		{
			// 5 on new node, two drained, and three draining
			ExpectedDrained:  3,
			ExpectedMigrated: 2,
			MaxParallel:      5,
			AddAlloc: func(i int, a *structs.Allocation, drainingID, runningID string) {
				switch i {
				case 0, 1, 2, 3, 4:
					a.NodeID = runningID
				case 8, 9:
					a.DesiredStatus = structs.AllocDesiredStatusStop
				}
			},
		},
		{
			// Not all on new node have health set
			Name:             "PendingHealth",
			ExpectedDrained:  1,
			ExpectedMigrated: 1,
			MaxParallel:      3,
			AddAlloc: func(i int, a *structs.Allocation, drainingID, runningID string) {
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
				}
			},
		},
		{
			// 5 max parallel - 1 migrating - 2 with unset health = 2 drainable
			Name:             "PendingHealthHigherMax",
			ExpectedDrained:  2,
			ExpectedMigrated: 1,
			MaxParallel:      5,
			AddAlloc: func(i int, a *structs.Allocation, drainingID, runningID string) {
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
				}
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			testHandleTaskGroup(t, testCase)
		})
	}
}

func testHandleTaskGroup(t *testing.T, tc handleTaskGroupTestCase) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	// Create nodes
	state := state.TestStateStore(t)
	drainingNode, runningNode := testNodes(t, state)

	job := mock.Job()
	job.TaskGroups[0].Count = 10
	if tc.Count > 0 {
		job.TaskGroups[0].Count = tc.Count
	}
	if tc.MaxParallel > 0 {
		job.TaskGroups[0].Migrate.MaxParallel = tc.MaxParallel
	}
	require.Nil(state.UpsertJob(102, job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		a := mock.Alloc()
		a.JobID = job.ID
		a.Job = job
		a.TaskGroup = job.TaskGroups[0].Name

		// Default to being healthy on the draining node
		a.NodeID = drainingNode.ID
		a.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: helper.BoolToPtr(true),
		}
		if tc.AddAlloc != nil {
			tc.AddAlloc(i, a, drainingNode.ID, runningNode.ID)
		}
		allocs = append(allocs, a)
	}

	require.Nil(state.UpsertAllocs(103, allocs))
	snap, err := state.Snapshot()
	require.Nil(err)

	res := newJobResult()
	require.Nil(handleTaskGroup(snap, job.TaskGroups[0], allocs, 102, res))
	assert.Lenf(res.drain, tc.ExpectedDrained, "Drain expected %d but found: %d",
		tc.ExpectedDrained, len(res.drain))
	assert.Lenf(res.migrated, tc.ExpectedMigrated, "Migrate expected %d but found: %d",
		tc.ExpectedMigrated, len(res.migrated))
	assert.Equal(tc.ExpectedDone, res.done)
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
