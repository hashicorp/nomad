package scheduler

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestServiceSched_JobRegister(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.Job()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
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
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

	// Ensure all allocations placed
	if len(out) != 10 {
		t.Fatalf("bad: %#v", out)
	}

	// Ensure different ports were used.
	used := make(map[int]struct{})
	for _, alloc := range out {
		for _, resource := range alloc.TaskResources {
			for _, port := range resource.Networks[0].DynamicPorts {
				if _, ok := used[port.Value]; ok {
					t.Fatalf("Port collision %v", port.Value)
				}
				used[port.Value] = struct{}{}
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
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.Job()
	job.TaskGroups[0].LocalDisk.Sticky = true
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

	// Process the evaluation
	if err := h.Process(NewServiceScheduler, eval); err != nil {
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
	if err := h1.Process(NewServiceScheduler, eval); err != nil {
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

func TestServiceSched_JobRegister_DiskConstraints(t *testing.T) {
	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a job with count 2 and disk as 60GB so that only one allocation
	// can fit
	job := mock.Job()
	job.TaskGroups[0].Count = 2
	job.TaskGroups[0].LocalDisk.DiskMB = 88 * 1024
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
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

	// Ensure the plan doesn't have annotations.
	if plan.Annotations != nil {
		t.Fatalf("expected no annotations")
	}

	// Ensure the eval has a blocked eval
	if len(h.CreateEvals) != 1 {
		t.Fatalf("bad: %#v", h.CreateEvals)
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
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

	// Ensure only one allocation was placed
	if len(out) != 1 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_JobRegister_Annotate(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.Job()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:           structs.GenerateUUID(),
		Priority:     job.Priority,
		TriggeredBy:  structs.EvalTriggerJobRegister,
		JobID:        job.ID,
		AnnotatePlan: true,
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

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 10 {
		t.Fatalf("bad: %#v", plan)
	}

	// Lookup the allocations by JobID
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

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
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job and set the task group count to zero.
	job := mock.Job()
	job.TaskGroups[0].Count = 0
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

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
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

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
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

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
	node.Reserved = node.Resources
	node.ComputeClass()
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create an ineligible node
	node2 := mock.Node()
	node2.Attributes["kernel.name"] = "windows"
	node2.ComputeClass()
	noErr(t, h.State.UpsertNode(h.NextIndex(), node2))

	// Create a jobs
	job := mock.Job()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

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
	noErr(t, node.ComputeClass())
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

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
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
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

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != 2 {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure two allocations placed
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)
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
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock blocked evaluation
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Status:      structs.EvalStatusBlocked,
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerMaxPlans,
		JobID:       job.ID,
	}

	// Insert it into the state store
	noErr(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

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
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a job with a high resource ask so that all the allocations can't
	// be placed on a single node.
	job := mock.Job()
	job.TaskGroups[0].Count = 3
	job.TaskGroups[0].Tasks[0].Resources.CPU = 3600
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
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
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

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
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock blocked evaluation
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Status:      structs.EvalStatusBlocked,
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

	// Insert it into the state store
	noErr(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

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
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job and set the task group count to zero.
	job := mock.Job()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock blocked evaluation
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Status:      structs.EvalStatusBlocked,
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

	// Insert it into the state store
	noErr(t, h.State.UpsertEvals(h.NextIndex(), []*structs.Evaluation{eval}))

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
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

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
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.Job()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
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
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		alloc.DesiredStatus = structs.AllocDesiredStatusStop
		terminal = append(terminal, alloc)
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), terminal))

	// Update the job
	job2 := mock.Job()
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
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

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
	node.Resources.CPU = 1000
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job with one allocation
	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Resources.CPU = 256
	job2 := job.Copy()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.Resources.CPU = 256
	allocs = append(allocs, alloc)
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Update the job to count 3
	job2.TaskGroups[0].Count = 3
	noErr(t, h.State.UpsertJob(h.NextIndex(), job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
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
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

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
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.Job()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
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
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		alloc.DesiredStatus = structs.AllocDesiredStatusStop
		terminal = append(terminal, alloc)
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), terminal))

	// Update the job to be count zero
	job2 := mock.Job()
	job2.ID = job.ID
	job2.TaskGroups[0].Count = 0
	noErr(t, h.State.UpsertJob(h.NextIndex(), job2))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
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
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

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
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.Job()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Update the job
	job2 := mock.Job()
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

func TestServiceSched_JobModify_InPlace(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations
	job := mock.Job()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = nodes[i].ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Update the job
	job2 := mock.Job()
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
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

	// Ensure all allocations placed
	if len(out) != 10 {
		for _, alloc := range out {
			t.Logf("%#v", alloc)
		}
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

func TestServiceSched_JobDeregister(t *testing.T) {
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
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create a mock evaluation to deregister the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerJobDeregister,
		JobID:       job.ID,
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

	// Ensure the plan evicted all nodes
	if len(plan.NodeUpdate["12345678-abcd-efab-cdef-123456789abc"]) != len(allocs) {
		t.Fatalf("bad: %#v", plan)
	}

	// Lookup the allocations by JobID
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

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

func TestServiceSched_NodeDown(t *testing.T) {
	h := NewHarness(t)

	// Register a node
	node := mock.Node()
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}

	// Cover each terminal case and ensure it doesn't change to lost
	allocs[7].DesiredStatus = structs.AllocDesiredStatusRun
	allocs[7].ClientStatus = structs.AllocClientStatusLost
	allocs[8].DesiredStatus = structs.AllocDesiredStatusRun
	allocs[8].ClientStatus = structs.AllocClientStatusFailed
	allocs[9].DesiredStatus = structs.AllocDesiredStatusRun
	allocs[9].ClientStatus = structs.AllocClientStatusComplete

	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Mark some allocs as running
	for i := 0; i < 4; i++ {
		out, _ := h.State.AllocByID(allocs[i].ID)
		out.ClientStatus = structs.AllocClientStatusRunning
		noErr(t, h.State.UpdateAllocsFromClient(h.NextIndex(), []*structs.Allocation{out}))
	}

	// Mark the node as down
	noErr(t, h.State.UpdateNodeStatus(h.NextIndex(), node.ID, structs.NodeStatusDown))

	// Create a mock evaluation to deal with drain
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

	// Test the scheduler marked all non-terminal allocations as lost
	if len(plan.NodeUpdate[node.ID]) != 7 {
		t.Fatalf("bad: %#v", plan)
	}

	for _, out := range plan.NodeUpdate[node.ID] {
		if out.ClientStatus != structs.AllocClientStatusLost && out.DesiredStatus != structs.AllocDesiredStatusStop {
			t.Fatalf("bad alloc: %#v", out)
		}
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_NodeUpdate(t *testing.T) {
	h := NewHarness(t)

	// Register a node
	node := mock.Node()
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Mark some allocs as running
	for i := 0; i < 4; i++ {
		out, _ := h.State.AllocByID(allocs[i].ID)
		out.ClientStatus = structs.AllocClientStatusRunning
		noErr(t, h.State.UpdateAllocsFromClient(h.NextIndex(), []*structs.Allocation{out}))
	}

	// Create a mock evaluation which won't trigger any new placements
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
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create a mock evaluation to deal with drain
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
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

	// Ensure all allocations placed
	out, _ = structs.FilterTerminalAllocs(out)
	if len(out) != 10 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_NodeDrain_Sticky(t *testing.T) {
	h := NewHarness(t)

	// Register a draining node
	node := mock.Node()
	node.Drain = true
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create an alloc on the draining node
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	alloc.Job.TaskGroups[0].Count = 1
	noErr(t, h.State.UpsertJob(h.NextIndex(), alloc.Job))
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to deal with drain
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    50,
		TriggeredBy: structs.EvalTriggerNodeUpdate,
		JobID:       alloc.Job.ID,
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

func TestServiceSched_NodeDrain_Down(t *testing.T) {
	h := NewHarness(t)

	// Register a draining node
	node := mock.Node()
	node.Drain = true
	node.Status = structs.NodeStatusDown
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job with allocations
	job := mock.Job()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Set the desired state of the allocs to stop
	var stop []*structs.Allocation
	for i := 0; i < 10; i++ {
		newAlloc := allocs[i].Copy()
		newAlloc.ClientStatus = structs.AllocDesiredStatusStop
		stop = append(stop, newAlloc)
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), stop))

	// Mark some of the allocations as running
	var running []*structs.Allocation
	for i := 4; i < 6; i++ {
		newAlloc := stop[i].Copy()
		newAlloc.ClientStatus = structs.AllocClientStatusRunning
		running = append(running, newAlloc)
	}
	noErr(t, h.State.UpdateAllocsFromClient(h.NextIndex(), running))

	// Mark some of the allocations as complete
	var complete []*structs.Allocation
	for i := 6; i < 10; i++ {
		newAlloc := stop[i].Copy()
		newAlloc.ClientStatus = structs.AllocClientStatusComplete
		complete = append(complete, newAlloc)
	}
	noErr(t, h.State.UpdateAllocsFromClient(h.NextIndex(), complete))

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
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	job.TaskGroups[0].Count = 2
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 2; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	node.Drain = true
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a mock evaluation to deal with drain
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

	queued := h.Evals[0].QueuedAllocations["web"]
	if queued != 2 {
		t.Fatalf("expected: %v, actual: %v", 2, queued)
	}
}

func TestServiceSched_NodeDrain_UpdateStrategy(t *testing.T) {
	h := NewHarness(t)

	// Register a draining node
	node := mock.Node()
	node.Drain = true
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Generate a fake job with allocations and an update policy.
	job := mock.Job()
	mp := 5
	job.Update = structs.UpdateStrategy{
		Stagger:     time.Second,
		MaxParallel: mp,
	}
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = node.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), allocs))

	// Create a mock evaluation to deal with drain
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

	// Ensure the plan evicted all allocs
	if len(plan.NodeUpdate[node.ID]) != mp {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure the plan allocated
	var planned []*structs.Allocation
	for _, allocList := range plan.NodeAllocation {
		planned = append(planned, allocList...)
	}
	if len(planned) != mp {
		t.Fatalf("bad: %#v", plan)
	}

	// Ensure there is a followup eval.
	if len(h.CreateEvals) != 1 ||
		h.CreateEvals[0].TriggeredBy != structs.EvalTriggerRollingUpdate {
		t.Fatalf("bad: %#v", h.CreateEvals)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestServiceSched_RetryLimit(t *testing.T) {
	h := NewHarness(t)
	h.Planner = &RejectPlan{h}

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.Job()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

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
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

	// Ensure no allocations placed
	if len(out) != 0 {
		t.Fatalf("bad: %#v", out)
	}

	// Should hit the retry limit
	h.AssertEvalStatus(t, structs.EvalStatusFailed)
}

func TestBatchSched_Run_CompleteAlloc(t *testing.T) {
	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a job
	job := mock.Job()
	job.TaskGroups[0].Count = 1
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a complete alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.ClientStatus = structs.AllocClientStatusComplete
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

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
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

	// Ensure no allocations placed
	if len(out) != 1 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestBatchSched_Run_DrainedAlloc(t *testing.T) {
	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a job
	job := mock.Job()
	job.TaskGroups[0].Count = 1
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a complete alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.DesiredStatus = structs.AllocDesiredStatusStop
	alloc.ClientStatus = structs.AllocClientStatusComplete
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

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
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

	// Ensure a replacement alloc was placed.
	if len(out) != 2 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestBatchSched_Run_FailedAlloc(t *testing.T) {
	h := NewHarness(t)

	// Create a node
	node := mock.Node()
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a job
	job := mock.Job()
	job.TaskGroups[0].Count = 1
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a failed alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.ClientStatus = structs.AllocClientStatusFailed
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

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
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

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

func TestBatchSched_Run_FailedAllocQueuedAllocations(t *testing.T) {
	h := NewHarness(t)

	node := mock.Node()
	node.Drain = true
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))

	// Create a job
	job := mock.Job()
	job.TaskGroups[0].Count = 1
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a failed alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.ClientStatus = structs.AllocClientStatusFailed
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

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
	noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	noErr(t, h.State.UpsertNode(h.NextIndex(), node2))

	// Create a job
	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Count = 1
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a successful alloc
	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = node.ID
	alloc.Name = "my-job.web[0]"
	alloc.ClientStatus = structs.AllocClientStatusComplete
	alloc.TaskStates = map[string]*structs.TaskState{
		"web": &structs.TaskState{
			State: structs.TaskStateDead,
			Events: []*structs.TaskEvent{
				{
					Type:     structs.TaskTerminated,
					ExitCode: 0,
				},
			},
		},
	}
	noErr(t, h.State.UpsertAllocs(h.NextIndex(), []*structs.Allocation{alloc}))

	// Create a mock evaluation to rerun the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}

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
	out, err := h.State.AllocsByJob(job.ID)
	noErr(t, err)

	// Ensure no replacement alloc was placed.
	if len(out) != 1 {
		t.Fatalf("bad: %#v", out)
	}

	h.AssertEvalStatus(t, structs.EvalStatusComplete)
}

func TestGenericSched_FilterCompleteAllocs(t *testing.T) {
	running := mock.Alloc()
	desiredStop := mock.Alloc()
	desiredStop.DesiredStatus = structs.AllocDesiredStatusStop

	new := mock.Alloc()
	new.CreateIndex = 10000

	oldSuccessful := mock.Alloc()
	oldSuccessful.CreateIndex = 30
	oldSuccessful.DesiredStatus = structs.AllocDesiredStatusStop
	oldSuccessful.ClientStatus = structs.AllocClientStatusComplete
	oldSuccessful.TaskStates = make(map[string]*structs.TaskState, 1)
	oldSuccessful.TaskStates["foo"] = &structs.TaskState{
		State:  structs.TaskStateDead,
		Events: []*structs.TaskEvent{{Type: structs.TaskTerminated, ExitCode: 0}},
	}

	unsuccessful := mock.Alloc()
	unsuccessful.DesiredStatus = structs.AllocDesiredStatusRun
	unsuccessful.ClientStatus = structs.AllocClientStatusFailed
	unsuccessful.TaskStates = make(map[string]*structs.TaskState, 1)
	unsuccessful.TaskStates["foo"] = &structs.TaskState{
		State:  structs.TaskStateDead,
		Events: []*structs.TaskEvent{{Type: structs.TaskTerminated, ExitCode: 1}},
	}

	cases := []struct {
		Batch          bool
		Input, Output  []*structs.Allocation
		TerminalAllocs map[string]*structs.Allocation
	}{
		{
			Input:          []*structs.Allocation{running},
			Output:         []*structs.Allocation{running},
			TerminalAllocs: map[string]*structs.Allocation{},
		},
		{
			Input:  []*structs.Allocation{running, desiredStop},
			Output: []*structs.Allocation{running},
			TerminalAllocs: map[string]*structs.Allocation{
				desiredStop.Name: desiredStop,
			},
		},
		{
			Batch:          true,
			Input:          []*structs.Allocation{running},
			Output:         []*structs.Allocation{running},
			TerminalAllocs: map[string]*structs.Allocation{},
		},
		{
			Batch:          true,
			Input:          []*structs.Allocation{new, oldSuccessful},
			Output:         []*structs.Allocation{new},
			TerminalAllocs: map[string]*structs.Allocation{},
		},
		{
			Batch:  true,
			Input:  []*structs.Allocation{unsuccessful},
			Output: []*structs.Allocation{},
			TerminalAllocs: map[string]*structs.Allocation{
				unsuccessful.Name: unsuccessful,
			},
		},
	}

	for i, c := range cases {
		g := &GenericScheduler{batch: c.Batch}
		out, terminalAllocs := g.filterCompleteAllocs(c.Input)

		if !reflect.DeepEqual(out, c.Output) {
			t.Log("Got:")
			for i, a := range out {
				t.Logf("%d: %#v", i, a)
			}
			t.Log("Want:")
			for i, a := range c.Output {
				t.Logf("%d: %#v", i, a)
			}
			t.Fatalf("Case %d failed", i+1)
		}

		if !reflect.DeepEqual(terminalAllocs, c.TerminalAllocs) {
			t.Log("Got:")
			for n, a := range terminalAllocs {
				t.Logf("%v: %#v", n, a)
			}
			t.Log("Want:")
			for n, a := range c.TerminalAllocs {
				t.Logf("%v: %#v", n, a)
			}
			t.Fatalf("Case %d failed", i+1)
		}

	}
}

func TestGenericSched_ChainedAlloc(t *testing.T) {
	h := NewHarness(t)

	// Create some nodes
	for i := 0; i < 10; i++ {
		node := mock.Node()
		noErr(t, h.State.UpsertNode(h.NextIndex(), node))
	}

	// Create a job
	job := mock.Job()
	noErr(t, h.State.UpsertJob(h.NextIndex(), job))

	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
	}
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
	noErr(t, h1.State.UpsertJob(h1.NextIndex(), job1))

	// Create a mock evaluation to update the job
	eval1 := &structs.Evaluation{
		ID:          structs.GenerateUUID(),
		Priority:    job1.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job1.ID,
	}
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
