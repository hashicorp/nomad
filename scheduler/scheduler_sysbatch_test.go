// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"sort"
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/kr/pretty"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestSysBatch_JobRegister(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	_ = createNodes(t, h, 10)

	// Create a job
	job := mock.SystemBatchJob()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to deregister the job
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
	err := h.Process(NewSysBatchScheduler, eval)
	require.NoError(t, err)

	// Ensure a single plan
	require.Len(t, h.Plans, 1)
	plan := h.Plans[0]

	// Ensure the plan does not have annotations
	require.Nil(t, plan.Annotations, "expected no annotations")

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	require.Len(t, planned, 10)

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	require.Len(t, out, 10)

	// Note that all sysbatch allocations have the same name derived from Job.Name
	allocNames := helper.ConvertSlice(out,
		func(alloc *structs.Allocation) string { return alloc.Name })
	expectAllocNames := []string{}
	for i := 0; i < 10; i++ {
		expectAllocNames = append(expectAllocNames, fmt.Sprintf("%s.pinger[0]", job.Name))
	}
	must.SliceContainsAll(t, expectAllocNames, allocNames)

	// Check the available nodes
	count, ok := out[0].Metrics.NodesAvailable["dc1"]
	require.True(t, ok)
	require.Equal(t, 10, count, "bad metrics %#v:", out[0].Metrics)

	must.Eq(t, 10, out[0].Metrics.NodesInPool,
		must.Sprint("expected NodesInPool metric to be set"))

	// Ensure no allocations are queued
	queued := h.Evals[0].QueuedAllocations["my-sysbatch"]
	require.Equal(t, 0, queued, "unexpected queued allocations")

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_JobRegister_AddNode_Running(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	nodes := createNodes(t, h, 10)

	// Generate a fake sysbatch job with allocations
	job := mock.SystemBatchJob()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.SysBatchAlloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-sysbatch.pinger[0]"
		alloc.ClientStatus = structs.AllocClientStatusRunning
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Add a new node.
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Create a mock evaluation to deal with the node update
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
	err := h.Process(NewSysBatchScheduler, eval)
	require.NoError(t, err)

	// Ensure a single plan
	require.Len(t, h.Plans, 1)
	plan := h.Plans[0]

	// Ensure the plan had no node updates
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	require.Empty(t, update)

	// Ensure the plan allocated on the new node
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	require.Len(t, planned, 1)

	// Ensure it allocated on the right node
	_, ok := plan.NodeAllocation[node.ID]
	require.True(t, ok, "allocated on wrong node: %#v", plan)

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	out, _ = structs.FilterTerminalAllocs(out)
	require.Len(t, out, 11)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_JobRegister_AddNode_Dead(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	nodes := createNodes(t, h, 10)

	// Generate a dead sysbatch job with complete allocations
	job := mock.SystemBatchJob()
	job.Status = structs.JobStatusDead // job is dead but not stopped
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.SysBatchAlloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-sysbatch.pinger[0]"
		alloc.ClientStatus = structs.AllocClientStatusComplete
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Add a new node.
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Create a mock evaluation to deal with the node update
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
	err := h.Process(NewSysBatchScheduler, eval)
	require.NoError(t, err)

	// Ensure a single plan
	require.Len(t, h.Plans, 1)
	plan := h.Plans[0]

	// Ensure the plan has no node update
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	require.Len(t, update, 0)

	// Ensure the plan allocates on the new node
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	require.Len(t, planned, 1)

	// Ensure it allocated on the right node
	_, ok := plan.NodeAllocation[node.ID]
	require.True(t, ok, "allocated on wrong node: %#v", plan)

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure 1 non-terminal allocation
	live, _ := structs.FilterTerminalAllocs(out)
	require.Len(t, live, 1)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_JobModify(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	nodes := createNodes(t, h, 10)

	// Generate a fake job with allocations
	job := mock.SystemBatchJob()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.SysBatchAlloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-sysbatch.pinger[0]"
		alloc.ClientStatus = structs.AllocClientStatusPending
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Add a few terminal status allocations, these should be reinstated
	var terminal []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.SysBatchAlloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = "my-sysbatch.pinger[0]"
		alloc.ClientStatus = structs.AllocClientStatusComplete
		terminal = append(terminal, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), terminal))

	// Update the job
	job2 := mock.SystemBatchJob()
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
	err := h.Process(NewSysBatchScheduler, eval)
	require.NoError(t, err)

	// Ensure a single plan
	require.Len(t, h.Plans, 1)
	plan := h.Plans[0]

	// Ensure the plan evicted all allocs
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	require.Equal(t, len(allocs), len(update))

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	require.Len(t, planned, 10)

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	out, _ = structs.FilterTerminalAllocs(out)
	require.Len(t, out, 10)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_JobModify_InPlace(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	nodes := createNodes(t, h, 10)

	job := mock.SystemBatchJob()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.SysBatchAlloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-sysbatch.pinger[0]"
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Update the job
	job2 := mock.SystemBatchJob()
	job2.ID = job.ID
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	// Create a mock evaluation to deal with update
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
	err := h.Process(NewSysBatchScheduler, eval)
	require.NoError(t, err)

	// Ensure a single plan
	require.Len(t, h.Plans, 1)
	plan := h.Plans[0]

	// Ensure the plan did not evict any allocs
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	require.Empty(t, update)

	// Ensure the plan updated the existing allocs
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	require.Len(t, planned, 10)

	for _, p := range planned {
		require.Equal(t, job2, p.Job, "should update job")
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	require.Len(t, out, 10)
	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_JobDeregister_Purged(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	nodes := createNodes(t, h, 10)

	// Create a sysbatch job
	job := mock.SystemBatchJob()

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.SysBatchAlloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-sysbatch.pinger[0]"
		allocs = append(allocs, alloc)
	}
	for _, alloc := range allocs {
		require.NoError(t, h.State.UpsertJobSummary(h.NextIndex(), mock.JobSysBatchSummary(alloc.JobID)))
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
	err := h.Process(NewSysBatchScheduler, eval)
	require.NoError(t, err)

	// Ensure a single plan
	require.Len(t, h.Plans, 1)
	plan := h.Plans[0]

	// Ensure the plan evicted the job from all nodes.
	for _, node := range nodes {
		require.Len(t, plan.NodeUpdate[node.ID], 1)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure no remaining allocations
	out, _ = structs.FilterTerminalAllocs(out)
	require.Empty(t, out)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_JobDeregister_Stopped(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	nodes := createNodes(t, h, 10)

	// Generate a stopped sysbatch job with allocations
	job := mock.SystemBatchJob()
	job.Stop = true
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.SysBatchAlloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-sysbatch.pinger[0]"
		allocs = append(allocs, alloc)
	}
	for _, alloc := range allocs {
		require.NoError(t, h.State.UpsertJobSummary(h.NextIndex(), mock.JobSysBatchSummary(alloc.JobID)))
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
	err := h.Process(NewSysBatchScheduler, eval)
	require.NoError(t, err)

	// Ensure a single plan
	require.Len(t, h.Plans, 1)
	plan := h.Plans[0]

	// Ensure the plan evicted the job from all nodes.
	for _, node := range nodes {
		require.Len(t, plan.NodeUpdate[node.ID], 1)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure no remaining allocations
	out, _ = structs.FilterTerminalAllocs(out)
	require.Empty(t, out)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_NodeDown(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Register a down node
	node := mock.Node()
	node.Status = structs.NodeStatusDown
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Generate a sysbatch job allocated on that node
	job := mock.SystemBatchJob()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	alloc := mock.SysBatchAlloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-sysbatch.pinger[0]"
	alloc.DesiredTransition.Migrate = pointer.Of(true)
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

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
	err := h.Process(NewSysBatchScheduler, eval)
	require.NoError(t, err)

	// Ensure a single plan
	require.Len(t, h.Plans, 1)
	plan := h.Plans[0]

	// Ensure the plan evicted all allocs
	require.Len(t, plan.NodeUpdate[node.ID], 1)

	// Ensure the plan updated the allocation.
	planned := make([]*structs.Allocation, 0)
	for _, allocList := range plan.NodeUpdate {
		planned = append(planned, allocList...)
	}
	require.Len(t, planned, 1)

	// Ensure the allocations is stopped
	p := planned[0]
	require.Equal(t, structs.AllocDesiredStatusStop, p.DesiredStatus)
	// removed badly designed assertion on client_status = lost
	// the actual client_status is pending

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_NodeDrain_Down(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Register a draining node
	node := mock.DrainNode()
	node.Status = structs.NodeStatusDown
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Generate a sysbatch job allocated on that node.
	job := mock.SystemBatchJob()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	alloc := mock.SysBatchAlloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-sysbatch.pinger[0]"
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

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
	err := h.Process(NewSysBatchScheduler, eval)
	require.NoError(t, err)

	// Ensure a single plan
	require.Len(t, h.Plans, 1)
	plan := h.Plans[0]

	// Ensure the plan evicted non terminal allocs
	require.Len(t, plan.NodeUpdate[node.ID], 1)

	// Ensure that the allocation is marked as lost
	var lost []string
	for _, alloc := range plan.NodeUpdate[node.ID] {
		lost = append(lost, alloc.ID)
	}
	require.Equal(t, []string{alloc.ID}, lost)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_NodeDrain(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Register a draining node
	node := mock.DrainNode()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Generate a sysbatch job allocated on that node.
	job := mock.SystemBatchJob()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	alloc := mock.SysBatchAlloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-sysbatch.pinger[0]"
	alloc.DesiredTransition.Migrate = pointer.Of(true)
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

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
	err := h.Process(NewSystemScheduler, eval)
	require.NoError(t, err)

	// Ensure a single plan
	require.Len(t, h.Plans, 1)
	plan := h.Plans[0]

	// Ensure the plan evicted all allocs
	require.Len(t, plan.NodeUpdate[node.ID], 1)

	// Ensure the plan updated the allocation.
	planned := make([]*structs.Allocation, 0)
	for _, allocList := range plan.NodeUpdate {
		planned = append(planned, allocList...)
	}
	require.Len(t, planned, 1)

	// Ensure the allocations is stopped
	require.Equal(t, structs.AllocDesiredStatusStop, planned[0].DesiredStatus)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_NodeUpdate(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Register a node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Generate a sysbatch job allocated on that node.
	job := mock.SystemBatchJob()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	alloc := mock.SysBatchAlloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-system.pinger[0]"
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

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
	err := h.Process(NewSysBatchScheduler, eval)
	require.NoError(t, err)

	// Ensure that queued allocations is zero
	val, ok := h.Evals[0].QueuedAllocations["pinger"]
	require.True(t, ok)
	require.Zero(t, val)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_RetryLimit(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)
	h.Planner = &RejectPlan{h}

	// Create some nodes
	_ = createNodes(t, h, 10)

	// Create a job
	job := mock.SystemBatchJob()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register
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
	err := h.Process(NewSysBatchScheduler, eval)
	require.NoError(t, err)

	// Ensure multiple plans
	require.NotEmpty(t, h.Plans)

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure no allocations placed
	require.Empty(t, out)

	// Should hit the retry limit
	h.AssertEvalStatus(t, structs.EvalStatusFailed)
}

// This test ensures that the scheduler doesn't increment the queued allocation
// count for a task group when allocations can't be created on currently
// available nodes because of constraint mismatches.
func TestSysBatch_Queued_With_Constraints(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	nodes := createNodes(t, h, 3)

	// Generate a sysbatch job which can't be placed on the node
	job := mock.SystemBatchJob()
	job.Constraints = []*structs.Constraint{
		{
			LTarget: "${attr.kernel.name}",
			RTarget: "not_existing_os",
			Operand: "=",
		},
	}
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to deal with the node update
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
	err := h.Process(NewSysBatchScheduler, eval)
	require.NoError(t, err)

	// Ensure that queued allocations is zero
	val, ok := h.Evals[0].QueuedAllocations["pinger"]
	require.True(t, ok)
	require.Zero(t, val)

	failedTGAllocs := h.Evals[0].FailedTGAllocs
	pretty.Println(failedTGAllocs)
	require.NotNil(t, failedTGAllocs)
	require.Contains(t, failedTGAllocs, "pinger")
	require.Equal(t, len(nodes), failedTGAllocs["pinger"].NodesEvaluated)
	require.Equal(t, len(nodes), failedTGAllocs["pinger"].NodesFiltered)

}

func TestSysBatch_Queued_With_Constraints_PartialMatch(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// linux machines
	linux := createNodes(t, h, 3)
	for i := 0; i < 3; i++ {
		node := mock.Node()
		node.Attributes["kernel.name"] = "darwin"
		node.ComputeClass()
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Generate a sysbatch job which can't be placed on the node
	job := mock.SystemBatchJob()
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to deal with the node update
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
	err := h.Process(NewSysBatchScheduler, eval)
	require.NoError(t, err)

	foundNodes := map[string]bool{}
	for n := range h.Plans[0].NodeAllocation {
		foundNodes[n] = true
	}
	expected := map[string]bool{}
	for _, n := range linux {
		expected[n.ID] = true
	}

	require.Equal(t, expected, foundNodes)
}

// This test ensures that the scheduler correctly ignores ineligible
// nodes when scheduling due to a new node being added. The job has two
// task groups constrained to a particular node class. The desired behavior
// should be that the TaskGroup constrained to the newly added node class is
// added and that the TaskGroup constrained to the ineligible node is ignored.
func TestSysBatch_JobConstraint_AddNode(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create two nodes
	var node *structs.Node
	node = mock.Node()
	node.NodeClass = "Class-A"
	require.NoError(t, node.ComputeClass())
	require.Nil(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	var nodeB *structs.Node
	nodeB = mock.Node()
	nodeB.NodeClass = "Class-B"
	require.NoError(t, nodeB.ComputeClass())
	require.Nil(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), nodeB))

	// Make a sysbatch job with two task groups, each constraint to a node class
	job := mock.SystemBatchJob()
	tgA := job.TaskGroups[0]
	tgA.Name = "groupA"
	tgA.Constraints = []*structs.Constraint{{
		LTarget: "${node.class}",
		RTarget: node.NodeClass,
		Operand: "=",
	}}
	tgB := job.TaskGroups[0].Copy()
	tgB.Name = "groupB"
	tgB.Constraints = []*structs.Constraint{{
		LTarget: "${node.class}",
		RTarget: nodeB.NodeClass,
		Operand: "=",
	}}

	// Upsert Job
	job.TaskGroups = []*structs.TaskGroup{tgA, tgB}
	require.Nil(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Evaluate the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.Nil(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	require.Nil(t, h.Process(NewSysBatchScheduler, eval))
	require.Equal(t, "complete", h.Evals[0].Status)

	// QueuedAllocations is drained
	val, ok := h.Evals[0].QueuedAllocations["groupA"]
	require.True(t, ok)
	require.Equal(t, 0, val)

	val, ok = h.Evals[0].QueuedAllocations["groupB"]
	require.True(t, ok)
	require.Equal(t, 0, val)

	// Single plan with two NodeAllocations
	require.Len(t, h.Plans, 1)
	require.Len(t, h.Plans[0].NodeAllocation, 2)

	// Mark the node as ineligible
	node.SchedulingEligibility = structs.NodeSchedulingIneligible

	// Evaluate the node update
	eval2 := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		NodeID:      node.ID,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.Nil(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval2}))

	// Process the 2nd evaluation
	require.Nil(t, h.Process(NewSysBatchScheduler, eval2))
	require.Equal(t, "complete", h.Evals[1].Status)

	// Ensure no new plans
	require.Len(t, h.Plans, 1)

	// Ensure all NodeAllocations are from first Eval
	for _, allocs := range h.Plans[0].NodeAllocation {
		require.Len(t, allocs, 1)
		require.Equal(t, eval.ID, allocs[0].EvalID)
	}

	// Add a new node Class-B
	var nodeBTwo *structs.Node
	nodeBTwo = mock.Node()
	nodeBTwo.NodeClass = "Class-B"
	require.NoError(t, nodeBTwo.ComputeClass())
	require.Nil(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), nodeBTwo))

	// Evaluate the new node
	eval3 := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		NodeID:      nodeBTwo.ID,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	// Ensure 3rd eval is complete
	require.Nil(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval3}))
	require.Nil(t, h.Process(NewSysBatchScheduler, eval3))
	require.Equal(t, "complete", h.Evals[2].Status)

	// Ensure `groupA` fails to be placed due to its constraint, but `groupB` doesn't
	require.Len(t, h.Evals[2].FailedTGAllocs, 1)
	require.Contains(t, h.Evals[2].FailedTGAllocs, "groupA")
	require.NotContains(t, h.Evals[2].FailedTGAllocs, "groupB")

	require.Len(t, h.Plans, 2)
	require.Len(t, h.Plans[1].NodeAllocation, 1)
	// Ensure all NodeAllocations are from first Eval
	for _, allocs := range h.Plans[1].NodeAllocation {
		require.Len(t, allocs, 1)
		require.Equal(t, eval3.ID, allocs[0].EvalID)
	}

	ws := memdb.NewWatchSet()

	allocsNodeOne, err := h.State.AllocsByNode(ws, node.ID)
	require.NoError(t, err)
	require.Len(t, allocsNodeOne, 1)

	allocsNodeTwo, err := h.State.AllocsByNode(ws, nodeB.ID)
	require.NoError(t, err)
	require.Len(t, allocsNodeTwo, 1)

	allocsNodeThree, err := h.State.AllocsByNode(ws, nodeBTwo.ID)
	require.NoError(t, err)
	require.Len(t, allocsNodeThree, 1)
}

// No errors reported when no available nodes prevent placement
func TestSysBatch_ExistingAllocNoNodes(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	var node *structs.Node
	// Create a node
	node = mock.Node()
	require.NoError(t, node.ComputeClass())
	require.Nil(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Make a sysbatch job
	job := mock.SystemBatchJob()
	job.Meta = map[string]string{"version": "1"}
	require.Nil(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Evaluate the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.Nil(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))
	require.Nil(t, h.Process(NewSysBatchScheduler, eval))
	require.Equal(t, "complete", h.Evals[0].Status)

	// QueuedAllocations is drained
	val, ok := h.Evals[0].QueuedAllocations["pinger"]
	require.True(t, ok)
	require.Equal(t, 0, val)

	// The plan has one NodeAllocations
	require.Equal(t, 1, len(h.Plans))

	// Mark the node as ineligible
	node.SchedulingEligibility = structs.NodeSchedulingIneligible

	// Evaluate the job
	eval2 := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		NodeID:      node.ID,
		Status:      structs.EvalStatusPending,
	}
	require.Nil(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval2}))
	require.Nil(t, h.Process(NewSysBatchScheduler, eval2))
	require.Equal(t, "complete", h.Evals[1].Status)

	// Create a new job version, deploy
	job2 := job.Copy()
	job2.Meta["version"] = "2"
	require.Nil(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	// Run evaluation as a plan
	eval3 := &structs.Evaluation{
		Namespace:    structs.DefaultNamespace,
		ID:           uuid.Generate(),
		Priority:     job2.Priority,
		TriggeredBy:  structs.EvalTriggerJobRegister,
		JobID:        job2.ID,
		Status:       structs.EvalStatusPending,
		AnnotatePlan: true,
	}

	// Ensure New eval is complete
	require.Nil(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval3}))
	require.Nil(t, h.Process(NewSysBatchScheduler, eval3))
	require.Equal(t, "complete", h.Evals[2].Status)

	// Ensure there are no FailedTGAllocs
	require.Equal(t, 0, len(h.Evals[2].FailedTGAllocs))
	require.Equal(t, 0, h.Evals[2].QueuedAllocations[job2.Name])
}

func TestSysBatch_ConstraintErrors(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	var node *structs.Node
	// Register some nodes
	// the tag "aaaaaa" is hashed so that the nodes are processed
	// in an order other than good, good, bad
	for _, tag := range []string{"aaaaaa", "foo", "foo", "foo"} {
		node = mock.Node()
		node.Meta["tag"] = tag
		require.NoError(t, node.ComputeClass())
		require.Nil(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Mark the last node as ineligible
	node.SchedulingEligibility = structs.NodeSchedulingIneligible

	// Make a job with a constraint that matches a subset of the nodes
	job := mock.SystemBatchJob()
	job.Priority = 100
	job.Constraints = append(job.Constraints,
		&structs.Constraint{
			LTarget: "${meta.tag}",
			RTarget: "foo",
			Operand: "=",
		})

	require.Nil(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Evaluate the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.Nil(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))
	require.Nil(t, h.Process(NewSysBatchScheduler, eval))
	require.Equal(t, "complete", h.Evals[0].Status)

	// QueuedAllocations is drained
	val, ok := h.Evals[0].QueuedAllocations["pinger"]
	require.True(t, ok)
	require.Equal(t, 0, val)

	// The plan has two NodeAllocations
	require.Equal(t, 1, len(h.Plans))
	require.Nil(t, h.Plans[0].Annotations)
	require.Equal(t, 2, len(h.Plans[0].NodeAllocation))

	// Two nodes were allocated and are pending. (unlike system jobs, sybatch
	// jobs are not auto set to running)
	ws := memdb.NewWatchSet()
	as, err := h.State.AllocsByJob(ws, structs.DefaultNamespace, job.ID, false)
	require.Nil(t, err)

	pending := 0
	for _, a := range as {
		if "pending" == a.Job.Status {
			pending++
		}
	}

	require.Equal(t, 2, len(as))
	require.Equal(t, 2, pending)

	// Failed allocations is empty
	require.Equal(t, 0, len(h.Evals[0].FailedTGAllocs))
}

func TestSysBatch_ChainedAlloc(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create some nodes
	_ = createNodes(t, h, 10)

	// Create a sysbatch job
	job := mock.SystemBatchJob()
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
	err := h.Process(NewSysBatchScheduler, eval)
	require.NoError(t, err)

	var allocIDs []string
	for _, allocList := range h.Plans[0].NodeAllocation {
		for _, alloc := range allocList {
			allocIDs = append(allocIDs, alloc.ID)
		}
	}
	sort.Strings(allocIDs)

	// Create a new harness to invoke the scheduler again
	h1 := NewHarnessWithState(t, h.State)
	job1 := mock.SystemBatchJob()
	job1.ID = job.ID
	job1.TaskGroups[0].Tasks[0].Env = make(map[string]string)
	job1.TaskGroups[0].Tasks[0].Env["foo"] = "bar"
	require.NoError(t, h1.State.UpsertJob(structs.MsgTypeTestSetup, h1.NextIndex(), nil, job1))

	// Insert two more nodes
	for i := 0; i < 2; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

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
	err = h1.Process(NewSysBatchScheduler, eval1)
	require.NoError(t, err)

	require.Len(t, h.Plans, 1)
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
	require.Equal(t, allocIDs, prevAllocs)

	// Ensuring two new allocations don't have any chained allocations
	require.Len(t, newAllocs, 2)
}

func TestSysBatch_PlanWithDrainedNode(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Register two nodes with two different classes
	node := mock.DrainNode()
	node.NodeClass = "green"
	require.NoError(t, node.ComputeClass())
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	node2 := mock.Node()
	node2.NodeClass = "blue"
	require.NoError(t, node2.ComputeClass())
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node2))

	// Create a sysbatch job with two task groups, each constrained on node class
	job := mock.SystemBatchJob()
	tg1 := job.TaskGroups[0]
	tg1.Constraints = append(tg1.Constraints,
		&structs.Constraint{
			LTarget: "${node.class}",
			RTarget: "green",
			Operand: "==",
		})

	tg2 := tg1.Copy()
	tg2.Name = "pinger2"
	tg2.Constraints[0].RTarget = "blue"
	job.TaskGroups = append(job.TaskGroups, tg2)
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create an allocation on each node
	alloc := mock.SysBatchAlloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-sysbatch.pinger[0]"
	alloc.DesiredTransition.Migrate = pointer.Of(true)
	alloc.TaskGroup = "pinger"

	alloc2 := mock.SysBatchAlloc()
	alloc2.Job = job
	alloc2.JobID = job.ID
	alloc2.NodeID = node2.ID
	alloc2.Name = "my-sysbatch.pinger2[0]"
	alloc2.TaskGroup = "pinger2"
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc, alloc2}))

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
	err := h.Process(NewSysBatchScheduler, eval)
	require.NoError(t, err)

	// Ensure a single plan
	require.Len(t, h.Plans, 1)
	plan := h.Plans[0]

	// Ensure the plan evicted the alloc on the failed node
	planned := plan.NodeUpdate[node.ID]
	require.Len(t, plan.NodeUpdate[node.ID], 1)

	// Ensure the plan didn't place
	require.Empty(t, plan.NodeAllocation)

	// Ensure the allocations is stopped
	require.Equal(t, structs.AllocDesiredStatusStop, planned[0].DesiredStatus)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_QueuedAllocsMultTG(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Register two nodes with two different classes
	node := mock.Node()
	node.NodeClass = "green"
	require.NoError(t, node.ComputeClass())
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	node2 := mock.Node()
	node2.NodeClass = "blue"
	require.NoError(t, node2.ComputeClass())
	require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node2))

	// Create a sysbatch job with two task groups, each constrained on node class
	job := mock.SystemBatchJob()
	tg1 := job.TaskGroups[0]
	tg1.Constraints = append(tg1.Constraints,
		&structs.Constraint{
			LTarget: "${node.class}",
			RTarget: "green",
			Operand: "==",
		})

	tg2 := tg1.Copy()
	tg2.Name = "pinger2"
	tg2.Constraints[0].RTarget = "blue"
	job.TaskGroups = append(job.TaskGroups, tg2)
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	err := h.Process(NewSysBatchScheduler, eval)
	require.NoError(t, err)

	// Ensure a single plan
	require.Len(t, h.Plans, 1)

	qa := h.Evals[0].QueuedAllocations
	require.Zero(t, qa["pinger"])
	require.Zero(t, qa["pinger2"])

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_Preemption(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)

	// Create nodes
	nodes := make([]*structs.Node, 0)
	for i := 0; i < 2; i++ {
		node := mock.Node()
		// TODO: remove in 0.11
		node.Resources = &structs.Resources{
			CPU:      3072,
			MemoryMB: 5034,
			DiskMB:   20 * 1024,
			Networks: []*structs.NetworkResource{{
				Device: "eth0",
				CIDR:   "192.168.0.100/32",
				MBits:  1000,
			}},
		}
		node.NodeResources = &structs.NodeResources{
			Cpu:    structs.NodeCpuResources{CpuShares: 3072},
			Memory: structs.NodeMemoryResources{MemoryMB: 5034},
			Disk:   structs.NodeDiskResources{DiskMB: 20 * 1024},
			Networks: []*structs.NetworkResource{{
				Device: "eth0",
				CIDR:   "192.168.0.100/32",
				MBits:  1000,
			}},
			NodeNetworks: []*structs.NodeNetworkResource{{
				Mode:   "host",
				Device: "eth0",
				Addresses: []structs.NodeNetworkAddress{{
					Family:  structs.NodeNetworkAF_IPv4,
					Alias:   "default",
					Address: "192.168.0.100",
				}},
			}},
		}
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
		nodes = append(nodes, node)
	}

	// Enable Preemption
	err := h.State.SchedulerSetConfig(h.NextIndex(), &structs.SchedulerConfiguration{
		PreemptionConfig: structs.PreemptionConfig{
			SysBatchSchedulerEnabled: true,
		},
	})
	require.NoError(t, err)

	// Create some low priority batch jobs and allocations for them
	// One job uses a reserved port
	job1 := mock.BatchJob()
	job1.Type = structs.JobTypeBatch
	job1.Priority = 20
	job1.TaskGroups[0].Tasks[0].Resources = &structs.Resources{
		CPU:      512,
		MemoryMB: 1024,
		Networks: []*structs.NetworkResource{{
			MBits: 200,
			ReservedPorts: []structs.Port{{
				Label: "web",
				Value: 80,
			}},
		}},
	}

	alloc1 := mock.Alloc()
	alloc1.Job = job1
	alloc1.JobID = job1.ID
	alloc1.NodeID = nodes[0].ID
	alloc1.Name = "my-job[0]"
	alloc1.TaskGroup = job1.TaskGroups[0].Name
	alloc1.AllocatedResources = &structs.AllocatedResources{
		Tasks: map[string]*structs.AllocatedTaskResources{
			"web": {
				Cpu:    structs.AllocatedCpuResources{CpuShares: 512},
				Memory: structs.AllocatedMemoryResources{MemoryMB: 1024},
				Networks: []*structs.NetworkResource{{
					Device: "eth0",
					IP:     "192.168.0.100",
					MBits:  200,
				}},
			},
		},
		Shared: structs.AllocatedSharedResources{DiskMB: 5 * 1024},
	}
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job1))

	job2 := mock.BatchJob()
	job2.Type = structs.JobTypeBatch
	job2.Priority = 20
	job2.TaskGroups[0].Tasks[0].Resources = &structs.Resources{
		CPU:      512,
		MemoryMB: 1024,
		Networks: []*structs.NetworkResource{{MBits: 200}},
	}

	alloc2 := mock.Alloc()
	alloc2.Job = job2
	alloc2.JobID = job2.ID
	alloc2.NodeID = nodes[0].ID
	alloc2.Name = "my-job[2]"
	alloc2.TaskGroup = job2.TaskGroups[0].Name
	alloc2.AllocatedResources = &structs.AllocatedResources{
		Tasks: map[string]*structs.AllocatedTaskResources{
			"web": {
				Cpu:    structs.AllocatedCpuResources{CpuShares: 512},
				Memory: structs.AllocatedMemoryResources{MemoryMB: 1024},
				Networks: []*structs.NetworkResource{{
					Device: "eth0",
					IP:     "192.168.0.100",
					MBits:  200,
				}},
			},
		},
		Shared: structs.AllocatedSharedResources{DiskMB: 5 * 1024},
	}
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	job3 := mock.Job()
	job3.Type = structs.JobTypeBatch
	job3.Priority = 40
	job3.TaskGroups[0].Tasks[0].Resources = &structs.Resources{
		CPU:      1024,
		MemoryMB: 2048,
		Networks: []*structs.NetworkResource{{
			Device: "eth0",
			MBits:  400,
		}},
	}

	alloc3 := mock.Alloc()
	alloc3.Job = job3
	alloc3.JobID = job3.ID
	alloc3.NodeID = nodes[0].ID
	alloc3.Name = "my-job[0]"
	alloc3.TaskGroup = job3.TaskGroups[0].Name
	alloc3.AllocatedResources = &structs.AllocatedResources{
		Tasks: map[string]*structs.AllocatedTaskResources{
			"web": {
				Cpu:    structs.AllocatedCpuResources{CpuShares: 1024},
				Memory: structs.AllocatedMemoryResources{MemoryMB: 25},
				Networks: []*structs.NetworkResource{{
					Device: "eth0",
					IP:     "192.168.0.100",
					MBits:  400,
				}},
			},
		},
		Shared: structs.AllocatedSharedResources{DiskMB: 5 * 1024},
	}
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc1, alloc2, alloc3}))

	// Create a high priority job and allocs for it
	// These allocs should not be preempted

	job4 := mock.BatchJob()
	job4.Type = structs.JobTypeBatch
	job4.Priority = 100
	job4.TaskGroups[0].Tasks[0].Resources = &structs.Resources{
		CPU:      1024,
		MemoryMB: 2048,
		Networks: []*structs.NetworkResource{{MBits: 100}},
	}

	alloc4 := mock.Alloc()
	alloc4.Job = job4
	alloc4.JobID = job4.ID
	alloc4.NodeID = nodes[0].ID
	alloc4.Name = "my-job4[0]"
	alloc4.TaskGroup = job4.TaskGroups[0].Name
	alloc4.AllocatedResources = &structs.AllocatedResources{
		Tasks: map[string]*structs.AllocatedTaskResources{
			"web": {
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 1024,
				},
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 2048,
				},
				Networks: []*structs.NetworkResource{
					{
						Device:        "eth0",
						IP:            "192.168.0.100",
						ReservedPorts: []structs.Port{{Label: "web", Value: 80}},
						MBits:         100,
					},
				},
			},
		},
		Shared: structs.AllocatedSharedResources{
			DiskMB: 2 * 1024,
		},
	}
	require.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job4))
	require.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc4}))

	// Create a system job such that it would need to preempt both allocs to succeed
	job := mock.SystemBatchJob()
	job.Priority = 100
	job.TaskGroups[0].Tasks[0].Resources = &structs.Resources{
		CPU:      1948,
		MemoryMB: 256,
		Networks: []*structs.NetworkResource{{
			MBits:        800,
			DynamicPorts: []structs.Port{{Label: "http"}},
		}},
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
	require.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err = h.Process(NewSysBatchScheduler, eval)
	require.Nil(t, err)

	// Ensure a single plan
	require.Equal(t, 1, len(h.Plans))
	plan := h.Plans[0]

	// Ensure the plan doesn't have annotations
	require.Nil(t, plan.Annotations)

	// Ensure the plan allocated on both nodes
	var planned []*structs.Allocation
	preemptingAllocId := ""
	require.Equal(t, 2, len(plan.NodeAllocation))

	// The alloc that got placed on node 1 is the preemptor
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
		for _, alloc := range allocList {
			if alloc.NodeID == nodes[0].ID {
				preemptingAllocId = alloc.ID
			}
		}
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	require.Equal(t, 2, len(out))

	// Verify that one node has preempted allocs
	require.NotNil(t, plan.NodePreemptions[nodes[0].ID])
	preemptedAllocs := plan.NodePreemptions[nodes[0].ID]

	// Verify that three jobs have preempted allocs
	require.Equal(t, 3, len(preemptedAllocs))

	expectedPreemptedJobIDs := []string{job1.ID, job2.ID, job3.ID}

	// We expect job1, job2 and job3 to have preempted allocations
	// job4 should not have any allocs preempted
	for _, alloc := range preemptedAllocs {
		require.Contains(t, expectedPreemptedJobIDs, alloc.JobID)
	}
	// Look up the preempted allocs by job ID
	ws = memdb.NewWatchSet()

	for _, jobId := range expectedPreemptedJobIDs {
		out, err = h.State.AllocsByJob(ws, structs.DefaultNamespace, jobId, false)
		require.NoError(t, err)
		for _, alloc := range out {
			require.Equal(t, structs.AllocDesiredStatusEvict, alloc.DesiredStatus)
			require.Equal(t, fmt.Sprintf("Preempted by alloc ID %v", preemptingAllocId), alloc.DesiredDescription)
		}
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_canHandle(t *testing.T) {
	ci.Parallel(t)

	s := SystemScheduler{sysbatch: true}
	t.Run("sysbatch register", func(t *testing.T) {
		require.True(t, s.canHandle(structs.EvalTriggerJobRegister))
	})
	t.Run("sysbatch scheduled", func(t *testing.T) {
		require.False(t, s.canHandle(structs.EvalTriggerScheduled))
	})
	t.Run("sysbatch periodic", func(t *testing.T) {
		require.True(t, s.canHandle(structs.EvalTriggerPeriodicJob))
	})
}
func createNodes(t *testing.T, h *Harness, n int) []*structs.Node {
	nodes := make([]*structs.Node, n)
	for i := 0; i < n; i++ {
		node := mock.Node()
		nodes[i] = node
		require.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}
	return nodes
}
