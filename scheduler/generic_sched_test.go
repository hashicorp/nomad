// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceSched_JobRegister(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Create a job
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
		t.Errorf("bad: %#v", h.CreateEvals)
		if h.Evals[0].BlockedEval != "" {
			t.Fatalf("bad: %#v", h.Evals[0])
		}
		t.FailNow()
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

	// Ensure allocations have unique names derived from Job.ID
	allocNames := helper.ConvertSlice(out,
		func(alloc *structs.Allocation) string { return alloc.Name })
	expectAllocNames := []string{}
	for i := 0; i < 10; i++ {
		expectAllocNames = append(expectAllocNames, fmt.Sprintf("%s.web[%d]", job.ID, i))
	}
	must.SliceContainsAll(t, expectAllocNames, allocNames)

	// Ensure different ports were used.
	used := make(map[int]map[string]struct{})
	for _, alloc := range out {
		for _, port := range alloc.AllocatedResources.Shared.Ports {
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

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_JobRegister_StickyAllocs(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Create a job
	job := mock.Job()
	job.TaskGroups[0].EphemeralDisk.Sticky = true
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, updated))

	// Create a mock evaluation to handle the update
	eval = &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))
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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Create a job with count 2 and disk as 60GB so that only one allocation
	// can fit
	job := mock.Job()
	job.TaskGroups[0].Count = 2
	job.TaskGroups[0].EphemeralDisk.SizeMB = 88 * 1024
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Create a job that uses distinct host and has count 1 higher than what is
	// possible.
	job := mock.Job()
	job.TaskGroups[0].Count = 11
	job.Constraints = append(job.Constraints, &structs.Constraint{Operand: structs.ConstraintDistinctHosts})
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		rack := "rack2"
		if i < 5 {
			rack = "rack1"
		}
		node.Meta["rack"] = rack
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
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
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 2; i++ {
		node := mock.Node()
		node.Meta["ssd"] = "true"
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
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
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

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
	assert.Nil(h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job), "UpsertJob")

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 6; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		assert.Nil(h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node), "UpsertNode")
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
	assert.Nil(h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs), "UpsertAllocs")

	// Update the count
	job2 := job.Copy()
	job2.TaskGroups[0].Count = 6
	assert.Nil(h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2), "UpsertJob")

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

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
			job.Datacenters = []string{"dc*"}
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
			assert.Nil(h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job), "UpsertJob")
			// Create some nodes, half in dc2
			var nodes []*structs.Node
			nodeMap := make(map[string]*structs.Node)
			for i := 0; i < 10; i++ {
				node := mock.Node()
				if i%2 == 0 {
					node.Datacenter = "dc2"
				}
				// setting a narrow range makes it more likely for this test to
				// hit bugs in NetworkIndex
				node.NodeResources.MinDynamicPort = 20000
				node.NodeResources.MaxDynamicPort = 20005
				nodes = append(nodes, node)
				assert.Nil(h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node), "UpsertNode")
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
			require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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

// TestServiceSched_JobRegister_Datacenter_Downgrade tests the case where an
// allocation fails during a deployment with canaries, an the job changes its
// datacenter. The replacement for the failed alloc should be placed in the
// datacenter of the original job.
func TestServiceSched_JobRegister_Datacenter_Downgrade(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create 5 nodes in each datacenter.
	// Use two loops so nodes are separated by datacenter.
	nodes := []*structs.Node{}
	for i := 0; i < 5; i++ {
		node := mock.Node()
		node.Name = fmt.Sprintf("node-dc1-%d", i)
		node.Datacenter = "dc1"
		nodes = append(nodes, node)
		must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}
	for i := 0; i < 5; i++ {
		node := mock.Node()
		node.Name = fmt.Sprintf("node-dc2-%d", i)
		node.Datacenter = "dc2"
		nodes = append(nodes, node)
		must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Create first version of the test job running in dc1.
	job1 := mock.Job()
	job1.Version = 1
	job1.Datacenters = []string{"dc1"}
	job1.Status = structs.JobStatusRunning
	job1.TaskGroups[0].Count = 3
	job1.TaskGroups[0].Update = &structs.UpdateStrategy{
		Stagger:          time.Duration(30 * time.Second),
		MaxParallel:      1,
		HealthCheck:      "checks",
		MinHealthyTime:   time.Duration(30 * time.Second),
		HealthyDeadline:  time.Duration(9 * time.Minute),
		ProgressDeadline: time.Duration(10 * time.Minute),
		AutoRevert:       true,
		Canary:           1,
	}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job1))

	// Create allocs for this job version with one being a canary and another
	// marked as failed.
	allocs := []*structs.Allocation{}
	for i := 0; i < 3; i++ {
		alloc := mock.Alloc()
		alloc.Job = job1
		alloc.JobID = job1.ID
		alloc.NodeID = nodes[i].ID
		alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy:     pointer.Of(true),
			Timestamp:   time.Now(),
			Canary:      false,
			ModifyIndex: h.NextIndex(),
		}
		if i == 0 {
			alloc.DeploymentStatus.Canary = true
		}
		if i == 1 {
			alloc.ClientStatus = structs.AllocClientStatusFailed
		}
		allocs = append(allocs, alloc)
	}
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Update job to place it in dc2.
	job2 := job1.Copy()
	job2.Version = 2
	job2.Datacenters = []string{"dc2"}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	eval := &structs.Evaluation{
		Namespace:   job2.Namespace,
		ID:          uuid.Generate(),
		Priority:    job2.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job2.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	processErr := h.Process(NewServiceScheduler, eval)
	must.NoError(t, processErr, must.Sprint("failed to process eval"))
	must.Len(t, 1, h.Plans)

	// Verify the plan places the new allocation in dc2 and the replacement
	// for the failed allocation from the previous job version in dc1.
	for nodeID, allocs := range h.Plans[0].NodeAllocation {
		var node *structs.Node
		for _, n := range nodes {
			if n.ID == nodeID {
				node = n
				break
			}
		}

		must.Len(t, 1, allocs)
		alloc := allocs[0]
		must.SliceContains(t, alloc.Job.Datacenters, node.Datacenter, must.Sprintf(
			"alloc for job in datacenter %q placed in %q",
			alloc.Job.Datacenters,
			node.Datacenter,
		))
	}
}

// TestServiceSched_JobRegister_NodePool_Downgrade tests the case where an
// allocation fails during a deployment with canaries, where the job changes
// node pool. The failed alloc should be placed in the node pool of the
// original job.
func TestServiceSched_JobRegister_NodePool_Downgrade(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Set global scheduler configuration.
	h.State.SchedulerSetConfig(h.NextIndex(), &structs.SchedulerConfiguration{
		SchedulerAlgorithm: structs.SchedulerAlgorithmBinpack,
	})

	// Create test node pools with different scheduler algorithms.
	poolBinpack := mock.NodePool()
	poolBinpack.Name = "pool-binpack"
	poolBinpack.SchedulerConfiguration = &structs.NodePoolSchedulerConfiguration{
		SchedulerAlgorithm: structs.SchedulerAlgorithmBinpack,
	}

	poolSpread := mock.NodePool()
	poolSpread.Name = "pool-spread"
	poolSpread.SchedulerConfiguration = &structs.NodePoolSchedulerConfiguration{
		SchedulerAlgorithm: structs.SchedulerAlgorithmSpread,
	}

	nodePools := []*structs.NodePool{
		poolBinpack,
		poolSpread,
	}
	h.State.UpsertNodePools(structs.MsgTypeTestSetup, h.NextIndex(), nodePools)

	// Create 5 nodes in each node pool.
	// Use two loops so nodes are separated by node pool.
	nodes := []*structs.Node{}
	for i := 0; i < 5; i++ {
		node := mock.Node()
		node.Name = fmt.Sprintf("node-binpack-%d", i)
		node.NodePool = poolBinpack.Name
		nodes = append(nodes, node)
		must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}
	for i := 0; i < 5; i++ {
		node := mock.Node()
		node.Name = fmt.Sprintf("node-spread-%d", i)
		node.NodePool = poolSpread.Name
		nodes = append(nodes, node)
		must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Create first version of the test job running in the binpack node pool.
	job1 := mock.Job()
	job1.Version = 1
	job1.NodePool = poolBinpack.Name
	job1.Status = structs.JobStatusRunning
	job1.TaskGroups[0].Count = 3
	job1.TaskGroups[0].Update = &structs.UpdateStrategy{
		Stagger:          time.Duration(30 * time.Second),
		MaxParallel:      1,
		HealthCheck:      "checks",
		MinHealthyTime:   time.Duration(30 * time.Second),
		HealthyDeadline:  time.Duration(9 * time.Minute),
		ProgressDeadline: time.Duration(10 * time.Minute),
		AutoRevert:       true,
		Canary:           1,
	}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job1))

	// Create allocs for this job version with one being a canary and another
	// marked as failed.
	allocs := []*structs.Allocation{}
	for i := 0; i < 3; i++ {
		alloc := mock.Alloc()
		alloc.Job = job1
		alloc.JobID = job1.ID
		alloc.NodeID = nodes[i].ID
		alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy:     pointer.Of(true),
			Timestamp:   time.Now(),
			Canary:      false,
			ModifyIndex: h.NextIndex(),
		}
		if i == 0 {
			alloc.DeploymentStatus.Canary = true
		}
		if i == 1 {
			alloc.ClientStatus = structs.AllocClientStatusFailed
		}
		allocs = append(allocs, alloc)
	}
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Update job to place it in the spread node pool.
	job2 := job1.Copy()
	job2.Version = 2
	job2.NodePool = poolSpread.Name
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	eval := &structs.Evaluation{
		Namespace:   job2.Namespace,
		ID:          uuid.Generate(),
		Priority:    job2.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job2.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	processErr := h.Process(NewServiceScheduler, eval)
	require.NoError(t, processErr, "failed to process eval")
	require.Len(t, h.Plans, 1)

	// Verify the plan places the new allocation in the spread node pool and
	// the replacement failure from the previous version in the binpack pool.
	for nodeID, allocs := range h.Plans[0].NodeAllocation {
		var node *structs.Node
		for _, n := range nodes {
			if n.ID == nodeID {
				node = n
				break
			}
		}

		must.Len(t, 1, allocs)
		alloc := allocs[0]
		must.Eq(t, alloc.Job.NodePool, node.NodePool, must.Sprintf(
			"alloc for job in node pool %q placed in node in node pool %q",
			alloc.Job.NodePool,
			node.NodePool,
		))
	}
}

