// +build ent

package scheduler

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func TestQuotaIterator_BelowQuota(t *testing.T) {
	assert := assert.New(t)
	state, ctx := testContext(t)

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

	// Create the source
	nodes := []*structs.Node{mock.Node()}
	static := NewStaticIterator(ctx, nodes)

	// Create the quota iterator
	quota := NewQuotaIterator(ctx, static)
	contextual := quota.(ContextualIterator)
	contextual.SetJob(job)
	contextual.SetTaskGroup(job.TaskGroups[0])
	quota.Reset()
	assert.Len(collectFeasible(quota), 1)
}

func TestQuotaIterator_AboveQuota(t *testing.T) {
	assert := assert.New(t)
	state, ctx := testContext(t)

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

	// Bump the resource usage
	job.TaskGroups[0].Tasks[0].Resources.CPU = 3000

	// Create the source
	nodes := []*structs.Node{mock.Node()}
	static := NewStaticIterator(ctx, nodes)

	// Create the quota iterator
	quota := NewQuotaIterator(ctx, static)
	contextual := quota.(ContextualIterator)
	contextual.SetJob(job)
	contextual.SetTaskGroup(job.TaskGroups[0])
	quota.Reset()
	assert.Len(collectFeasible(quota), 0)

	// Check that it marks the dimension that is exhausted
	assert.Len(ctx.Metrics().QuotaExhausted, 1)
	assert.Contains(ctx.Metrics().QuotaExhausted[0], "cpu")

	// Check it marks the quota limit being reached
	elig := ctx.Eligibility()
	assert.NotNil(elig)
	assert.Equal(qs.Name, elig.QuotaLimitReached())
}

func TestQuotaIterator_BelowQuota_PlannedAdditions(t *testing.T) {
	assert := assert.New(t)
	state, ctx := testContext(t)

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

	// Create the source
	nodes := []*structs.Node{mock.Node()}
	static := NewStaticIterator(ctx, nodes)

	// Add a planned alloc to node1 that is still below quota
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].ID] = []*structs.Allocation{mock.Alloc()}

	// Create the quota iterator
	quota := NewQuotaIterator(ctx, static)
	contextual := quota.(ContextualIterator)
	contextual.SetJob(job)
	contextual.SetTaskGroup(job.TaskGroups[0])
	quota.Reset()
	assert.Len(collectFeasible(quota), 1)
}

func TestQuotaIterator_AboveQuota_PlannedAdditions(t *testing.T) {
	assert := assert.New(t)
	state, ctx := testContext(t)

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

	// Create the source
	nodes := []*structs.Node{mock.Node()}
	static := NewStaticIterator(ctx, nodes)

	// Add a planned alloc to node1 that fills the quota
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].ID] = []*structs.Allocation{
		mock.Alloc(),
		mock.Alloc(),
		mock.Alloc(),
		mock.Alloc(),
	}

	// Create the quota iterator
	quota := NewQuotaIterator(ctx, static)
	contextual := quota.(ContextualIterator)
	contextual.SetJob(job)
	contextual.SetTaskGroup(job.TaskGroups[0])
	quota.Reset()
	assert.Len(collectFeasible(quota), 0)

	// Check it marks the quota limit being reached
	elig := ctx.Eligibility()
	assert.NotNil(elig)
	assert.Equal(qs.Name, elig.QuotaLimitReached())
}

func TestQuotaIterator_BelowQuota_DiscountStopping(t *testing.T) {
	assert := assert.New(t)
	state, ctx := testContext(t)

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

	// Create the source
	nodes := []*structs.Node{mock.Node()}
	static := NewStaticIterator(ctx, nodes)

	// Add a planned alloc to node1 that fills the quota
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].ID] = []*structs.Allocation{
		mock.Alloc(),
		mock.Alloc(),
		mock.Alloc(),
		mock.Alloc(),
	}

	// Add a planned eviction that makes it possible however to still place
	plan.NodeUpdate[nodes[0].ID] = []*structs.Allocation{mock.Alloc()}

	// Create the quota iterator
	quota := NewQuotaIterator(ctx, static)
	contextual := quota.(ContextualIterator)
	contextual.SetJob(job)
	contextual.SetTaskGroup(job.TaskGroups[0])
	quota.Reset()
	assert.Len(collectFeasible(quota), 1)
}
