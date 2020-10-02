package scheduler

import (
	"context"
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
	"github.com/stretchr/testify/require"
)

func TestSystemSched_JobRegister(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.SystemJob()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to deregister the job
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
	err := h.Process(NewSystemScheduler, eval)
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

	// Check the available nodes
	if count, ok := out[0].Metrics.NodesAvailable["dc1"]; !ok || count != 10 {
		t.Fatalf("bad: %#v", out[0].Metrics)
	}

	// Ensure no allocations are queued
	queued := h.Evals[0].QueuedAllocations["web"]
	if queued != 0 {
		t.Fatalf("expected queued allocations: %v, actual: %v", 0, queued)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSystemSched_JobRegister_StickyAllocs(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.SystemJob()
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
	if err := h.Process(NewSystemScheduler, eval); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure the plan allocated
	plan := h.Plans[0]
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 10 {
		t.Fatalf("bad: %#v", plan)
	}

	// Get an allocation and mark it as failed
	alloc := planned[4].Copy()
	alloc.ClientStatus = structs.AllocClientStatusFailed
	require.NoError(t, h.State.UpdateAllocsFromClient(context.Background(), h.NextIndex(), []*structs.Allocation{alloc}))

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
	if err := h1.Process(NewSystemScheduler, eval); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have created only one new allocation
	plan = h1.Plans[0]
	var newPlanned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		newPlanned = append(newPlanned, allocList...)
	}
	if len(newPlanned) != 1 {
		t.Fatalf("bad plan: %#v", plan)
	}
	// Ensure that the new allocation was placed on the same node as the older
	// one
	if newPlanned[0].NodeID != alloc.NodeID || newPlanned[0].PreviousAllocation != alloc.ID {
		t.Fatalf("expected: %#v, actual: %#v", alloc, newPlanned[0])
	}
}

func TestSystemSched_JobRegister_EphemeralDiskConstraint(t *testing.T) {
	h := NewHarness(t)

	// Create a nodes
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a job
	job := mock.SystemJob()
	job.TaskGroups[0].EphemeralDisk.SizeMB = 60 * 1024
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create another job with a lot of disk resource ask so that it doesn't fit
	// the node
	job1 := mock.SystemJob()
	job1.TaskGroups[0].EphemeralDisk.SizeMB = 60 * 1024
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job1))

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
	if err := h.Process(NewSystemScheduler, eval); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	if len(out) != 1 {
		t.Fatalf("bad: %#v", out)
	}

	// Create a new harness to test the scheduling result for the second job
	h1 := NewHarnessWithState(t, h.State)
	// Create a mock evaluation to register the job
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
	if err := h1.Process(NewSystemScheduler, eval1); err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err = h1.State.AllocsByJob(ws, job.Namespace, job1.ID, false)
	require.NoError(t, err)
	if len(out) != 0 {
		t.Fatalf("bad: %#v", out)
	}
}

func TestSystemSched_ExhaustResources(t *testing.T) {
	h := NewHarness(t)

	// Create a nodes
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Enable Preemption
	h.State.SchedulerSetConfig(h.NextIndex(), &structs.SchedulerConfiguration{
		PreemptionConfig: structs.PreemptionConfig{
			SystemSchedulerEnabled: true,
		},
	})

	// Create a service job which consumes most of the system resources
	svcJob := mock.Job()
	svcJob.TaskGroups[0].Count = 1
	svcJob.TaskGroups[0].Tasks[0].Resources.CPU = 3600
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), svcJob))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    svcJob.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       svcJob.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))
	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a system job
	job := mock.SystemJob()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval1 := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval1}))
	// Process the evaluation
	if err := h.Process(NewSystemScheduler, eval1); err != nil {
		t.Fatalf("err: %v", err)
	}

	// System scheduler will preempt the service job and would have placed eval1
	require := require.New(t)

	newPlan := h.Plans[1]
	require.Len(newPlan.NodeAllocation, 1)
	require.Len(newPlan.NodePreemptions, 1)

	for _, allocList := range newPlan.NodeAllocation {
		require.Len(allocList, 1)
		require.Equal(job.ID, allocList[0].JobID)
	}

	for _, allocList := range newPlan.NodePreemptions {
		require.Len(allocList, 1)
		require.Equal(svcJob.ID, allocList[0].JobID)
	}
	// Ensure that we have no queued allocations on the second eval
	queued := h.Evals[1].QueuedAllocations["web"]
	if queued != 0 {
		t.Fatalf("expected: %v, actual: %v", 1, queued)
	}
}

