// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func TestAllocations_List(t *testing.T) {
	testutil.RequireRoot(t)
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	a := c.Allocations()

	// wait for node
	_ = oneNodeFromNodeList(t, c.Nodes())

	// Querying when no allocs exist returns nothing
	allocs, qm, err := a.List(nil)
	must.NoError(t, err)
	must.Zero(t, qm.LastIndex)
	must.Len(t, 0, allocs)

	// Create a job and attempt to register it
	job := testJob()
	resp, wm, err := c.Jobs().Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp)
	must.UUIDv4(t, resp.EvalID)
	assertWriteMeta(t, wm)

	// List the allocations again
	qo := &QueryOptions{
		WaitIndex: wm.LastIndex,
	}
	allocs, qm, err = a.List(qo)
	must.NoError(t, err)
	must.NonZero(t, qm.LastIndex)

	// Check that we got the allocation back
	must.Len(t, 1, allocs)
	must.Eq(t, resp.EvalID, allocs[0].EvalID)

	// Resources should be unset by default
	must.Nil(t, allocs[0].AllocatedResources)
}

func TestAllocations_PrefixList(t *testing.T) {
	testutil.RequireRoot(t)
	testutil.Parallel(t)

	testAPIClient, testServer := makeClient(t, nil, func(c *testutil.TestServerConfig) { c.DevMode = true })
	t.Cleanup(testServer.Stop)
	_ = oneNodeFromNodeList(t, testAPIClient.Nodes())

	// Querying when no allocs exist returns nothing
	allocs, qm, err := testAPIClient.Allocations().PrefixList("")
	must.NoError(t, err)
	must.Zero(t, qm.LastIndex)
	must.Len(t, 0, allocs)

	// Create a job and register it
	job := testJob()
	resp, wm, err := testAPIClient.Jobs().Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp)
	must.UUIDv4(t, resp.EvalID)
	assertWriteMeta(t, wm)

	// Get a list of the job allocations, so we have data to move onto prefix
	// matching.
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(
			func() error {
				allocs, _, err := testAPIClient.Jobs().Allocations(*job.ID, false, nil)
				if err != nil {
					return err
				}
				if len(allocs) != 1 {
					return errors.New("waiting for job allocations")
				}
				return nil
			},
		),
		wait.Timeout(5*time.Second),
		wait.Gap(100*time.Millisecond),
	))
	jobAllocs, _, err := testAPIClient.Jobs().Allocations(*job.ID, false, nil)
	must.NoError(t, err)
	must.Len(t, 1, jobAllocs)

	// Perform a test of prefix matching by using the first 4 characters which
	// should be more than enoigh to be unique and give a consistent result.
	allocs, _, err = testAPIClient.Allocations().PrefixList(jobAllocs[0].ID[:4])
	must.NoError(t, err)
	must.Len(t, 1, allocs)
	must.Eq(t, jobAllocs[0].ID, allocs[0].ID)

	// Test a prefix that does not match anything.
	allocs, _, err = testAPIClient.Allocations().PrefixList(generateUUID())
	must.NoError(t, err)
	must.Len(t, 0, allocs)
}

func TestAllocations_List_Resources(t *testing.T) {
	testutil.RequireRoot(t)
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	a := c.Allocations()

	// wait for node
	_ = oneNodeFromNodeList(t, c.Nodes())

	// Create a job and register it
	job := testJob()
	resp, wm, err := c.Jobs().Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp)
	must.UUIDv4(t, resp.EvalID)
	assertWriteMeta(t, wm)

	qo := &QueryOptions{
		Params:    map[string]string{"resources": "true"},
		WaitIndex: wm.LastIndex,
	}
	var allocationStubs []*AllocationListStub
	var qm *QueryMeta
	allocationStubs, qm, err = a.List(qo)
	must.NoError(t, err)

	// Check that we got the allocation back with resources
	must.Positive(t, qm.LastIndex)
	must.Len(t, 1, allocationStubs)
	alloc := allocationStubs[0]
	must.Eq(t, resp.EvalID, alloc.EvalID,
		must.Sprintf("registration: %#v", resp),
		must.Sprintf("allocation:   %#v", alloc),
	)
	must.NotNil(t, alloc.AllocatedResources)
}

func TestAllocations_CreateIndexSort(t *testing.T) {
	testutil.Parallel(t)

	allocs := []*AllocationListStub{
		{CreateIndex: 2},
		{CreateIndex: 1},
		{CreateIndex: 5},
	}
	sort.Sort(AllocIndexSort(allocs))

	expect := []*AllocationListStub{
		{CreateIndex: 5},
		{CreateIndex: 2},
		{CreateIndex: 1},
	}
	must.Eq(t, allocs, expect)
}

