// +build ent

package scheduler

import (
	"fmt"
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
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