// Test job registration with even spread across dc
func TestServiceSched_EvenSpread(t *testing.T) {
	ci.Parallel(t)

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
	assert.Nil(h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job), "UpsertJob")
	// Create some nodes, half in dc2
	var nodes []*structs.Node
	nodeMap := make(map[string]*structs.Node)
	for i := 0; i < 10; i++ {
		node := mock.Node()
		if i%2 == 0 {
			node.Datacenter = "dc2"
		}
		nodes = append(nodes, node)
		assert.Nil(h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node), "UpsertNode")
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
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Create a job
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Create a job and set the task group count to zero.
	job := mock.Job()
	job.TaskGroups[0].Count = 0
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create NO nodes
	// Create a job
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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

	_, ok = metrics.NodesAvailable["dc1"]
	must.False(t, ok, must.Sprintf(
		"expected NodesAvailable metric to be unpopulated when there are no nodes"))

	must.Zero(t, metrics.NodesInPool, must.Sprint(
		"expected NodesInPool metric to be unpopulated when there are no nodes"))

	// Check queued allocations
	queued := outEval.QueuedAllocations["web"]
	if queued != 10 {
		t.Fatalf("expected queued: %v, actual: %v", 10, queued)
	}
	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_JobRegister_CreateBlockedEval(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create a full node
	node := mock.Node()
	node.ReservedResources = &structs.NodeReservedResources{
		Cpu: structs.NodeReservedCpuResources{
			CpuShares: node.NodeResources.Cpu.CpuShares,
		},
	}
	node.ComputeClass()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Create an ineligible node
	node2 := mock.Node()
	node2.Attributes["kernel.name"] = "windows"
	node2.ComputeClass()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node2))

	// Create a jobs
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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

	must.Eq(t, 2, metrics.NodesInPool, must.Sprint("expected NodesInPool metric to be set"))

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_JobRegister_FeasibleAndInfeasibleTG(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create one node
	node := mock.Node()
	node.NodeClass = "class_0"
	require.NoError(t, node.ComputeClass())
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

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
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))
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

func TestServiceSched_JobRegister_SchedulerAlgorithm(t *testing.T) {
	ci.Parallel(t)

	// Test node pools.
	poolNoSchedConfig := mock.NodePool()
	poolNoSchedConfig.SchedulerConfiguration = nil

	poolBinpack := mock.NodePool()
	poolBinpack.SchedulerConfiguration = &structs.NodePoolSchedulerConfiguration{
		SchedulerAlgorithm: structs.SchedulerAlgorithmBinpack,
	}

	poolSpread := mock.NodePool()
	poolSpread.SchedulerConfiguration = &structs.NodePoolSchedulerConfiguration{
		SchedulerAlgorithm: structs.SchedulerAlgorithmSpread,
	}

	testCases := []struct {
		name               string
		nodePool           string
		schedulerAlgorithm structs.SchedulerAlgorithm
		expectedAlgorithm  structs.SchedulerAlgorithm
	}{
		{
			name:               "global binpack",
			nodePool:           poolNoSchedConfig.Name,
			schedulerAlgorithm: structs.SchedulerAlgorithmBinpack,
			expectedAlgorithm:  structs.SchedulerAlgorithmBinpack,
		},
		{
			name:               "global spread",
			nodePool:           poolNoSchedConfig.Name,
			schedulerAlgorithm: structs.SchedulerAlgorithmSpread,
			expectedAlgorithm:  structs.SchedulerAlgorithmSpread,
		},
		{
			name:               "node pool binpack overrides global config",
			nodePool:           poolBinpack.Name,
			schedulerAlgorithm: structs.SchedulerAlgorithmSpread,
			expectedAlgorithm:  structs.SchedulerAlgorithmBinpack,
		},
		{
			name:               "node pool spread overrides global config",
			nodePool:           poolSpread.Name,
			schedulerAlgorithm: structs.SchedulerAlgorithmBinpack,
			expectedAlgorithm:  structs.SchedulerAlgorithmSpread,
		},
	}

	jobTypes := []string{
		"batch",
		"service",
	}

	for _, jobType := range jobTypes {
		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%s/%s", jobType, tc.name), func(t *testing.T) {
				h := NewHarness(t)

				// Create node pools.
				nodePools := []*structs.NodePool{
					poolNoSchedConfig,
					poolBinpack,
					poolSpread,
				}
				h.State.UpsertNodePools(structs.MsgTypeTestSetup, h.NextIndex(), nodePools)

				// Create two test nodes. Use two to prevent flakiness due to
				// the scheduler shuffling nodes.
				for i := 0; i < 2; i++ {
					node := mock.Node()
					node.NodePool = tc.nodePool
					must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
				}

				// Set global scheduler configuration.
				h.State.SchedulerSetConfig(h.NextIndex(), &structs.SchedulerConfiguration{
					SchedulerAlgorithm: tc.schedulerAlgorithm,
				})

				// Create test job.
				var job *structs.Job
				switch jobType {
				case "batch":
					job = mock.BatchJob()
				case "service":
					job = mock.Job()
				}
				job.TaskGroups[0].Count = 1
				job.NodePool = tc.nodePool
				must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

				// Register an existing job.
				existingJob := mock.Job()
				existingJob.TaskGroups[0].Count = 1
				existingJob.NodePool = tc.nodePool
				must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, existingJob))

				// Process eval for existing job to place an existing alloc.
				eval := &structs.Evaluation{
					Namespace:   structs.DefaultNamespace,
					ID:          uuid.Generate(),
					Priority:    existingJob.Priority,
					TriggeredBy: structs.EvalTriggerJobRegister,
					JobID:       existingJob.ID,
					Status:      structs.EvalStatusPending,
				}
				must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

				var scheduler Factory
				switch jobType {
				case "batch":
					scheduler = NewBatchScheduler
				case "service":
					scheduler = NewServiceScheduler
				}
				err := h.Process(scheduler, eval)
				must.NoError(t, err)

				must.Len(t, 1, h.Plans)
				allocs, err := h.State.AllocsByJob(nil, existingJob.Namespace, existingJob.ID, false)
				must.NoError(t, err)
				must.Len(t, 1, allocs)

				// Process eval for test job.
				eval = &structs.Evaluation{
					Namespace:   structs.DefaultNamespace,
					ID:          uuid.Generate(),
					Priority:    job.Priority,
					TriggeredBy: structs.EvalTriggerJobRegister,
					JobID:       job.ID,
					Status:      structs.EvalStatusPending,
				}
				must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))
				err = h.Process(scheduler, eval)
				must.NoError(t, err)

				must.Len(t, 2, h.Plans)
				allocs, err = h.State.AllocsByJob(nil, job.Namespace, job.ID, false)
				must.NoError(t, err)
				must.Len(t, 1, allocs)

				// Expect new alloc to be either in the empty node or in the
				// node with the existing alloc depending on the expected
				// scheduler algorithm.
				var expectedAllocCount int
				switch tc.expectedAlgorithm {
				case structs.SchedulerAlgorithmSpread:
					expectedAllocCount = 1
				case structs.SchedulerAlgorithmBinpack:
					expectedAllocCount = 2
				}

				alloc := allocs[0]
				nodeAllocs, err := h.State.AllocsByNode(nil, alloc.NodeID)
				must.NoError(t, err)
				must.Len(t, expectedAllocCount, nodeAllocs)
			})
		}
	}
}

