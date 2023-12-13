// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
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
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Allocations()

	// Querying when no allocs exist returns nothing
	allocs, qm, err := a.PrefixList("")
	must.NoError(t, err)
	must.Zero(t, qm.LastIndex)
	must.Len(t, 0, allocs)

	// TODO: do something that causes an allocation to actually happen
	// so we can query for them.
	return

	//job := &Job{
	//ID:   stringToPtr("job1"),
	//Name: stringToPtr("Job #1"),
	//Type: stringToPtr(JobTypeService),
	//}

	//eval, _, err := c.Jobs().Register(job, nil)
	//if err != nil {
	//t.Fatalf("err: %s", err)
	//}

	//// List the allocations by prefix
	//allocs, qm, err = a.PrefixList("foobar")
	//if err != nil {
	//t.Fatalf("err: %s", err)
	//}
	//if qm.LastIndex == 0 {
	//t.Fatalf("bad index: %d", qm.LastIndex)
	//}

	//// Check that we got the allocation back
	//if len(allocs) == 0 || allocs[0].EvalID != eval {
	//t.Fatalf("bad: %#v", allocs)
	//}
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

func TestAllocations_Services(t *testing.T) {
	t.Skip("needs to be implemented")
	// TODO(jrasell) add tests once registration process is in place.
}
