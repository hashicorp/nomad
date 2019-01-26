// +build ent

package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func TestPlanApply_EvalPlanQuota_Under(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	state := testStateStore(t)

	// Create the quota spec
	qs := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(100, []*structs.QuotaSpec{qs}))

	// Create the namespace
	ns := mock.Namespace()
	ns.Quota = qs.Name
	assert.Nil(state.UpsertNamespaces(200, []*structs.Namespace{ns}))

	// Create the job
	job := mock.Job()
	job.Namespace = ns.Name

	// Create the node
	node := mock.Node()
	state.UpsertNode(300, node)

	alloc := mock.Alloc()
	alloc.Namespace = ns.Name
	plan := &structs.Plan{
		Job: job,
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc},
		},
	}

	snap, _ := state.Snapshot()
	over, err := evaluatePlanQuota(snap, plan)
	assert.Nil(err)
	assert.False(over)
}

func TestPlanApply_EvalPlanQuota_Above(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	state := testStateStore(t)

	// Create the quota spec
	qs := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(100, []*structs.QuotaSpec{qs}))

	// Create the namespace
	ns := mock.Namespace()
	ns.Quota = qs.Name
	assert.Nil(state.UpsertNamespaces(200, []*structs.Namespace{ns}))

	// Create the job
	job := mock.Job()
	job.Namespace = ns.Name

	// Create the node
	node := mock.Node()
	state.UpsertNode(300, node)

	// Create an alloc that exceeds quota
	alloc := mock.Alloc()
	alloc.Namespace = ns.Name
	alloc.TaskResources["web"].CPU = 3000
	plan := &structs.Plan{
		Job: job,
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc},
		},
	}

	snap, _ := state.Snapshot()
	over, err := evaluatePlanQuota(snap, plan)
	assert.Nil(err)
	assert.True(over)
}

func TestPlanApply_EvalPlan_AboveQuota(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	state := testStateStore(t)

	// Create the quota spec
	qs := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(100, []*structs.QuotaSpec{qs}))

	// Create the namespace
	ns := mock.Namespace()
	ns.Quota = qs.Name
	assert.Nil(state.UpsertNamespaces(200, []*structs.Namespace{ns}))

	// Create the job
	job := mock.Job()
	job.Namespace = ns.Name

	// Create the node
	node := mock.Node()
	state.UpsertNode(1000, node)
	snap, _ := state.Snapshot()

	// Create an alloc that exceeds quota
	alloc := mock.Alloc()
	alloc.Namespace = ns.Name
	alloc.TaskResources["web"].CPU = 3000

	plan := &structs.Plan{
		Job: job,
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc},
		},
		Deployment: mock.Deployment(),
		DeploymentUpdates: []*structs.DeploymentStatusUpdate{
			{
				DeploymentID:      uuid.Generate(),
				Status:            "foo",
				StatusDescription: "bar",
			},
		},
	}

	pool := NewEvaluatePool(workerPoolSize, workerPoolBufferSize)
	defer pool.Shutdown()

	result, err := evaluatePlan(pool, snap, plan, testlog.HCLogger(t))
	assert.Nil(err)
	assert.NotNil(result)
	assert.Empty(result.NodeAllocation)
	assert.EqualValues(1000, result.RefreshIndex)
	assert.Nil(result.Deployment)
	assert.Empty(result.DeploymentUpdates)
}

func TestPlanApply_EvalPlanQuota_NilJob(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	state := testStateStore(t)

	// Create the quota spec
	qs := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(100, []*structs.QuotaSpec{qs}))

	// Create the namespace
	ns := mock.Namespace()
	ns.Quota = qs.Name
	assert.Nil(state.UpsertNamespaces(200, []*structs.Namespace{ns}))

	// Create the node
	node := mock.Node()
	state.UpsertNode(300, node)

	alloc := mock.Alloc()
	alloc.Namespace = ns.Name
	plan := &structs.Plan{
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc},
		},
	}

	snap, _ := state.Snapshot()
	over, err := evaluatePlanQuota(snap, plan)
	assert.Nil(err)
	assert.False(over)
}