// This test just ensures the scheduler handles the eval type to avoid
// regressions.
func TestServiceSched_EvaluateMaxPlanEval(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create a job and set the task group count to zero.
	job := mock.Job()
	job.TaskGroups[0].Count = 0
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Create a job with a high resource ask so that all the allocations can't
	// be placed on a single node.
	job := mock.Job()
	job.TaskGroups[0].Count = 3
	job.TaskGroups[0].Tasks[0].Resources.CPU = 3600
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create a job
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Create a job and set the task group count to zero.
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
		t.Errorf("bad: %#v", h.Evals)
		if h.Evals[0].BlockedEval != "" {
			t.Fatalf("bad: %#v", h.Evals[0])
		}
		t.FailNow()
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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Add a few terminal status allocations, these should be ignored
	var terminal []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		alloc.DesiredStatus = structs.AllocDesiredStatusStop
		alloc.ClientStatus = structs.AllocClientStatusFailed // #10446
		terminal = append(terminal, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), terminal))

	// Update the job
	job2 := mock.Job()
	job2.ID = job.ID

	// Update the task, such that it cannot be done in-place
	job2.TaskGroups[0].Tasks[0].Config["command"] = "/bin/other"
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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

func TestServiceSched_JobModify_Datacenters(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	require := require.New(t)

	// Create some nodes in 3 DCs
	var nodes []*structs.Node
	for i := 1; i < 4; i++ {
		node := mock.Node()
		node.Datacenter = fmt.Sprintf("dc%d", i)
		nodes = append(nodes, node)
		h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node)
	}

	// Generate a fake job with allocations
	job := mock.Job()
	job.TaskGroups[0].Count = 3
	job.Datacenters = []string{"dc1", "dc2", "dc3"}
	require.NoError(h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for i := 0; i < 3; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	require.NoError(h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Update the job to 2 DCs
	job2 := job.Copy()
	job2.TaskGroups[0].Count = 4
	job2.Datacenters = []string{"dc1", "dc2"}
	require.NoError(h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	require.NoError(err)
	h.AssertEvalStatus(t, structs.EvalStatusComplete)

	// Ensure a single plan
	require.Len(h.Plans, 1)
	plan := h.Plans[0]

	require.Len(plan.NodeUpdate, 1) // alloc in DC3 gets destructive update
	require.Len(plan.NodeUpdate[nodes[2].ID], 1)
	require.Equal(allocs[2].ID, plan.NodeUpdate[nodes[2].ID][0].ID)

	require.Len(plan.NodeAllocation, 2) // only 2 eligible nodes
	placed := map[string]*structs.Allocation{}
	for node, placedAllocs := range plan.NodeAllocation {
		require.True(
			slices.Contains([]string{nodes[0].ID, nodes[1].ID}, node),
			"allocation placed on ineligible node",
		)
		for _, alloc := range placedAllocs {
			placed[alloc.ID] = alloc
		}
	}
	require.Len(placed, 4)
	require.Equal(nodes[0].ID, placed[allocs[0].ID].NodeID, "alloc should not have moved")
	require.Equal(nodes[1].ID, placed[allocs[1].ID].NodeID, "alloc should not have moved")
}

// Have a single node and submit a job. Increment the count such that all fit
// on the node but the node doesn't have enough resources to fit the new count +
// 1. This tests that we properly discount the resources of existing allocs.
func TestServiceSched_JobModify_IncrCount_NodeLimit(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create one node
	node := mock.Node()
	node.NodeResources.Cpu.CpuShares = 1000
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Generate a fake job with one allocation
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Resources.CPU = 256
	job2 := job.Copy()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.AllocatedResources.Tasks["web"].Cpu.CpuShares = 256
	allocs = append(allocs, alloc)
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Update the job to count 3
	job2.TaskGroups[0].Count = 3
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = structs.AllocName(alloc.JobID, alloc.TaskGroup, uint(i))
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

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
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), terminal))

	// Update the job to be count zero
	job2 := mock.Job()
	job2.ID = job.ID
	job2.TaskGroups[0].Count = 0
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

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
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	dstate, ok := plan.Deployment.TaskGroups[job.TaskGroups[0].Name]
	if !ok {
		t.Fatalf("bad: %#v", plan)
	}
	if dstate.DesiredTotal != 10 && dstate.DesiredCanaries != 0 {
		t.Fatalf("bad: %#v", dstate)
	}
}

// This tests that the old allocation is stopped before placing.
// It is critical to test that the updated job attempts to place more
// allocations as this allows us to assert that destructive changes are done
// first.
func TestServiceSched_JobModify_Rolling_FullNode(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create a node and clear the reserved resources
	node := mock.Node()
	node.ReservedResources = nil
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

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
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	alloc := mock.Alloc()
	alloc.AllocatedResources = allocated
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

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
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	dstate, ok := plan.Deployment.TaskGroups[job.TaskGroups[0].Name]
	if !ok {
		t.Fatalf("bad: %#v", plan)
	}
	if dstate.DesiredTotal != 5 || dstate.DesiredCanaries != 0 {
		t.Fatalf("bad: %#v", dstate)
	}
}

