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
	"github.com/hashicorp/nomad/scheduler/tests"
	"github.com/kr/pretty"
	"github.com/shoenig/test/must"
)

func TestSysBatch_JobRegister(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	// Create some nodes
	_ = createNodes(t, h, 10)

	// Create a job
	job := mock.SystemBatchJob()
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to deregister the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	// Ensure a single plan
	must.Len(t, 1, h.Plans)
	plan := h.Plans[0]

	// Ensure the plan does not have annotations
	must.Nil(t, plan.Annotations, must.Sprint("expected no annotations"))

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	must.Len(t, 10, planned)

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	must.NoError(t, err)

	// Ensure all allocations placed
	must.Len(t, 10, out)

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
	must.True(t, ok)
	must.Eq(t, 10, count, must.Sprintf("bad metrics %#v:", out[0].Metrics))

	must.Eq(t, 10, out[0].Metrics.NodesInPool,
		must.Sprint("expected NodesInPool metric to be set"))

	// Ensure no allocations are queued
	queued := h.Evals[0].QueuedAllocations["my-sysbatch"]
	must.Eq(t, 0, queued, must.Sprint("unexpected queued allocations"))

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_JobRegister_AddNode_Running(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	// Create some nodes
	nodes := createNodes(t, h, 10)

	// Generate a fake sysbatch job with allocations
	job := mock.SystemBatchJob()
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Add a new node.
	node := mock.Node()
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Create a mock evaluation to deal with the node update
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	// Ensure a single plan
	must.Len(t, 1, h.Plans)
	plan := h.Plans[0]

	// Ensure the plan had no node updates
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	must.SliceLen(t, 0, update)

	// Ensure the plan allocated on the new node
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	must.Len(t, 1, planned)

	// Ensure it allocated on the right node
	_, ok := plan.NodeAllocation[node.ID]
	must.True(t, ok, must.Sprintf("allocated on wrong node: %#v", plan))

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	must.NoError(t, err)

	// Ensure all allocations placed
	out, _ = structs.FilterTerminalAllocs(out)
	must.Len(t, 11, out)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_JobRegister_AddNode_Dead(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	// Create some nodes
	nodes := createNodes(t, h, 10)

	// Generate a dead sysbatch job with complete allocations
	job := mock.SystemBatchJob()
	job.Status = structs.JobStatusDead // job is dead but not stopped
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Add a new node.
	node := mock.Node()
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Create a mock evaluation to deal with the node update
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	// Ensure a single plan
	must.Len(t, 1, h.Plans)
	plan := h.Plans[0]

	// Ensure the plan has no node update
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	must.Len(t, 0, update)

	// Ensure the plan allocates on the new node
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	must.Len(t, 1, planned)

	// Ensure it allocated on the right node
	_, ok := plan.NodeAllocation[node.ID]
	must.True(t, ok, must.Sprintf("allocated on wrong node: %#v", plan))

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	must.NoError(t, err)

	// Ensure 1 non-terminal allocation
	live, _ := structs.FilterTerminalAllocs(out)
	must.Len(t, 1, live)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_JobModify(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	// Create some nodes
	nodes := createNodes(t, h, 10)

	// Generate a fake job with allocations
	job := mock.SystemBatchJob()
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

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
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), terminal))

	// Update the job
	job2 := mock.SystemBatchJob()
	job2.ID = job.ID

	// Update the task, such that it cannot be done in-place
	job2.TaskGroups[0].Tasks[0].Config["command"] = "/bin/other"
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	// Ensure a single plan
	must.Len(t, 1, h.Plans)
	plan := h.Plans[0]

	// Ensure the plan evicted all allocs
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	must.Eq(t, len(allocs), len(update))

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	must.Len(t, 10, planned)

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	must.NoError(t, err)

	// Ensure all allocations placed
	out, _ = structs.FilterTerminalAllocs(out)
	must.Len(t, 10, out)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_JobModify_InPlace(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	// Create some nodes
	nodes := createNodes(t, h, 10)

	job := mock.SystemBatchJob()
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.SysBatchAlloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-sysbatch.pinger[0]"
		allocs = append(allocs, alloc)
	}
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Update the job
	job2 := mock.SystemBatchJob()
	job2.ID = job.ID
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

	// Create a mock evaluation to deal with update
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	// Ensure a single plan
	must.Len(t, 1, h.Plans)
	plan := h.Plans[0]

	// Ensure the plan did not evict any allocs
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	must.SliceLen(t, 0, update)

	// Ensure the plan updated the existing allocs
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	must.Len(t, 10, planned)

	for _, p := range planned {
		must.Eq(t, job2, p.Job, must.Sprint("should update job"))
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	must.NoError(t, err)

	// Ensure all allocations placed
	must.Len(t, 10, out)
	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_JobDeregister_Purged(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

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
		must.NoError(t, h.State.UpsertJobSummary(h.NextIndex(), mock.JobSysBatchSummary(alloc.JobID)))
	}
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Create a mock evaluation to deregister the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobDeregister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	// Ensure a single plan
	must.Len(t, 1, h.Plans)
	plan := h.Plans[0]

	// Ensure the plan evicted the job from all nodes.
	for _, node := range nodes {
		must.Len(t, 1, plan.NodeUpdate[node.ID])
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	must.NoError(t, err)

	// Ensure no remaining allocations
	out, _ = structs.FilterTerminalAllocs(out)
	must.SliceLen(t, 0, out)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_JobDeregister_Stopped(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	// Create some nodes
	nodes := createNodes(t, h, 10)

	// Generate a stopped sysbatch job with allocations
	job := mock.SystemBatchJob()
	job.Stop = true
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
		must.NoError(t, h.State.UpsertJobSummary(h.NextIndex(), mock.JobSysBatchSummary(alloc.JobID)))
	}
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// Create a mock evaluation to deregister the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobDeregister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	// Ensure a single plan
	must.Len(t, 1, h.Plans)
	plan := h.Plans[0]

	// Ensure the plan evicted the job from all nodes.
	for _, node := range nodes {
		must.Len(t, 1, plan.NodeUpdate[node.ID])
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	must.NoError(t, err)

	// Ensure no remaining allocations
	out, _ = structs.FilterTerminalAllocs(out)
	must.SliceLen(t, 0, out)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_NodeDown(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	// Register a down node
	node := mock.Node()
	node.Status = structs.NodeStatusDown
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Generate a sysbatch job allocated on that node
	job := mock.SystemBatchJob()
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	alloc := mock.SysBatchAlloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-sysbatch.pinger[0]"
	alloc.DesiredTransition.Migrate = pointer.Of(true)
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

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
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	// Ensure a single plan
	must.Len(t, 1, h.Plans)
	plan := h.Plans[0]

	// Ensure the plan evicted all allocs
	must.Len(t, 1, plan.NodeUpdate[node.ID])

	// Ensure the plan updated the allocation.
	planned := make([]*structs.Allocation, 0)
	for _, allocList := range plan.NodeUpdate {
		planned = append(planned, allocList...)
	}
	must.Len(t, 1, planned)

	// Ensure the allocations is stopped
	p := planned[0]
	must.Eq(t, structs.AllocDesiredStatusStop, p.DesiredStatus)
	// removed badly designed assertion on client_status = lost
	// the actual client_status is pending

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_NodeDrain_Down(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	// Register a draining node
	node := mock.DrainNode()
	node.Status = structs.NodeStatusDown
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Generate a sysbatch job allocated on that node.
	job := mock.SystemBatchJob()
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	alloc := mock.SysBatchAlloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-sysbatch.pinger[0]"
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

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
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	// Ensure a single plan
	must.Len(t, 1, h.Plans)
	plan := h.Plans[0]

	// Ensure the plan evicted non terminal allocs
	must.Len(t, 1, plan.NodeUpdate[node.ID])

	// Ensure that the allocation is marked as lost
	var lost []string
	for _, alloc := range plan.NodeUpdate[node.ID] {
		lost = append(lost, alloc.ID)
	}
	must.Eq(t, []string{alloc.ID}, lost)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_NodeDrain(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	// Register a draining node
	node := mock.DrainNode()
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Generate a sysbatch job allocated on that node.
	job := mock.SystemBatchJob()
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	alloc := mock.SysBatchAlloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-sysbatch.pinger[0]"
	alloc.DesiredTransition.Migrate = pointer.Of(true)
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

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
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSystemScheduler, eval)
	must.NoError(t, err)

	// Ensure a single plan
	must.Len(t, 1, h.Plans)
	plan := h.Plans[0]

	// Ensure the plan evicted all allocs
	must.Len(t, 1, plan.NodeUpdate[node.ID])

	// Ensure the plan updated the allocation.
	planned := make([]*structs.Allocation, 0)
	for _, allocList := range plan.NodeUpdate {
		planned = append(planned, allocList...)
	}
	must.Len(t, 1, planned)

	// Ensure the allocations is stopped
	must.Eq(t, structs.AllocDesiredStatusStop, planned[0].DesiredStatus)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_NodeUpdate(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	// Register a node
	node := mock.Node()
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Generate a sysbatch job allocated on that node.
	job := mock.SystemBatchJob()
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	alloc := mock.SysBatchAlloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-system.pinger[0]"
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc}))

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
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	// Ensure that queued allocations is zero
	val, ok := h.Evals[0].QueuedAllocations["pinger"]
	must.True(t, ok)
	must.Zero(t, val)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_RetryLimit(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)
	h.Planner = &tests.RejectPlan{h}

	// Create some nodes
	_ = createNodes(t, h, 10)

	// Create a job
	job := mock.SystemBatchJob()
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	// Ensure multiple plans
	must.SliceNotEmpty(t, h.Plans)

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	must.NoError(t, err)

	// Ensure no allocations placed
	must.SliceLen(t, 0, out)

	// Should hit the retry limit
	h.AssertEvalStatus(t, structs.EvalStatusFailed)
}

// This test ensures that the scheduler doesn't increment the queued allocation
// count for a task group when allocations can't be created on currently
// available nodes because of constraint mismatches.
func TestSysBatch_Queued_With_Constraints(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

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
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to deal with the node update
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	// Ensure that queued allocations is zero
	val, ok := h.Evals[0].QueuedAllocations["pinger"]
	must.True(t, ok)
	must.Zero(t, val)

	failedTGAllocs := h.Evals[0].FailedTGAllocs
	pretty.Println(failedTGAllocs)
	must.NotNil(t, failedTGAllocs)
	must.MapContainsKey(t, failedTGAllocs, "pinger")
	must.Eq(t, len(nodes), failedTGAllocs["pinger"].NodesEvaluated)
	must.Eq(t, len(nodes), failedTGAllocs["pinger"].NodesFiltered)

}

func TestSysBatch_Queued_With_Constraints_PartialMatch(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	// linux machines
	linux := createNodes(t, h, 3)
	for i := 0; i < 3; i++ {
		node := mock.Node()
		node.Attributes["kernel.name"] = "darwin"
		node.ComputeClass()
		must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}

	// Generate a sysbatch job which can't be placed on the node
	job := mock.SystemBatchJob()
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to deal with the node update
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	foundNodes := map[string]bool{}
	for n := range h.Plans[0].NodeAllocation {
		foundNodes[n] = true
	}
	expected := map[string]bool{}
	for _, n := range linux {
		expected[n.ID] = true
	}

	must.Eq(t, expected, foundNodes)
}

// This test ensures that the scheduler correctly ignores ineligible
// nodes when scheduling due to a new node being added. The job has two
// task groups constrained to a particular node class. The desired behavior
// should be that the TaskGroup constrained to the newly added node class is
// added and that the TaskGroup constrained to the ineligible node is ignored.
func TestSysBatch_JobConstraint_AddNode(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	// Create two nodes
	var node *structs.Node
	node = mock.Node()
	node.NodeClass = "Class-A"
	must.NoError(t, node.ComputeClass())
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	var nodeB *structs.Node
	nodeB = mock.Node()
	nodeB.NodeClass = "Class-B"
	must.NoError(t, nodeB.ComputeClass())
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), nodeB))

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
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Evaluate the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	must.NoError(t, h.Process(NewSysBatchScheduler, eval))
	must.Eq(t, "complete", h.Evals[0].Status)

	// QueuedAllocations is drained
	val, ok := h.Evals[0].QueuedAllocations["groupA"]
	must.True(t, ok)
	must.Eq(t, 0, val)

	val, ok = h.Evals[0].QueuedAllocations["groupB"]
	must.True(t, ok)
	must.Eq(t, 0, val)

	// Single plan with two NodeAllocations
	must.Len(t, 1, h.Plans)
	must.MapLen(t, 2, h.Plans[0].NodeAllocation)

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
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval2}))

	// Process the 2nd evaluation
	must.NoError(t, h.Process(NewSysBatchScheduler, eval2))
	must.Eq(t, "complete", h.Evals[1].Status)

	// Ensure no new plans
	must.Len(t, 1, h.Plans)

	// Ensure all NodeAllocations are from first Eval
	for _, allocs := range h.Plans[0].NodeAllocation {
		must.Len(t, 1, allocs)
		must.Eq(t, eval.ID, allocs[0].EvalID)
	}

	// Add a new node Class-B
	var nodeBTwo *structs.Node
	nodeBTwo = mock.Node()
	nodeBTwo.NodeClass = "Class-B"
	must.NoError(t, nodeBTwo.ComputeClass())
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), nodeBTwo))

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
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval3}))
	must.NoError(t, h.Process(NewSysBatchScheduler, eval3))
	must.Eq(t, "complete", h.Evals[2].Status)

	must.Len(t, 2, h.Plans)
	must.MapLen(t, 1, h.Plans[1].NodeAllocation)
	// Ensure all NodeAllocations are from first Eval
	for _, allocs := range h.Plans[1].NodeAllocation {
		must.Len(t, 1, allocs)
		must.Eq(t, eval3.ID, allocs[0].EvalID)
	}

	ws := memdb.NewWatchSet()

	allocsNodeOne, err := h.State.AllocsByNode(ws, node.ID)
	must.NoError(t, err)
	must.Len(t, 1, allocsNodeOne)

	allocsNodeTwo, err := h.State.AllocsByNode(ws, nodeB.ID)
	must.NoError(t, err)
	must.Len(t, 1, allocsNodeTwo)

	allocsNodeThree, err := h.State.AllocsByNode(ws, nodeBTwo.ID)
	must.NoError(t, err)
	must.Len(t, 1, allocsNodeThree)
}