func TestAllocations_Info(t *testing.T) {
	testutil.RequireRoot(t)
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	a := c.Allocations()

	// wait for node
	_ = oneNodeFromNodeList(t, c.Nodes())

	// Create a job and attempt to register it
	job := testJob()
	resp, wm, err := c.Jobs().Register(job, nil)
	must.NoError(t, err)
	must.NotNil(t, resp)
	must.UUIDv4(t, resp.EvalID)
	assertWriteMeta(t, wm)

	// List allocations.
	qo := &QueryOptions{
		WaitIndex: wm.LastIndex,
	}
	allocs, qm, err := a.List(qo)
	must.NoError(t, err)
	must.NonZero(t, qm.LastIndex)

	// Check that we got one allocation.
	must.Len(t, 1, allocs)
	must.Eq(t, resp.EvalID, allocs[0].EvalID)

	// Fetch alloc info.
	qo.WaitIndex = qm.LastIndex
	alloc, _, err := a.Info(allocs[0].ID, qo)

	must.NotNil(t, alloc.NetworkStatus)
}

func TestAllocations_RescheduleInfo(t *testing.T) {
	testutil.Parallel(t)

	// Create a job, task group and alloc
	job := &Job{
		Name:      pointerOf("foo"),
		Namespace: pointerOf(DefaultNamespace),
		ID:        pointerOf("bar"),
		ParentID:  pointerOf("lol"),
		TaskGroups: []*TaskGroup{
			{
				Name: pointerOf("bar"),
				Tasks: []*Task{
					{
						Name: "task1",
					},
				},
			},
		},
	}
	job.Canonicalize()

	alloc := &Allocation{
		ID:        generateUUID(),
		Namespace: DefaultNamespace,
		EvalID:    generateUUID(),
		Name:      "foo-bar[1]",
		NodeID:    generateUUID(),
		TaskGroup: *job.TaskGroups[0].Name,
		JobID:     *job.ID,
		Job:       job,
	}

	type testCase struct {
		desc              string
		reschedulePolicy  *ReschedulePolicy
		rescheduleTracker *RescheduleTracker
		time              time.Time
		expAttempted      int
		expTotal          int
	}

	testCases := []testCase{
		{
			desc:         "no reschedule policy",
			expAttempted: 0,
			expTotal:     0,
		},
		{
			desc: "no reschedule events",
			reschedulePolicy: &ReschedulePolicy{
				Attempts: pointerOf(3),
				Interval: pointerOf(15 * time.Minute),
			},
			expAttempted: 0,
			expTotal:     3,
		},
		{
			desc: "all reschedule events within interval",
			reschedulePolicy: &ReschedulePolicy{
				Attempts: pointerOf(3),
				Interval: pointerOf(15 * time.Minute),
			},
			time: time.Now(),
			rescheduleTracker: &RescheduleTracker{
				Events: []*RescheduleEvent{
					{
						RescheduleTime: time.Now().Add(-5 * time.Minute).UTC().UnixNano(),
					},
				},
			},
			expAttempted: 1,
			expTotal:     3,
		},
		{
			desc: "some reschedule events outside interval",
			reschedulePolicy: &ReschedulePolicy{
				Attempts: pointerOf(3),
				Interval: pointerOf(15 * time.Minute),
			},
			time: time.Now(),
			rescheduleTracker: &RescheduleTracker{
				Events: []*RescheduleEvent{
					{
						RescheduleTime: time.Now().Add(-45 * time.Minute).UTC().UnixNano(),
					},
					{
						RescheduleTime: time.Now().Add(-30 * time.Minute).UTC().UnixNano(),
					},
					{
						RescheduleTime: time.Now().Add(-10 * time.Minute).UTC().UnixNano(),
					},
					{
						RescheduleTime: time.Now().Add(-5 * time.Minute).UTC().UnixNano(),
					},
				},
			},
			expAttempted: 2,
			expTotal:     3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			alloc.RescheduleTracker = tc.rescheduleTracker
			job.TaskGroups[0].ReschedulePolicy = tc.reschedulePolicy
			attempted, total := alloc.RescheduleInfo(tc.time)
			must.Eq(t, tc.expAttempted, attempted)
			must.Eq(t, tc.expTotal, total)
		})
	}

}