func TestServiceSched_JobModify_Canaries(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

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
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	staleDState, ok := plan.Deployment.TaskGroups[job.TaskGroups[0].Name]
	require.True(t, ok)

	require.Equal(t, 0, len(staleDState.PlacedCanaries))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Generate a fake job with allocations and create an older deployment
	job := mock.Job()
	d := mock.Deployment()
	d.JobID = job.ID
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	require.NoError(t, h.State.UpsertDeployment(h.NextIndex(), d))

	taskName := job.TaskGroups[0].Tasks[0].Name

	adr := structs.AllocatedDeviceResource{
		Type:      "gpu",
		Vendor:    "nvidia",
		Name:      "1080ti",
		DeviceIDs: []string{uuid.Generate()},
	}

	asr := structs.AllocatedSharedResources{
		Ports:    structs.AllocatedPorts{{Label: "http"}},
		Networks: structs.Networks{{Mode: "bridge"}},
	}

	// Create allocs that are part of the old deployment
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.AllocForNode(nodes[i])
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		alloc.DeploymentID = d.ID
		alloc.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: pointer.Of(true)}
		alloc.AllocatedResources.Tasks[taskName].Devices = []*structs.AllocatedDeviceResource{&adr}
		alloc.AllocatedResources.Shared = asr
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

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
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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

	// Verify the allocated networks and devices did not change
	rp := structs.Port{Label: "admin", Value: 5000}
	for _, alloc := range out {
		// Verify Shared Allocared Resources Persisted
		require.Equal(t, alloc.AllocatedResources.Shared.Ports, asr.Ports)
		require.Equal(t, alloc.AllocatedResources.Shared.Networks, asr.Networks)

		for _, resources := range alloc.AllocatedResources.Tasks {
			if resources.Networks[0].ReservedPorts[0] != rp {
				t.Fatalf("bad: %#v", alloc)
			}
			if len(resources.Devices) == 0 || reflect.DeepEqual(resources.Devices[0], adr) {
				t.Fatalf("bad devices has changed: %#v", alloc)
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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Generate a fake job with 0.8 allocations
	job := mock.Job()
	job.TaskGroups[0].Count = 1
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create 0.8 alloc
	alloc := mock.Alloc()
	alloc.Job = job.Copy()
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.AllocatedResources = nil // 0.8 didn't have this
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

	// Update the job inplace
	job2 := job.Copy()

	job2.TaskGroups[0].Tasks[0].Services[0].Tags[0] = "newtag"
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	// Create a mock evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		node.Meta["rack"] = fmt.Sprintf("rack%d", i)
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
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
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)
	require := require.New(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
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

	require.NoError(h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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

	require.NoError(h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Create and process a mock evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))
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
	require.NoError(h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	// Create and process a mock evaluation
	eval = &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))
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
	ci.Parallel(t)

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
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Create a mock evaluation to deregister the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobDeregister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)
	require := require.New(t)

	// Generate a fake job with allocations
	job := mock.Job()
	job.Stop = true
	require.NoError(h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		allocs = append(allocs, alloc)
	}
	require.NoError(h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

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
	require.NoError(h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

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
			require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

			// Generate a fake job with allocations and an update policy.
			job := mock.Job()
			require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

			alloc := mock.Alloc()
			alloc.Job = job
			alloc.JobID = job.ID
			alloc.NodeID = node.ID
			alloc.Name = fmt.Sprintf("my-job.web[%d]", i)

			alloc.DesiredStatus = tc.desired
			alloc.ClientStatus = tc.client

			// Mark for migration if necessary
			alloc.DesiredTransition.Migrate = pointer.Of(tc.migrate)

			allocs := []*structs.Allocation{alloc}
			require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

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
			require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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

func TestServiceSched_StopAfterClientDisconnect(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		stop        time.Duration
		when        time.Time
		rescheduled bool
	}{
		{
			rescheduled: true,
		},
		{
			stop:        1 * time.Second,
			rescheduled: false,
		},
		{
			stop:        1 * time.Second,
			when:        time.Now().UTC().Add(-10 * time.Second),
			rescheduled: true,
		},
		{
			stop:        1 * time.Second,
			when:        time.Now().UTC().Add(10 * time.Minute),
			rescheduled: false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf(""), func(t *testing.T) {
			h := NewHarness(t)

			// Node, which is down
			node := mock.Node()
			node.Status = structs.NodeStatusDown
			require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

			// Job with allocations and stop_after_client_disconnect
			job := mock.Job()
			job.TaskGroups[0].Count = 1
			job.TaskGroups[0].StopAfterClientDisconnect = &tc.stop
			require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

			// Alloc for the running group
			alloc := mock.Alloc()
			alloc.Job = job
			alloc.JobID = job.ID
			alloc.NodeID = node.ID
			alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
			alloc.DesiredStatus = structs.AllocDesiredStatusRun
			alloc.ClientStatus = structs.AllocClientStatusRunning
			if !tc.when.IsZero() {
				alloc.AllocStates = []*structs.AllocState{{
					Field: structs.AllocStateFieldClientStatus,
					Value: structs.AllocClientStatusLost,
					Time:  tc.when,
				}}
			}
			allocs := []*structs.Allocation{alloc}
			require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

			// Create a mock evaluation to deal with drain
			evals := []*structs.Evaluation{{
				Namespace:   structs.DefaultNamespace,
				ID:          uuid.Generate(),
				Priority:    50,
				TriggeredBy: structs.EvalTriggerNodeDrain,
				JobID:       job.ID,
				NodeID:      node.ID,
				Status:      structs.EvalStatusPending,
			}}
			eval := evals[0]
			require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), evals))

			// Process the evaluation
			err := h.Process(NewServiceScheduler, eval)
			require.NoError(t, err)
			require.Equal(t, h.Evals[0].Status, structs.EvalStatusComplete)
			require.Len(t, h.Plans, 1, "plan")

			// One followup eval created, either delayed or blocked
			require.Len(t, h.CreateEvals, 1)
			e := h.CreateEvals[0]
			require.Equal(t, eval.ID, e.PreviousEval)

			if tc.rescheduled {
				require.Equal(t, "blocked", e.Status)
			} else {
				require.Equal(t, "pending", e.Status)
				require.NotEmpty(t, e.WaitUntil)
			}

			// This eval is still being inserted in the state store
			ws := memdb.NewWatchSet()
			testutil.WaitForResult(func() (bool, error) {
				found, err := h.State.EvalByID(ws, e.ID)
				if err != nil {
					return false, err
				}
				if found == nil {
					return false, nil
				}
				return true, nil
			}, func(err error) {
				require.NoError(t, err)
			})

			alloc, err = h.State.AllocByID(ws, alloc.ID)
			require.NoError(t, err)

			// Allocations have been transitioned to lost
			require.Equal(t, structs.AllocDesiredStatusStop, alloc.DesiredStatus)
			require.Equal(t, structs.AllocClientStatusLost, alloc.ClientStatus)
			// At least 1, 2 if we manually set the tc.when
			require.NotEmpty(t, alloc.AllocStates)

			if tc.rescheduled {
				// Register a new node, leave it up, process the followup eval
				node = mock.Node()
				require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
				require.NoError(t, h.Process(NewServiceScheduler, eval))

				as, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
				require.NoError(t, err)

				testutil.WaitForResult(func() (bool, error) {
					as, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
					if err != nil {
						return false, err
					}
					return len(as) == 2, nil
				}, func(err error) {
					require.NoError(t, err)
				})

				a2 := as[0]
				if a2.ID == alloc.ID {
					a2 = as[1]
				}

				require.Equal(t, structs.AllocClientStatusPending, a2.ClientStatus)
				require.Equal(t, structs.AllocDesiredStatusRun, a2.DesiredStatus)
				require.Equal(t, node.ID, a2.NodeID)

				// No blocked evals
				require.Empty(t, h.ReblockEvals)
				require.Len(t, h.CreateEvals, 1)
				require.Equal(t, h.CreateEvals[0].ID, e.ID)
			} else {
				// No new alloc was created
				as, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
				require.NoError(t, err)

				require.Len(t, as, 1)
				old := as[0]

				require.Equal(t, alloc.ID, old.ID)
				require.Equal(t, structs.AllocClientStatusLost, old.ClientStatus)
				require.Equal(t, structs.AllocDesiredStatusStop, old.DesiredStatus)
			}
		})
	}
}

func TestServiceSched_NodeUpdate(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Register a node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Mark some allocs as running
	ws := memdb.NewWatchSet()
	for i := 0; i < 4; i++ {
		out, _ := h.State.AllocByID(ws, allocs[i].ID)
		out.ClientStatus = structs.AllocClientStatusRunning
		require.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{out}))
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
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Register a draining node
	node := mock.DrainNode()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		alloc.DesiredTransition.Migrate = pointer.Of(true)
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

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
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Register a draining node
	node := mock.DrainNode()
	node.Status = structs.NodeStatusDown
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Generate a fake job with allocations
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Set the desired state of the allocs to stop
	var stop []*structs.Allocation
	for i := 0; i < 6; i++ {
		newAlloc := allocs[i].Copy()
		newAlloc.ClientStatus = structs.AllocDesiredStatusStop
		newAlloc.DesiredTransition.Migrate = pointer.Of(true)
		stop = append(stop, newAlloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), stop))

	// Mark some of the allocations as running
	var running []*structs.Allocation
	for i := 4; i < 6; i++ {
		newAlloc := stop[i].Copy()
		newAlloc.ClientStatus = structs.AllocClientStatusRunning
		running = append(running, newAlloc)
	}
	require.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), running))

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
	require.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), complete))

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

	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Register a draining node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	job.TaskGroups[0].Count = 2
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for i := 0; i < 2; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		alloc.DesiredTransition.Migrate = pointer.Of(true)
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	node.DrainStrategy = mock.DrainNode().DrainStrategy
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

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
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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

