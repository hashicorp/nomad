package scheduler

import (
	"reflect"
	"sort"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestSystemSched_JobRegister(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.SystemJob()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to deregister the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

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
	out, err := h.State.AllocsByJob(ws, job.ID, false)
	noErr(t, err)

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

func TestSystemeSched_JobRegister_StickyAllocs(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.SystemJob()
	job.TaskGroups[0].EphemeralDisk.Sticky = true
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

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
	noErr(t, h.State.UpdateAllocsFromClient(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to handle the update
	eval = &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
	}
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
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a job
	job := mock.SystemJob()
	job.TaskGroups[0].EphemeralDisk.SizeMB = 60 * 1024
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create another job with a lot of disk resource ask so that it doesn't fit
	// the node
	job1 := mock.SystemJob()
	job1.TaskGroups[0].EphemeralDisk.SizeMB = 60 * 1024
	noErr(t, h.State.UpsertJob(h.NextIndex(), job1))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

	// Process the evaluation
	if err := h.Process(NewSystemScheduler, eval); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Lookup the allocations by JobID
	ws := memdb.NewWatchSet()
	out, err := h.State.AllocsByJob(ws, job.ID, false)
	noErr(t, err)

	// Ensure all allocations placed
	if len(out) != 1 {
		t.Fatalf("bad: %#v", out)
	}

	// Create a new harness to test the scheduling result for the second job
	h1 := NewHarnessWithState(t, h.State)
	// Create a mock evaluation to register the job
	eval1 := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job1.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job1.ID,
	}

	// Process the evaluation
	if err := h1.Process(NewSystemScheduler, eval1); err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err = h1.State.AllocsByJob(ws, job1.ID, false)
	noErr(t, err)
	if len(out) != 0 {
		t.Fatalf("bad: %#v", out)
	}
}

func TestSystemSched_ExhaustResources(t *testing.T) {
	h := NewHarness(t)

	// Create a nodes
	node := mock.Node()
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a service job which consumes most of the system resources
	svcJob := mock.Job()
	svcJob.TaskGroups[0].Count = 1
	svcJob.TaskGroups[0].Tasks[0].Resources.CPU = 3600
	noErr(t, h.State.UpsertJob(h.NextIndex(), svcJob))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    svcJob.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       svcJob.ID,
	}

	// Process the evaluation
	err := h.Process(NewServiceScheduler, eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a system job
	job := mock.SystemJob()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval1 := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

	// Process the evaluation
	if err := h.Process(NewSystemScheduler, eval1); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure that we have one allocation queued from the system job eval
	queued := h.Evals[1].QueuedAllocations["web"]
	if queued != 1 {
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
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job constraining on node class
	job := mock.SystemJob()
	fooConstraint := &structs.Constraint{
		LTarget: "${node.class}",
		RTarget: "foo",
		Operand: "==",
	}
	job.Constraints = append(job.Constraints, fooConstraint)
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to deregister the job
	eval := &structs.Evaluation{
		ID:           structs.GenerateUUID(),
		Priority:     job.Priority,
		TriggeredBy:  structs.EvalTriggerJobRegister,
		JobID:        job.ID,
		AnnotatePlan: true,
	}

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
	out, err := h.State.AllocsByJob(ws, job.ID, false)
	noErr(t, err)

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
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.SystemJob()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-job.web[0]"
		allocs = append(allocs, alloc)
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Add a new node.
	node := mock.Node()
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a mock evaluation to deal with the node update
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
	}

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
	out, err := h.State.AllocsByJob(ws, job.ID, false)
	noErr(t, err)

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
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

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
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.SystemJob()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-job.web[0]"
		allocs = append(allocs, alloc)
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

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
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), terminal))

	// Update the job
	job2 := mock.SystemJob()
	job2.ID = job.ID

	// Update the task, such that it cannot be done in-place
	job2.TaskGroups[0].Tasks[0].Config["command"] = "/bin/other"
	noErr(t, h.State.UpsertJob(h.NextIndex(), job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

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
	out, err := h.State.AllocsByJob(ws, job.ID, false)
	noErr(t, err)

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
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.SystemJob()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-job.web[0]"
		allocs = append(allocs, alloc)
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Update the job
	job2 := mock.SystemJob()
	job2.ID = job.ID
	job2.Update = structs.UpdateStrategy{
		Stagger:     30 * time.Second,
		MaxParallel: 5,
	}

	// Update the task, such that it cannot be done in-place
	job2.TaskGroups[0].Tasks[0].Config["command"] = "/bin/other"
	noErr(t, h.State.UpsertJob(h.NextIndex(), job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

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
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.SystemJob()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for _, node := range nodes {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = "my-job.web[0]"
		allocs = append(allocs, alloc)
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Update the job
	job2 := mock.SystemJob()
	job2.ID = job.ID
	noErr(t, h.State.UpsertJob(h.NextIndex(), job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

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
	out, err := h.State.AllocsByJob(ws, job.ID, false)
	noErr(t, err)

	// Ensure all allocations placed
	if len(out) != 10 {
		t.Fatalf("bad: %#v", out)
	}
	h.AssertEvalStatus(t, structs.EvalStatusComplete)

	// Verify the network did not change
	rp := structs.Port{Label: "main", Value: 5000}
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
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
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
		noErr(t, h.State.UpsertJobSummary(h.NextIndex(), mock.JobSummary(alloc.JobID)))
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create a mock evaluation to deregister the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobDeregister,
		JobID:       job.ID,
	}

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
	out, err := h.State.AllocsByJob(ws, job.ID, false)
	noErr(t, err)

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
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.SystemJob()
	job.Stop = true
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

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
		noErr(t, h.State.UpsertJobSummary(h.NextIndex(), mock.JobSummary(alloc.JobID)))
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create a mock evaluation to deregister the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobDeregister,
		JobID:       job.ID,
	}

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
	out, err := h.State.AllocsByJob(ws, job.ID, false)
	noErr(t, err)

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
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job allocated on that node.
	job := mock.SystemJob()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		NodeID:      node.ID,
	}

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
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job allocated on that node.
	job := mock.SystemJob()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to deal with the node update
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		NodeID:      node.ID,
	}

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
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job allocated on that node.
	job := mock.SystemJob()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		NodeID:      node.ID,
	}

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
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job allocated on that node.
	job := mock.SystemJob()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to deal
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		NodeID:      node.ID,
	}

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
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.SystemJob()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to deregister the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

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
	out, err := h.State.AllocsByJob(ws, job.ID, false)
	noErr(t, err)

	// Ensure no allocations placed
	if len(out) != 0 {
		t.Fatalf("bad: %#v", out)
	}

	// Should hit the retry limit
	h.AssertEvalStatus(t, structs.EvalStatusFailed)
}

