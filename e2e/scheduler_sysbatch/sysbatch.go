// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler_sysbatch

import (
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type SysBatchSchedulerTest struct {
	framework.TC
	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "SysBatchScheduler",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(SysBatchSchedulerTest),
		},
	})
}

func (tc *SysBatchSchedulerTest) BeforeAll(f *framework.F) {
	// Ensure cluster has leader before running tests
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 4)
}

func (tc *SysBatchSchedulerTest) TestJobRunBasic(f *framework.F) {
	t := f.T()
	nomadClient := tc.Nomad()

	// submit a fast sysbatch job
	jobID := "sysbatch_run_basic"
	tc.jobIDs = append(tc.jobIDs, jobID)
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "scheduler_sysbatch/input/sysbatch_job_fast.nomad", jobID, "")

	// get our allocations for this sysbatch job
	jobs := nomadClient.Jobs()
	allocs, _, err := jobs.Allocations(jobID, true, nil)
	require.NoError(t, err)

	// make sure this is job is being run on "all" the linux clients
	require.True(t, len(allocs) >= 3)

	// wait for every alloc to reach completion
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsStatus(t, nomadClient, allocIDs, structs.AllocClientStatusComplete)
}

func (tc *SysBatchSchedulerTest) TestJobStopEarly(f *framework.F) {
	t := f.T()
	nomadClient := tc.Nomad()

	// submit a slow sysbatch job
	jobID := "sysbatch_stop_early"
	tc.jobIDs = append(tc.jobIDs, jobID)
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "scheduler_sysbatch/input/sysbatch_job_slow.nomad", jobID, "")

	// get our allocations for this sysbatch job
	jobs := nomadClient.Jobs()
	allocs, _, err := jobs.Allocations(jobID, true, nil)
	require.NoError(t, err)

	// make sure this is job is being run on "all" the linux clients
	require.True(t, len(allocs) >= 3)

	// wait for every alloc to reach running status
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsStatus(t, nomadClient, allocIDs, structs.AllocClientStatusRunning)

	// stop the job before allocs reach completion
	_, _, err = jobs.Deregister(jobID, false, nil)
	require.NoError(t, err)
}

func (tc *SysBatchSchedulerTest) TestJobReplaceRunning(f *framework.F) {
	t := f.T()
	nomadClient := tc.Nomad()

	// submit a slow sysbatch job
	jobID := "sysbatch_replace_running"
	tc.jobIDs = append(tc.jobIDs, jobID)
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "scheduler_sysbatch/input/sysbatch_job_slow.nomad", jobID, "")

	// get out allocations for this sysbatch job
	jobs := nomadClient.Jobs()
	allocs, _, err := jobs.Allocations(jobID, true, nil)
	require.NoError(t, err)

	// make sure this is job is being run on "all" the linux clients
	require.True(t, len(allocs) >= 3)

	// wait for every alloc to reach running status
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsStatus(t, nomadClient, allocIDs, structs.AllocClientStatusRunning)

	// replace the slow job with the fast job
	intermediate := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "scheduler_sysbatch/input/sysbatch_job_fast.nomad", jobID, "")

	// get the allocs for the new updated job
	var updated []*api.AllocationListStub
	for _, alloc := range intermediate {
		if alloc.JobVersion == 1 {
			updated = append(updated, alloc)
		}
	}

	// should be equal number of old and new allocs
	newAllocIDs := e2eutil.AllocIDsFromAllocationListStubs(updated)

	// make sure this new job is being run on "all" the linux clients
	require.True(t, len(updated) >= 3)

	// wait for the allocs of the fast job to complete
	e2eutil.WaitForAllocsStatus(t, nomadClient, newAllocIDs, structs.AllocClientStatusComplete)
}