// TestServiceSched_NodeDrain_TaskHandle asserts that allocations with task
// handles have them propagated to replacement allocations when drained.
func TestServiceSched_NodeDrain_TaskHandle(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		alloc.DesiredTransition.Migrate = pointer.Of(true)
		alloc.TaskStates = map[string]*structs.TaskState{
			"web": {
				TaskHandle: &structs.TaskHandle{
					Version:     1,
					DriverState: []byte("test-driver-state"),
				},
			},
		}
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	node.DrainStrategy = mock.DrainNode().DrainStrategy
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

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
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	require.NoError(t, err)

	// Ensure a single plan
	require.Len(t, h.Plans, 1)
	plan := h.Plans[0]

	// Ensure the plan evicted all allocs
	require.Len(t, plan.NodeUpdate[node.ID], len(allocs))

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	require.Len(t, planned, len(allocs))

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	out, _ = structs.FilterTerminalAllocs(out)
	require.Len(t, out, len(allocs))

	// Ensure task states were propagated
	for _, a := range out {
		require.NotEmpty(t, a.TaskStates)
		require.NotEmpty(t, a.TaskStates["web"])
		require.NotNil(t, a.TaskStates["web"].TaskHandle)
		assert.Equal(t, 1, a.TaskStates["web"].TaskHandle.Version)
		assert.Equal(t, []byte("test-driver-state"), a.TaskStates["web"].TaskHandle.DriverState)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_RetryLimit(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)
	h.Planner = &RejectPlan{h}

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Create a job
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
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

	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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

	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Create a mock evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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

	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{newAlloc}))

	// Create another mock evaluation
	eval = &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)
	require := require.New(t)
	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
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

	require.NoError(h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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

	require.NoError(h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Create a mock evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
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

	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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

	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Create a mock evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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

		require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{newAlloc}))

		// Create another mock evaluation
		eval = &structs.Evaluation{
			Namespace:   structs.DefaultNamespace,
			ID:          uuid.Generate(),
			Priority:    50,
			TriggeredBy: structs.EvalTriggerNodeUpdate,
			JobID:       job.ID,
			Status:      structs.EvalStatusPending,
		}
		require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))
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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
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
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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

	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Create a mock evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	for _, failedDeployment := range []bool{false, true} {
		t.Run(fmt.Sprintf("Failed Deployment: %v", failedDeployment), func(t *testing.T) {
			h := NewHarness(t)
			require := require.New(t)
			// Create some nodes
			var nodes []*structs.Node
			for i := 0; i < 10; i++ {
				node := mock.Node()
				nodes = append(nodes, node)
				require.NoError(h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
			}

			// Generate a fake job with allocations and a reschedule policy.
			job := mock.Job()
			job.TaskGroups[0].Count = 2
			job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
				Attempts: 1,
				Interval: 15 * time.Minute,
			}
			jobIndex := h.NextIndex()
			require.Nil(h.State.UpsertJob(structs.MsgTypeTestSetup, jobIndex, nil, job))

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
			allocs[1].DesiredTransition.Reschedule = pointer.Of(true)

			require.Nil(h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

			// Create a mock evaluation
			eval := &structs.Evaluation{
				Namespace:   structs.DefaultNamespace,
				ID:          uuid.Generate(),
				Priority:    50,
				TriggeredBy: structs.EvalTriggerNodeUpdate,
				JobID:       job.ID,
				Status:      structs.EvalStatusPending,
			}
			require.Nil(h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Create a job
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 1
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a complete alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.ClientStatus = structs.AllocClientStatusComplete
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Create a job
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 1
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Create a job
	job := mock.Job()
	job.ID = "my-job"
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 3
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Desired = 3
	// Mark one as lost and then schedule
	// [(0, run, running), (1, run, running), (1, stop, lost)]

	// Create two running allocations
	var allocs []*structs.Allocation
	for i := 0; i <= 1; i++ {
		alloc := mock.AllocForNodeWithoutReservedPort(node)
		alloc.Job = job
		alloc.JobID = job.ID
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
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	node := mock.DrainNode()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Create a job
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 1
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create two nodes, one that is drained and has a successfully finished
	// alloc and a fresh undrained one
	node := mock.DrainNode()
	node2 := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node2))

	// Create a job
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 1
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to rerun the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Create a mock evaluation to trigger the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Update the job
	job2 := mock.Job()
	job2.ID = job.ID
	job2.Type = structs.JobTypeBatch
	job2.Version++
	job2.TaskGroups[0].Tasks[0].Env = map[string]string{"foo": "bar"}
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

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
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create two nodes, one that is drained and has a successfully finished
	// alloc and a fresh undrained one
	node := mock.DrainNode()
	node2 := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node2))

	// Create a job
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 1
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a running alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.ClientStatus = structs.AllocClientStatusRunning
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

	// Create an update job
	job2 := job.Copy()
	job2.TaskGroups[0].Tasks[0].Env = map[string]string{"foo": "bar"}
	job2.Version++
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create two nodes, one that is drained and has a successfully finished
	// alloc and a fresh undrained one
	node := mock.DrainNode()
	node2 := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node2))

	// Create a job
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 1
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Create a job
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 1
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
		alloc := mock.AllocForNodeWithoutReservedPort(node)
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.Name = "my-job.web[0]"
		alloc.ClientStatus = structs.AllocClientStatusRunning
		alloc.Metrics = scoreMetric
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Update the job's modify index to force an inplace upgrade
	updatedJob := job.Copy()
	updatedJob.JobModifyIndex = job.JobModifyIndex + 1
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, updatedJob))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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

func TestGenericSched_AllocFit_Lifecycle(t *testing.T) {
	ci.Parallel(t)

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
			require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

			// Create a job with sidecar & init tasks
			job := mock.VariableLifecycleJob(testCase.TaskResources, testCase.MainTaskCount, testCase.InitTaskCount, testCase.SideTaskCount)

			require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

			// Create a mock evaluation to register the job
			eval := &structs.Evaluation{
				Namespace:   structs.DefaultNamespace,
				ID:          uuid.Generate(),
				Priority:    job.Priority,
				TriggeredBy: structs.EvalTriggerJobRegister,
				JobID:       job.ID,
				Status:      structs.EvalStatusPending,
			}
			require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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

func TestGenericSched_AllocFit_MemoryOversubscription(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)
	node := mock.Node()
	node.NodeResources.Cpu.CpuShares = 10000
	node.NodeResources.Memory.MemoryMB = 1224
	node.ReservedResources.Memory.MemoryMB = 60
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	job := mock.Job()
	job.TaskGroups[0].Count = 10
	job.TaskGroups[0].Tasks[0].Resources.CPU = 100
	job.TaskGroups[0].Tasks[0].Resources.MemoryMB = 200
	job.TaskGroups[0].Tasks[0].Resources.MemoryMaxMB = 500
	job.TaskGroups[0].Tasks[0].Resources.DiskMB = 1
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	require.NoError(t, err)

	// expectedAllocs should be floor((nodeResources.MemoryMB-reservedResources.MemoryMB) / job.MemoryMB)
	expectedAllocs := 5
	require.Len(t, h.Plans, 1)

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	require.Len(t, out, expectedAllocs)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestGenericSched_ChainedAlloc(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Create a job
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))
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
	require.NoError(t, h1.State.UpsertJob(structs.MsgTypeTestSetup, h1.NextIndex(), nil, job1))

	// Create a mock evaluation to update the job
	eval1 := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job1.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job1.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval1}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Register a draining node
	node := mock.DrainNode()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Create an alloc on the draining node
	alloc := mock.Alloc()
	alloc.Name = "my-job.web[0]"
	alloc.NodeID = node.ID
	alloc.Job.TaskGroups[0].Count = 1
	alloc.Job.TaskGroups[0].EphemeralDisk.Sticky = true
	alloc.DesiredTransition.Migrate = pointer.Of(true)
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, alloc.Job))
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

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

	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Generate a fake job
	job := mock.Job()
	job.JobModifyIndex = job.CreateIndex + 1
	job.ModifyIndex = job.CreateIndex + 1
	job.Stop = true
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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

	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

	h := NewHarness(t)

	// Generate a fake job
	job := mock.Job()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a deployment for an old version of the job
	d := mock.Deployment()
	d.JobID = job.ID
	require.NoError(t, h.State.UpsertDeployment(h.NextIndex(), d))

	// Upsert again to bump job version
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to kick the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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
	ci.Parallel(t)

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

func TestServiceSched_Preemption(t *testing.T) {
	ci.Parallel(t)

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
				Mode:   "host",
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
	require.NoError(h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Create a couple of jobs and schedule them
	job1 := mock.Job()
	job1.TaskGroups[0].Count = 1
	job1.TaskGroups[0].Networks = nil
	job1.Priority = 30
	r1 := job1.TaskGroups[0].Tasks[0].Resources
	r1.CPU = 500
	r1.MemoryMB = 1024
	require.NoError(h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job1))

	job2 := mock.Job()
	job2.TaskGroups[0].Count = 1
	job2.TaskGroups[0].Networks = nil
	job2.Priority = 50
	r2 := job2.TaskGroups[0].Tasks[0].Resources
	r2.CPU = 350
	r2.MemoryMB = 512
	require.NoError(h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

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

	require.NoError(h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval1, eval2}))

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
	job3.TaskGroups[0].Networks = nil
	r3 := job3.TaskGroups[0].Tasks[0].Resources
	r3.CPU = 900
	r3.MemoryMB = 1700
	require.NoError(h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job3))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job3.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job3.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

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

// TestServiceSched_Migrate_NonCanary asserts that when rescheduling
// non-canary allocations, a single allocation is migrated
func TestServiceSched_Migrate_NonCanary(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	node1 := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node1))

	job := mock.Job()
	job.Stable = true
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Update = &structs.UpdateStrategy{
		MaxParallel: 1,
		Canary:      1,
	}
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	deployment := &structs.Deployment{
		ID:             uuid.Generate(),
		JobID:          job.ID,
		Namespace:      job.Namespace,
		JobVersion:     job.Version,
		JobModifyIndex: job.JobModifyIndex,
		JobCreateIndex: job.CreateIndex,
		TaskGroups: map[string]*structs.DeploymentState{
			"web": {DesiredTotal: 1},
		},
		Status:            structs.DeploymentStatusSuccessful,
		StatusDescription: structs.DeploymentStatusDescriptionSuccessful,
	}
	require.NoError(t, h.State.UpsertDeployment(h.NextIndex(), deployment))

	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node1.ID
	alloc.DeploymentID = deployment.ID
	alloc.Name = "my-job.web[0]"
	alloc.DesiredStatus = structs.AllocDesiredStatusRun
	alloc.ClientStatus = structs.AllocClientStatusRunning
	alloc.DesiredTransition.Migrate = pointer.Of(true)
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerAllocStop,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	require.NoError(t, err)

	// Ensure a single plan
	require.Len(t, h.Plans, 1)
	plan := h.Plans[0]

	require.Contains(t, plan.NodeAllocation, node1.ID)
	allocs := plan.NodeAllocation[node1.ID]
	require.Len(t, allocs, 1)

}

