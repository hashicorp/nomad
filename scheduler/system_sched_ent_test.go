// +build ent

package scheduler

import (
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

// Tests registering a system job that will exceed the quota limit
func TestSystemSched_JobRegister_QuotaLimit(t *testing.T) {
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

	// Quota Limit: (2000 CPU, 2000 MB)
	// Should be able to place 4
	// Quota would be (2000, 1024)
	assert.Nil(h.State.UpsertJob(h.NextIndex(), job))

	// Create several node
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		nodes = append(nodes, mock.Node())
		assert.Nil(h.State.UpsertNode(h.NextIndex(), nodes[i]))
	}

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   ns.Name,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

	// Process the evaluation
	assert.Nil(h.Process(NewSystemScheduler, eval))

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

	// XXX This should be checked once the system scheduler creates blocked
	// evals
	// Ensure the eval has spawned blocked eval
	//assert.Len(h.CreateEvals, 1)

	// Lookup the allocations by JobID and make sure we have the right amount of
	// each type
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	assert.Nil(err)

	assert.Len(out, 4)
	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}
