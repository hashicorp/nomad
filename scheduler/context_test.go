package scheduler

import (
	"log"
	"os"
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

func testContext(t testing.TB) (*state.StateStore, *EvalContext) {
	state, err := state.NewStateStore(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	plan := &structs.Plan{
		NodeUpdate:     make(map[string][]*structs.Allocation),
		NodeAllocation: make(map[string][]*structs.Allocation),
	}

	logger := log.New(os.Stderr, "", log.LstdFlags)

	ctx := NewEvalContext(state, plan, logger)
	return state, ctx
}

func TestEvalContext_ProposedAlloc(t *testing.T) {
	state, ctx := testContext(t)
	nodes := []*RankedNode{
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				ID: structs.GenerateUUID(),
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
			},
		},
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				ID: structs.GenerateUUID(),
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
			},
		},
	}

	// Add existing allocations
	j1, j2 := mock.Job(), mock.Job()
	alloc1 := &structs.Allocation{
		ID:     structs.GenerateUUID(),
		EvalID: structs.GenerateUUID(),
		NodeID: nodes[0].Node.ID,
		JobID:  j1.ID,
		Job:    j1,
		Resources: &structs.Resources{
			CPU:      2048,
			MemoryMB: 2048,
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
		TaskGroup:     "web",
	}
	alloc2 := &structs.Allocation{
		ID:     structs.GenerateUUID(),
		EvalID: structs.GenerateUUID(),
		NodeID: nodes[1].Node.ID,
		JobID:  j2.ID,
		Job:    j2,
		Resources: &structs.Resources{
			CPU:      1024,
			MemoryMB: 1024,
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
		TaskGroup:     "web",
	}
	noErr(t, state.UpsertJobSummary(998, mock.JobSummary(alloc1.JobID)))
	noErr(t, state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID)))
	noErr(t, state.UpsertAllocs(1000, []*structs.Allocation{alloc1, alloc2}))

	// Add a planned eviction to alloc1
	plan := ctx.Plan()
	plan.NodeUpdate[nodes[0].Node.ID] = []*structs.Allocation{alloc1}

	// Add a planned placement to node1
	plan.NodeAllocation[nodes[1].Node.ID] = []*structs.Allocation{
		&structs.Allocation{
			Resources: &structs.Resources{
				CPU:      1024,
				MemoryMB: 1024,
			},
		},
	}

	proposed, err := ctx.ProposedAllocs(nodes[0].Node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(proposed) != 0 {
		t.Fatalf("bad: %#v", proposed)
	}

	proposed, err = ctx.ProposedAllocs(nodes[1].Node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(proposed) != 2 {
		t.Fatalf("bad: %#v", proposed)
	}
}

func TestEvalEligibility_JobStatus(t *testing.T) {
	e := NewEvalEligibility()
	cc := "v1:100"

	// Get the job before its been set.
	if status := e.JobStatus(cc); status != EvalComputedClassUnknown {
		t.Fatalf("JobStatus() returned %v; want %v", status, EvalComputedClassUnknown)
	}

	// Set the job and get its status.
	e.SetJobEligibility(false, cc)
	if status := e.JobStatus(cc); status != EvalComputedClassIneligible {
		t.Fatalf("JobStatus() returned %v; want %v", status, EvalComputedClassIneligible)
	}

	e.SetJobEligibility(true, cc)
	if status := e.JobStatus(cc); status != EvalComputedClassEligible {
		t.Fatalf("JobStatus() returned %v; want %v", status, EvalComputedClassEligible)
	}

	// Check that if I pass an empty class it returns escaped
	if status := e.JobStatus(""); status != EvalComputedClassEscaped {
		t.Fatalf("JobStatus() returned %v; want %v", status, EvalComputedClassEscaped)
	}
}

func TestEvalEligibility_TaskGroupStatus(t *testing.T) {
	e := NewEvalEligibility()
	cc := "v1:100"
	tg := "foo"

	// Get the tg before its been set.
	if status := e.TaskGroupStatus(tg, cc); status != EvalComputedClassUnknown {
		t.Fatalf("TaskGroupStatus() returned %v; want %v", status, EvalComputedClassUnknown)
	}

	// Set the tg and get its status.
	e.SetTaskGroupEligibility(false, tg, cc)
	if status := e.TaskGroupStatus(tg, cc); status != EvalComputedClassIneligible {
		t.Fatalf("TaskGroupStatus() returned %v; want %v", status, EvalComputedClassIneligible)
	}

	e.SetTaskGroupEligibility(true, tg, cc)
	if status := e.TaskGroupStatus(tg, cc); status != EvalComputedClassEligible {
		t.Fatalf("TaskGroupStatus() returned %v; want %v", status, EvalComputedClassEligible)
	}

	// Check that if I pass an empty class it returns escaped
	if status := e.TaskGroupStatus(tg, ""); status != EvalComputedClassEscaped {
		t.Fatalf("TaskGroupStatus() returned %v; want %v", status, EvalComputedClassEscaped)
	}
}

func TestEvalEligibility_SetJob(t *testing.T) {
	e := NewEvalEligibility()
	ne1 := &structs.Constraint{
		LTarget: "${attr.kernel.name}",
		RTarget: "linux",
		Operand: "=",
	}
	e1 := &structs.Constraint{
		LTarget: "${attr.unique.kernel.name}",
		RTarget: "linux",
		Operand: "=",
	}
	e2 := &structs.Constraint{
		LTarget: "${meta.unique.key_foo}",
		RTarget: "linux",
		Operand: "<",
	}
	e3 := &structs.Constraint{
		LTarget: "${meta.unique.key_foo}",
		RTarget: "Windows",
		Operand: "<",
	}

	job := mock.Job()
	jobCon := []*structs.Constraint{ne1, e1, e2}
	job.Constraints = jobCon

	// Set the task constraints
	tg := job.TaskGroups[0]
	tg.Constraints = []*structs.Constraint{e1}
	tg.Tasks[0].Constraints = []*structs.Constraint{e3}

	e.SetJob(job)
	if !e.HasEscaped() {
		t.Fatalf("HasEscaped() should be true")
	}

	if !e.jobEscaped {
		t.Fatalf("SetJob() should mark job as escaped")
	}
	if escaped, ok := e.tgEscapedConstraints[tg.Name]; !ok || !escaped {
		t.Fatalf("SetJob() should mark task group as escaped")
	}
}

func TestEvalEligibility_GetClasses(t *testing.T) {
	e := NewEvalEligibility()
	e.SetJobEligibility(true, "v1:1")
	e.SetJobEligibility(false, "v1:2")
	e.SetTaskGroupEligibility(true, "foo", "v1:3")
	e.SetTaskGroupEligibility(false, "bar", "v1:4")
	e.SetTaskGroupEligibility(true, "bar", "v1:5")

	// Mark an existing eligible class as ineligible in the TG.
	e.SetTaskGroupEligibility(false, "fizz", "v1:1")
	e.SetTaskGroupEligibility(false, "fizz", "v1:3")

	expClasses := map[string]bool{
		"v1:1": true,
		"v1:2": false,
		"v1:3": true,
		"v1:4": false,
		"v1:5": true,
	}

	actClasses := e.GetClasses()
	if !reflect.DeepEqual(actClasses, expClasses) {
		t.Fatalf("GetClasses() returned %#v; want %#v", actClasses, expClasses)
	}
}