// TestServiceSched_Migrate_CanaryStatus asserts that migrations/rescheduling
// of allocations use the proper versions of allocs rather than latest:
// Canaries should be replaced by canaries, and non-canaries should be replaced
// with the latest promoted version.
func TestServiceSched_Migrate_CanaryStatus(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	node1 := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node1))

	totalCount := 3
	desiredCanaries := 1

	job := mock.Job()
	job.Stable = true
	job.TaskGroups[0].Count = totalCount
	job.TaskGroups[0].Update = &structs.UpdateStrategy{
		MaxParallel: 1,
		Canary:      desiredCanaries,
	}
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	deployment := &structs.Deployment{
		ID:             uuid.Generate(),
		JobID:          job.ID,
		Namespace:      job.Namespace,
		JobVersion:     job.Version,
		JobModifyIndex: job.JobModifyIndex,
		JobCreateIndex: job.CreateIndex,
		TaskGroups: map[string]*structs.DeploymentState{
			"web": {DesiredTotal: totalCount},
		},
		Status:            structs.DeploymentStatusSuccessful,
		StatusDescription: structs.DeploymentStatusDescriptionSuccessful,
	}
	require.NoError(t, h.State.UpsertDeployment(h.NextIndex(), deployment))

	var allocs []*structs.Allocation
	for i := 0; i < 3; i++ {
		alloc := mock.AllocForNodeWithoutReservedPort(node1)
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.DeploymentID = deployment.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// new update with new task group
	job2 := job.Copy()
	job2.Stable = false
	job2.TaskGroups[0].Tasks[0].Config["command"] = "/bin/other"
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	// Create a mock evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	require.NoError(t, err)

	// Ensure a single plan
	require.Len(t, h.Plans, 1)
	plan := h.Plans[0]

	// Ensure a deployment was created
	require.NotNil(t, plan.Deployment)
	updateDeployment := plan.Deployment.ID

	// Check status first - should be 4 allocs, only one is canary
	{
		ws := memdb.NewWatchSet()
		allocs, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, true)
		require.NoError(t, err)
		require.Len(t, allocs, 4)

		sort.Slice(allocs, func(i, j int) bool { return allocs[i].CreateIndex < allocs[j].CreateIndex })

		for _, a := range allocs[:3] {
			require.Equal(t, structs.AllocDesiredStatusRun, a.DesiredStatus)
			require.Equal(t, uint64(0), a.Job.Version)
			require.False(t, a.DeploymentStatus.IsCanary())
			require.Equal(t, node1.ID, a.NodeID)
			require.Equal(t, deployment.ID, a.DeploymentID)
		}
		require.Equal(t, structs.AllocDesiredStatusRun, allocs[3].DesiredStatus)
		require.Equal(t, uint64(1), allocs[3].Job.Version)
		require.True(t, allocs[3].DeploymentStatus.Canary)
		require.Equal(t, node1.ID, allocs[3].NodeID)
		require.Equal(t, updateDeployment, allocs[3].DeploymentID)
	}

	// now, drain node1 and ensure all are migrated to node2
	node1 = node1.Copy()
	node1.Status = structs.NodeStatusDown
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node1))

	node2 := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node2))

	neval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		NodeID:      node1.ID,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{neval}))

	// Process the evaluation
	err = h.Process(NewServiceScheduler, eval)
	require.NoError(t, err)

	// Now test that all node1 allocs are migrated while preserving Version and Canary info
	{
		// FIXME: This is a bug, we ought to reschedule canaries in this case but don't
		rescheduleCanary := false

		expectedMigrations := 3
		if rescheduleCanary {
			expectedMigrations++
		}

		ws := memdb.NewWatchSet()
		allocs, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, true)
		require.NoError(t, err)
		require.Len(t, allocs, 4+expectedMigrations)

		nodeAllocs := map[string][]*structs.Allocation{}
		for _, a := range allocs {
			nodeAllocs[a.NodeID] = append(nodeAllocs[a.NodeID], a)
		}

		require.Len(t, nodeAllocs[node1.ID], 4)
		for _, a := range nodeAllocs[node1.ID] {
			require.Equal(t, structs.AllocDesiredStatusStop, a.DesiredStatus)
			require.Equal(t, node1.ID, a.NodeID)
		}

		node2Allocs := nodeAllocs[node2.ID]
		require.Len(t, node2Allocs, expectedMigrations)
		sort.Slice(node2Allocs, func(i, j int) bool { return node2Allocs[i].Job.Version < node2Allocs[j].Job.Version })

		for _, a := range node2Allocs[:3] {
			require.Equal(t, structs.AllocDesiredStatusRun, a.DesiredStatus)
			require.Equal(t, uint64(0), a.Job.Version)
			require.Equal(t, node2.ID, a.NodeID)
			require.Equal(t, deployment.ID, a.DeploymentID)
		}
		if rescheduleCanary {
			require.Equal(t, structs.AllocDesiredStatusRun, node2Allocs[3].DesiredStatus)
			require.Equal(t, uint64(1), node2Allocs[3].Job.Version)
			require.Equal(t, node2.ID, node2Allocs[3].NodeID)
			require.Equal(t, updateDeployment, node2Allocs[3].DeploymentID)
		}
	}
}

// TestDowngradedJobForPlacement_PicksTheLatest asserts that downgradedJobForPlacement
// picks the latest deployment that have either been marked as promoted or is considered
// non-destructive so it doesn't use canaries.
func TestDowngradedJobForPlacement_PicksTheLatest(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// This test tests downgradedJobForPlacement directly to ease testing many different scenarios
	// without invoking the full machinary of scheduling and updating deployment state tracking.
	//
	// It scafold the parts of scheduler and state stores so we can mimic the updates.
	updates := []struct {
		// Version of the job this update represent
		version uint64

		// whether this update is marked as promoted: Promoted is only true if the job
		// update is a "destructive" update and has been updated manually
		promoted bool

		// requireCanaries indicate whether the job update requires placing canaries due to
		// it being a destructive update compared to the latest promoted deployment.
		requireCanaries bool

		// the expected version for migrating a stable non-canary alloc after applying this update
		expectedVersion uint64
	}{
		// always use latest promoted deployment
		{1, true, true, 1},
		{2, true, true, 2},
		{3, true, true, 3},

		// ignore most recent non promoted
		{4, false, true, 3},
		{5, false, true, 3},
		{6, false, true, 3},

		// use latest promoted after promotion
		{7, true, true, 7},

		// non destructive updates that don't require canaries and are treated as promoted
		{8, false, false, 8},
	}

	job := mock.Job()
	job.Version = 0
	job.Stable = true
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	initDeployment := &structs.Deployment{
		ID:             uuid.Generate(),
		JobID:          job.ID,
		Namespace:      job.Namespace,
		JobVersion:     job.Version,
		JobModifyIndex: job.JobModifyIndex,
		JobCreateIndex: job.CreateIndex,
		TaskGroups: map[string]*structs.DeploymentState{
			"web": {
				DesiredTotal: 1,
				Promoted:     true,
			},
		},
		Status:            structs.DeploymentStatusSuccessful,
		StatusDescription: structs.DeploymentStatusDescriptionSuccessful,
	}
	require.NoError(t, h.State.UpsertDeployment(h.NextIndex(), initDeployment))

	deploymentIDs := []string{initDeployment.ID}

	for i, u := range updates {
		t.Run(fmt.Sprintf("%d: %#+v", i, u), func(t *testing.T) {
			t.Logf("case: %#+v", u)
			nj := job.Copy()
			nj.Version = u.version
			nj.TaskGroups[0].Tasks[0].Env["version"] = fmt.Sprintf("%v", u.version)
			nj.TaskGroups[0].Count = 1
			require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, nj))

			desiredCanaries := 1
			if !u.requireCanaries {
				desiredCanaries = 0
			}
			deployment := &structs.Deployment{
				ID:             uuid.Generate(),
				JobID:          nj.ID,
				Namespace:      nj.Namespace,
				JobVersion:     nj.Version,
				JobModifyIndex: nj.JobModifyIndex,
				JobCreateIndex: nj.CreateIndex,
				TaskGroups: map[string]*structs.DeploymentState{
					"web": {
						DesiredTotal:    1,
						Promoted:        u.promoted,
						DesiredCanaries: desiredCanaries,
					},
				},
				Status:            structs.DeploymentStatusSuccessful,
				StatusDescription: structs.DeploymentStatusDescriptionSuccessful,
			}
			require.NoError(t, h.State.UpsertDeployment(h.NextIndex(), deployment))

			deploymentIDs = append(deploymentIDs, deployment.ID)

			sched := h.Scheduler(NewServiceScheduler).(*GenericScheduler)

			sched.job = nj
			sched.deployment = deployment
			placement := &allocPlaceResult{
				taskGroup: nj.TaskGroups[0],
			}

			// Here, assert the downgraded job version
			foundDeploymentID, foundJob, err := sched.downgradedJobForPlacement(placement)
			require.NoError(t, err)
			require.Equal(t, u.expectedVersion, foundJob.Version)
			require.Equal(t, deploymentIDs[u.expectedVersion], foundDeploymentID)
		})
	}
}

