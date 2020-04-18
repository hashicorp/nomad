package scheduler

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceSched_JobRegister(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan doesn't have annotations.
	if plan.Annotations != nil {
		t.Fatalf("expected no annotations")
	}

	// Ensure the eval has no spawned blocked eval
	if len(h.CreateEvals) != 0 {
		t.Fatalf("bad: %#v", h.CreateEvals)
		if h.Evals[0].BlockedEval != "" {
			t.Fatalf("bad: %#v", h.Evals[0])
		}
	}

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 10 {
		t.Fatalf("bad: %#v", plan)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	if len(out) != 10 {
		t.Fatalf("bad: %#v", out)
	}

	// Ensure different ports were used.
	used := make(map[int]map[string]struct{})
	for _, alloc := range out {
		for _, resource := range alloc.TaskResources {
			for _, port := range resource.Networks[0].DynamicPorts {
				nodeMap, ok := used[port.Value]
				if !ok {
					nodeMap = make(map[string]struct{})
					used[port.Value] = nodeMap
				}
				if _, ok := nodeMap[alloc.NodeID]; ok {
					t.Fatalf("Port collision on node %q %v", alloc.NodeID, port.Value)
				}
				nodeMap[alloc.NodeID] = struct{}{}
			}
		}
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_JobRegister_StickyAllocs(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.Job()
	job.TaskGroups[0].EphemeralDisk.Sticky = true
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	if err := h.Process(NewServiceScheduler, eval); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure the plan allocated
	plan := h.Plans[0]
	planned := make(map[string]*structs.Allocation)
	for _, allocList := range plan.NodeAllocation {
		for _, alloc := range allocList {
			planned[alloc.ID] = alloc
		}
	}
	if len(planned) != 10 {
		t.Fatalf("bad: %#v", plan)
	}

	// Update the job to force a rolling upgrade
	updated := job.Copy()
	updated.TaskGroups[0].Tasks[0].Resources.CPU += 10
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), updated))

	// Create a mock evaluation to handle the update
	eval = &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))
	h1 := NewHarnessWithState(t, h.State)
	if err := h1.Process(NewServiceScheduler, eval); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have created only one new allocation
	// Ensure a single plan
	if len(h1.Plans) != 1 {
		t.Fatalf("bad: %#v", h1.Plans)
	}
	plan = h1.Plans[0]
	var newPlanned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		newPlanned = append(newPlanned, allocList...)
	}
	if len(newPlanned) != 10 {
		t.Fatalf("bad plan: %#v", plan)
	}
	// Ensure that the new allocations were placed on the same node as the older
	// ones
	for _, new := range newPlanned {
		if new.PreviousAllocation == "" {
			t.Fatalf("new alloc %q doesn't have a previous allocation", new.ID)
		}

		old, ok := planned[new.PreviousAllocation]
		if !ok {
			t.Fatalf("new alloc %q previous allocation doesn't match any prior placed alloc (%q)", new.ID, new.PreviousAllocation)
		}
		if new.NodeID != old.NodeID {
			t.Fatalf("new alloc and old alloc node doesn't match; got %q; want %q", new.NodeID, old.NodeID)
		}
	}
}