func (tc *SysBatchSchedulerTest) TestJobReplaceDead(f *framework.F) {
	t := f.T()
	nomadClient := tc.Nomad()

	// submit a fast sysbatch job
	jobID := "sysbatch_replace_dead"
	tc.jobIDs = append(tc.jobIDs, jobID)
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "scheduler_sysbatch/input/sysbatch_job_fast.nomad", jobID, "")

	// get the allocations for this sysbatch job
	jobs := nomadClient.Jobs()
	allocs, _, err := jobs.Allocations(jobID, true, nil)
	require.NoError(t, err)

	// make sure this is job is being run on "all" the linux clients
	require.True(t, len(allocs) >= 3)

	// wait for every alloc to reach complete status
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsStatus(t, nomadClient, allocIDs, structs.AllocClientStatusComplete)

	// replace the fast job with the slow job
	intermediate := e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "scheduler_sysbatch/input/sysbatch_job_slow.nomad", jobID, "")

	// get the allocs for the new updated job
	var updated []*api.AllocationListStub
	for _, alloc := range intermediate {
		if alloc.JobVersion == 1 {
			updated = append(updated, alloc)
		}
	}

	// should be equal number of old and new allocs
	upAllocIDs := e2eutil.AllocIDsFromAllocationListStubs(updated)

	// make sure this new job is being run on "all" the linux clients
	require.True(t, len(updated) >= 3)

	// wait for the allocs of the slow job to be running
	e2eutil.WaitForAllocsStatus(t, nomadClient, upAllocIDs, structs.AllocClientStatusRunning)
}

func (tc *SysBatchSchedulerTest) TestJobRunPeriodic(f *framework.F) {
	t := f.T()
	nomadClient := tc.Nomad()

	// submit a fast sysbatch job
	jobID := "sysbatch_job_periodic"
	tc.jobIDs = append(tc.jobIDs, jobID)
	err := e2eutil.Register(jobID, "scheduler_sysbatch/input/sysbatch_periodic.nomad")
	require.NoError(t, err)

	// force the cron job to run
	jobs := nomadClient.Jobs()
	_, _, err = jobs.PeriodicForce(jobID, nil)
	require.NoError(t, err)

	// find the cron job that got launched
	jobsList, _, err := jobs.List(nil)
	require.NoError(t, err)
	cronJobID := ""
	for _, job := range jobsList {
		if strings.HasPrefix(job.Name, "sysbatch_job_periodic/periodic-") {
			cronJobID = job.Name
			break
		}
	}
	require.NotEmpty(t, cronJobID)
	tc.jobIDs = append(tc.jobIDs, cronJobID)

	// wait for allocs of the cron job
	var allocs []*api.AllocationListStub
	require.True(t, assert.Eventually(t, func() bool {
		var err error
		allocs, _, err = jobs.Allocations(cronJobID, false, nil)
		require.NoError(t, err)
		return len(allocs) >= 3
	}, 30*time.Second, time.Second))

	// wait for every cron job alloc to reach completion
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsStatus(t, nomadClient, allocIDs, structs.AllocClientStatusComplete)
}

func (tc *SysBatchSchedulerTest) TestJobRunDispatch(f *framework.F) {
	t := f.T()
	nomadClient := tc.Nomad()

	// submit a fast sysbatch dispatch job
	jobID := "sysbatch_job_dispatch"
	tc.jobIDs = append(tc.jobIDs, jobID)
	err := e2eutil.Register(jobID, "scheduler_sysbatch/input/sysbatch_dispatch.nomad")
	require.NoError(t, err)

	// dispatch the sysbatch job
	jobs := nomadClient.Jobs()
	result, _, err := jobs.Dispatch(jobID, map[string]string{
		"KEY": "value",
	}, nil, "", nil)
	require.NoError(t, err)

	// grab the new dispatched jobID
	dispatchID := result.DispatchedJobID
	tc.jobIDs = append(tc.jobIDs, dispatchID)

	// wait for allocs of the dispatched job
	var allocs []*api.AllocationListStub
	require.True(t, assert.Eventually(t, func() bool {
		var err error
		allocs, _, err = jobs.Allocations(dispatchID, false, nil)
		require.NoError(t, err)
		return len(allocs) >= 3
	}, 30*time.Second, time.Second))

	// wait for every dispatch alloc to reach completion
	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsStatus(t, nomadClient, allocIDs, structs.AllocClientStatusComplete)
}

func (tc *SysBatchSchedulerTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()

	// Mark all nodes eligible
	nodesAPI := tc.Nomad().Nodes()
	nodes, _, _ := nodesAPI.List(nil)
	for _, node := range nodes {
		_, _ = nodesAPI.ToggleEligibility(node.ID, true, nil)
	}

	jobs := nomadClient.Jobs()

	// Stop all jobs in test
	for _, id := range tc.jobIDs {
		_, _, _ = jobs.Deregister(id, true, nil)
	}
	tc.jobIDs = []string{}

	// Garbage collect
	_ = nomadClient.System().GarbageCollect()
}
