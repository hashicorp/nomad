// +build pro ent

package scheduler

import (
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestServiceSched_Preemption(t *testing.T) {
	require := require.New(t)
	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	node.Resources = nil
	node.ReservedResources = nil
	node.NodeResources = &structs.NodeResources{
		Cpu: structs.NodeCpuResources{
			CpuShares: 1000,
		},
		Memory: structs.NodeMemoryResources{
			MemoryMB: 2048,
		},
		Disk: structs.NodeDiskResources{
			DiskMB: 100 * 1024,
		},
		Networks: []*structs.NetworkResource{
			{
				Device: "eth0",
				CIDR:   "192.168.0.100/32",
				MBits:  1000,
			},
		},
	}
	node.ReservedResources = &structs.NodeReservedResources{
		Cpu: structs.NodeReservedCpuResources{
			CpuShares: 50,
		},
		Memory: structs.NodeReservedMemoryResources{
			MemoryMB: 256,
		},
		Disk: structs.NodeReservedDiskResources{
			DiskMB: 4 * 1024,
		},
		Networks: structs.NodeReservedNetworkResources{
			ReservedHostPorts: "22",
		},
	}
	require.NoError(h.State.UpsertNode(h.NextIndex(), node))

	// Create a couple of jobs and schedule them
	job1 := mock.Job()
	job1.TaskGroups[0].Count = 1
	job1.Priority = 30
	r1 := job1.TaskGroups[0].Tasks[0].Resources
	r1.CPU = 500
	r1.MemoryMB = 1024
	r1.Networks = nil
	require.NoError(h.State.UpsertJob(h.NextIndex(), job1))

	job2 := mock.Job()
	job2.TaskGroups[0].Count = 1
	job2.Priority = 50
	r2 := job2.TaskGroups[0].Tasks[0].Resources
	r2.CPU = 350
	r2.MemoryMB = 512
	r2.Networks = nil
	require.NoError(h.State.UpsertJob(h.NextIndex(), job2))

	// Create a mock evaluation to register the jobs
	eval1 := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job1.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job1.ID,
		Status:      structs.EvalStatusPending,
	}
	eval2 := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job2.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job2.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval1, eval2}))

	expectedPreemptedAllocs := make(map[string]struct{})
	// Process the two evals for job1 and job2 and make sure they allocated
	for index, eval := range []*structs.Evaluation{eval1, eval2} {
		// Process the evaluation
		err := h.Process(NewServiceScheduler, eval)
		require.Nil(err)

		plan := h.Plans[index]

		// Ensure the plan doesn't have annotations.
		require.Nil(plan.Annotations)

		// Ensure the eval has no spawned blocked eval
		require.Equal(0, len(h.CreateEvals))

		// Ensure the plan allocated
		var planned []*structs.Allocation
		for _, allocList := range plan.NodeAllocation {
			planned = append(planned, allocList...)
		}
		require.Equal(1, len(planned))
		expectedPreemptedAllocs[planned[0].ID] = struct{}{}
	}

	// Create a higher priority job
	job3 := mock.Job()
	job3.Priority = 100
	job3.TaskGroups[0].Count = 1
	r3 := job3.TaskGroups[0].Tasks[0].Resources
	r3.CPU = 900
	r3.MemoryMB = 1700
	r3.Networks = nil
	require.NoError(h.State.UpsertJob(h.NextIndex(), job3))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job3.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job3.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	require.Nil(err)

	// New plan should be the third one in the harness
	plan := h.Plans[2]

	// Ensure the eval has no spawned blocked eval
	require.Equal(0, len(h.CreateEvals))

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	require.Equal(1, len(planned))

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job3.Namespace, job3.ID, false)
	require.NoError(err)

	// Ensure all allocations placed
	require.Equal(1, len(out))
	actualPreemptedAllocs := make(map[string]struct{})
	for _, id := range out[0].PreemptedAllocations {
		actualPreemptedAllocs[id] = struct{}{}
	}
	require.Equal(expectedPreemptedAllocs, actualPreemptedAllocs)
}