func TestSysBatch_JobConstraint_AllFiltered(t *testing.T) {
	ci.Parallel(t)
	h := tests.NewHarness(t)

	// Create two nodes, one with a custom class
	node := mock.Node()
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	node2 := mock.Node()
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node2))

	// Create a job with a constraint
	job := mock.SystemBatchJob()
	job.Priority = structs.JobDefaultPriority
	fooConstraint := &structs.Constraint{
		LTarget: "${node.unique.name}",
		RTarget: "something-else",
		Operand: "==",
	}
	job.Constraints = []*structs.Constraint{fooConstraint}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to start the job, which will run on the foo node
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSystemScheduler, eval)
	must.NoError(t, err)

	// Ensure a single eval
	must.Len(t, 1, h.Evals)
	eval = h.Evals[0]

	// Ensure that the eval reports failed allocation
	must.Eq(t, len(eval.FailedTGAllocs), 1)
	// Ensure that the failed allocation is due to constraint on both nodes
	must.Eq(t, eval.FailedTGAllocs[job.TaskGroups[0].Name].ConstraintFiltered[fooConstraint.String()], 2)
}

func TestSysBatch_JobConstraint_RunMultiple(t *testing.T) {
	ci.Parallel(t)
	h := tests.NewHarness(t)

	// Create two nodes, one with a custom class
	fooNode := mock.Node()
	fooNode.Name = "foo"
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), fooNode))

	barNode := mock.Node()
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), barNode))

	// Create a job with a constraint
	job := mock.SystemBatchJob()
	job.Priority = structs.JobDefaultPriority
	fooConstraint := &structs.Constraint{
		LTarget: "${node.unique.name}",
		RTarget: fooNode.Name,
		Operand: "==",
	}
	job.Constraints = []*structs.Constraint{fooConstraint}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to start the job, which will run on the foo node
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSystemScheduler, eval)
	must.NoError(t, err)

	// Create a mock evaluation to run the job again, which will not place any
	// new allocations (fooNode is already running, barNode is constrained), but
	// will not report failed allocations
	eval2 := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval2}))

	err = h.Process(NewSystemScheduler, eval2)
	must.NoError(t, err)

	// Ensure a single plan
	must.Len(t, 1, h.Plans)

	// Ensure that no evals report a failed allocation
	for _, eval := range h.Evals {
		must.Eq(t, 0, len(eval.FailedTGAllocs))
	}

	// Ensure that plan includes allocation running on fooNode
	must.Len(t, 1, h.Plans[0].NodeAllocation[fooNode.ID])
	// Ensure that plan does not include allocation running on barNode
	must.Len(t, 0, h.Plans[0].NodeAllocation[barNode.ID])
}