func TestAllocations_Stop(t *testing.T) {
	testutil.RequireRoot(t)
	testutil.Parallel(t)

	t.Run("default", func(t *testing.T) {
		c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
			c.DevMode = true
		})
		defer s.Stop()
		a := c.Allocations()

		// wait for node
		_ = oneNodeFromNodeList(t, c.Nodes())

		// Create a job and register it
		job := testJob()
		_, wm, err := c.Jobs().Register(job, nil)
		must.NoError(t, err)

		// List allocations.
		stubs, qm, err := a.List(&QueryOptions{WaitIndex: wm.LastIndex})
		must.NoError(t, err)
		must.SliceLen(t, 1, stubs)

		// Stop the first allocation.
		resp, err := a.Stop(&Allocation{ID: stubs[0].ID}, &QueryOptions{WaitIndex: qm.LastIndex})
		must.NoError(t, err)
		test.UUIDv4(t, resp.EvalID)
		test.NonZero(t, resp.LastIndex)

		// Stop allocation that doesn't exist.
		resp, err = a.Stop(&Allocation{ID: "invalid"}, &QueryOptions{WaitIndex: qm.LastIndex})
		must.Error(t, err)
	})

	t.Run("rescheduled", func(t *testing.T) {
		c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
			c.DevMode = true
		})
		defer s.Stop()
		a := c.Allocations()

		// wait for node
		_ = oneNodeFromNodeList(t, c.Nodes())

		// Create a job and register it
		job := testJob()
		_, wm, err := c.Jobs().Register(job, nil)
		must.NoError(t, err)

		// List allocations.
		stubs, qm, err := a.List(&QueryOptions{WaitIndex: wm.LastIndex})
		must.NoError(t, err)
		must.SliceLen(t, 1, stubs)

		// Stop the first allocation.
		resp, err := a.Stop(&Allocation{ID: stubs[0].ID}, &QueryOptions{
			Params:    map[string]string{"reschedule": "true"},
			WaitIndex: qm.LastIndex,
		})
		must.NoError(t, err)
		alloc, _, err := a.Info(stubs[0].ID, &QueryOptions{WaitIndex: resp.LastIndex})
		must.NoError(t, err)
		must.True(t, alloc.DesiredTransition.ShouldReschedule(), must.Sprint("allocation should be marked for rescheduling"))
	})

	t.Run("no shutdown delay", func(t *testing.T) {
		c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
			c.DevMode = true
		})
		defer s.Stop()
		a := c.Allocations()

		// wait for node
		_ = oneNodeFromNodeList(t, c.Nodes())

		// Create a job and register it
		job := testJob()
		_, wm, err := c.Jobs().Register(job, nil)
		must.NoError(t, err)

		// List allocations.
		stubs, qm, err := a.List(&QueryOptions{WaitIndex: wm.LastIndex})
		must.NoError(t, err)
		must.SliceLen(t, 1, stubs)

		// Stop the first allocation.
		resp, err := a.Stop(&Allocation{ID: stubs[0].ID}, &QueryOptions{
			Params:    map[string]string{"no_shutdown_delay": "true"},
			WaitIndex: qm.LastIndex,
		})
		must.NoError(t, err)
		alloc, _, err := a.Info(stubs[0].ID, &QueryOptions{WaitIndex: resp.LastIndex})
		must.NoError(t, err)
		must.True(t, alloc.DesiredTransition.ShouldIgnoreShutdownDelay(), must.Sprint("allocation should be marked for no shutdown delay"))
	})
}

// TestAllocations_ExecErrors ensures errors are properly formatted
func TestAllocations_ExecErrors(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Allocations()

	job := &Job{
		Name:      pointerOf("foo"),
		Namespace: pointerOf(DefaultNamespace),
		ID:        pointerOf("bar"),
		ParentID:  pointerOf("lol"),
		TaskGroups: []*TaskGroup{
			{
				Name: pointerOf("bar"),
				Tasks: []*Task{
					{
						Name: "task1",
					},
				},
			},
		},
	}
	job.Canonicalize()

	allocID := generateUUID()

	alloc := &Allocation{
		ID:        allocID,
		Namespace: DefaultNamespace,
		EvalID:    generateUUID(),
		Name:      "foo-bar[1]",
		NodeID:    generateUUID(),
		TaskGroup: *job.TaskGroups[0].Name,
		JobID:     *job.ID,
		Job:       job,
	}
	// Querying when no allocs exist returns nothing
	sizeCh := make(chan TerminalSize, 1)

	// make a request that will result in an error
	// ensure the error is what we expect
	exitCode, err := a.Exec(context.Background(), alloc, "bar", false, []string{"command"}, os.Stdin, os.Stdout, os.Stderr, sizeCh, nil)

	must.Eq(t, -2, exitCode)
	must.EqError(t, err, fmt.Sprintf("Unknown allocation \"%s\"", allocID))
}

