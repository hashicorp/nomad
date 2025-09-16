// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler_system

import (
	"testing"
	"time"

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
	t.Run("testCanaryUpdate", testCanaryUpdate)
}

func testJobUpdateOnIneligbleNode(t *testing.T) {
	job, cleanup := jobs3.Submit(t,
		"./input/system_job0.nomad",
		jobs3.WaitComplete("group"),
		jobs3.DisableRandomJobID(),
	)
	t.Cleanup(cleanup)

	allocs := job.Allocs()
	must.True(t, len(allocs) >= 3)

	// Mark one node as ineligible
	nodesAPI := job.NodesApi()
	disabledNodeID := allocs[0].NodeID
	_, err := nodesAPI.ToggleEligibility(disabledNodeID, false, nil)
	must.NoError(t, err)

	// make sure to mark all nodes as eligible once we're done
	t.Cleanup(func() {
		nodes, _, err := nodesAPI.List(nil)
		must.NoError(t, err)
		for _, n := range nodes {
			_, err := nodesAPI.ToggleEligibility(n.ID, true, nil)
			must.NoError(t, err)
		}
	})

	// Assert all jobs still running
	allocs = job.Allocs()
	must.SliceNotEmpty(t, allocs)

	allocForDisabledNode := make(map[string]*api.AllocationListStub)

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
	job2, cleanup2 := jobs3.Submit(t,
		"./input/system_job1.nomad",
		jobs3.WaitComplete("group"),
		jobs3.DisableRandomJobID(),
	)
	t.Cleanup(cleanup2)

	// Get updated allocations
	allocs = job2.Allocs()
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

func testCanaryUpdate(t *testing.T) {
	_, cleanup := jobs3.Submit(t,
		"./input/system_canary_v0.nomad.hcl",
		jobs3.WaitComplete("group"),
		jobs3.DisableRandomJobID(),
	)
	t.Cleanup(cleanup)

	// Update job
	job2, cleanup2 := jobs3.Submit(t,
		"./input/system_canary_v1.nomad.hcl",
		jobs3.DisableRandomJobID(),
		jobs3.Detach(),
	)
	t.Cleanup(cleanup2)

	// Get updated allocations
	allocs := job2.Allocs()
	must.SliceNotEmpty(t, allocs)

	deploymentsApi := job2.DeploymentsApi()

	// wait for the canary allocations to become healthy
	timeout := time.After(30 * time.Second)
CANARYWAIT:
	for {
		select {
		case <-timeout:
			must.Unreachable(t, must.Sprint("timeout reached waiting for healthy status of canary allocs"))
		default:
		}

		deployments, _, err := deploymentsApi.List(nil)
		must.NoError(t, err)
		for _, d := range deployments {
			for _, tg := range d.TaskGroups { // this job has 1 tg
				if d.JobVersion == 1 && tg.HealthyAllocs >= tg.DesiredCanaries {
					break CANARYWAIT
				}
			}
		}
	}

	// filter allocation from v1 version of the job, they should all be canaries
	// and there should be exactly 2
	canaryAllocs := []*api.AllocationListStub{}
	for _, a := range allocs {
		if a.JobVersion == 1 {
			must.True(t, a.DeploymentStatus.Canary)
			canaryAllocs = append(canaryAllocs, a)
		}
	}
	must.SliceLen(t, 2, canaryAllocs, must.Sprint("expected 2 canary allocs"))

	// promote canaries
	deployments, _, err := deploymentsApi.List(nil)
	must.NoError(t, err)
	must.SliceLen(t, 2, deployments)
	_, _, err = deploymentsApi.PromoteAll(deployments[0].ID, nil)
	must.NoError(t, err)

	// wait for the promotions to become healthy
PROMOTIONWAIT:
	for {
		select {
		case <-timeout:
			must.Unreachable(t, must.Sprint("timeout reached waiting for healthy status of promoted allocs"))
		default:
		}

		deploymentsApi := job2.DeploymentsApi()
		deployments, _, err := deploymentsApi.List(nil)
		must.NoError(t, err)
		for _, d := range deployments {
			for _, tg := range d.TaskGroups { // this job has 1 tg
				if d.JobVersion == 1 && tg.HealthyAllocs >= tg.DesiredTotal {
					break PROMOTIONWAIT
				}
			}
		}
	}

	// expect 4 allocations for job version 1, none of them canary
	newAllocs := job2.Allocs()
	must.SliceNotEmpty(t, newAllocs)

	promotedAllocs := []*api.AllocationListStub{}
	for _, a := range newAllocs {
		if a.JobVersion == 1 {
			promotedAllocs = append(promotedAllocs, a)
		}
	}
	must.SliceLen(t, 4, promotedAllocs, must.Sprint("expected 4 allocations for promoted job"))
}