func TestSystemSched_JobRegister_Annotate(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		if i < 9 {
			node.NodeClass = "foo"
		} else {
			node.NodeClass = "bar"
		}
		node.ComputeClass()
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job constraining on node class
	job := mock.SystemJob()
	fooConstraint := &structs.Constraint{
		LTarget: "${node.class}",
		RTarget: "foo",
		Operand: "==",
	}
	job.Constraints = append(job.Constraints, fooConstraint)
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to deregister the job
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
	err := h.Process(NewSystemScheduler, eval)
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
	if len(planned) != 9 {
		t.Fatalf("bad: %#v %d", planned, len(planned))
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	if len(out) != 9 {
		t.Fatalf("bad: %#v", out)
	}

	// Check the available nodes
	if count, ok := out[0].Metrics.NodesAvailable["dc1"]; !ok || count != 10 {
		t.Fatalf("bad: %#v", out[0].Metrics)
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

	expected := &structs.DesiredUpdates{Place: 9}
	if !reflect.DeepEqual(desiredChanges, expected) {
		t.Fatalf("Unexpected desired updates; got %#v; want %#v", desiredChanges, expected)
	}
}

func TestSystemSched_JobRegister_AddNode(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.SystemJob()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-job.web[0]"
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Add a new node.
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a mock evaluation to deal with the node update
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
	err := h.Process(NewSystemScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan had no node updates
	var update []*structs.Allocation
	for _, updateList := range plan.NodeUpdate {
		update = append(update, updateList...)
	}
	if len(update) != 0 {
		t.Log(len(update))
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure the plan allocated on the new node
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 1 {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure it allocated on the right node
	if _, ok := plan.NodeAllocation[node.ID]; !ok {
		t.Fatalf("allocated on wrong node: %#v", plan)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure all allocations placed
	out, _ = structs.FilterTerminalAllocs(out)
	if len(out) != 11 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSystemSched_JobRegister_AllocFail(t *testing.T) {
	h := NewHarness(t)

	// Create NO nodes
	// Create a job
	job := mock.SystemJob()
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
	err := h.Process(NewSystemScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure no plan as this should be a no-op.
	if len(h.Plans) != 0 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSystemSched_JobModify(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.SystemJob()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-job.web[0]"
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
		alloc.Name = "my-job.web[0]"
		alloc.DesiredStatus = structs.AllocDesiredStatusStop
		terminal = append(terminal, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), terminal))

	// Update the job
	job2 := mock.SystemJob()
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
	err := h.Process(NewSystemScheduler, eval)
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

func TestSystemSched_JobModify_Rolling(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.SystemJob()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-job.web[0]"
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Update the job
	job2 := mock.SystemJob()
	job2.ID = job.ID
	job2.Update = structs.UpdateStrategy{
		Stagger:     30 * time.Second,
		MaxParallel: 5,
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
	err := h.Process(NewSystemScheduler, eval)
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
	if len(update) != job2.Update.MaxParallel {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != job2.Update.MaxParallel {
		t.Fatalf("bad: %#v", plan)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)

	// Ensure a follow up eval was created
	eval = h.Evals[0]
	if eval.NextEval == "" {
		t.Fatalf("missing next eval")
	}

	// Check for create
	if len(h.CreateEvals) == 0 {
		t.Fatalf("missing created eval")
	}
	create := h.CreateEvals[0]
	if eval.NextEval != create.ID {
		t.Fatalf("ID mismatch")
	}
	if create.PreviousEval != eval.ID {
		t.Fatalf("missing previous eval")
	}

	if create.TriggeredBy != structs.EvalTriggerRollingUpdate {
		t.Fatalf("bad: %#v", create)
	}
}

func TestSystemSched_JobModify_InPlace(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.SystemJob()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-job.web[0]"
		allocs = append(allocs, alloc)
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Update the job
	job2 := mock.SystemJob()
	job2.ID = job.ID
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
	err := h.Process(NewSystemScheduler, eval)
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
}

func TestSystemSched_JobDeregister_Purged(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.SystemJob()

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-job.web[0]"
		allocs = append(allocs, alloc)
	}
	for _, alloc := range allocs {
		require.NoError(t, h.State.UpsertJobSummary(h.NextIndex(), mock.JobSummary(alloc.JobID)))
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
	err := h.Process(NewSystemScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan evicted the job from all nodes.
	for _, node := range nodes {
		if len(plan.NodeUpdate[node.ID]) != 1 {
			t.Fatalf("bad: %#v", plan)
		}
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure no remaining allocations
	out, _ = structs.FilterTerminalAllocs(out)
	if len(out) != 0 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSystemSched_JobDeregister_Stopped(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.SystemJob()
	job.Stop = true
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-job.web[0]"
		allocs = append(allocs, alloc)
	}
	for _, alloc := range allocs {
		require.NoError(t, h.State.UpsertJobSummary(h.NextIndex(), mock.JobSummary(alloc.JobID)))
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
	err := h.Process(NewSystemScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan evicted the job from all nodes.
	for _, node := range nodes {
		if len(plan.NodeUpdate[node.ID]) != 1 {
			t.Fatalf("bad: %#v", plan)
		}
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.Namespace, job.ID, false)
	require.NoError(t, err)

	// Ensure no remaining allocations
	out, _ = structs.FilterTerminalAllocs(out)
	if len(out) != 0 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSystemSched_NodeDown(t *testing.T) {
	h := NewHarness(t)

	// Register a down node
	node := mock.Node()
	node.Status = structs.NodeStatusDown
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job allocated on that node.
	job := mock.SystemJob()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.DesiredTransition.Migrate = helper.BoolToPtr(true)
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

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
	err := h.Process(NewSystemScheduler, eval)
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

	// Ensure the plan updated the allocation.
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeUpdate {
		planned = append(planned, allocList...)
	}
	if len(planned) != 1 {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure the allocations is stopped
	if p := planned[0]; p.DesiredStatus != structs.AllocDesiredStatusStop &&
		p.ClientStatus != structs.AllocClientStatusLost {
		t.Fatalf("bad: %#v", planned[0])
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSystemSched_NodeDrain_Down(t *testing.T) {
	h := NewHarness(t)

	// Register a draining node
	node := mock.Node()
	node.Drain = true
	node.Status = structs.NodeStatusDown
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job allocated on that node.
	job := mock.SystemJob()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

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
	if len(plan.NodeUpdate[node.ID]) != 1 {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure that the allocation is marked as lost
	var lostAllocs []string
	for _, alloc := range plan.NodeUpdate[node.ID] {
		lostAllocs = append(lostAllocs, alloc.ID)
	}
	expected := []string{alloc.ID}

	if !reflect.DeepEqual(lostAllocs, expected) {
		t.Fatalf("expected: %v, actual: %v", expected, lostAllocs)
	}
	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSystemSched_NodeDrain(t *testing.T) {
	h := NewHarness(t)

	// Register a draining node
	node := mock.Node()
	node.Drain = true
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job allocated on that node.
	job := mock.SystemJob()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.DesiredTransition.Migrate = helper.BoolToPtr(true)
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

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
	err := h.Process(NewSystemScheduler, eval)
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

	// Ensure the plan updated the allocation.
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeUpdate {
		planned = append(planned, allocList...)
	}
	if len(planned) != 1 {
		t.Log(len(planned))
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure the allocations is stopped
	if planned[0].DesiredStatus != structs.AllocDesiredStatusStop {
		t.Fatalf("bad: %#v", planned[0])
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSystemSched_NodeUpdate(t *testing.T) {
	h := NewHarness(t)

	// Register a node
	node := mock.Node()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job allocated on that node.
	job := mock.SystemJob()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to deal
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
	err := h.Process(NewSystemScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure that queued allocations is zero
	if val, ok := h.Evals[0].QueuedAllocations["web"]; !ok || val != 0 {
		t.Fatalf("bad queued allocations: %#v", h.Evals[0].QueuedAllocations)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSystemSched_RetryLimit(t *testing.T) {
	h := NewHarness(t)
	h.Planner = &RejectPlan{h}

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.SystemJob()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to deregister the job
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
	err := h.Process(NewSystemScheduler, eval)
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

// This test ensures that the scheduler doesn't increment the queued allocation
// count for a task group when allocations can't be created on currently
// available nodes because of constrain mismatches.
func TestSystemSched_Queued_With_Constraints(t *testing.T) {
	h := NewHarness(t)

	// Register a node
	node := mock.Node()
	node.Attributes["kernel.name"] = "darwin"
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a system job which can't be placed on the node
	job := mock.SystemJob()
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to deal
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
	err := h.Process(NewSystemScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure that queued allocations is zero
	if val, ok := h.Evals[0].QueuedAllocations["web"]; !ok || val != 0 {
		t.Fatalf("bad queued allocations: %#v", h.Evals[0].QueuedAllocations)
	}

}

// This test ensures that the scheduler correctly ignores ineligible
// nodes when scheduling due to a new node being added. The job has two
// task groups contrained to a particular node class. The desired behavior
// should be that the TaskGroup constrained to the newly added node class is
// added and that the TaskGroup constrained to the ineligible node is ignored.
func TestSystemSched_JobConstraint_AddNode(t *testing.T) {
	h := NewHarness(t)

	// Create two nodes
	var node *structs.Node
	node = mock.Node()
	node.NodeClass = "Class-A"
	node.ComputeClass()
	require.Nil(t, h.State.UpsertNode(h.NextIndex(), node))

	var nodeB *structs.Node
	nodeB = mock.Node()
	nodeB.NodeClass = "Class-B"
	nodeB.ComputeClass()
	require.Nil(t, h.State.UpsertNode(h.NextIndex(), nodeB))

	// Make a job with two task groups, each constraint to a node class
	job := mock.SystemJob()
	tgA := job.TaskGroups[0]
	tgA.Name = "groupA"
	tgA.Constraints = []*structs.Constraint{
		{
			LTarget: "${node.class}",
			RTarget: node.NodeClass,
			Operand: "=",
		},
	}
	tgB := job.TaskGroups[0].Copy()
	tgB.Name = "groupB"
	tgB.Constraints = []*structs.Constraint{
		{
			LTarget: "${node.class}",
			RTarget: nodeB.NodeClass,
			Operand: "=",
		},
	}

	// Upsert Job
	job.TaskGroups = []*structs.TaskGroup{tgA, tgB}
	require.Nil(t, h.State.UpsertJob(h.NextIndex(), job))

	// Evaluate the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.Nil(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

	require.Nil(t, h.Process(NewSystemScheduler, eval))
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

	require.Nil(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval2}))
	require.Nil(t, h.Process(NewSystemScheduler, eval2))
	require.Equal(t, "complete", h.Evals[1].Status)

	// Ensure no new plans
	require.Equal(t, 1, len(h.Plans))

	// Ensure all NodeAllocations are from first Eval
	for _, allocs := range h.Plans[0].NodeAllocation {
		require.Len(t, allocs, 1)
		require.Equal(t, eval.ID, allocs[0].EvalID)
	}

	// Add a new node Class-B
	var nodeBTwo *structs.Node
	nodeBTwo = mock.Node()
	nodeBTwo.ComputeClass()
	nodeBTwo.NodeClass = "Class-B"
	require.Nil(t, h.State.UpsertNode(h.NextIndex(), nodeBTwo))

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

	// Ensure New eval is complete
	require.Nil(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval3}))
	require.Nil(t, h.Process(NewSystemScheduler, eval3))
	require.Equal(t, "complete", h.Evals[2].Status)

	// Ensure no failed TG allocs
	require.Equal(t, 0, len(h.Evals[2].FailedTGAllocs))

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
func TestSystemSched_ExistingAllocNoNodes(t *testing.T) {
	h := NewHarness(t)

	var node *structs.Node
	// Create a node
	node = mock.Node()
	node.ComputeClass()
	require.Nil(t, h.State.UpsertNode(h.NextIndex(), node))

	// Make a job
	job := mock.SystemJob()
	require.Nil(t, h.State.UpsertJob(h.NextIndex(), job))

	// Evaluate the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.Nil(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))
	require.Nil(t, h.Process(NewSystemScheduler, eval))
	require.Equal(t, "complete", h.Evals[0].Status)

	// QueuedAllocations is drained
	val, ok := h.Evals[0].QueuedAllocations["web"]
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
	require.Nil(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval2}))
	require.Nil(t, h.Process(NewSystemScheduler, eval2))
	require.Equal(t, "complete", h.Evals[1].Status)

	// Create a new job version, deploy
	job2 := job.Copy()
	job2.Meta["version"] = "2"
	require.Nil(t, h.State.UpsertJob(h.NextIndex(), job2))

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
	require.Nil(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval3}))
	require.Nil(t, h.Process(NewSystemScheduler, eval3))
	require.Equal(t, "complete", h.Evals[2].Status)

	// Ensure there are no FailedTGAllocs
	require.Equal(t, 0, len(h.Evals[2].FailedTGAllocs))
	require.Equal(t, 0, h.Evals[2].QueuedAllocations[job2.Name])
}

// No errors reported when constraints prevent placement
func TestSystemSched_ConstraintErrors(t *testing.T) {
	h := NewHarness(t)

	var node *structs.Node
	// Register some nodes
	// the tag "aaaaaa" is hashed so that the nodes are processed
	// in an order other than good, good, bad
	for _, tag := range []string{"aaaaaa", "foo", "foo", "foo"} {
		node = mock.Node()
		node.Meta["tag"] = tag
		node.ComputeClass()
		require.Nil(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Mark the last node as ineligible
	node.SchedulingEligibility = structs.NodeSchedulingIneligible

	// Make a job with a constraint that matches a subset of the nodes
	job := mock.SystemJob()
	job.Constraints = append(job.Constraints,
		&structs.Constraint{
			LTarget: "${meta.tag}",
			RTarget: "foo",
			Operand: "=",
		})

	require.Nil(t, h.State.UpsertJob(h.NextIndex(), job))

	// Evaluate the job
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	require.Nil(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))
	require.Nil(t, h.Process(NewSystemScheduler, eval))
	require.Equal(t, "complete", h.Evals[0].Status)

	// QueuedAllocations is drained
	val, ok := h.Evals[0].QueuedAllocations["web"]
	require.True(t, ok)
	require.Equal(t, 0, val)

	// The plan has two NodeAllocations
	require.Equal(t, 1, len(h.Plans))
	require.Nil(t, h.Plans[0].Annotations)
	require.Equal(t, 2, len(h.Plans[0].NodeAllocation))

	// Two nodes were allocated and are running
	ws := memdb.NewWatchSet()
	as, err := h.State.AllocsByJob(ws, structs.DefaultNamespace, job.ID, false)
	require.Nil(t, err)

	running := 0
	for _, a := range as {
		if "running" == a.Job.Status {
			running++
		}
	}

	require.Equal(t, 2, len(as))
	require.Equal(t, 2, running)

	// Failed allocations is empty
	require.Equal(t, 0, len(h.Evals[0].FailedTGAllocs))
}

func TestSystemSched_ChainedAlloc(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.SystemJob()
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
	if err := h.Process(NewSystemScheduler, eval); err != nil {
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
	job1 := mock.SystemJob()
	job1.ID = job.ID
	job1.TaskGroups[0].Tasks[0].Env = make(map[string]string)
	job1.TaskGroups[0].Tasks[0].Env["foo"] = "bar"
	require.NoError(t, h1.State.UpsertJob(h1.NextIndex(), job1))

	// Insert two more nodes
	for i := 0; i < 2; i++ {
		node := mock.Node()
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
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
	require.NoError(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval1}))
	// Process the evaluation
	if err := h1.Process(NewSystemScheduler, eval1); err != nil {
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

func TestSystemSched_PlanWithDrainedNode(t *testing.T) {
	h := NewHarness(t)

	// Register two nodes with two different classes
	node := mock.Node()
	node.NodeClass = "green"
	node.Drain = true
	node.ComputeClass()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	node2 := mock.Node()
	node2.NodeClass = "blue"
	node2.ComputeClass()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node2))

	// Create a Job with two task groups, each constrained on node class
	job := mock.SystemJob()
	tg1 := job.TaskGroups[0]
	tg1.Constraints = append(tg1.Constraints,
		&structs.Constraint{
			LTarget: "${node.class}",
			RTarget: "green",
			Operand: "==",
		})

	tg2 := tg1.Copy()
	tg2.Name = "web2"
	tg2.Constraints[0].RTarget = "blue"
	job.TaskGroups = append(job.TaskGroups, tg2)
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create an allocation on each node
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.DesiredTransition.Migrate = helper.BoolToPtr(true)
	alloc.TaskGroup = "web"

	alloc2 := mock.Alloc()
	alloc2.Job = job
	alloc2.JobID = job.ID
	alloc2.NodeID = node2.ID
	alloc2.Name = "my-job.web2[0]"
	alloc2.TaskGroup = "web2"
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc, alloc2}))

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
	err := h.Process(NewSystemScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}
	plan := h.Plans[0]

	// Ensure the plan evicted the alloc on the failed node
	planned := plan.NodeUpdate[node.ID]
	if len(planned) != 1 {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure the plan didn't place
	if len(plan.NodeAllocation) != 0 {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure the allocations is stopped
	if planned[0].DesiredStatus != structs.AllocDesiredStatusStop {
		t.Fatalf("bad: %#v", planned[0])
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSystemSched_QueuedAllocsMultTG(t *testing.T) {
	h := NewHarness(t)

	// Register two nodes with two different classes
	node := mock.Node()
	node.NodeClass = "green"
	node.ComputeClass()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))

	node2 := mock.Node()
	node2.NodeClass = "blue"
	node2.ComputeClass()
	require.NoError(t, h.State.UpsertNode(h.NextIndex(), node2))

	// Create a Job with two task groups, each constrained on node class
	job := mock.SystemJob()
	tg1 := job.TaskGroups[0]
	tg1.Constraints = append(tg1.Constraints,
		&structs.Constraint{
			LTarget: "${node.class}",
			RTarget: "green",
			Operand: "==",
		})

	tg2 := tg1.Copy()
	tg2.Name = "web2"
	tg2.Constraints[0].RTarget = "blue"
	job.TaskGroups = append(job.TaskGroups, tg2)
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job))

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
	err := h.Process(NewSystemScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure a single plan
	if len(h.Plans) != 1 {
		t.Fatalf("bad: %#v", h.Plans)
	}

	qa := h.Evals[0].QueuedAllocations
	if qa["web"] != 0 || qa["web2"] != 0 {
		t.Fatalf("bad queued allocations %#v", qa)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestSystemSched_Preemption(t *testing.T) {
	h := NewHarness(t)

	// Create nodes
	var nodes []*structs.Node
	for i := 0; i < 2; i++ {
		node := mock.Node()
		// TODO(preetha): remove in 0.11
		node.Resources = &structs.Resources{
			CPU:      3072,
			MemoryMB: 5034,
			DiskMB:   20 * 1024,
			Networks: []*structs.NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					MBits:  1000,
				},
			},
		}
		node.NodeResources = &structs.NodeResources{
			Cpu: structs.NodeCpuResources{
				CpuShares: 3072,
			},
			Memory: structs.NodeMemoryResources{
				MemoryMB: 5034,
			},
			Disk: structs.NodeDiskResources{
				DiskMB: 20 * 1024,
			},
			Networks: []*structs.NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					MBits:  1000,
				},
			},
			NodeNetworks: []*structs.NodeNetworkResource{
				{
					Mode:   "host",
					Device: "eth0",
					Addresses: []structs.NodeNetworkAddress{
						{
							Family:  structs.NodeNetworkAF_IPv4,
							Alias:   "default",
							Address: "192.168.0.100",
						},
					},
				},
			},
		}
		require.NoError(t, h.State.UpsertNode(h.NextIndex(), node))
		nodes = append(nodes, node)
	}

	// Enable Preemption
	h.State.SchedulerSetConfig(h.NextIndex(), &structs.SchedulerConfiguration{
		PreemptionConfig: structs.PreemptionConfig{
			SystemSchedulerEnabled: true,
		},
	})

	// Create some low priority batch jobs and allocations for them
	// One job uses a reserved port
	job1 := mock.BatchJob()
	job1.Type = structs.JobTypeBatch
	job1.Priority = 20
	job1.TaskGroups[0].Tasks[0].Resources = &structs.Resources{
		CPU:      512,
		MemoryMB: 1024,
		Networks: []*structs.NetworkResource{
			{
				MBits: 200,
				ReservedPorts: []structs.Port{
					{
						Label: "web",
						Value: 80,
					},
				},
			},
		},
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
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 512,
				},
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 1024,
				},
				Networks: []*structs.NetworkResource{
					{
						Device:        "eth0",
						IP:            "192.168.0.100",
						ReservedPorts: []structs.Port{{Label: "web", Value: 80}},
						MBits:         200,
					},
				},
			},
		},
		Shared: structs.AllocatedSharedResources{
			DiskMB: 5 * 1024,
		},
	}

	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job1))

	job2 := mock.BatchJob()
	job2.Type = structs.JobTypeBatch
	job2.Priority = 20
	job2.TaskGroups[0].Tasks[0].Resources = &structs.Resources{
		CPU:      512,
		MemoryMB: 1024,
		Networks: []*structs.NetworkResource{
			{
				MBits: 200,
			},
		},
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
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 512,
				},
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 1024,
				},
				Networks: []*structs.NetworkResource{
					{
						Device: "eth0",
						IP:     "192.168.0.100",
						MBits:  200,
					},
				},
			},
		},
		Shared: structs.AllocatedSharedResources{
			DiskMB: 5 * 1024,
		},
	}
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job2))

	job3 := mock.Job()
	job3.Type = structs.JobTypeBatch
	job3.Priority = 40
	job3.TaskGroups[0].Tasks[0].Resources = &structs.Resources{
		CPU:      1024,
		MemoryMB: 2048,
		Networks: []*structs.NetworkResource{
			{
				Device: "eth0",
				MBits:  400,
			},
		},
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
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 1024,
				},
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 25,
				},
				Networks: []*structs.NetworkResource{
					{
						Device:        "eth0",
						IP:            "192.168.0.100",
						ReservedPorts: []structs.Port{{Label: "web", Value: 80}},
						MBits:         400,
					},
				},
			},
		},
		Shared: structs.AllocatedSharedResources{
			DiskMB: 5 * 1024,
		},
	}
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc1, alloc2, alloc3}))

	// Create a high priority job and allocs for it
	// These allocs should not be preempted

	job4 := mock.BatchJob()
	job4.Type = structs.JobTypeBatch
	job4.Priority = 100
	job4.TaskGroups[0].Tasks[0].Resources = &structs.Resources{
		CPU:      1024,
		MemoryMB: 2048,
		Networks: []*structs.NetworkResource{
			{
				MBits: 100,
			},
		},
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
	require.NoError(t, h.State.UpsertJob(h.NextIndex(), job4))
	require.NoError(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc4}))

	// Create a system job such that it would need to preempt both allocs to succeed
	job := mock.SystemJob()
	job.TaskGroups[0].Tasks[0].Resources = &structs.Resources{
		CPU:      1948,
		MemoryMB: 256,
		Networks: []*structs.NetworkResource{
			{
				MBits:        800,
				DynamicPorts: []structs.Port{{Label: "http"}},
			},
		},
	}
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
	err := h.Process(NewSystemScheduler, eval)
	require := require.New(t)
	require.Nil(err)

	// Ensure a single plan
	require.Equal(1, len(h.Plans))
	plan := h.Plans[0]

	// Ensure the plan doesn't have annotations.
	require.Nil(plan.Annotations)

	// Ensure the plan allocated on both nodes
	var planned []*structs.Allocation
	preemptingAllocId := ""
	require.Equal(2, len(plan.NodeAllocation))

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
	require.NoError(err)

	// Ensure all allocations placed
	require.Equal(2, len(out))

	// Verify that one node has preempted allocs
	require.NotNil(plan.NodePreemptions[nodes[0].ID])
	preemptedAllocs := plan.NodePreemptions[nodes[0].ID]

	// Verify that three jobs have preempted allocs
	require.Equal(3, len(preemptedAllocs))

	expectedPreemptedJobIDs := []string{job1.ID, job2.ID, job3.ID}

	// We expect job1, job2 and job3 to have preempted allocations
	// job4 should not have any allocs preempted
	for _, alloc := range preemptedAllocs {
		require.Contains(expectedPreemptedJobIDs, alloc.JobID)
	}
	// Look up the preempted allocs by job ID
	ws = memdb.NewWatchSet()

	for _, jobId := range expectedPreemptedJobIDs {
		out, err = h.State.AllocsByJob(ws, structs.DefaultNamespace, jobId, false)
		require.NoError(err)
		for _, alloc := range out {
			require.Equal(structs.AllocDesiredStatusEvict, alloc.DesiredStatus)
			require.Equal(fmt.Sprintf("Preempted by alloc ID %v", preemptingAllocId), alloc.DesiredDescription)
		}
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)

}