// This test ensures that the scheduler doesn't increment the queued allocation
// count for a task group when allocations can't be created on currently
// availabe nodes because of constrain mismatches.
func TestSystemSched_Queued_With_Constraints(t *testing.T) {
	h := NewHarness(t)

	// Register a node
	node := mock.Node()
	node.Attributes["kernel.name"] = "darwin"
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a system job which can't be placed on the node
	job := mock.SystemJob()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to deal
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		NodeID:      node.ID,
	}

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

func TestSystemSched_ChainedAlloc(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.SystemJob()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}
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
	noErr(t, h1.State.UpsertJob(h1.NextIndex(), job1))

	// Insert two more nodes
	for i := 0; i < 2; i++ {
		node := mock.Node()
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a mock evaluation to update the job
	eval1 := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job1.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job1.ID,
	}
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

	// Ensure that the new allocations has their corresponging original
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
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	node2 := mock.Node()
	node2.NodeClass = "blue"
	node2.ComputeClass()
	noErr(t, h.State.UpsertNode(h.NextIndex(), node2))

	// Create a Job with two task groups, each constrianed on node class
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
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create an allocation on each node
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.TaskGroup = "web"

	alloc2 := mock.Alloc()
	alloc2.Job = job
	alloc2.JobID = job.ID
	alloc2.NodeID = node2.ID
	alloc2.Name = "my-job.web2[0]"
	alloc2.TaskGroup = "web2"
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc, alloc2}))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		NodeID:      node.ID,
	}

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
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	node2 := mock.Node()
	node2.NodeClass = "blue"
	node2.ComputeClass()
	noErr(t, h.State.UpsertNode(h.NextIndex(), node2))

	// Create a Job with two task groups, each constrianed on node class
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
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       job.ID,
		NodeID:      node.ID,
	}

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