// No errors reported when no available nodes prevent placement
func TestSysBatch_ExistingAllocNoNodes(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	var node *structs.Node
	// Create a node
	node = mock.Node()
	must.NoError(t, node.ComputeClass())
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// Make a sysbatch job
	job := mock.SystemBatchJob()
	job.Meta = map[string]string{"version": "1"}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Evaluate the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))
	must.NoError(t, h.Process(NewSysBatchScheduler, eval))
	must.Eq(t, "complete", h.Evals[0].Status)

	// QueuedAllocations is drained
	val, ok := h.Evals[0].QueuedAllocations["pinger"]
	must.True(t, ok)
	must.Eq(t, 0, val)

	// The plan has one NodeAllocations
	must.Eq(t, 1, len(h.Plans))

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
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval2}))
	must.NoError(t, h.Process(NewSysBatchScheduler, eval2))
	must.Eq(t, "complete", h.Evals[1].Status)

	// Create a new job version, deploy
	job2 := job.Copy()
	job2.Meta["version"] = "2"
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

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
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval3}))
	must.NoError(t, h.Process(NewSysBatchScheduler, eval3))
	must.Eq(t, "complete", h.Evals[2].Status)

	// Ensure there are no FailedTGAllocs
	must.Eq(t, 0, len(h.Evals[2].FailedTGAllocs))
	must.Eq(t, 0, h.Evals[2].QueuedAllocations[job2.Name])
}