func TestAllocation_ServerTerminalStatus(t *testing.T) {
	testutil.Parallel(t)

	testCases := []struct {
		inputAllocation *Allocation
		expectedOutput  bool
		name            string
	}{
		{
			inputAllocation: &Allocation{DesiredStatus: AllocDesiredStatusEvict},
			expectedOutput:  true,
			name:            "alloc desired status evict",
		},
		{
			inputAllocation: &Allocation{DesiredStatus: AllocDesiredStatusStop},
			expectedOutput:  true,
			name:            "alloc desired status stop",
		},
		{
			inputAllocation: &Allocation{DesiredStatus: AllocDesiredStatusRun},
			expectedOutput:  false,
			name:            "alloc desired status run",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expectedOutput, tc.inputAllocation.ServerTerminalStatus())
		})
	}
}

func TestAllocation_ClientTerminalStatus(t *testing.T) {
	testutil.Parallel(t)

	testCases := []struct {
		inputAllocation *Allocation
		expectedOutput  bool
		name            string
	}{
		{
			inputAllocation: &Allocation{ClientStatus: AllocClientStatusLost},
			expectedOutput:  true,
			name:            "alloc client status lost",
		},
		{
			inputAllocation: &Allocation{ClientStatus: AllocClientStatusFailed},
			expectedOutput:  true,
			name:            "alloc client status failed",
		},
		{
			inputAllocation: &Allocation{ClientStatus: AllocClientStatusComplete},
			expectedOutput:  true,
			name:            "alloc client status complete",
		},
		{
			inputAllocation: &Allocation{ClientStatus: AllocClientStatusRunning},
			expectedOutput:  false,
			name:            "alloc client status complete",
		},
		{
			inputAllocation: &Allocation{ClientStatus: AllocClientStatusPending},
			expectedOutput:  false,
			name:            "alloc client status running",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expectedOutput, tc.inputAllocation.ClientTerminalStatus())
		})
	}
}

func TestAllocations_ShouldMigrate(t *testing.T) {
	testutil.Parallel(t)

	must.True(t, DesiredTransition{Migrate: pointerOf(true)}.ShouldMigrate())
	must.False(t, DesiredTransition{}.ShouldMigrate())
	must.False(t, DesiredTransition{Migrate: pointerOf(false)}.ShouldMigrate())
}

func TestAllocations_ShouldReschedule(t *testing.T) {
	testutil.Parallel(t)

	must.True(t, DesiredTransition{Reschedule: pointerOf(true)}.ShouldReschedule())
	must.False(t, DesiredTransition{}.ShouldReschedule())
	must.False(t, DesiredTransition{Reschedule: pointerOf(false)}.ShouldReschedule())
}

func TestAllocations_ShouldForceReschedule(t *testing.T) {
	testutil.Parallel(t)

	must.True(t, DesiredTransition{ForceReschedule: pointerOf(true)}.ShouldForceReschedule())
	must.False(t, DesiredTransition{}.ShouldForceReschedule())
	must.False(t, DesiredTransition{ForceReschedule: pointerOf(false)}.ShouldForceReschedule())
}

func TestAllocations_ShouldIgnoreShutdownDelay(t *testing.T) {
	testutil.Parallel(t)

	must.True(t, DesiredTransition{NoShutdownDelay: pointerOf(true)}.ShouldIgnoreShutdownDelay())
	must.False(t, DesiredTransition{}.ShouldIgnoreShutdownDelay())
	must.False(t, DesiredTransition{NoShutdownDelay: pointerOf(false)}.ShouldIgnoreShutdownDelay())
}

func TestAllocations_ShouldDisableMigrationPlacement(t *testing.T) {
	testutil.Parallel(t)

	must.True(t, DesiredTransition{MigrateDisablePlacement: pointerOf(true)}.ShouldDisableMigrationPlacement())
	must.False(t, DesiredTransition{}.ShouldDisableMigrationPlacement())
	must.False(t, DesiredTransition{MigrateDisablePlacement: pointerOf(false)}.ShouldDisableMigrationPlacement())
}

func TestAllocations_Services(t *testing.T) {
	t.Skip("needs to be implemented")
	// TODO(jrasell) add tests once registration process is in place.
}
