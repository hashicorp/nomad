// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler_system

import (
	"context"
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
		jobs3.DisableRandomJobID(),
		jobs3.Timeout(60*time.Second),
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

	// Update job
	job2, cleanup2 := jobs3.Submit(t,
		"./input/system_job1.nomad",
		jobs3.DisableRandomJobID(),
		jobs3.Timeout(60*time.Second),
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
		jobs3.DisableRandomJobID(),
		jobs3.Timeout(60*time.Second),
	)
	t.Cleanup(cleanup)

	// Update job
	job2, cleanup2 := jobs3.Submit(t,
		"./input/system_canary_v1.nomad.hcl",
		jobs3.DisableRandomJobID(),
		jobs3.Timeout(60*time.Second),
		jobs3.Detach(),
	)
	t.Cleanup(cleanup2)

	// how many eligible nodes do we have?
	nodesApi := job2.NodesApi()
	nodesList, _, err := nodesApi.List(nil)
	must.Nil(t, err)
	must.SliceNotEmpty(t, nodesList)

	numberOfEligibleNodes := 0
	for _, n := range nodesList {
		if n.SchedulingEligibility == api.NodeSchedulingEligible {
			numberOfEligibleNodes += 1
		}
	}

	// Get updated allocations
	allocs := job2.Allocs()
	must.SliceNotEmpty(t, allocs)

	deploymentsApi := job2.DeploymentsApi()
	deploymentsList, _, err := deploymentsApi.List(nil)
	must.NoError(t, err)

	var deployment *api.Deployment
	for _, d := range deploymentsList {
		if d.JobID == job2.JobID() && d.Status == api.DeploymentStatusRunning {
			deployment = d
		}
	}
	must.NotNil(t, deployment)

	// wait for the canary allocations to become healthy
	timeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	job2.WaitForDeploymentFunc(timeout, deployment.ID, func(d *api.Deployment) bool {
		for _, tg := range d.TaskGroups { // we only have 1 tg in this job
			if d.JobVersion == 1 && tg.HealthyAllocs >= tg.DesiredCanaries {
				return true
			}
		}
		return false
	})

	// find allocations from v1 version of the job, they should all be canaries
	// and there should be exactly 2
	count := 0
	for _, a := range allocs {
		if a.JobVersion == 1 {
			must.True(t, a.DeploymentStatus.Canary)
			count += 1
		}
	}
	must.Eq(t, numberOfEligibleNodes/2, count, must.Sprint("expected canaries to be placed on 50% of eligible nodes"))

	// promote canaries
	deployments, _, err := deploymentsApi.List(nil)
	must.NoError(t, err)
	must.SliceLen(t, 2, deployments)
	_, _, err = deploymentsApi.PromoteAll(deployments[0].ID, nil)
	must.NoError(t, err)

	// promoting canaries on a system job should result in a new deployment
	deploymentsList, _, err = deploymentsApi.List(nil)
	must.NoError(t, err)

	for _, d := range deploymentsList {
		if d.JobID == job2.JobID() && d.Status == api.DeploymentStatusRunning {
			deployment = d
			break
		}
	}
	must.NotNil(t, deployment)

	// wait for the promotions to become healthy
	job2.WaitForDeploymentFunc(timeout, deployment.ID, func(d *api.Deployment) bool {
		for _, tg := range d.TaskGroups { // we only have 1 tg in this job
			if d.JobVersion == 1 && tg.HealthyAllocs >= tg.DesiredTotal {
				return true
			}
		}
		return false
	})

	// expect the number of allocations for promoted deployment to be the same
	// as the number of eligible nodes
	newAllocs := job2.Allocs()
	must.SliceNotEmpty(t, newAllocs)

	promotedAllocs := 0
	for _, a := range newAllocs {
		if a.JobVersion == 1 {
			promotedAllocs += 1
		}
	}
	must.Eq(t, numberOfEligibleNodes, promotedAllocs)
}