func TestSysBatch_ConstraintErrors(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	var node *structs.Node
	// Register some nodes
	// the tag "aaaaaa" is hashed so that the nodes are processed
	// in an order other than good, good, bad
	for _, tag := range []string{"aaaaaa", "foo", "foo", "foo"} {
		node = mock.Node()
		node.Meta["tag"] = tag
		must.NoError(t, node.ComputeClass())
		must.Nil(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
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

	must.Nil(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Evaluate the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))
	must.NoError(t, h.Process(NewSysBatchScheduler, eval))
	must.Eq(t, "complete", h.Evals[0].Status)

	// QueuedAllocations is drained
	val, ok := h.Evals[0].QueuedAllocations["pinger"]
	must.True(t, ok)
	must.Eq(t, 0, val)

	// The plan has two NodeAllocations
	must.Eq(t, 1, len(h.Plans))
	must.Nil(t, h.Plans[0].Annotations)
	must.Eq(t, 2, len(h.Plans[0].NodeAllocation))

	// Two nodes were allocated and are pending. (unlike system jobs, sybatch
	// jobs are not auto set to running)
	ws := memdb.NewWatchSet()
	as, err := h.State.AllocsByJob(ws, structs.DefaultNamespace, job.ID, false)
	must.NoError(t, err)

	pending := 0
	for _, a := range as {
		if "pending" == a.Job.Status {
			pending++
		}
	}

	must.Eq(t, 2, len(as))
	must.Eq(t, 2, pending)

	// Failed allocations is empty
	must.Eq(t, 0, len(h.Evals[0].FailedTGAllocs))
}

func TestSysBatch_ChainedAlloc(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	// Create some nodes
	_ = createNodes(t, h, 10)

	// Create a sysbatch job
	job := mock.SystemBatchJob()
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	var allocIDs []string
	for _, allocList := range h.Plans[0].NodeAllocation {
		for _, alloc := range allocList {
			allocIDs = append(allocIDs, alloc.ID)
		}
	}
	sort.Strings(allocIDs)

	// Create a new harness to invoke the scheduler again
	h1 := tests.NewHarnessWithState(t, h.State)
	job1 := mock.SystemBatchJob()
	job1.ID = job.ID
	job1.TaskGroups[0].Tasks[0].Env = make(map[string]string)
	job1.TaskGroups[0].Tasks[0].Env["foo"] = "bar"
	must.NoError(t, h1.State.UpsertJob(structs.MsgTypeTestSetup, h1.NextIndex(), nil, job1))

	// Insert two more nodes
	for i := 0; i < 2; i++ {
		node := mock.Node()
		must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
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
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval1}))
	// Process the evaluation
	err = h1.Process(NewSysBatchScheduler, eval1)
	must.NoError(t, err)

	must.Len(t, 1, h.Plans)
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
	must.Eq(t, allocIDs, prevAllocs)

	// Ensuring two new allocations don't have any chained allocations
	must.Len(t, 2, newAllocs)
}

