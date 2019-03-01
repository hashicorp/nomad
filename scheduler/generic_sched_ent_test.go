// +build pro ent

package scheduler

import (
	"fmt"
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test that scaling up a job in a way that will cause the job to exceed the
// quota limit places the maximum number of allocations
func TestServiceSched_JobModify_IncrCount_QuotaLimit(t *testing.T) {
	assert := assert.New(t)
	h := NewHarness(t)

	// Create the quota spec
	qs := mock.QuotaSpec()
	assert.Nil(h.State.UpsertQuotaSpecs(h.NextIndex(), []*structs.QuotaSpec{qs}))

	// Create the namespace
	ns := mock.Namespace()
	ns.Quota = qs.Name
	assert.Nil(h.State.UpsertNamespaces(h.NextIndex(), []*structs.Namespace{ns}))

	// Create the job with two task groups with slightly different resource
	// requirements
	job := mock.Job()
	job.Namespace = ns.Name
	job.TaskGroups = append(job.TaskGroups, job.TaskGroups[0].Copy())

	job.TaskGroups[0].Count = 2
	r1 := job.TaskGroups[0].Tasks[0].Resources
	r1.CPU = 500
	r1.MemoryMB = 256
	r1.Networks = nil

	// Quota Limit: (2000 CPU, 2000 MB)
	// Total Usage at count 2 : (1000, 512)
	// Should be able to place 4 o
	// Quota would be (2000, 1024)
	assert.Nil(h.State.UpsertJob(h.NextIndex(), job))

	// Create several node
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		nodes = append(nodes, mock.Node())
		assert.Nil(h.State.UpsertNode(h.NextIndex(), nodes[i]))
	}

	var allocs []*structs.Allocation
	for i := 0; i < 2; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Namespace = ns.Name
		alloc.TaskGroup = job.TaskGroups[0].Name
		alloc.Name = fmt.Sprintf("%s.%s[%d]", job.ID, alloc.TaskGroup, i)
		alloc.Resources = r1.Copy()
		alloc.TaskResources = map[string]*structs.Resources{
			"web": r1.Copy(),
		}
		allocs = append(allocs, alloc)
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Update the task group count to 10 each
	job2 := job.Copy()
	job2.TaskGroups[0].Count = 10
	noErr(t, h.State.UpsertJob(h.NextIndex(), job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   ns.Name,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	noErr(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	assert.Nil(h.Process(NewServiceScheduler, eval))

	// Ensure a single plan
	assert.Len(h.Plans, 1)
	plan := h.Plans[0]

	// Ensure the plan didn't evicted the alloc
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	assert.Empty(update)

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	assert.Len(planned, 4)

	// Ensure the plan had a failures
	assert.Len(h.Evals, 1)

	// Ensure the eval has spawned blocked eval
	assert.Len(h.CreateEvals, 1)

	// Ensure that eval says that it was because of a quota limit
	blocked := h.CreateEvals[0]
	assert.Equal(qs.Name, blocked.QuotaLimitReached)

	// Lookup the allocations by JobID and make sure we have the right amount of
	// each type
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	assert.Nil(err)

	assert.Len(out, 4)
	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

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
