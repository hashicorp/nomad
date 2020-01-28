package hostvolumes

import (
	"time"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

type BasicHostVolumeTest struct {
	framework.TC
	jobIds []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Host Volumes",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(BasicHostVolumeTest),
		},
	})
}

func (tc *BasicHostVolumeTest) BeforeAll(f *framework.F) {
	// Ensure cluster has leader before running tests
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	// Ensure that we have at least 1 client nodes in ready state
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *BasicHostVolumeTest) TestSingleHostVolume(f *framework.F) {
	require := require.New(f.T())

	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	jobID := "hostvol" + uuid[0:8]
	tc.jobIds = append(tc.jobIds, jobID)
	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, "hostvolumes/input/single_mount.nomad", jobID, "")

	waitForTaskState := func(desiredState string) {
		require.Eventually(func() bool {
			allocs, _, _ := nomadClient.Jobs().Allocations(jobID, false, nil)
			if len(allocs) != 1 {
				return false
			}
			first := allocs[0]
			taskState := first.TaskStates["test"]
			if taskState == nil {
				return false
			}
			return taskState.State == desiredState
		}, 30*time.Second, 1*time.Second)
	}

	waitForClientAllocStatus := func(desiredStatus string) {
		require.Eventually(func() bool {
			allocSummaries, _, _ := nomadClient.Jobs().Allocations(jobID, false, nil)
			if len(allocSummaries) != 1 {
				return false
			}

			alloc, _, _ := nomadClient.Allocations().Info(allocSummaries[0].ID, nil)
			if alloc == nil {
				return false
			}

			return alloc.ClientStatus == desiredStatus
		}, 30*time.Second, 1*time.Second)
	}

	waitForRestartCount := func(desiredCount uint64) {
		require.Eventually(func() bool {
			allocs, _, _ := nomadClient.Jobs().Allocations(jobID, false, nil)
			if len(allocs) != 1 {
				return false
			}
			first := allocs[0]
			return first.TaskStates["test"].Restarts == desiredCount
		}, 30*time.Second, 1*time.Second)
	}

	// Verify scheduling
	for _, allocStub := range allocs {
		node, _, err := nomadClient.Nodes().Info(allocStub.NodeID, nil)
		require.Nil(err)

		_, ok := node.HostVolumes["shared_data"]
		require.True(ok, "Node does not have the requested volume")
	}

	// Wrap in retry to wait until running
	waitForTaskState(structs.TaskStateRunning)

	// Client should be running
	waitForClientAllocStatus(structs.AllocClientStatusRunning)

	// Should not be restarted
	waitForRestartCount(0)

	// Ensure allocs can be restarted
	for _, allocStub := range allocs {
		alloc, _, err := nomadClient.Allocations().Info(allocStub.ID, nil)
		require.Nil(err)

		err = nomadClient.Allocations().Restart(alloc, "", nil)
		require.Nil(err)
	}

	// Should be restarted once
	waitForRestartCount(1)

	// Wrap in retry to wait until running again
	waitForTaskState(structs.TaskStateRunning)

	// Client should be running again
	waitForClientAllocStatus(structs.AllocClientStatusRunning)
}

func (tc *BasicHostVolumeTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	jobs := nomadClient.Jobs()
	// Stop all jobs in test
	for _, id := range tc.jobIds {
		jobs.Deregister(id, true, nil)
	}
	// Garbage collect
	nomadClient.System().GarbageCollect()
}
