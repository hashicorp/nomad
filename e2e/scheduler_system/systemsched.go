// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler_system

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/v3/cluster3"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
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
		"./input/system_job0.hcl",
		jobs3.WaitComplete("group"),
	)
	t.Cleanup(cleanup)

	allocs := job.Allocs()
	must.True(t, len(allocs) >= 3)

	// Mark one node as ineligible
	nodesAPI := job.NodesApi()
	disabledNodeID := allocs[0].NodeID
	_, err := nodesAPI.ToggleEligibility(disabledNodeID, false, nil)
	must.NoError(t, err)

	// Assert all jobs still running
	allocs = job.Allocs()
	must.SliceNotEmpty(t, allocs)

	allocForDisabledNode := make(map[string]*api.AllocationListStub)

	// Wait for allocs to run and collect allocs on ineligible node
	// Allocation could have failed, ensure there is one thats running
	// and that it is the correct version (0)

	// e2eutil.WaitForAllocsNotPending(t, nomadClient, allocIDs)
	for _, alloc := range allocs {
		if alloc.NodeID == disabledNodeID {
			allocForDisabledNode[alloc.ID] = alloc
		}
	}

	// Filter down to only our latest running alloc
	for _, alloc := range allocForDisabledNode {
		must.Eq(t, uint64(0), alloc.JobVersion)
		if alloc.ClientStatus == structs.AllocClientStatusComplete {
			// remove the old complete alloc from map
			delete(allocForDisabledNode, alloc.ID)
		}
	}
	must.MapNotEmpty(t, allocForDisabledNode)
	must.MapLen(t, 1, allocForDisabledNode)

	// Update job
	job, cleanup = jobs3.Submit(t,
		"./input/system_job1.hcl",
		jobs3.WaitComplete("group"),
	)

	// Get updated allocations
	allocs = job.Allocs()
	must.SliceNotEmpty(t, allocs)

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
