package systemsched

import (
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

type SystemSchedTest struct {
	framework.TC
	jobIDs         []string
	disabledNodeID string
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
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "systemsched/input/system_job0.nomad", jobID, "")

	jobs := nomadClient.Jobs()
	allocs, _, err := jobs.Allocations(jobID, true, nil)

	// Mark one node as ineligible
	nodesAPI := tc.Nomad().Nodes()
	disabledNodeID := allocs[0].NodeID
	_, err = nodesAPI.ToggleEligibility(disabledNodeID, false, nil)
	require.NoError(t, err)

	// Assert all jobs still running
	jobs = nomadClient.Jobs()
	allocs, _, err = jobs.Allocations(jobID, true, nil)
	require.NoError(t, err)
	var disabledAlloc *api.AllocationListStub

	for _, alloc := range allocs {
		// Ensure alloc is either running or complete
		testutil.WaitForResultRetries(30, func() (bool, error) {
			time.Sleep(time.Millisecond * 100)
			alloc, _, err := nomadClient.Allocations().Info(alloc.ID, nil)
			if err != nil {
				return false, err
			}

			return (alloc.ClientStatus == structs.AllocClientStatusRunning ||
					alloc.ClientStatus == structs.AllocClientStatusComplete),
				fmt.Errorf("expected status running, but was: %s", alloc.ClientStatus)
		}, func(err error) {
			t.Fatalf("failed to wait on alloc: %v", err)
		})
		if alloc.NodeID == disabledNodeID {
			require.Equal(t, "run", alloc.DesiredStatus)
			disabledAlloc = alloc
		}
	}

	require.NotNil(t, disabledAlloc)

	// Update job
	spew.Dump("DREW UPDATE JOB")
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "systemsched/input/system_job1.nomad", jobID, "")

	// Get updated allocations
	jobs = nomadClient.Jobs()
	allocs, _, err = jobs.Allocations(jobID, false, nil)
	require.NoError(t, err)
	var allocIDs []string
	for _, alloc := range allocs {
		allocIDs = append(allocIDs, alloc.ID)
	}

	e2eutil.WaitForAllocsRunning(t, nomadClient, allocIDs)

	allocs, _, err = jobs.Allocations(jobID, false, nil)
	require.NoError(t, err)

	// Ensure disabled node alloc is still version 0
	var foundPreviousAlloc bool
	for _, alloc := range allocs {
		spew.Dump(alloc)
		if alloc.ID == disabledAlloc.ID {
			foundPreviousAlloc = true
			require.Equal(t, uint64(0), alloc.JobVersion)
		} else {
			spew.Dump(alloc)
			require.Equal(t, uint64(1), alloc.JobVersion)
		}
		require.Equal(t, "run", alloc.DesiredStatus)
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