func TestSysBatch_PlanWithDrainedNode(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	// Register two nodes with two different classes
	node := mock.DrainNode()
	node.NodeClass = "green"
	must.NoError(t, node.ComputeClass())
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	node2 := mock.Node()
	node2.NodeClass = "blue"
	must.NoError(t, node2.ComputeClass())
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node2))

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
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc, alloc2}))

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
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	// Ensure a single plan
	must.Len(t, 1, h.Plans)
	plan := h.Plans[0]

	// Ensure the plan evicted the alloc on the failed node
	planned := plan.NodeUpdate[node.ID]
	must.Len(t, 1, plan.NodeUpdate[node.ID])

	// Ensure the plan didn't place
	must.MapEmpty(t, plan.NodeAllocation)

	// Ensure the allocations is stopped
	must.Eq(t, structs.AllocDesiredStatusStop, planned[0].DesiredStatus)

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_QueuedAllocsMultTG(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	// Register two nodes with two different classes
	node := mock.Node()
	node.NodeClass = "green"
	must.NoError(t, node.ComputeClass())
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	node2 := mock.Node()
	node2.NodeClass = "blue"
	must.NoError(t, node2.ComputeClass())
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node2))

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
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

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
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err := h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	// Ensure a single plan
	must.Len(t, 1, h.Plans)

	qa := h.Evals[0].QueuedAllocations
	must.Zero(t, qa["pinger"])
	must.Zero(t, qa["pinger2"])

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_Preemption(t *testing.T) {
	ci.Parallel(t)

	h := tests.NewHarness(t)

	legacyCpuResources, processorResources := tests.CpuResources(3072)

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
			Processors: processorResources,
			Cpu:        legacyCpuResources,
			Memory:     structs.NodeMemoryResources{MemoryMB: 5034},
			Disk:       structs.NodeDiskResources{DiskMB: 20 * 1024},
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
		must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
		nodes = append(nodes, node)
	}

	// Enable Preemption
	err := h.State.SchedulerSetConfig(h.NextIndex(), &structs.SchedulerConfiguration{
		PreemptionConfig: structs.PreemptionConfig{
			SysBatchSchedulerEnabled: true,
		},
	})
	must.NoError(t, err)

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
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job1))

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
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job2))

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
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc1, alloc2, alloc3}))

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
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job4))
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{alloc4}))

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
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	err = h.Process(NewSysBatchScheduler, eval)
	must.NoError(t, err)

	// Ensure a single plan
	must.Eq(t, 1, len(h.Plans))
	plan := h.Plans[0]

	// Ensure the plan doesn't have annotations
	must.Nil(t, plan.Annotations)

	// Ensure the plan allocated on both nodes
	var planned []*structs.Allocation
	preemptingAllocId := ""
	must.Eq(t, 2, len(plan.NodeAllocation))

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
	must.NoError(t, err)

	// Ensure all allocations placed
	must.Eq(t, 2, len(out))

	// Verify that one node has preempted allocs
	must.NotNil(t, plan.NodePreemptions[nodes[0].ID])
	preemptedAllocs := plan.NodePreemptions[nodes[0].ID]

	// Verify that three jobs have preempted allocs
	must.Eq(t, 3, len(preemptedAllocs))

	expectedPreemptedJobIDs := []string{job1.ID, job2.ID, job3.ID}

	// We expect job1, job2 and job3 to have preempted allocations
	// job4 should not have any allocs preempted
	for _, alloc := range preemptedAllocs {
		must.SliceContains(t, expectedPreemptedJobIDs, alloc.JobID)
	}
	// Look up the preempted allocs by job ID
	ws = memdb.NewWatchSet()

	for _, jobId := range expectedPreemptedJobIDs {
		out, err = h.State.AllocsByJob(ws, structs.DefaultNamespace, jobId, false)
		must.NoError(t, err)
		for _, alloc := range out {
			must.Eq(t, structs.AllocDesiredStatusEvict, alloc.DesiredStatus)
			must.Eq(t, fmt.Sprintf("Preempted by alloc ID %v", preemptingAllocId), alloc.DesiredDescription)
		}
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSysBatch_canHandle(t *testing.T) {
	ci.Parallel(t)

	s := SystemScheduler{sysbatch: true}
	t.Run("sysbatch register", func(t *testing.T) {
		must.True(t, s.canHandle(structs.EvalTriggerJobRegister))
	})
	t.Run("sysbatch scheduled", func(t *testing.T) {
		must.False(t, s.canHandle(structs.EvalTriggerScheduled))
	})
	t.Run("sysbatch periodic", func(t *testing.T) {
		must.True(t, s.canHandle(structs.EvalTriggerPeriodicJob))
	})
}
func createNodes(t *testing.T, h *tests.Harness, n int) []*structs.Node {
	nodes := make([]*structs.Node, n)
	for i := 0; i < n; i++ {
		node := mock.Node()
		nodes[i] = node
		must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	}
	return nodes
}
