// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler_system

import (
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

type SystemSchedTest struct {
	framework.TC
	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "SystemScheduler",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(SystemSchedTest),
		},
	})
}

func (tc *SystemSchedTest) BeforeAll(f *framework.F) {
	// Ensure cluster has leader before running tests
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 4)
}

func (tc *SystemSchedTest) TestJobUpdateOnIneligbleNode(f *framework.F) {
	t := f.T()
	nomadClient := tc.Nomad()

	jobID := "system_deployment"
	tc.jobIDs = append(tc.jobIDs, jobID)
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "scheduler_system/input/system_job0.nomad", jobID, "")

	jobs := nomadClient.Jobs()
	allocs, _, err := jobs.Allocations(jobID, true, nil)
	require.NoError(t, err)
	require.True(t, len(allocs) >= 3)

	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)

	// Wait for allocations to get past initial pending state
	e2eutil.WaitForAllocsNotPending(t, nomadClient, allocIDs)

	// Mark one node as ineligible
	nodesAPI := tc.Nomad().Nodes()
	disabledNodeID := allocs[0].NodeID
	_, err = nodesAPI.ToggleEligibility(disabledNodeID, false, nil)
	require.NoError(t, err)

	// Assert all jobs still running
	jobs = nomadClient.Jobs()
	allocs, _, err = jobs.Allocations(jobID, true, nil)
	require.NoError(t, err)

	allocIDs = e2eutil.AllocIDsFromAllocationListStubs(allocs)
	allocForDisabledNode := make(map[string]*api.AllocationListStub)

	// Wait for allocs to run and collect allocs on ineligible node
	// Allocation could have failed, ensure there is one thats running
	// and that it is the correct version (0)
	e2eutil.WaitForAllocsNotPending(t, nomadClient, allocIDs)
	for _, alloc := range allocs {
		if alloc.NodeID == disabledNodeID {
			allocForDisabledNode[alloc.ID] = alloc
		}
	}

	// Filter down to only our latest running alloc
	for _, alloc := range allocForDisabledNode {
		require.Equal(t, uint64(0), alloc.JobVersion)
		if alloc.ClientStatus == structs.AllocClientStatusComplete {
			// remove the old complete alloc from map
			delete(allocForDisabledNode, alloc.ID)
		}
	}
	require.NotEmpty(t, allocForDisabledNode)
	require.Len(t, allocForDisabledNode, 1)

	// Update job
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "scheduler_system/input/system_job1.nomad", jobID, "")

	// Get updated allocations
	jobs = nomadClient.Jobs()
	allocs, _, err = jobs.Allocations(jobID, false, nil)
	require.NoError(t, err)

	// Wait for allocs to start
	allocIDs = e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsNotPending(t, nomadClient, allocIDs)

	// Get latest alloc status now that they are no longer pending
	allocs, _, err = jobs.Allocations(jobID, false, nil)
	require.NoError(t, err)

	var foundPreviousAlloc bool
	for _, dAlloc := range allocForDisabledNode {
		for _, alloc := range allocs {
			if alloc.ID == dAlloc.ID {
				foundPreviousAlloc = true
				require.Equal(t, uint64(0), alloc.JobVersion)
			} else if alloc.ClientStatus == structs.AllocClientStatusRunning {
				// Ensure allocs running on non disabled node are
				// newer version
				require.Equal(t, uint64(1), alloc.JobVersion)
			}
		}
	}
	require.True(t, foundPreviousAlloc, "unable to find previous alloc for ineligible node")
}

func (tc *SystemSchedTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()

	// Mark all nodes eligible
	nodesAPI := tc.Nomad().Nodes()
	nodes, _, _ := nodesAPI.List(nil)
	for _, node := range nodes {
		nodesAPI.ToggleEligibility(node.ID, true, nil)
	}

	jobs := nomadClient.Jobs()
	// Stop all jobs in test
	for _, id := range tc.jobIDs {
		jobs.Deregister(id, true, nil)
	}
	tc.jobIDs = []string{}
	// Garbage collect
	nomadClient.System().GarbageCollect()
}
