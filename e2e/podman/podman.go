// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package podman

import (
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

type PodmanTest struct {
	framework.TC
	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Podman",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(PodmanTest),
		},
	})
}

func (tc *PodmanTest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 2)
}

func (tc *PodmanTest) TestRedisDeployment(f *framework.F) {
	t := f.T()
	// https://github.com/hashicorp/nomad-driver-podman/issues/57
	t.Skip("skipping podman test until driver api issue is resolved")
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	jobID := "deployment" + uuid[0:8]
	tc.jobIDs = append(tc.jobIDs, jobID)
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "podman/input/redis.nomad", jobID, "")
	ds := e2eutil.DeploymentsForJob(t, nomadClient, jobID)
	require.Equal(t, 1, len(ds))

	jobs := nomadClient.Jobs()
	allocs, _, err := jobs.Allocations(jobID, true, nil)
	require.NoError(t, err)

	var allocIDs []string
	for _, alloc := range allocs {
		allocIDs = append(allocIDs, alloc.ID)
	}

	// Wait for allocations to get past initial pending state
	e2eutil.WaitForAllocsNotPending(t, nomadClient, allocIDs)

	jobs = nomadClient.Jobs()
	allocs, _, err = jobs.Allocations(jobID, true, nil)
	require.NoError(t, err)

	require.Len(t, allocs, 1)
	require.Equal(t, allocs[0].ClientStatus, "running")
}

func (tc *PodmanTest) AfterEach(f *framework.F) {
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