// TestServiceSched_RunningWithNextAllocation asserts that if a running allocation has
// NextAllocation Set, the allocation is not ignored and will be stopped
func TestServiceSched_RunningWithNextAllocation(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	node1 := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node1))

	totalCount := 2
	job := mock.Job()
	job.Version = 0
	job.Stable = true
	job.TaskGroups[0].Count = totalCount
	job.TaskGroups[0].Update = nil
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for i := 0; i < totalCount+1; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node1.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}

	// simulate a case where .NextAllocation is set but alloc is still running
	allocs[2].PreviousAllocation = allocs[0].ID
	allocs[0].NextAllocation = allocs[2].ID
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// new update with new task group
	job2 := job.Copy()
	job2.Version = 1
	job2.TaskGroups[0].Tasks[0].Config["command"] = "/bin/other"
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	// Create a mock evaluation
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	require.NoError(t, err)

	// assert that all original allocations have been stopped
	for _, alloc := range allocs {
		updated, err := h.State.AllocByID(nil, alloc.ID)
		require.NoError(t, err)
		require.Equalf(t, structs.AllocDesiredStatusStop, updated.DesiredStatus, "alloc %v", alloc.ID)
	}

	// assert that the new job has proper allocations

	jobAllocs, err := h.State.AllocsByJob(nil, job.Namespace, job.ID, true)
	require.NoError(t, err)

	require.Len(t, jobAllocs, 5)

	allocsByVersion := map[uint64][]string{}
	for _, alloc := range jobAllocs {
		allocsByVersion[alloc.Job.Version] = append(allocsByVersion[alloc.Job.Version], alloc.ID)
	}
	require.Len(t, allocsByVersion[1], 2)
	require.Len(t, allocsByVersion[0], 3)
}