func TestServiceSched_JobRegister_DiskConstraints(t *testing.T) {
	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a job with count 2 and disk as 60GB so that only one allocation
	// can fit
	job := mock.Job()
	job.TaskGroups[0].Count = 2
	job.TaskGroups[0].EphemeralDisk.SizeMB = 88 * 1024
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan doesn't have annotations.
	if plan.Annotations != nil {
		t.Fatalf("expected no annotations")
	}

	// Ensure the eval has a blocked eval
	if len(h.CreateEvals) != 1 {
		t.Fatalf("bad: %#v", h.CreateEvals)
	}

	if h.CreateEvals[0].TriggeredBy != structs.EvalTriggerQueuedAllocs {
		t.Fatalf("bad: %#v", h.CreateEvals[0])
	}

	// Ensure the plan allocated only one allocation
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 1 {
		t.Fatalf("bad: %#v", plan)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure only one allocation was placed
	if len(out) != 1 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_JobRegister_DistinctHosts(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job that uses distinct host and has count 1 higher than what is
	// possible.
	job := mock.Job()
	job.TaskGroups[0].Count = 11
	job.Constraints = append(job.Constraints, &structs.Constraint{Operand: structs.ConstraintDistinctHosts})
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the eval has spawned blocked eval
	if len(h.CreateEvals) != 1 {
		t.Fatalf("bad: %#v", h.CreateEvals)
	}

	// Ensure the plan failed to alloc
	outEval := h.Evals[0]
	if len(outEval.FailedTGAllocs) != 1 {
		t.Fatalf("bad: %+v", outEval)
	}

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 10 {
		t.Fatalf("bad: %#v", plan)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	if len(out) != 10 {
		t.Fatalf("bad: %#v", out)
	}

	// Ensure different node was used per.
	used := make(map[string]struct{})
	for _, alloc := range out {
		if _, ok := used[alloc.NodeID]; ok {
			t.Fatalf("Node collision %v", alloc.NodeID)
		}
		used[alloc.NodeID] = struct{}{}
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_JobRegister_DistinctProperty(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		rack := "rack2"
		if i < 5 {
			rack = "rack1"
		}
		node.Meta["rack"] = rack
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job that uses distinct property and has count higher than what is
	// possible.
	job := mock.Job()
	job.TaskGroups[0].Count = 8
	job.Constraints = append(job.Constraints,
		&structs.Constraint{
			Operand: structs.ConstraintDistinctProperty,
			LTarget: "${meta.rack}",
			RTarget: "2",
		})
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan doesn't have annotations.
	if plan.Annotations != nil {
		t.Fatalf("expected no annotations")
	}

	// Ensure the eval has spawned blocked eval
	if len(h.CreateEvals) != 1 {
		t.Fatalf("bad: %#v", h.CreateEvals)
	}

	// Ensure the plan failed to alloc
	outEval := h.Evals[0]
	if len(outEval.FailedTGAllocs) != 1 {
		t.Fatalf("bad: %+v", outEval)
	}

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 4 {
		t.Fatalf("bad: %#v", plan)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	if len(out) != 4 {
		t.Fatalf("bad: %#v", out)
	}

	// Ensure each node was only used twice
	used := make(map[string]uint64)
	for _, alloc := range out {
		if count, _ := used[alloc.NodeID]; count > 2 {
			t.Fatalf("Node %v used too much: %d", alloc.NodeID, count)
		}
		used[alloc.NodeID]++
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_JobRegister_DistinctProperty_TaskGroup(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 2; i++ {
		node := mock.Node()
		node.Meta["ssd"] = "true"
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job that uses distinct property only on one task group.
	job := mock.Job()
	job.TaskGroups = append(job.TaskGroups, job.TaskGroups[0].Copy())
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Constraints = append(job.TaskGroups[0].Constraints,
		&structs.Constraint{
			Operand: structs.ConstraintDistinctProperty,
			LTarget: "${meta.ssd}",
		})

	job.TaskGroups[1].Name = "tg2"
	job.TaskGroups[1].Count = 2
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan doesn't have annotations.
	if plan.Annotations != nil {
		t.Fatalf("expected no annotations")
	}

	// Ensure the eval hasn't spawned blocked eval
	if len(h.CreateEvals) != 0 {
		t.Fatalf("bad: %#v", h.CreateEvals[0])
	}

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 3 {
		t.Fatalf("bad: %#v", plan)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	if len(out) != 3 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_JobRegister_DistinctProperty_TaskGroup_Incr(t *testing.T) {
	h := NewHarness(t)
	assert := assert.New(t)

	// Create a job that uses distinct property over the node-id
	job := mock.Job()
	job.TaskGroups[0].Count = 3
	job.TaskGroups[0].Constraints = append(job.TaskGroups[0].Constraints,
		&structs.Constraint{
			Operand: structs.ConstraintDistinctProperty,
			LTarget: "${node.unique.id}",
		})
	assert.Nil(h.State.UpsertJob(h.NextIndex(), job), "UpsertJob")

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 6; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		assert.Nil(h.State.UpsertNode(h.NextIndex(), node), "UpsertNode")
	}

	// Create some allocations
	var allocs []*structs.Allocation
	for i := 0; i < 3; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	assert.Nil(h.State.UpsertAllocs(h.NextIndex(), allocs), "UpsertAllocs")

	// Update the count
	job2 := job.Copy()
	job2.TaskGroups[0].Count = 6
	assert.Nil(h.State.UpsertJob(h.NextIndex(), job2), "UpsertJob")

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	assert.Nil(h.Process(NewServiceScheduler, eval), "Process")

	// Ensure a single plan
	assert.Len(h.Plans, 1, "Number of plans")
	plan := h.Plans[0]

	// Ensure the plan doesn't have annotations.
	assert.Nil(plan.Annotations, "Plan.Annotations")

	// Ensure the eval hasn't spawned blocked eval
	assert.Len(h.CreateEvals, 0, "Created Evals")

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	assert.Len(planned, 6, "Planned Allocations")

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	assert.Nil(err, "AllocsByJob")

	// Ensure all allocations placed
	assert.Len(out, 6, "Placed Allocations")

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

// Test job registration with spread configured
func TestServiceSched_Spread(t *testing.T) {
	assert := assert.New(t)

	start := uint8(100)
	step := uint8(10)

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("%d%% in dc1", start)
		t.Run(name, func(t *testing.T) {
			h := NewHarness(t)
			remaining := uint8(100 - start)
			// Create a job that uses spread over data center
			job := mock.Job()
			job.Datacenters = []string{"dc1", "dc2"}
			job.TaskGroups[0].Count = 10
			job.TaskGroups[0].Spreads = append(job.TaskGroups[0].Spreads,
				&structs.Spread{
					Attribute: "${node.datacenter}",
					Weight:    100,
					SpreadTarget: []*structs.SpreadTarget{
						{
							Value:   "dc1",
							Percent: start,
						},
						{
							Value:   "dc2",
							Percent: remaining,
						},
					},
				})
			assert.Nil(h.State.UpsertJob(h.NextIndex(), job), "UpsertJob")
			// Create some nodes, half in dc2
			var nodes []*structs.Node
			nodeMap := make(map[string]*structs.Node)
			for i := 0; i < 10; i++ {
				node := mock.Node()
				if i%2 == 0 {
					node.Datacenter = "dc2"
				}
				nodes = append(nodes, node)
				assert.Nil(h.State.UpsertNode(h.NextIndex(), node), "UpsertNode")
				nodeMap[node.ID] = node
			}

			// Create a mock evaluation to register the job
			eval := &structs.Evaluation{
				Namespace:   structs.DefaultNamespace,
				ID:          uuid.Generate(),
				Priority:    job.Priority,
				TriggeredBy: structs.EvalTriggerJobRegister,
				JobID:       job.ID,
				Status:      structs.EvalStatusPending,
			}
			require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

			// Process the evaluation
			assert.Nil(h.Process(NewServiceScheduler, eval), "Process")

			// Ensure a single plan
			assert.Len(h.Plans, 1, "Number of plans")
			plan := h.Plans[0]

			// Ensure the plan doesn't have annotations.
			assert.Nil(plan.Annotations, "Plan.Annotations")

			// Ensure the eval hasn't spawned blocked eval
			assert.Len(h.CreateEvals, 0, "Created Evals")

			// Ensure the plan allocated
			var planned []*structs.Allocation
			dcAllocsMap := make(map[string]int)
			for nodeId, allocList := range plan.NodeAllocation {
				planned = append(planned, allocList...)
				dc := nodeMap[nodeId].Datacenter
				c := dcAllocsMap[dc]
				c += len(allocList)
				dcAllocsMap[dc] = c
			}
			assert.Len(planned, 10, "Planned Allocations")

			expectedCounts := make(map[string]int)
			expectedCounts["dc1"] = 10 - i
			if i > 0 {
				expectedCounts["dc2"] = i
			}
			require.Equal(t, expectedCounts, dcAllocsMap)

			h.AssertEvalStatus(t, structs.EvalStatusComplete)
		})
		start = start - step
	}
}

// Test job registration with even spread across dc
func TestServiceSched_EvenSpread(t *testing.T) {
	assert := assert.New(t)

	h := NewHarness(t)
	// Create a job that uses even spread over data center
	job := mock.Job()
	job.Datacenters = []string{"dc1", "dc2"}
	job.TaskGroups[0].Count = 10
	job.TaskGroups[0].Spreads = append(job.TaskGroups[0].Spreads,
		&structs.Spread{
			Attribute: "${node.datacenter}",
			Weight:    100,
		})
	assert.Nil(h.State.UpsertJob(h.NextIndex(), job), "UpsertJob")
	// Create some nodes, half in dc2
	var nodes []*structs.Node
	nodeMap := make(map[string]*structs.Node)
	for i := 0; i < 10; i++ {
		node := mock.Node()
		if i%2 == 0 {
			node.Datacenter = "dc2"
		}
		nodes = append(nodes, node)
		assert.Nil(h.State.UpsertNode(h.NextIndex(), node), "UpsertNode")
		nodeMap[node.ID] = node
	}

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	assert.Nil(h.Process(NewServiceScheduler, eval), "Process")

	// Ensure a single plan
	assert.Len(h.Plans, 1, "Number of plans")
	plan := h.Plans[0]

	// Ensure the plan doesn't have annotations.
	assert.Nil(plan.Annotations, "Plan.Annotations")

	// Ensure the eval hasn't spawned blocked eval
	assert.Len(h.CreateEvals, 0, "Created Evals")

	// Ensure the plan allocated
	var planned []*structs.Allocation
	dcAllocsMap := make(map[string]int)
	for nodeId, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
		dc := nodeMap[nodeId].Datacenter
		c := dcAllocsMap[dc]
		c += len(allocList)
		dcAllocsMap[dc] = c
	}
	assert.Len(planned, 10, "Planned Allocations")

	// Expect even split allocs across datacenter
	expectedCounts := make(map[string]int)
	expectedCounts["dc1"] = 5
	expectedCounts["dc2"] = 5

	require.Equal(t, expectedCounts, dcAllocsMap)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_JobRegister_Annotate(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:    structs.DefaultNamespace,
		ID:           uuid.Generate(),
		Priority:     job.Priority,
		TriggeredBy:  structs.EvalTriggerJobRegister,
		JobID:        job.ID,
		AnnotatePlan: true,
		Status:       structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 10 {
		t.Fatalf("bad: %#v", plan)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	if len(out) != 10 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)

	// Ensure the plan had annotations.
	if plan.Annotations == nil {
		t.Fatalf("expected annotations")
	}

	desiredTGs := plan.Annotations.DesiredTGUpdates
	if l := len(desiredTGs); l != 1 {
		t.Fatalf("incorrect number of task groups; got %v; want %v", l, 1)
	}

	desiredChanges, ok := desiredTGs["web"]
	if !ok {
		t.Fatalf("expected task group web to have desired changes")
	}

	expected := &structs.DesiredUpdates{Place: 10}
	if !reflect.DeepEqual(desiredChanges, expected) {
		t.Fatalf("Unexpected desired updates; got %#v; want %#v", desiredChanges, expected)
	}
}

func TestServiceSched_JobRegister_CountZero(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job and set the task group count to zero.
	job := mock.Job()
	job.TaskGroups[0].Count = 0
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure there was no plan
	if len(h.Plans) != 0 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure no allocations placed
	if len(out) != 0 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_JobRegister_AllocFail(t *testing.T) {
	h := NewHarness(t)

	// Create NO nodes
	// Create a job
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure no plan
	if len(h.Plans) != 0 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	// Ensure there is a follow up eval.
	if len(h.CreateEvals) != 1 || h.CreateEvals[0].Status != structs.EvalStatusBlocked {
		t.Fatalf("bad: %#v", h.CreateEvals)
	}

	if len(h.Evals) != 1 {
		t.Fatalf("incorrect number of updated eval: %#v", h.Evals)
	}
	outEval := h.Evals[0]

	// Ensure the eval has its spawned blocked eval
	if outEval.BlockedEval != h.CreateEvals[0].ID {
		t.Fatalf("bad: %#v", outEval)
	}

	// Ensure the plan failed to alloc
	if outEval == nil || len(outEval.FailedTGAllocs) != 1 {
		t.Fatalf("bad: %#v", outEval)
	}

	metrics, ok := outEval.FailedTGAllocs[job.TaskGroups[0].Name]
	if !ok {
		t.Fatalf("no failed metrics: %#v", outEval.FailedTGAllocs)
	}

	// Check the coalesced failures
	if metrics.CoalescedFailures != 9 {
		t.Fatalf("bad: %#v", metrics)
	}

	// Check the available nodes
	if count, ok := metrics.NodesAvailable["dc1"]; !ok || count != 0 {
		t.Fatalf("bad: %#v", metrics)
	}

	// Check queued allocations
	queued := outEval.QueuedAllocations["web"]
	if queued != 10 {
		t.Fatalf("expected queued: %v, actual: %v", 10, queued)
	}
	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_JobRegister_CreateBlockedEval(t *testing.T) {
	h := NewHarness(t)

	// Create a full node
	node := mock.Node()
	node.ReservedResources = &structs.NodeReservedResources{
		Cpu: structs.NodeReservedCpuResources{
			CpuShares: node.NodeResources.Cpu.CpuShares,
		},
	}
	node.ComputeClass()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create an ineligible node
	node2 := mock.Node()
	node2.Attributes["kernel.name"] = "windows"
	node2.ComputeClass()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node2))

	// Create a jobs
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure no plan
	if len(h.Plans) != 0 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	// Ensure the plan has created a follow up eval.
	if len(h.CreateEvals) != 1 {
		t.Fatalf("bad: %#v", h.CreateEvals)
	}

	created := h.CreateEvals[0]
	if created.Status != structs.EvalStatusBlocked {
		t.Fatalf("bad: %#v", created)
	}

	classes := created.ClassEligibility
	if len(classes) != 2 || !classes[node.ComputedClass] || classes[node2.ComputedClass] {
		t.Fatalf("bad: %#v", classes)
	}

	if created.EscapedComputedClass {
		t.Fatalf("bad: %#v", created)
	}

	// Ensure there is a follow up eval.
	if len(h.CreateEvals) != 1 || h.CreateEvals[0].Status != structs.EvalStatusBlocked {
		t.Fatalf("bad: %#v", h.CreateEvals)
	}

	if len(h.Evals) != 1 {
		t.Fatalf("incorrect number of updated eval: %#v", h.Evals)
	}
	outEval := h.Evals[0]

	// Ensure the plan failed to alloc
	if outEval == nil || len(outEval.FailedTGAllocs) != 1 {
		t.Fatalf("bad: %#v", outEval)
	}

	metrics, ok := outEval.FailedTGAllocs[job.TaskGroups[0].Name]
	if !ok {
		t.Fatalf("no failed metrics: %#v", outEval.FailedTGAllocs)
	}

	// Check the coalesced failures
	if metrics.CoalescedFailures != 9 {
		t.Fatalf("bad: %#v", metrics)
	}

	// Check the available nodes
	if count, ok := metrics.NodesAvailable["dc1"]; !ok || count != 2 {
		t.Fatalf("bad: %#v", metrics)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_JobRegister_FeasibleAndInfeasibleTG(t *testing.T) {
	h := NewHarness(t)

	// Create one node
	node := mock.Node()
	node.NodeClass = "class_0"
	require.NoError(t, node.ComputeClass())
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a job that constrains on a node class
	job := mock.Job()
	job.TaskGroups[0].Count = 2
	job.TaskGroups[0].Constraints = append(job.Constraints,
		&structs.Constraint{
			LTarget: "${node.class}",
			RTarget: "class_0",
			Operand: "=",
		},
	)
	tg2 := job.TaskGroups[0].Copy()
	tg2.Name = "web2"
	tg2.Constraints[1].RTarget = "class_1"
	job.TaskGroups = append(job.TaskGroups, tg2)
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))
	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 2 {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure two allocations placed
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)
	if len(out) != 2 {
		t.Fatalf("bad: %#v", out)
	}

	if len(h.Evals) != 1 {
		t.Fatalf("incorrect number of updated eval: %#v", h.Evals)
	}
	outEval := h.Evals[0]

	// Ensure the eval has its spawned blocked eval
	if outEval.BlockedEval != h.CreateEvals[0].ID {
		t.Fatalf("bad: %#v", outEval)
	}

	// Ensure the plan failed to alloc one tg
	if outEval == nil || len(outEval.FailedTGAllocs) != 1 {
		t.Fatalf("bad: %#v", outEval)
	}

	metrics, ok := outEval.FailedTGAllocs[tg2.Name]
	if !ok {
		t.Fatalf("no failed metrics: %#v", outEval.FailedTGAllocs)
	}

	// Check the coalesced failures
	if metrics.CoalescedFailures != tg2.Count-1 {
		t.Fatalf("bad: %#v", metrics)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

// This test just ensures the scheduler handles the eval type to avoid
// regressions.
func TestServiceSched_EvaluateMaxPlanEval(t *testing.T) {
	h := NewHarness(t)

	// Create a job and set the task group count to zero.
	job := mock.Job()
	job.TaskGroups[0].Count = 0
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock blocked evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Status:      structs.EvalStatusBlocked,
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerMaxPlans,
		JobID:       job.ID,
	}

	// Insert it into the state store
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure there was no plan
	if len(h.Plans) != 0 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_Plan_Partial_Progress(t *testing.T) {
	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a job with a high resource ask so that all the allocations can't
	// be placed on a single node.
	job := mock.Job()
	job.TaskGroups[0].Count = 3
	job.TaskGroups[0].Tasks[0].Resources.CPU = 3600
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan doesn't have annotations.
	if plan.Annotations != nil {
		t.Fatalf("expected no annotations")
	}

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 1 {
		t.Fatalf("bad: %#v", plan)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure only one allocations placed
	if len(out) != 1 {
		t.Fatalf("bad: %#v", out)
	}

	queued := h.Evals[0].QueuedAllocations["web"]
	if queued != 2 {
		t.Fatalf("expected: %v, actual: %v", 2, queued)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_EvaluateBlockedEval(t *testing.T) {
	h := NewHarness(t)

	// Create a job
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock blocked evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Status:      structs.EvalStatusBlocked,
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

	// Insert it into the state store
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure there was no plan
	if len(h.Plans) != 0 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	// Ensure that the eval was reblocked
	if len(h.ReblockEvals) != 1 {
		t.Fatalf("bad: %#v", h.ReblockEvals)
	}
	if h.ReblockEvals[0].ID != eval.ID {
		t.Fatalf("expect same eval to be reblocked; got %q; want %q", h.ReblockEvals[0].ID, eval.ID)
	}

	// Ensure the eval status was not updated
	if len(h.Evals) != 0 {
		t.Fatalf("Existing eval should not have status set")
	}
}

func TestServiceSched_EvaluateBlockedEval_Finished(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job and set the task group count to zero.
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock blocked evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Status:      structs.EvalStatusBlocked,
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

	// Insert it into the state store
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan doesn't have annotations.
	if plan.Annotations != nil {
		t.Fatalf("expected no annotations")
	}

	// Ensure the eval has no spawned blocked eval
	if len(h.Evals) != 1 {
		t.Fatalf("bad: %#v", h.Evals)
		if h.Evals[0].BlockedEval != "" {
			t.Fatalf("bad: %#v", h.Evals[0])
		}
	}

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 10 {
		t.Fatalf("bad: %#v", plan)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	if len(out) != 10 {
		t.Fatalf("bad: %#v", out)
	}

	// Ensure the eval was not reblocked
	if len(h.ReblockEvals) != 0 {
		t.Fatalf("Existing eval should not have been reblocked as it placed all allocations")
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)

	// Ensure queued allocations is zero
	queued := h.Evals[0].QueuedAllocations["web"]
	if queued != 0 {
		t.Fatalf("expected queued: %v, actual: %v", 0, queued)
	}
}

func TestServiceSched_JobModify(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Add a few terminal status allocations, these should be ignored
	var terminal []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		alloc.DesiredStatus = structs.AllocDesiredStatusStop
		terminal = append(terminal, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), terminal))

	// Update the job
	job2 := mock.Job()
	job2.ID = job.ID

	// Update the task, such that it cannot be done in-place
	job2.TaskGroups[0].Tasks[0].Config["command"] = "/bin/other"
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan evicted all allocs
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	if len(update) != len(allocs) {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 10 {
		t.Fatalf("bad: %#v", plan)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	out, _ = structs.FilterTerminalAllocs(out)
	if len(out) != 10 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

// Have a single node and submit a job. Increment the count such that all fit
// on the node but the node doesn't have enough resources to fit the new count +
// 1. This tests that we properly discount the resources of existing allocs.
func TestServiceSched_JobModify_IncrCount_NodeLimit(t *testing.T) {
	h := NewHarness(t)

	// Create one node
	node := mock.Node()
	node.NodeResources.Cpu.CpuShares = 1000
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job with one allocation
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Resources.CPU = 256
	job2 := job.Copy()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.AllocatedResources.Tasks["web"].Cpu.CpuShares = 256
	allocs = append(allocs, alloc)
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Update the job to count 3
	job2.TaskGroups[0].Count = 3
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan didn't evicted the alloc
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	if len(update) != 0 {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 3 {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure the plan had no failures
	if len(h.Evals) != 1 {
		t.Fatalf("incorrect number of updated eval: %#v", h.Evals)
	}
	outEval := h.Evals[0]
	if outEval == nil || len(outEval.FailedTGAllocs) != 0 {
		t.Fatalf("bad: %#v", outEval)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	out, _ = structs.FilterTerminalAllocs(out)
	if len(out) != 3 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_JobModify_CountZero(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = structs.AllocName(alloc.JobID, alloc.TaskGroup, uint(i))
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Add a few terminal status allocations, these should be ignored
	var terminal []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = structs.AllocName(alloc.JobID, alloc.TaskGroup, uint(i))
		alloc.DesiredStatus = structs.AllocDesiredStatusStop
		terminal = append(terminal, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), terminal))

	// Update the job to be count zero
	job2 := mock.Job()
	job2.ID = job.ID
	job2.TaskGroups[0].Count = 0
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan evicted all allocs
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	if len(update) != len(allocs) {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure the plan didn't allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 0 {
		t.Fatalf("bad: %#v", plan)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	out, _ = structs.FilterTerminalAllocs(out)
	if len(out) != 0 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_JobModify_Rolling(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Update the job
	job2 := mock.Job()
	job2.ID = job.ID
	desiredUpdates := 4
	job2.TaskGroups[0].Update = &structs.UpdateStrategy{
		MaxParallel:     desiredUpdates,
		HealthCheck:     structs.UpdateStrategyHealthCheck_Checks,
		MinHealthyTime:  10 * time.Second,
		HealthyDeadline: 10 * time.Minute,
	}

	// Update the task, such that it cannot be done in-place
	job2.TaskGroups[0].Tasks[0].Config["command"] = "/bin/other"
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan evicted only MaxParallel
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	if len(update) != desiredUpdates {
		t.Fatalf("bad: got %d; want %d: %#v", len(update), desiredUpdates, plan)
	}

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != desiredUpdates {
		t.Fatalf("bad: %#v", plan)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)

	// Check that the deployment id is attached to the eval
	if h.Evals[0].DeploymentID == "" {
		t.Fatalf("Eval not annotated with deployment id")
	}

	// Ensure a deployment was created
	if plan.Deployment == nil {
		t.Fatalf("bad: %#v", plan)
	}
	state, ok := plan.Deployment.TaskGroups[job.TaskGroups[0].Name]
	if !ok {
		t.Fatalf("bad: %#v", plan)
	}
	if state.DesiredTotal != 10 && state.DesiredCanaries != 0 {
		t.Fatalf("bad: %#v", state)
	}
}

// This tests that the old allocation is stopped before placing.
// It is critical to test that the updated job attempts to place more
// allocations as this allows us to assert that destructive changes are done
// first.
func TestServiceSched_JobModify_Rolling_FullNode(t *testing.T) {
	h := NewHarness(t)

	// Create a node and clear the reserved resources
	node := mock.Node()
	node.ReservedResources = nil
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a resource ask that is the same as the resources available on the
	// node
	cpu := node.NodeResources.Cpu.CpuShares
	mem := node.NodeResources.Memory.MemoryMB

	request := &structs.Resources{
		CPU:      int(cpu),
		MemoryMB: int(mem),
	}
	allocated := &structs.AllocatedResources{
		Tasks: map[string]*structs.AllocatedTaskResources{
			"web": {
				Cpu: structs.AllocatedCpuResources{
					CpuShares: cpu,
				},
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: mem,
				},
			},
		},
	}

	// Generate a fake job with one alloc that consumes the whole node
	job := mock.Job()
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Tasks[0].Resources = request
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	alloc := mock.Alloc()
	alloc.AllocatedResources = allocated
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Update the job to place more versions of the task group, drop the count
	// and force destructive updates
	job2 := job.Copy()
	job2.TaskGroups[0].Count = 5
	job2.TaskGroups[0].Update = &structs.UpdateStrategy{
		MaxParallel:     5,
		HealthCheck:     structs.UpdateStrategyHealthCheck_Checks,
		MinHealthyTime:  10 * time.Second,
		HealthyDeadline: 10 * time.Minute,
	}
	job2.TaskGroups[0].Tasks[0].Resources = mock.Job().TaskGroups[0].Tasks[0].Resources

	// Update the task, such that it cannot be done in-place
	job2.TaskGroups[0].Tasks[0].Config["command"] = "/bin/other"
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job2))

	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan evicted only MaxParallel
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	if len(update) != 1 {
		t.Fatalf("bad: got %d; want %d: %#v", len(update), 1, plan)
	}

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 5 {
		t.Fatalf("bad: %#v", plan)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)

	// Check that the deployment id is attached to the eval
	if h.Evals[0].DeploymentID == "" {
		t.Fatalf("Eval not annotated with deployment id")
	}

	// Ensure a deployment was created
	if plan.Deployment == nil {
		t.Fatalf("bad: %#v", plan)
	}
	state, ok := plan.Deployment.TaskGroups[job.TaskGroups[0].Name]
	if !ok {
		t.Fatalf("bad: %#v", plan)
	}
	if state.DesiredTotal != 5 || state.DesiredCanaries != 0 {
		t.Fatalf("bad: %#v", state)
	}
}

func TestServiceSched_JobModify_Canaries(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Update the job
	job2 := mock.Job()
	job2.ID = job.ID
	desiredUpdates := 2
	job2.TaskGroups[0].Update = &structs.UpdateStrategy{
		MaxParallel:     desiredUpdates,
		Canary:          desiredUpdates,
		HealthCheck:     structs.UpdateStrategyHealthCheck_Checks,
		MinHealthyTime:  10 * time.Second,
		HealthyDeadline: 10 * time.Minute,
	}

	// Update the task, such that it cannot be done in-place
	job2.TaskGroups[0].Tasks[0].Config["command"] = "/bin/other"
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan evicted nothing
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	if len(update) != 0 {
		t.Fatalf("bad: got %d; want %d: %#v", len(update), 0, plan)
	}

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != desiredUpdates {
		t.Fatalf("bad: %#v", plan)
	}
	for _, canary := range planned {
		if canary.DeploymentStatus == nil || !canary.DeploymentStatus.Canary {
			t.Fatalf("expected canary field to be set on canary alloc %q", canary.ID)
		}
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)

	// Check that the deployment id is attached to the eval
	if h.Evals[0].DeploymentID == "" {
		t.Fatalf("Eval not annotated with deployment id")
	}

	// Ensure a deployment was created
	if plan.Deployment == nil {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure local state was not altered in scheduler
	staleState, ok := plan.Deployment.TaskGroups[job.TaskGroups[0].Name]
	require.True(t, ok)

	require.Equal(t, 0, len(staleState.PlacedCanaries))

	ws := memdb.NewWatchSet()

	// Grab the latest state
	deploy, err := h.State.DeploymentByID(ws, plan.Deployment.ID)
	require.NoError(t, err)

	state, ok := deploy.TaskGroups[job.TaskGroups[0].Name]
	require.True(t, ok)

	require.Equal(t, 10, state.DesiredTotal)
	require.Equal(t, state.DesiredCanaries, desiredUpdates)

	// Assert the canaries were added to the placed list
	if len(state.PlacedCanaries) != desiredUpdates {
		assert.Fail(t, "expected PlacedCanaries to equal desiredUpdates", state)
	}
}

func TestServiceSched_JobModify_InPlace(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations and create an older deployment
	job := mock.Job()
	d := mock.Deployment()
	d.JobID = job.ID
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))
	require.NoError(t, h.State.UpsertDeployment(h.NextIndex(), d))

	// Create allocs that are part of the old deployment
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		alloc.DeploymentID = d.ID
		alloc.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: helper.BoolToPtr(true)}
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Update the job
	job2 := mock.Job()
	job2.ID = job.ID
	desiredUpdates := 4
	job2.TaskGroups[0].Update = &structs.UpdateStrategy{
		MaxParallel:     desiredUpdates,
		HealthCheck:     structs.UpdateStrategyHealthCheck_Checks,
		MinHealthyTime:  10 * time.Second,
		HealthyDeadline: 10 * time.Minute,
	}
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan did not evict any allocs
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	if len(update) != 0 {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure the plan updated the existing allocs
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 10 {
		t.Fatalf("bad: %#v", plan)
	}
	for _, p := range planned {
		if p.Job != job2 {
			t.Fatalf("should update job")
		}
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	if len(out) != 10 {
		t.Fatalf("bad: %#v", out)
	}
	h.AssertEvalStatus(t, structs.EvalStatusComplete)

	// Verify the network did not change
	rp := structs.Port{Label: "admin", Value: 5000}
	for _, alloc := range out {
		for _, resources := range alloc.TaskResources {
			if resources.Networks[0].ReservedPorts[0] != rp {
				t.Fatalf("bad: %#v", alloc)
			}
		}
	}

	// Verify the deployment id was changed and health cleared
	for _, alloc := range out {
		if alloc.DeploymentID == d.ID {
			t.Fatalf("bad: deployment id not cleared")
		} else if alloc.DeploymentStatus != nil {
			t.Fatalf("bad: deployment status not cleared")
		}
	}
}

// TestServiceSched_JobModify_InPlace08 asserts that inplace updates of
// allocations created with Nomad 0.8 do not cause panics.
//
// COMPAT(0.11) - While we do not guarantee that upgrades from 0.8 -> 0.10
// (skipping 0.9) are safe, we do want to avoid panics in the scheduler which
// cause unrecoverable server outages with no chance of recovery.
//
// Safe to remove in 0.11.0 as no one should ever be trying to upgrade from 0.8
// to 0.11!
func TestServiceSched_JobModify_InPlace08(t *testing.T) {
	h := NewHarness(t)

	// Create node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job with 0.8 allocations
	job := mock.Job()
	job.TaskGroups[0].Count = 1
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create 0.8 alloc
	alloc := mock.Alloc()
	alloc.Job = job.Copy()
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.AllocatedResources = nil // 0.8 didn't have this
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Update the job inplace
	job2 := job.Copy()

	job2.TaskGroups[0].Tasks[0].Services[0].Tags[0] = "newtag"
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job2))

	// Create a mock evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	require.NoError(t, err)

	// Ensure a single plan
	require.Len(t, h.Plans, 1)
	plan := h.Plans[0]

	// Ensure the plan did not evict any allocs
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	require.Zero(t, update)

	// Ensure the plan updated the existing alloc
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	require.Len(t, planned, 1)
	for _, p := range planned {
		require.Equal(t, job2, p.Job)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	require.Len(t, out, 1)
	h.AssertEvalStatus(t, structs.EvalStatusComplete)

	newAlloc := out[0]

	// Verify AllocatedResources was set
	require.NotNil(t, newAlloc.AllocatedResources)
}

func TestServiceSched_JobModify_DistinctProperty(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		node.Meta["rack"] = fmt.Sprintf("rack%d", i)
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job that uses distinct property and has count higher than what is
	// possible.
	job := mock.Job()
	job.TaskGroups[0].Count = 11
	job.Constraints = append(job.Constraints,
		&structs.Constraint{
			Operand: structs.ConstraintDistinctProperty,
			LTarget: "${meta.rack}",
		})
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	oldJob := job.Copy()
	oldJob.JobModifyIndex -= 1
	oldJob.TaskGroups[0].Count = 4

	// Place 4 of 10
	var allocs []*structs.Allocation
	for i := 0; i < 4; i++ {
		alloc := mock.Alloc()
		alloc.Job = oldJob
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan doesn't have annotations.
	if plan.Annotations != nil {
		t.Fatalf("expected no annotations")
	}

	// Ensure the eval hasn't spawned blocked eval
	if len(h.CreateEvals) != 1 {
		t.Fatalf("bad: %#v", h.CreateEvals)
	}

	// Ensure the plan failed to alloc
	outEval := h.Evals[0]
	if len(outEval.FailedTGAllocs) != 1 {
		t.Fatalf("bad: %+v", outEval)
	}

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 10 {
		t.Fatalf("bad: %#v", planned)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	if len(out) != 10 {
		t.Fatalf("bad: %#v", out)
	}

	// Ensure different node was used per.
	used := make(map[string]struct{})
	for _, alloc := range out {
		if _, ok := used[alloc.NodeID]; ok {
			t.Fatalf("Node collision %v", alloc.NodeID)
		}
		used[alloc.NodeID] = struct{}{}
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

// TestServiceSched_JobModify_NodeReschedulePenalty ensures that
// a failing allocation gets rescheduled with a penalty to the old
// node, but an updated job doesn't apply the penalty.
func TestServiceSched_JobModify_NodeReschedulePenalty(t *testing.T) {
	h := NewHarness(t)
	require := require.New(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	job.TaskGroups[0].Count = 2
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts:      1,
		Interval:      15 * time.Minute,
		Delay:         5 * time.Second,
		MaxDelay:      1 * time.Minute,
		DelayFunction: "constant",
	}
	tgName := job.TaskGroups[0].Name
	now := time.Now()

	require.NoError(h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 2; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	// Mark one of the allocations as failed
	allocs[1].ClientStatus = structs.AllocClientStatusFailed
	allocs[1].TaskStates = map[string]*structs.TaskState{tgName: {State: "dead",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-10 * time.Second)}}
	failedAlloc := allocs[1]
	failedAllocID := failedAlloc.ID
	successAllocID := allocs[0].ID

	require.NoError(h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create and process a mock evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))
	require.NoError(h.Process(NewServiceScheduler, eval))

	// Ensure we have one plan
	require.Equal(1, len(h.Plans))

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(err)

	// Verify that one new allocation got created with its restart tracker info
	require.Equal(3, len(out))
	var newAlloc *structs.Allocation
	for _, alloc := range out {
		if alloc.ID != successAllocID && alloc.ID != failedAllocID {
			newAlloc = alloc
		}
	}
	require.Equal(failedAllocID, newAlloc.PreviousAllocation)
	require.Equal(1, len(newAlloc.RescheduleTracker.Events))
	require.Equal(failedAllocID, newAlloc.RescheduleTracker.Events[0].PrevAllocID)

	// Verify that the node-reschedule penalty was applied to the new alloc
	for _, scoreMeta := range newAlloc.Metrics.ScoreMetaData {
		if scoreMeta.NodeID == failedAlloc.NodeID {
			require.Equal(-1.0, scoreMeta.Scores["node-reschedule-penalty"],
				"eval to replace failed alloc missing node-reshedule-penalty: %v",
				scoreMeta.Scores,
			)
		}
	}

	// Update the job, such that it cannot be done in-place
	job2 := job.Copy()
	job2.TaskGroups[0].Tasks[0].Config["command"] = "/bin/other"
	require.NoError(h.State.UpsertJob(h.NextIndex(), job2))

	// Create and process a mock evaluation
	eval = &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))
	require.NoError(h.Process(NewServiceScheduler, eval))

	// Lookup the new allocations by JobID
	out, err = h.State.AllocsByJob(ws, job.Namespace, job2.ID, false)
	require.NoError(err)
	out, _ = structs.FilterTerminalAllocs(out)
	require.Equal(2, len(out))

	// No new allocs have node-reschedule-penalty
	for _, alloc := range out {
		require.Nil(alloc.RescheduleTracker)
		require.NotNil(alloc.Metrics)
		for _, scoreMeta := range alloc.Metrics.ScoreMetaData {
			if scoreMeta.NodeID != failedAlloc.NodeID {
				require.Equal(0.0, scoreMeta.Scores["node-reschedule-penalty"],
					"eval for updated job should not include node-reshedule-penalty: %v",
					scoreMeta.Scores,
				)
			}
		}
	}
}

func TestServiceSched_JobDeregister_Purged(t *testing.T) {
	h := NewHarness(t)

	// Generate a fake job with allocations
	job := mock.Job()

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		allocs = append(allocs, alloc)
	}
	for _, alloc := range allocs {
		h.State.UpsertJobSummary(h.NextIndex(), mock.JobSummary(alloc.JobID))
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create a mock evaluation to deregister the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobDeregister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan evicted all nodes
	if len(plan.NodeUpdate["12345678-abcd-efab-cdef-123456789abc"]) != len(allocs) {
		t.Fatalf("bad: %#v", plan)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure that the job field on the allocation is still populated
	for _, alloc := range out {
		if alloc.Job == nil {
			t.Fatalf("bad: %#v", alloc)
		}
	}

	// Ensure no remaining allocations
	out, _ = structs.FilterTerminalAllocs(out)
	if len(out) != 0 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_JobDeregister_Stopped(t *testing.T) {
	h := NewHarness(t)
	require := require.New(t)

	// Generate a fake job with allocations
	job := mock.Job()
	job.Stop = true
	require.NoError(h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		allocs = append(allocs, alloc)
	}
	require.NoError(h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create a summary where the queued allocs are set as we want to assert
	// they get zeroed out.
	summary := mock.JobSummary(job.ID)
	web := summary.Summary["web"]
	web.Queued = 2
	require.NoError(h.State.UpsertJobSummary(h.NextIndex(), summary))

	// Create a mock evaluation to deregister the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobDeregister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	require.NoError(h.Process(NewServiceScheduler, eval))

	// Ensure a single plan
	require.Len(h.Plans, 1)
	plan := h.Plans[0]

	// Ensure the plan evicted all nodes
	require.Len(plan.NodeUpdate["12345678-abcd-efab-cdef-123456789abc"], len(allocs))

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(err)

	// Ensure that the job field on the allocation is still populated
	for _, alloc := range out {
		require.NotNil(alloc.Job)
	}

	// Ensure no remaining allocations
	out, _ = structs.FilterTerminalAllocs(out)
	require.Empty(out)

	// Assert the job summary is cleared out
	sout, err := h.State.JobSummaryByID(ws, job.Namespace, job.ID)
	require.NoError(err)
	require.NotNil(sout)
	require.Contains(sout.Summary, "web")
	webOut := sout.Summary["web"]
	require.Zero(webOut.Queued)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_NodeDown(t *testing.T) {
	cases := []struct {
		desired    string
		client     string
		migrate    bool
		reschedule bool
		terminal   bool
		lost       bool
	}{
		{
			desired: structs.AllocDesiredStatusStop,
			client:  structs.AllocClientStatusRunning,
			lost:    true,
		},
		{
			desired: structs.AllocDesiredStatusRun,
			client:  structs.AllocClientStatusPending,
			migrate: true,
		},
		{
			desired: structs.AllocDesiredStatusRun,
			client:  structs.AllocClientStatusRunning,
			migrate: true,
		},
		{
			desired:  structs.AllocDesiredStatusRun,
			client:   structs.AllocClientStatusLost,
			terminal: true,
		},
		{
			desired:  structs.AllocDesiredStatusRun,
			client:   structs.AllocClientStatusComplete,
			terminal: true,
		},
		{
			desired:    structs.AllocDesiredStatusRun,
			client:     structs.AllocClientStatusFailed,
			reschedule: true,
		},
		{
			desired: structs.AllocDesiredStatusEvict,
			client:  structs.AllocClientStatusRunning,
			lost:    true,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf(""), func(t *testing.T) {
			h := NewHarness(t)

			// Register a node
			node := mock.Node()
			node.Status = structs.NodeStatusDown
			require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

			// Generate a fake job with allocations and an update policy.
			job := mock.Job()
			require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

			alloc := mock.Alloc()
			alloc.Job = job
			alloc.JobID = job.ID
			alloc.NodeID = node.ID
			alloc.Name = fmt.Sprintf("my-job.web[%d]", i)

			alloc.DesiredStatus = tc.desired
			alloc.ClientStatus = tc.client

			// Mark for migration if necessary
			alloc.DesiredTransition.Migrate = helper.BoolToPtr(tc.migrate)

			allocs := []*structs.Allocation{alloc}
			require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

			// Create a mock evaluation to deal with drain
			eval := &structs.Evaluation{
				Namespace:   structs.DefaultNamespace,
				ID:          uuid.Generate(),
				Priority:    50,
				TriggeredBy: structs.EvalTriggerNodeUpdate,
				JobID:       job.ID,
				NodeID:      node.ID,
				Status:      structs.EvalStatusPending,
			}
			require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

			// Process the evaluation
			err := h.Process(NewServiceScheduler, eval)
			require.NoError(t, err)

			if tc.terminal {
				// No plan for terminal state allocs
				require.Len(t, h.Plans, 0)
			} else {
				require.Len(t, h.Plans, 1)

				plan := h.Plans[0]
				out := plan.NodeUpdate[node.ID]
				require.Len(t, out, 1)

				outAlloc := out[0]
				if tc.migrate {
					require.NotEqual(t, structs.AllocClientStatusLost, outAlloc.ClientStatus)
				} else if tc.reschedule {
					require.Equal(t, structs.AllocClientStatusFailed, outAlloc.ClientStatus)
				} else if tc.lost {
					require.Equal(t, structs.AllocClientStatusLost, outAlloc.ClientStatus)
				} else {
					require.Fail(t, "unexpected alloc update")
				}
			}

			h.AssertEvalStatus(t, structs.EvalStatusComplete)
		})
	}
}

func TestServiceSched_NodeUpdate(t *testing.T) {
	h := NewHarness(t)

	// Register a node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Mark some allocs as running
	ws := memdb.NewWatchSet()
	for i := 0; i < 4; i++ {
		out, _ := h.State.AllocByID(ws, allocs[i].ID)
		out.ClientStatus = structs.AllocClientStatusRunning
		require.NoError(t, h.State.UpdateAllocsFromClient(h.NextIndex(), []*structs.Allocation{out}))
	}

	// Create a mock evaluation which won't trigger any new placements
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		NodeID:      node.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if val, ok := h.Evals[0].QueuedAllocations["web"]; !ok || val != 0 {
		t.Fatalf("bad queued allocations: %v", h.Evals[0].QueuedAllocations)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_NodeDrain(t *testing.T) {
	h := NewHarness(t)

	// Register a draining node
	node := mock.Node()
	node.Drain = true
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		alloc.DesiredTransition.Migrate = helper.BoolToPtr(true)
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		NodeID:      node.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan evicted all allocs
	if len(plan.NodeUpdate[node.ID]) != len(allocs) {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 10 {
		t.Fatalf("bad: %#v", plan)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	out, _ = structs.FilterTerminalAllocs(out)
	if len(out) != 10 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_NodeDrain_Down(t *testing.T) {
	h := NewHarness(t)

	// Register a draining node
	node := mock.Node()
	node.Drain = true
	node.Status = structs.NodeStatusDown
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job with allocations
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Set the desired state of the allocs to stop
	var stop []*structs.Allocation
	for i := 0; i < 6; i++ {
		newAlloc := allocs[i].Copy()
		newAlloc.ClientStatus = structs.AllocDesiredStatusStop
		newAlloc.DesiredTransition.Migrate = helper.BoolToPtr(true)
		stop = append(stop, newAlloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), stop))

	// Mark some of the allocations as running
	var running []*structs.Allocation
	for i := 4; i < 6; i++ {
		newAlloc := stop[i].Copy()
		newAlloc.ClientStatus = structs.AllocClientStatusRunning
		running = append(running, newAlloc)
	}
	require.NoError(t, h.State.UpdateAllocsFromClient(h.NextIndex(), running))

	// Mark some of the allocations as complete
	var complete []*structs.Allocation
	for i := 6; i < 10; i++ {
		newAlloc := allocs[i].Copy()
		newAlloc.TaskStates = make(map[string]*structs.TaskState)
		newAlloc.TaskStates["web"] = &structs.TaskState{
			State: structs.TaskStateDead,
			Events: []*structs.TaskEvent{
				{
					Type:     structs.TaskTerminated,
					ExitCode: 0,
				},
			},
		}
		newAlloc.ClientStatus = structs.AllocClientStatusComplete
		complete = append(complete, newAlloc)
	}
	require.NoError(t, h.State.UpdateAllocsFromClient(h.NextIndex(), complete))

	// Create a mock evaluation to deal with the node update
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		NodeID:      node.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan evicted non terminal allocs
	if len(plan.NodeUpdate[node.ID]) != 6 {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure that all the allocations which were in running or pending state
	// has been marked as lost
	var lostAllocs []string
	for _, alloc := range plan.NodeUpdate[node.ID] {
		lostAllocs = append(lostAllocs, alloc.ID)
	}
	sort.Strings(lostAllocs)

	var expectedLostAllocs []string
	for i := 0; i < 6; i++ {
		expectedLostAllocs = append(expectedLostAllocs, allocs[i].ID)
	}
	sort.Strings(expectedLostAllocs)

	if !reflect.DeepEqual(expectedLostAllocs, lostAllocs) {
		t.Fatalf("expected: %v, actual: %v", expectedLostAllocs, lostAllocs)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_NodeDrain_Queued_Allocations(t *testing.T) {
	h := NewHarness(t)

	// Register a draining node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	job.TaskGroups[0].Count = 2
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 2; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		alloc.DesiredTransition.Migrate = helper.BoolToPtr(true)
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	node.Drain = true
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		NodeID:      node.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	queued := h.Evals[0].QueuedAllocations["web"]
	if queued != 2 {
		t.Fatalf("expected: %v, actual: %v", 2, queued)
	}
}

func TestServiceSched_RetryLimit(t *testing.T) {
	h := NewHarness(t)
	h.Planner = &RejectPlan{h}

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure multiple plans
	if len(h.Plans) == 0 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure no allocations placed
	if len(out) != 0 {
		t.Fatalf("bad: %#v", out)
	}

	// Should hit the retry limit
	h.AssertEvalStatus(t, structs.EvalStatusFailed)
}

func TestServiceSched_Reschedule_OnceNow(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	job.TaskGroups[0].Count = 2
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts:      1,
		Interval:      15 * time.Minute,
		Delay:         5 * time.Second,
		MaxDelay:      1 * time.Minute,
		DelayFunction: "constant",
	}
	tgName := job.TaskGroups[0].Name
	now := time.Now()

	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 2; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	// Mark one of the allocations as failed
	allocs[1].ClientStatus = structs.AllocClientStatusFailed
	allocs[1].TaskStates = map[string]*structs.TaskState{tgName: {State: "dead",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-10 * time.Second)}}
	failedAllocID := allocs[1].ID
	successAllocID := allocs[0].ID

	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create a mock evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure multiple plans
	if len(h.Plans) == 0 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Verify that one new allocation got created with its restart tracker info
	assert := assert.New(t)
	assert.Equal(3, len(out))
	var newAlloc *structs.Allocation
	for _, alloc := range out {
		if alloc.ID != successAllocID && alloc.ID != failedAllocID {
			newAlloc = alloc
		}
	}
	assert.Equal(failedAllocID, newAlloc.PreviousAllocation)
	assert.Equal(1, len(newAlloc.RescheduleTracker.Events))
	assert.Equal(failedAllocID, newAlloc.RescheduleTracker.Events[0].PrevAllocID)

	// Mark this alloc as failed again, should not get rescheduled
	newAlloc.ClientStatus = structs.AllocClientStatusFailed

	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{newAlloc}))

	// Create another mock evaluation
	eval = &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err = h.Process(NewServiceScheduler, eval)
	assert.Nil(err)
	// Verify no new allocs were created this time
	out, err = h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)
	assert.Equal(3, len(out))

}

// Tests that alloc reschedulable at a future time creates a follow up eval
func TestServiceSched_Reschedule_Later(t *testing.T) {
	h := NewHarness(t)
	require := require.New(t)
	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	job.TaskGroups[0].Count = 2
	delayDuration := 15 * time.Second
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts:      1,
		Interval:      15 * time.Minute,
		Delay:         delayDuration,
		MaxDelay:      1 * time.Minute,
		DelayFunction: "constant",
	}
	tgName := job.TaskGroups[0].Name
	now := time.Now()

	require.NoError(h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 2; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	// Mark one of the allocations as failed
	allocs[1].ClientStatus = structs.AllocClientStatusFailed
	allocs[1].TaskStates = map[string]*structs.TaskState{tgName: {State: "dead",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now}}
	failedAllocID := allocs[1].ID

	require.NoError(h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create a mock evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure multiple plans
	if len(h.Plans) == 0 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(err)

	// Verify no new allocs were created
	require.Equal(2, len(out))

	// Verify follow up eval was created for the failed alloc
	alloc, err := h.State.AllocByID(ws, failedAllocID)
	require.Nil(err)
	require.NotEmpty(alloc.FollowupEvalID)

	// Ensure there is a follow up eval.
	if len(h.CreateEvals) != 1 || h.CreateEvals[0].Status != structs.EvalStatusPending {
		t.Fatalf("bad: %#v", h.CreateEvals)
	}
	followupEval := h.CreateEvals[0]
	require.Equal(now.Add(delayDuration), followupEval.WaitUntil)
}

func TestServiceSched_Reschedule_MultipleNow(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	maxRestartAttempts := 3
	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	job.TaskGroups[0].Count = 2
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts:      maxRestartAttempts,
		Interval:      30 * time.Minute,
		Delay:         5 * time.Second,
		DelayFunction: "constant",
	}
	tgName := job.TaskGroups[0].Name
	now := time.Now()

	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 2; i++ {
		alloc := mock.Alloc()
		alloc.ClientStatus = structs.AllocClientStatusRunning
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	// Mark one of the allocations as failed
	allocs[1].ClientStatus = structs.AllocClientStatusFailed
	allocs[1].TaskStates = map[string]*structs.TaskState{tgName: {State: "dead",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-10 * time.Second)}}

	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create a mock evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	expectedNumAllocs := 3
	expectedNumReschedTrackers := 1

	failedAllocId := allocs[1].ID
	failedNodeID := allocs[1].NodeID

	assert := assert.New(t)
	for i := 0; i < maxRestartAttempts; i++ {
		// Process the evaluation
		err := h.Process(NewServiceScheduler, eval)
		require.NoError(t, err)

		// Ensure multiple plans
		if len(h.Plans) == 0 {
			t.Fatalf("bad: %#v", h.Plans)
		}

		// Lookup the allocations by JobID
		ws := memdb.NewWatchSet()
		out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
		require.NoError(t, err)

		// Verify that a new allocation got created with its restart tracker info
		assert.Equal(expectedNumAllocs, len(out))

		// Find the new alloc with ClientStatusPending
		var pendingAllocs []*structs.Allocation
		var prevFailedAlloc *structs.Allocation

		for _, alloc := range out {
			if alloc.ClientStatus == structs.AllocClientStatusPending {
				pendingAllocs = append(pendingAllocs, alloc)
			}
			if alloc.ID == failedAllocId {
				prevFailedAlloc = alloc
			}
		}
		assert.Equal(1, len(pendingAllocs))
		newAlloc := pendingAllocs[0]
		assert.Equal(expectedNumReschedTrackers, len(newAlloc.RescheduleTracker.Events))

		// Verify the previous NodeID in the most recent reschedule event
		reschedEvents := newAlloc.RescheduleTracker.Events
		assert.Equal(failedAllocId, reschedEvents[len(reschedEvents)-1].PrevAllocID)
		assert.Equal(failedNodeID, reschedEvents[len(reschedEvents)-1].PrevNodeID)

		// Verify that the next alloc of the failed alloc is the newly rescheduled alloc
		assert.Equal(newAlloc.ID, prevFailedAlloc.NextAllocation)

		// Mark this alloc as failed again
		newAlloc.ClientStatus = structs.AllocClientStatusFailed
		newAlloc.TaskStates = map[string]*structs.TaskState{tgName: {State: "dead",
			StartedAt:  now.Add(-12 * time.Second),
			FinishedAt: now.Add(-10 * time.Second)}}

		failedAllocId = newAlloc.ID
		failedNodeID = newAlloc.NodeID

		require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{newAlloc}))

		// Create another mock evaluation
		eval = &structs.Evaluation{
			Namespace:   structs.DefaultNamespace,
			ID:          uuid.Generate(),
			Priority:    50,
			TriggeredBy: structs.EvalTriggerNodeUpdate,
			JobID:       job.ID,
			Status:      structs.EvalStatusPending,
		}
		require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))
		expectedNumAllocs += 1
		expectedNumReschedTrackers += 1
	}

	// Process last eval again, should not reschedule
	err := h.Process(NewServiceScheduler, eval)
	assert.Nil(err)

	// Verify no new allocs were created because restart attempts were exhausted
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)
	assert.Equal(5, len(out)) // 2 original, plus 3 reschedule attempts
}

// Tests that old reschedule attempts are pruned
func TestServiceSched_Reschedule_PruneEvents(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	job.TaskGroups[0].Count = 2
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		DelayFunction: "exponential",
		MaxDelay:      1 * time.Hour,
		Delay:         5 * time.Second,
		Unlimited:     true,
	}
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 2; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	now := time.Now()
	// Mark allocations as failed with restart info
	allocs[1].TaskStates = map[string]*structs.TaskState{job.TaskGroups[0].Name: {State: "dead",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-15 * time.Minute)}}
	allocs[1].ClientStatus = structs.AllocClientStatusFailed

	allocs[1].RescheduleTracker = &structs.RescheduleTracker{
		Events: []*structs.RescheduleEvent{
			{RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
				PrevAllocID: uuid.Generate(),
				PrevNodeID:  uuid.Generate(),
				Delay:       5 * time.Second,
			},
			{RescheduleTime: now.Add(-40 * time.Minute).UTC().UnixNano(),
				PrevAllocID: allocs[0].ID,
				PrevNodeID:  uuid.Generate(),
				Delay:       10 * time.Second,
			},
			{RescheduleTime: now.Add(-30 * time.Minute).UTC().UnixNano(),
				PrevAllocID: allocs[0].ID,
				PrevNodeID:  uuid.Generate(),
				Delay:       20 * time.Second,
			},
			{RescheduleTime: now.Add(-20 * time.Minute).UTC().UnixNano(),
				PrevAllocID: allocs[0].ID,
				PrevNodeID:  uuid.Generate(),
				Delay:       40 * time.Second,
			},
			{RescheduleTime: now.Add(-10 * time.Minute).UTC().UnixNano(),
				PrevAllocID: allocs[0].ID,
				PrevNodeID:  uuid.Generate(),
				Delay:       80 * time.Second,
			},
			{RescheduleTime: now.Add(-3 * time.Minute).UTC().UnixNano(),
				PrevAllocID: allocs[0].ID,
				PrevNodeID:  uuid.Generate(),
				Delay:       160 * time.Second,
			},
		},
	}
	expectedFirstRescheduleEvent := allocs[1].RescheduleTracker.Events[1]
	expectedDelay := 320 * time.Second
	failedAllocID := allocs[1].ID
	successAllocID := allocs[0].ID

	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create a mock evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure multiple plans
	if len(h.Plans) == 0 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Verify that one new allocation got created with its restart tracker info
	assert := assert.New(t)
	assert.Equal(3, len(out))
	var newAlloc *structs.Allocation
	for _, alloc := range out {
		if alloc.ID != successAllocID && alloc.ID != failedAllocID {
			newAlloc = alloc
		}
	}

	assert.Equal(failedAllocID, newAlloc.PreviousAllocation)
	// Verify that the new alloc copied the last 5 reschedule attempts
	assert.Equal(6, len(newAlloc.RescheduleTracker.Events))
	assert.Equal(expectedFirstRescheduleEvent, newAlloc.RescheduleTracker.Events[0])

	mostRecentRescheduleEvent := newAlloc.RescheduleTracker.Events[5]
	// Verify that the failed alloc ID is in the most recent reschedule event
	assert.Equal(failedAllocID, mostRecentRescheduleEvent.PrevAllocID)
	// Verify that the delay value was captured correctly
	assert.Equal(expectedDelay, mostRecentRescheduleEvent.Delay)

}

// Tests that deployments with failed allocs result in placements as long as the
// deployment is running.
func TestDeployment_FailedAllocs_Reschedule(t *testing.T) {
	for _, failedDeployment := range []bool{false, true} {
		t.Run(fmt.Sprintf("Failed Deployment: %v", failedDeployment), func(t *testing.T) {
			h := NewHarness(t)
			require := require.New(t)
			// Create some nodes
			var nodes []*structs.Node
			for i := 0; i < 10; i++ {
				node := mock.Node()
				nodes = append(nodes, node)
				require.NoError(h.State.UpsertNode(h.NextIndex(), node))
			}

			// Generate a fake job with allocations and a reschedule policy.
			job := mock.Job()
			job.TaskGroups[0].Count = 2
			job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
				Attempts: 1,
				Interval: 15 * time.Minute,
			}
			jobIndex := h.NextIndex()
			require.Nil(h.State.UpsertJob(jobIndex, job))

			deployment := mock.Deployment()
			deployment.JobID = job.ID
			deployment.JobCreateIndex = jobIndex
			deployment.JobVersion = job.Version
			if failedDeployment {
				deployment.Status = structs.DeploymentStatusFailed
			}

			require.Nil(h.State.UpsertDeployment(h.NextIndex(), deployment))

			var allocs []*structs.Allocation
			for i := 0; i < 2; i++ {
				alloc := mock.Alloc()
				alloc.Job = job
				alloc.JobID = job.ID
				alloc.NodeID = nodes[i].ID
				alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
				alloc.DeploymentID = deployment.ID
				allocs = append(allocs, alloc)
			}
			// Mark one of the allocations as failed in the past
			allocs[1].ClientStatus = structs.AllocClientStatusFailed
			allocs[1].TaskStates = map[string]*structs.TaskState{"web": {State: "start",
				StartedAt:  time.Now().Add(-12 * time.Hour),
				FinishedAt: time.Now().Add(-10 * time.Hour)}}
			allocs[1].DesiredTransition.Reschedule = helper.BoolToPtr(true)

			require.Nil(h.State.UpsertAllocs(h.NextIndex(), allocs))

			// Create a mock evaluation
			eval := &structs.Evaluation{
				Namespace:   structs.DefaultNamespace,
				ID:          uuid.Generate(),
				Priority:    50,
				TriggeredBy: structs.EvalTriggerNodeUpdate,
				JobID:       job.ID,
				Status:      structs.EvalStatusPending,
			}
			require.Nil(h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

			// Process the evaluation
			require.Nil(h.Process(NewServiceScheduler, eval))

			if failedDeployment {
				// Verify no plan created
				require.Len(h.Plans, 0)
			} else {
				require.Len(h.Plans, 1)
				plan := h.Plans[0]

				// Ensure the plan allocated
				var planned []*structs.Allocation
				for _, allocList := range plan.NodeAllocation {
					planned = append(planned, allocList...)
				}
				if len(planned) != 1 {
					t.Fatalf("bad: %#v", plan)
				}
			}
		})
	}
}

func TestBatchSched_Run_CompleteAlloc(t *testing.T) {
	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a job
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 1
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a complete alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.ClientStatus = structs.AllocClientStatusComplete
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewBatchScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure no plan as it should be a no-op
	if len(h.Plans) != 0 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure no allocations placed
	if len(out) != 1 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestBatchSched_Run_FailedAlloc(t *testing.T) {
	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a job
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 1
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	tgName := job.TaskGroups[0].Name
	now := time.Now()

	// Create a failed alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.ClientStatus = structs.AllocClientStatusFailed
	alloc.TaskStates = map[string]*structs.TaskState{tgName: {State: "dead",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-10 * time.Second)}}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewBatchScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure a replacement alloc was placed.
	if len(out) != 2 {
		t.Fatalf("bad: %#v", out)
	}

	// Ensure that the scheduler is recording the correct number of queued
	// allocations
	queued := h.Evals[0].QueuedAllocations["web"]
	if queued != 0 {
		t.Fatalf("expected: %v, actual: %v", 1, queued)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestBatchSched_Run_LostAlloc(t *testing.T) {
	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a job
	job := mock.Job()
	job.ID = "my-job"
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 3
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Desired = 3
	// Mark one as lost and then schedule
	// [(0, run, running), (1, run, running), (1, stop, lost)]

	// Create two running allocations
	var allocs []*structs.Allocation
	for i := 0; i <= 1; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		alloc.ClientStatus = structs.AllocClientStatusRunning
		allocs = append(allocs, alloc)
	}

	// Create a failed alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[1]"
	alloc.DesiredStatus = structs.AllocDesiredStatusStop
	alloc.ClientStatus = structs.AllocClientStatusComplete
	allocs = append(allocs, alloc)
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewBatchScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure a replacement alloc was placed.
	if len(out) != 4 {
		t.Fatalf("bad: %#v", out)
	}

	// Assert that we have the correct number of each alloc name
	expected := map[string]int{
		"my-job.web[0]": 1,
		"my-job.web[1]": 2,
		"my-job.web[2]": 1,
	}
	actual := make(map[string]int, 3)
	for _, alloc := range out {
		actual[alloc.Name] += 1
	}
	require.Equal(t, actual, expected)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestBatchSched_Run_FailedAllocQueuedAllocations(t *testing.T) {
	h := NewHarness(t)

	node := mock.Node()
	node.Drain = true
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a job
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 1
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	tgName := job.TaskGroups[0].Name
	now := time.Now()

	// Create a failed alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.ClientStatus = structs.AllocClientStatusFailed
	alloc.TaskStates = map[string]*structs.TaskState{tgName: {State: "dead",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-10 * time.Second)}}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewBatchScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure that the scheduler is recording the correct number of queued
	// allocations
	queued := h.Evals[0].QueuedAllocations["web"]
	if queued != 1 {
		t.Fatalf("expected: %v, actual: %v", 1, queued)
	}
}

func TestBatchSched_ReRun_SuccessfullyFinishedAlloc(t *testing.T) {
	h := NewHarness(t)

	// Create two nodes, one that is drained and has a successfully finished
	// alloc and a fresh undrained one
	node := mock.Node()
	node.Drain = true
	node2 := mock.Node()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node2))

	// Create a job
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 1
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a successful alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.ClientStatus = structs.AllocClientStatusComplete
	alloc.TaskStates = map[string]*structs.TaskState{
		"web": {
			State: structs.TaskStateDead,
			Events: []*structs.TaskEvent{
				{
					Type:     structs.TaskTerminated,
					ExitCode: 0,
				},
			},
		},
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to rerun the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewBatchScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure no plan
	if len(h.Plans) != 0 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure no replacement alloc was placed.
	if len(out) != 1 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

// This test checks that terminal allocations that receive an in-place updated
// are not added to the plan
func TestBatchSched_JobModify_InPlace_Terminal(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		alloc.ClientStatus = structs.AllocClientStatusComplete
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create a mock evaluation to trigger the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewBatchScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure no plan
	if len(h.Plans) != 0 {
		t.Fatalf("bad: %#v", h.Plans[0])
	}
}

// This test ensures that terminal jobs from older versions are ignored.
func TestBatchSched_JobModify_Destructive_Terminal(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		alloc.ClientStatus = structs.AllocClientStatusComplete
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Update the job
	job2 := mock.Job()
	job2.ID = job.ID
	job2.Type = structs.JobTypeBatch
	job2.Version++
	job2.TaskGroups[0].Tasks[0].Env = map[string]string{"foo": "bar"}
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job2))

	allocs = nil
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job2
		alloc.JobID = job2.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		alloc.ClientStatus = structs.AllocClientStatusComplete
		alloc.TaskStates = map[string]*structs.TaskState{
			"web": {
				State: structs.TaskStateDead,
				Events: []*structs.TaskEvent{
					{
						Type:     structs.TaskTerminated,
						ExitCode: 0,
					},
				},
			},
		}
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewBatchScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a plan
	if len(h.Plans) != 0 {
		t.Fatalf("bad: %#v", h.Plans)
	}
}

// This test asserts that an allocation from an old job that is running on a
// drained node is cleaned up.
func TestBatchSched_NodeDrain_Running_OldJob(t *testing.T) {
	h := NewHarness(t)

	// Create two nodes, one that is drained and has a successfully finished
	// alloc and a fresh undrained one
	node := mock.Node()
	node.Drain = true
	node2 := mock.Node()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node2))

	// Create a job
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 1
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a running alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.ClientStatus = structs.AllocClientStatusRunning
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create an update job
	job2 := job.Copy()
	job2.TaskGroups[0].Tasks[0].Env = map[string]string{"foo": "bar"}
	job2.Version++
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job2))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewBatchScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	plan := h.Plans[0]

	// Ensure the plan evicted 1
	if len(plan.NodeUpdate[node.ID]) != 1 {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure the plan places 1
	if len(plan.NodeAllocation[node2.ID]) != 1 {
		t.Fatalf("bad: %#v", plan)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

// This test asserts that an allocation from a job that is complete on a
// drained node is ignored up.
func TestBatchSched_NodeDrain_Complete(t *testing.T) {
	h := NewHarness(t)

	// Create two nodes, one that is drained and has a successfully finished
	// alloc and a fresh undrained one
	node := mock.Node()
	node.Drain = true
	node2 := mock.Node()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node2))

	// Create a job
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 1
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a complete alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.ClientStatus = structs.AllocClientStatusComplete
	alloc.TaskStates = make(map[string]*structs.TaskState)
	alloc.TaskStates["web"] = &structs.TaskState{
		State: structs.TaskStateDead,
		Events: []*structs.TaskEvent{
			{
				Type:     structs.TaskTerminated,
				ExitCode: 0,
			},
		},
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewBatchScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure no plan
	if len(h.Plans) != 0 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

// This is a slightly odd test but it ensures that we handle a scale down of a
// task group's count and that it works even if all the allocs have the same
// name.
func TestBatchSched_ScaleDown_SameName(t *testing.T) {
	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a job
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 1
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	scoreMetric := &structs.AllocMetric{
		NodesEvaluated: 10,
		NodesFiltered:  3,
		ScoreMetaData: []*structs.NodeScoreMeta{
			{
				NodeID: node.ID,
				Scores: map[string]float64{
					"bin-packing": 0.5435,
				},
			},
		},
	}
	// Create a few running alloc
	var allocs []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-job.web[0]"
		alloc.ClientStatus = structs.AllocClientStatusRunning
		alloc.Metrics = scoreMetric
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Update the job's modify index to force an inplace upgrade
	updatedJob := job.Copy()
	updatedJob.JobModifyIndex = job.JobModifyIndex + 1
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), updatedJob))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewBatchScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	plan := h.Plans[0]

	require := require.New(t)
	// Ensure the plan evicted 4 of the 5
	require.Equal(4, len(plan.NodeUpdate[node.ID]))

	// Ensure that the scheduler did not overwrite the original score metrics for the i
	for _, inPlaceAllocs := range plan.NodeAllocation {
		for _, alloc := range inPlaceAllocs {
			require.Equal(scoreMetric, alloc.Metrics)
		}
	}
	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestGenericSched_AllocFit(t *testing.T) {
	testCases := []struct {
		Name             string
		NodeCpu          int64
		TaskResources    structs.Resources
		MainTaskCount    int
		InitTaskCount    int
		SideTaskCount    int
		ShouldPlaceAlloc bool
	}{
		{
			Name:    "simple init + sidecar",
			NodeCpu: 1200,
			TaskResources: structs.Resources{
				CPU:      500,
				MemoryMB: 256,
			},
			MainTaskCount:    1,
			InitTaskCount:    1,
			SideTaskCount:    1,
			ShouldPlaceAlloc: true,
		},
		{
			Name:    "too big init + sidecar",
			NodeCpu: 1200,
			TaskResources: structs.Resources{
				CPU:      700,
				MemoryMB: 256,
			},
			MainTaskCount:    1,
			InitTaskCount:    1,
			SideTaskCount:    1,
			ShouldPlaceAlloc: false,
		},
		{
			Name:    "many init + sidecar",
			NodeCpu: 1200,
			TaskResources: structs.Resources{
				CPU:      100,
				MemoryMB: 100,
			},
			MainTaskCount:    3,
			InitTaskCount:    5,
			SideTaskCount:    5,
			ShouldPlaceAlloc: true,
		},
		{
			Name:    "too many init + sidecar",
			NodeCpu: 1200,
			TaskResources: structs.Resources{
				CPU:      100,
				MemoryMB: 100,
			},
			MainTaskCount:    10,
			InitTaskCount:    10,
			SideTaskCount:    10,
			ShouldPlaceAlloc: false,
		},
		{
			Name:    "too many too big",
			NodeCpu: 1200,
			TaskResources: structs.Resources{
				CPU:      1000,
				MemoryMB: 100,
			},
			MainTaskCount:    10,
			InitTaskCount:    10,
			SideTaskCount:    10,
			ShouldPlaceAlloc: false,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			h := NewHarness(t)
			node := mock.Node()
			node.NodeResources.Cpu.CpuShares = testCase.NodeCpu
			require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

			// Create a job with sidecar & init tasks
			job := mock.VariableLifecycleJob(testCase.TaskResources, testCase.MainTaskCount, testCase.InitTaskCount, testCase.SideTaskCount)

			require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

			// Create a mock evaluation to register the job
			eval := &structs.Evaluation{
				Namespace:   structs.DefaultNamespace,
				ID:          uuid.Generate(),
				Priority:    job.Priority,
				TriggeredBy: structs.EvalTriggerJobRegister,
				JobID:       job.ID,
				Status:      structs.EvalStatusPending,
			}
			require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

			// Process the evaluation
			err := h.Process(NewServiceScheduler, eval)
			require.NoError(t, err)

			allocs := 0
			if testCase.ShouldPlaceAlloc {
				allocs = 1
			}
			// Ensure no plan as it should be a no-op
			require.Len(t, h.Plans, allocs)

			// Lookup the allocations by JobID
			ws := memdb.NewWatchSet()
			out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
			require.NoError(t, err)

			// Ensure no allocations placed
			require.Len(t, out, allocs)

			h.AssertEvalStatus(t, structs.EvalStatusComplete)
		})
	}
}

func TestGenericSched_ChainedAlloc(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))
	// Process the evaluation
	if err := h.Process(NewServiceScheduler, eval); err != nil {
		t.Fatalf("err: %v", err)
	}

	var allocIDs []string
	for _, allocList := range h.Plans[0].NodeAllocation {
		for _, alloc := range allocList {
			allocIDs = append(allocIDs, alloc.ID)
		}
	}
	sort.Strings(allocIDs)

	// Create a new harness to invoke the scheduler again
	h1 := NewHarnessWithState(t, h.State)
	job1 := mock.Job()
	job1.ID = job.ID
	job1.TaskGroups[0].Tasks[0].Env["foo"] = "bar"
	job1.TaskGroups[0].Count = 12
	require.NoError(t, h1.State.UpsertJob(h1.NextIndex(), job1))

	// Create a mock evaluation to update the job
	eval1 := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job1.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job1.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval1}))

	// Process the evaluation
	if err := h1.Process(NewServiceScheduler, eval1); err != nil {
		t.Fatalf("err: %v", err)
	}

	plan := h1.Plans[0]

	// Collect all the chained allocation ids and the new allocations which
	// don't have any chained allocations
	var prevAllocs []string
	var newAllocs []string
	for _, allocList := range plan.NodeAllocation {
		for _, alloc := range allocList {
			if alloc.PreviousAllocation == "" {
				newAllocs = append(newAllocs, alloc.ID)
				continue
			}
			prevAllocs = append(prevAllocs, alloc.PreviousAllocation)
		}
	}
	sort.Strings(prevAllocs)

	// Ensure that the new allocations has their corresponding original
	// allocation ids
	if !reflect.DeepEqual(prevAllocs, allocIDs) {
		t.Fatalf("expected: %v, actual: %v", len(allocIDs), len(prevAllocs))
	}

	// Ensuring two new allocations don't have any chained allocations
	if len(newAllocs) != 2 {
		t.Fatalf("expected: %v, actual: %v", 2, len(newAllocs))
	}
}

func TestServiceSched_NodeDrain_Sticky(t *testing.T) {
	h := NewHarness(t)

	// Register a draining node
	node := mock.Node()
	node.Drain = true
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create an alloc on the draining node
	alloc := mock.Alloc()
	alloc.Name = "my-job.web[0]"
	alloc.NodeID = node.ID
	alloc.Job.TaskGroups[0].Count = 1
	alloc.Job.TaskGroups[0].EphemeralDisk.Sticky = true
	alloc.DesiredTransition.Migrate = helper.BoolToPtr(true)
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), alloc.Job))
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       alloc.Job.ID,
		NodeID:      node.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan evicted all allocs
	if len(plan.NodeUpdate[node.ID]) != 1 {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure the plan didn't create any new allocations
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 0 {
		t.Fatalf("bad: %#v", plan)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

// This test ensures that when a job is stopped, the scheduler properly cancels
// an outstanding deployment.
func TestServiceSched_CancelDeployment_Stopped(t *testing.T) {
	h := NewHarness(t)

	// Generate a fake job
	job := mock.Job()
	job.JobModifyIndex = job.CreateIndex + 1
	job.ModifyIndex = job.CreateIndex + 1
	job.Stop = true
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a deployment
	d := mock.Deployment()
	d.JobID = job.ID
	d.JobCreateIndex = job.CreateIndex
	d.JobModifyIndex = job.JobModifyIndex - 1
	require.NoError(t, h.State.UpsertDeployment(h.NextIndex(), d))

	// Create a mock evaluation to deregister the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobDeregister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan cancelled the existing deployment
	ws := memdb.NewWatchSet()
	out, err := h.State.LatestDeploymentByJobID(ws, job.Namespace, job.ID)
	require.NoError(t, err)

	if out == nil {
		t.Fatalf("No deployment for job")
	}
	if out.ID != d.ID {
		t.Fatalf("Latest deployment for job is different than original deployment")
	}
	if out.Status != structs.DeploymentStatusCancelled {
		t.Fatalf("Deployment status is %q, want %q", out.Status, structs.DeploymentStatusCancelled)
	}
	if out.StatusDescription != structs.DeploymentStatusDescriptionStoppedJob {
		t.Fatalf("Deployment status description is %q, want %q",
			out.StatusDescription, structs.DeploymentStatusDescriptionStoppedJob)
	}

	// Ensure the plan didn't allocate anything
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 0 {
		t.Fatalf("bad: %#v", plan)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

// This test ensures that when a job is updated and had an old deployment, the scheduler properly cancels
// the deployment.
func TestServiceSched_CancelDeployment_NewerJob(t *testing.T) {
	h := NewHarness(t)

	// Generate a fake job
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a deployment for an old version of the job
	d := mock.Deployment()
	d.JobID = job.ID
	require.NoError(t, h.State.UpsertDeployment(h.NextIndex(), d))

	// Upsert again to bump job version
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to kick the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan cancelled the existing deployment
	ws := memdb.NewWatchSet()
	out, err := h.State.LatestDeploymentByJobID(ws, job.Namespace, job.ID)
	require.NoError(t, err)

	if out == nil {
		t.Fatalf("No deployment for job")
	}
	if out.ID != d.ID {
		t.Fatalf("Latest deployment for job is different than original deployment")
	}
	if out.Status != structs.DeploymentStatusCancelled {
		t.Fatalf("Deployment status is %q, want %q", out.Status, structs.DeploymentStatusCancelled)
	}
	if out.StatusDescription != structs.DeploymentStatusDescriptionNewerJob {
		t.Fatalf("Deployment status description is %q, want %q",
			out.StatusDescription, structs.DeploymentStatusDescriptionNewerJob)
	}
	// Ensure the plan didn't allocate anything
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 0 {
		t.Fatalf("bad: %#v", plan)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

// Various table driven tests for carry forward
// of past reschedule events
func Test_updateRescheduleTracker(t *testing.T) {

	t1 := time.Now().UTC()
	alloc := mock.Alloc()
	prevAlloc := mock.Alloc()

	type testCase struct {
		desc                     string
		prevAllocEvents          []*structs.RescheduleEvent
		reschedPolicy            *structs.ReschedulePolicy
		expectedRescheduleEvents []*structs.RescheduleEvent
		reschedTime              time.Time
	}

	testCases := []testCase{
		{
			desc:            "No past events",
			prevAllocEvents: nil,
			reschedPolicy:   &structs.ReschedulePolicy{Unlimited: false, Interval: 24 * time.Hour, Attempts: 2, Delay: 5 * time.Second},
			reschedTime:     t1,
			expectedRescheduleEvents: []*structs.RescheduleEvent{
				{
					RescheduleTime: t1.UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          5 * time.Second,
				},
			},
		},
		{
			desc: "one past event, linear delay",
			prevAllocEvents: []*structs.RescheduleEvent{
				{RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID: prevAlloc.ID,
					PrevNodeID:  prevAlloc.NodeID,
					Delay:       5 * time.Second}},
			reschedPolicy: &structs.ReschedulePolicy{Unlimited: false, Interval: 24 * time.Hour, Attempts: 2, Delay: 5 * time.Second},
			reschedTime:   t1,
			expectedRescheduleEvents: []*structs.RescheduleEvent{
				{
					RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          5 * time.Second,
				},
				{
					RescheduleTime: t1.UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          5 * time.Second,
				},
			},
		},
		{
			desc: "one past event, fibonacci delay",
			prevAllocEvents: []*structs.RescheduleEvent{
				{RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID: prevAlloc.ID,
					PrevNodeID:  prevAlloc.NodeID,
					Delay:       5 * time.Second}},
			reschedPolicy: &structs.ReschedulePolicy{Unlimited: false, Interval: 24 * time.Hour, Attempts: 2, Delay: 5 * time.Second, DelayFunction: "fibonacci", MaxDelay: 60 * time.Second},
			reschedTime:   t1,
			expectedRescheduleEvents: []*structs.RescheduleEvent{
				{
					RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          5 * time.Second,
				},
				{
					RescheduleTime: t1.UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          5 * time.Second,
				},
			},
		},
		{
			desc: "eight past events, fibonacci delay, unlimited",
			prevAllocEvents: []*structs.RescheduleEvent{
				{
					RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          5 * time.Second,
				},
				{
					RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          5 * time.Second,
				},
				{
					RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          10 * time.Second,
				},
				{
					RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          15 * time.Second,
				},
				{
					RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          25 * time.Second,
				},
				{
					RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          40 * time.Second,
				},
				{
					RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          65 * time.Second,
				},
				{
					RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          105 * time.Second,
				},
			},
			reschedPolicy: &structs.ReschedulePolicy{Unlimited: true, Delay: 5 * time.Second, DelayFunction: "fibonacci", MaxDelay: 240 * time.Second},
			reschedTime:   t1,
			expectedRescheduleEvents: []*structs.RescheduleEvent{
				{
					RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          15 * time.Second,
				},
				{
					RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          25 * time.Second,
				},
				{
					RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          40 * time.Second,
				},
				{
					RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          65 * time.Second,
				},
				{
					RescheduleTime: t1.Add(-1 * time.Hour).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          105 * time.Second,
				},
				{
					RescheduleTime: t1.UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          170 * time.Second,
				},
			},
		},
		{
			desc: " old attempts past interval, exponential delay, limited",
			prevAllocEvents: []*structs.RescheduleEvent{
				{
					RescheduleTime: t1.Add(-2 * time.Hour).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          5 * time.Second,
				},
				{
					RescheduleTime: t1.Add(-70 * time.Minute).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          10 * time.Second,
				},
				{
					RescheduleTime: t1.Add(-30 * time.Minute).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          20 * time.Second,
				},
				{
					RescheduleTime: t1.Add(-10 * time.Minute).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          40 * time.Second,
				},
			},
			reschedPolicy: &structs.ReschedulePolicy{Unlimited: false, Interval: 1 * time.Hour, Attempts: 5, Delay: 5 * time.Second, DelayFunction: "exponential", MaxDelay: 240 * time.Second},
			reschedTime:   t1,
			expectedRescheduleEvents: []*structs.RescheduleEvent{
				{
					RescheduleTime: t1.Add(-30 * time.Minute).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          20 * time.Second,
				},
				{
					RescheduleTime: t1.Add(-10 * time.Minute).UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          40 * time.Second,
				},
				{
					RescheduleTime: t1.UnixNano(),
					PrevAllocID:    prevAlloc.ID,
					PrevNodeID:     prevAlloc.NodeID,
					Delay:          80 * time.Second,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			require := require.New(t)
			prevAlloc.RescheduleTracker = &structs.RescheduleTracker{Events: tc.prevAllocEvents}
			prevAlloc.Job.LookupTaskGroup(prevAlloc.TaskGroup).ReschedulePolicy = tc.reschedPolicy
			updateRescheduleTracker(alloc, prevAlloc, tc.reschedTime)
			require.Equal(tc.expectedRescheduleEvents, alloc.RescheduleTracker.Events)
		})
	}

}
