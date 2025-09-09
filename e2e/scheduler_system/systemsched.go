// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler_system

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/cluster3"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestSystemScheduler(t *testing.T) {
	cluster3.Establish(t,
		cluster3.Leader(),
		cluster3.LinuxClients(3),
	)

	t.Run("testJobUpdateOnIneligibleNode", testJobUpdateOnIneligbleNode)
}

func testJobUpdateOnIneligbleNode(t *testing.T) {
	job, cleanup := jobs3.Submit(t,
		"./input/secrets.hcl",
		jobs3.WaitComplete("group"),
	)
	t.Cleanup(cleanup)

	job.ID := "system_deployment"
	tc.jobIDs = append(tc.jobIDs, jobID)
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "scheduler_system/input/system_job0.nomad", jobID, "")

	jobs := nomadClient.Jobs()
	allocs, _, err := jobs.Allocations(jobID, true, nil)
	must.NoError(t, err)
	must.True(t, len(allocs) >= 3)

	allocIDs := e2eutil.AllocIDsFromAllocationListStubs(allocs)

	// Wait for allocations to get past initial pending state
	e2eutil.WaitForAllocsNotPending(t, nomadClient, allocIDs)

	// Mark one node as ineligible
	nodesAPI := tc.Nomad().Nodes()
	disabledNodeID := allocs[0].NodeID
	_, err = nodesAPI.ToggleEligibility(disabledNodeID, false, nil)
	must.NoError(t, err)

	// Assert all jobs still running
	jobs = nomadClient.Jobs()
	allocs, _, err = jobs.Allocations(jobID, true, nil)
	must.NoError(t, err)

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
	must.MapNotEmpty(t, allocForDisabledNode)
	must.MapLen(t, 1, allocForDisabledNode)

	// Update job
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "scheduler_system/input/system_job1.nomad", jobID, "")

	// Get updated allocations
	jobs = nomadClient.Jobs()
	allocs, _, err = jobs.Allocations(jobID, false, nil)
	must.NoError(t, err)

	// Wait for allocs to start
	allocIDs = e2eutil.AllocIDsFromAllocationListStubs(allocs)
	e2eutil.WaitForAllocsNotPending(t, nomadClient, allocIDs)

	// Get latest alloc status now that they are no longer pending
	allocs, _, err = jobs.Allocations(jobID, false, nil)
	must.NoError(t, err)

	var foundPreviousAlloc bool
	for _, dAlloc := range allocForDisabledNode {
		for _, alloc := range allocs {
			if alloc.ID == dAlloc.ID {
				foundPreviousAlloc = true
				must.Eq(t, uint64(0), alloc.JobVersion)
			} else if alloc.ClientStatus == structs.AllocClientStatusRunning {
				// Ensure allocs running on non disabled node are
				// newer version
				must.Eq(t, uint64(1), alloc.JobVersion)
			}
		}
	}
	must.True(t, foundPreviousAlloc, must.Sprint("unable to find previous alloc for ineligible node"))
}