func TestServiceSched_CSIVolumesPerAlloc(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)
	require := require.New(t)

	// Create some nodes, each running the CSI plugin
	for i := 0; i < 5; i++ {
		node := mock.Node()
		node.CSINodePlugins = map[string]*structs.CSIInfo{
			"test-plugin": {
				PluginID: "test-plugin",
				Healthy:  true,
				NodeInfo: &structs.CSINodeInfo{MaxVolumes: 2},
			},
		}
		require.NoError(h.State.UpsertNode(
			structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// create per-alloc volumes
	vol0 := structs.NewCSIVolume("volume-unique[0]", 0)
	vol0.PluginID = "test-plugin"
	vol0.Namespace = structs.DefaultNamespace
	vol0.AccessMode = structs.CSIVolumeAccessModeSingleNodeWriter
	vol0.AttachmentMode = structs.CSIVolumeAttachmentModeFilesystem

	vol1 := vol0.Copy()
	vol1.ID = "volume-unique[1]"
	vol2 := vol0.Copy()
	vol2.ID = "volume-unique[2]"

	// create shared volume
	shared := vol0.Copy()
	shared.ID = "volume-shared"
	// TODO: this should cause a test failure, see GH-10157
	// replace this value with structs.CSIVolumeAccessModeSingleNodeWriter
	// once its been fixed
	shared.AccessMode = structs.CSIVolumeAccessModeMultiNodeReader

	require.NoError(h.State.UpsertCSIVolume(
		h.NextIndex(), []*structs.CSIVolume{shared, vol0, vol1, vol2}))

	// Create a job that uses both
	job := mock.Job()
	job.TaskGroups[0].Count = 3
	job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{
		"shared": {
			Type:     "csi",
			Name:     "shared",
			Source:   "volume-shared",
			ReadOnly: true,
		},
		"unique": {
			Type:     "csi",
			Name:     "unique",
			Source:   "volume-unique",
			PerAlloc: true,
		},
	}

	require.NoError(h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(h.State.UpsertEvals(structs.MsgTypeTestSetup,
		h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation and expect a single plan without annotations
	err := h.Process(NewServiceScheduler, eval)
	require.NoError(err)
	require.Len(h.Plans, 1, "expected one plan")
	require.Nil(h.Plans[0].Annotations, "expected no annotations")

	// Expect the eval has not spawned a blocked eval
	require.Equal(len(h.CreateEvals), 0)
	require.Equal("", h.Evals[0].BlockedEval, "did not expect a blocked eval")
	require.Equal(structs.EvalStatusComplete, h.Evals[0].Status)

	// Ensure the plan allocated and we got expected placements
	var planned []*structs.Allocation
	for _, allocList := range h.Plans[0].NodeAllocation {
		planned = append(planned, allocList...)
	}
	require.Len(planned, 3, "expected 3 planned allocations")

	out, err := h.State.AllocsByJob(nil, job.Namespace, job.ID, false)
	require.NoError(err)
	require.Len(out, 3, "expected 3 placed allocations")

	// Allocations don't have references to the actual volumes assigned, but
	// because we set a max of 2 volumes per Node plugin, we can verify that
	// they've been properly scheduled by making sure they're all on separate
	// clients.
	seen := map[string]struct{}{}
	for _, alloc := range out {
		_, ok := seen[alloc.NodeID]
		require.False(ok, "allocations should be scheduled to separate nodes")
		seen[alloc.NodeID] = struct{}{}
	}

	// Update the job to 5 instances
	job.TaskGroups[0].Count = 5
	require.NoError(h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a new eval and process it. It should not create a new plan.
	eval.ID = uuid.Generate()
	require.NoError(h.State.UpsertEvals(structs.MsgTypeTestSetup,
		h.NextIndex(), []*structs.Evaluation{eval}))
	err = h.Process(NewServiceScheduler, eval)
	require.NoError(err)
	require.Len(h.Plans, 1, "expected one plan")

	// Expect the eval to have failed
	require.NotEqual("", h.Evals[1].BlockedEval,
		"expected a blocked eval to be spawned")
	require.Equal(2, h.Evals[1].QueuedAllocations["web"], "expected 2 queued allocs")
	require.Equal(5, h.Evals[1].FailedTGAllocs["web"].
		ConstraintFiltered["missing CSI Volume volume-unique[3]"])

	// Upsert 2 more per-alloc volumes
	vol4 := vol0.Copy()
	vol4.ID = "volume-unique[3]"
	vol5 := vol0.Copy()
	vol5.ID = "volume-unique[4]"
	require.NoError(h.State.UpsertCSIVolume(
		h.NextIndex(), []*structs.CSIVolume{vol4, vol5}))

	// Process again with failure fixed. It should create a new plan
	eval.ID = uuid.Generate()
	require.NoError(h.State.UpsertEvals(structs.MsgTypeTestSetup,
		h.NextIndex(), []*structs.Evaluation{eval}))
	err = h.Process(NewServiceScheduler, eval)
	require.NoError(err)
	require.Len(h.Plans, 2, "expected two plans")
	require.Nil(h.Plans[1].Annotations, "expected no annotations")

	require.Equal("", h.Evals[2].BlockedEval, "did not expect a blocked eval")
	require.Len(h.Evals[2].FailedTGAllocs, 0)

	// Ensure the plan allocated and we got expected placements
	planned = []*structs.Allocation{}
	for _, allocList := range h.Plans[1].NodeAllocation {
		planned = append(planned, allocList...)
	}
	require.Len(planned, 2, "expected 2 new planned allocations")

	out, err = h.State.AllocsByJob(nil, job.Namespace, job.ID, false)
	require.NoError(err)
	require.Len(out, 5, "expected 5 placed allocations total")

	// Make sure they're still all on seperate clients
	seen = map[string]struct{}{}
	for _, alloc := range out {
		_, ok := seen[alloc.NodeID]
		require.False(ok, "allocations should be scheduled to separate nodes")
		seen[alloc.NodeID] = struct{}{}
	}

}

func TestServiceSched_CSITopology(t *testing.T) {
	ci.Parallel(t)
	h := NewHarness(t)

	zones := []string{"zone-0", "zone-1", "zone-2", "zone-3"}

	// Create some nodes, each running a CSI plugin with topology for
	// a different "zone"
	for i := 0; i < 12; i++ {
		node := mock.Node()
		node.Datacenter = zones[i%4]
		node.CSINodePlugins = map[string]*structs.CSIInfo{
			"test-plugin-" + zones[i%4]: {
				PluginID: "test-plugin-" + zones[i%4],
				Healthy:  true,
				NodeInfo: &structs.CSINodeInfo{
					MaxVolumes: 3,
					AccessibleTopology: &structs.CSITopology{
						Segments: map[string]string{"zone": zones[i%4]}},
				},
			},
		}
		require.NoError(t, h.State.UpsertNode(
			structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// create 2 per-alloc volumes for those zones
	vol0 := structs.NewCSIVolume("myvolume[0]", 0)
	vol0.PluginID = "test-plugin-zone-0"
	vol0.Namespace = structs.DefaultNamespace
	vol0.AccessMode = structs.CSIVolumeAccessModeSingleNodeWriter
	vol0.AttachmentMode = structs.CSIVolumeAttachmentModeFilesystem
	vol0.RequestedTopologies = &structs.CSITopologyRequest{
		Required: []*structs.CSITopology{{
			Segments: map[string]string{"zone": "zone-0"},
		}},
	}

	vol1 := vol0.Copy()
	vol1.ID = "myvolume[1]"
	vol1.PluginID = "test-plugin-zone-1"
	vol1.RequestedTopologies.Required[0].Segments["zone"] = "zone-1"

	require.NoError(t, h.State.UpsertCSIVolume(
		h.NextIndex(), []*structs.CSIVolume{vol0, vol1}))

	// Create a job that uses those volumes
	job := mock.Job()
	job.Datacenters = zones
	job.TaskGroups[0].Count = 2
	job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{
		"myvolume": {
			Type:     "csi",
			Name:     "unique",
			Source:   "myvolume",
			PerAlloc: true,
		},
	}

	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup,
		h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation and expect a single plan without annotations
	err := h.Process(NewServiceScheduler, eval)
	require.NoError(t, err)
	require.Len(t, h.Plans, 1, "expected one plan")
	require.Nil(t, h.Plans[0].Annotations, "expected no annotations")

	// Expect the eval has not spawned a blocked eval
	require.Equal(t, len(h.CreateEvals), 0)
	require.Equal(t, "", h.Evals[0].BlockedEval, "did not expect a blocked eval")
	require.Equal(t, structs.EvalStatusComplete, h.Evals[0].Status)

}

// TestPropagateTaskState asserts that propagateTaskState only copies state
// when the previous allocation is lost or draining.
func TestPropagateTaskState(t *testing.T) {
	ci.Parallel(t)

	const taskName = "web"
	taskHandle := &structs.TaskHandle{
		Version:     1,
		DriverState: []byte("driver-state"),
	}

	cases := []struct {
		name      string
		prevAlloc *structs.Allocation
		prevLost  bool
		copied    bool
	}{
		{
			name: "LostWithState",
			prevAlloc: &structs.Allocation{
				ClientStatus:      structs.AllocClientStatusRunning,
				DesiredTransition: structs.DesiredTransition{},
				TaskStates: map[string]*structs.TaskState{
					taskName: {
						TaskHandle: taskHandle,
					},
				},
			},
			prevLost: true,
			copied:   true,
		},
		{
			name: "DrainedWithState",
			prevAlloc: &structs.Allocation{
				ClientStatus: structs.AllocClientStatusRunning,
				DesiredTransition: structs.DesiredTransition{
					Migrate: pointer.Of(true),
				},
				TaskStates: map[string]*structs.TaskState{
					taskName: {
						TaskHandle: taskHandle,
					},
				},
			},
			prevLost: false,
			copied:   true,
		},
		{
			name: "LostWithoutState",
			prevAlloc: &structs.Allocation{
				ClientStatus:      structs.AllocClientStatusRunning,
				DesiredTransition: structs.DesiredTransition{},
				TaskStates: map[string]*structs.TaskState{
					taskName: {},
				},
			},
			prevLost: true,
			copied:   false,
		},
		{
			name: "DrainedWithoutState",
			prevAlloc: &structs.Allocation{
				ClientStatus: structs.AllocClientStatusRunning,
				DesiredTransition: structs.DesiredTransition{
					Migrate: pointer.Of(true),
				},
				TaskStates: map[string]*structs.TaskState{
					taskName: {},
				},
			},
			prevLost: false,
			copied:   false,
		},
		{
			name: "TerminalWithState",
			prevAlloc: &structs.Allocation{
				ClientStatus:      structs.AllocClientStatusComplete,
				DesiredTransition: structs.DesiredTransition{},
				TaskStates: map[string]*structs.TaskState{
					taskName: {
						TaskHandle: taskHandle,
					},
				},
			},
			prevLost: false,
			copied:   false,
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.name, func(t *testing.T) {
			newAlloc := &structs.Allocation{
				// Required by propagateTaskState and populated
				// by the scheduler's node iterator.
				AllocatedResources: &structs.AllocatedResources{
					Tasks: map[string]*structs.AllocatedTaskResources{
						taskName: nil, // value isn't used
					},
				},
			}

			propagateTaskState(newAlloc, tc.prevAlloc, tc.prevLost)

			if tc.copied {
				// Assert state was copied
				require.NotNil(t, newAlloc.TaskStates)
				require.Contains(t, newAlloc.TaskStates, taskName)
				require.Equal(t, taskHandle, newAlloc.TaskStates[taskName].TaskHandle)
			} else {
				// Assert state was *not* copied
				require.Empty(t, newAlloc.TaskStates,
					"expected task states not to be copied")
			}
		})
	}
}

// Tests that a client disconnect generates attribute updates and follow up evals.
func TestServiceSched_Client_Disconnect_Creates_Updates_and_Evals(t *testing.T) {
	h := NewHarness(t)
	count := 1
	maxClientDisconnect := 10 * time.Minute

	disconnectedNode, job, unknownAllocs := initNodeAndAllocs(t, h, count, maxClientDisconnect,
		structs.NodeStatusReady, structs.AllocClientStatusRunning)

	// Now disconnect the node
	disconnectedNode.Status = structs.NodeStatusDisconnected
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), disconnectedNode))

	// Create an evaluation triggered by the disconnect
	evals := []*structs.Evaluation{{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		NodeID:      disconnectedNode.ID,
		Status:      structs.EvalStatusPending,
	}}
	nodeStatusUpdateEval := evals[0]
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), evals))

	// Process the evaluation
	err := h.Process(NewServiceScheduler, nodeStatusUpdateEval)
	require.NoError(t, err)
	require.Equal(t, structs.EvalStatusComplete, h.Evals[0].Status)
	require.Len(t, h.Plans, 1, "plan")

	// One followup delayed eval created
	require.Len(t, h.CreateEvals, 1)
	followUpEval := h.CreateEvals[0]
	require.Equal(t, nodeStatusUpdateEval.ID, followUpEval.PreviousEval)
	require.Equal(t, "pending", followUpEval.Status)
	require.NotEmpty(t, followUpEval.WaitUntil)

	// Insert eval in the state store
	testutil.WaitForResult(func() (bool, error) {
		found, err := h.State.EvalByID(nil, followUpEval.ID)
		if err != nil {
			return false, err
		}
		if found == nil {
			return false, nil
		}

		require.Equal(t, nodeStatusUpdateEval.ID, found.PreviousEval)
		require.Equal(t, "pending", found.Status)
		require.NotEmpty(t, found.WaitUntil)

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// Validate that the ClientStatus updates are part of the plan.
	require.Len(t, h.Plans[0].NodeAllocation[disconnectedNode.ID], count)
	// Pending update should have unknown status.
	for _, nodeAlloc := range h.Plans[0].NodeAllocation[disconnectedNode.ID] {
		require.Equal(t, nodeAlloc.ClientStatus, structs.AllocClientStatusUnknown)
	}

	// Simulate that NodeAllocation got processed.
	err = h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), h.Plans[0].NodeAllocation[disconnectedNode.ID])
	require.NoError(t, err, "plan.NodeUpdate")

	// Validate that the StateStore Upsert applied the ClientStatus we specified.
	for _, alloc := range unknownAllocs {
		alloc, err = h.State.AllocByID(nil, alloc.ID)
		require.NoError(t, err)
		require.Equal(t, alloc.ClientStatus, structs.AllocClientStatusUnknown)

		// Allocations have been transitioned to unknown
		require.Equal(t, structs.AllocDesiredStatusRun, alloc.DesiredStatus)
		require.Equal(t, structs.AllocClientStatusUnknown, alloc.ClientStatus)
	}
}

func initNodeAndAllocs(t *testing.T, h *Harness, allocCount int,
	maxClientDisconnect time.Duration, nodeStatus, clientStatus string) (*structs.Node, *structs.Job, []*structs.Allocation) {
	// Node, which is ready
	node := mock.Node()
	node.Status = nodeStatus
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Job with allocations and max_client_disconnect
	job := mock.Job()
	job.TaskGroups[0].Count = allocCount
	job.TaskGroups[0].MaxClientDisconnect = &maxClientDisconnect
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	allocs := make([]*structs.Allocation, allocCount)
	for i := 0; i < allocCount; i++ {
		// Alloc for the running group
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		alloc.DesiredStatus = structs.AllocDesiredStatusRun
		alloc.ClientStatus = clientStatus

		allocs[i] = alloc
	}

	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))
	return node, job, allocs
}
