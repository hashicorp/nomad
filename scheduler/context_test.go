// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func testContext(t testing.TB) (*state.StateStore, *EvalContext) {
	state := state.TestStateStore(t)
	plan := &structs.Plan{
		EvalID:          uuid.Generate(),
		NodeUpdate:      make(map[string][]*structs.Allocation),
		NodeAllocation:  make(map[string][]*structs.Allocation),
		NodePreemptions: make(map[string][]*structs.Allocation),
	}

	logger := testlog.HCLogger(t)

	ctx := NewEvalContext(nil, state, plan, logger)
	return state, ctx
}

func TestEvalContext_ProposedAlloc(t *testing.T) {
	ci.Parallel(t)

	state, ctx := testContext(t)
	nodes := []*RankedNode{
		{
			Node: &structs.Node{
				// Perfect fit
				ID: uuid.Generate(),
				NodeResources: &structs.NodeResources{
					Cpu: structs.NodeCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.NodeMemoryResources{
						MemoryMB: 2048,
					},
				},
			},
		},
		{
			Node: &structs.Node{
				// Perfect fit
				ID: uuid.Generate(),
				NodeResources: &structs.NodeResources{
					Cpu: structs.NodeCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.NodeMemoryResources{
						MemoryMB: 2048,
					},
				},
			},
		},
	}

	// Add existing allocations
	j1, j2 := mock.Job(), mock.Job()
	alloc1 := &structs.Allocation{
		ID:        uuid.Generate(),
		Namespace: structs.DefaultNamespace,
		EvalID:    uuid.Generate(),
		NodeID:    nodes[0].Node.ID,
		JobID:     j1.ID,
		Job:       j1,
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 2048,
					},
				},
			},
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
		TaskGroup:     "web",
	}
	alloc2 := &structs.Allocation{
		ID:        uuid.Generate(),
		Namespace: structs.DefaultNamespace,
		EvalID:    uuid.Generate(),
		NodeID:    nodes[1].Node.ID,
		JobID:     j2.ID,
		Job:       j2,
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1024,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 1024,
					},
				},
			},
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
		TaskGroup:     "web",
	}
	require.NoError(t, state.UpsertJobSummary(998, mock.JobSummary(alloc1.JobID)))
	require.NoError(t, state.UpsertJobSummary(999, mock.JobSummary(alloc2.JobID)))
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc1, alloc2}))

	// Add a planned eviction to alloc1
	plan := ctx.Plan()
	plan.NodeUpdate[nodes[0].Node.ID] = []*structs.Allocation{alloc1}

	// Add a planned placement to node1
	plan.NodeAllocation[nodes[1].Node.ID] = []*structs.Allocation{
		{
			AllocatedResources: &structs.AllocatedResources{
				Tasks: map[string]*structs.AllocatedTaskResources{
					"web": {
						Cpu: structs.AllocatedCpuResources{
							CpuShares: 1024,
						},
						Memory: structs.AllocatedMemoryResources{
							MemoryMB: 1024,
						},
					},
				},
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

// TestEvalContext_ProposedAlloc_EvictPreempt asserts both Evicted and
// Preempted allocs are removed from the allocs propsed for a node.
//
// See https://github.com/hashicorp/nomad/issues/6787
func TestEvalContext_ProposedAlloc_EvictPreempt(t *testing.T) {
	ci.Parallel(t)
	state, ctx := testContext(t)
	nodes := []*RankedNode{
		{
			Node: &structs.Node{
				ID: uuid.Generate(),
				NodeResources: &structs.NodeResources{
					Cpu: structs.NodeCpuResources{
						CpuShares: 1024 * 3,
					},
					Memory: structs.NodeMemoryResources{
						MemoryMB: 1024 * 3,
					},
				},
			},
		},
	}

	// Add existing allocations
	j1, j2, j3 := mock.Job(), mock.Job(), mock.Job()
	allocEvict := &structs.Allocation{
		ID:        uuid.Generate(),
		Namespace: structs.DefaultNamespace,
		EvalID:    uuid.Generate(),
		NodeID:    nodes[0].Node.ID,
		JobID:     j1.ID,
		Job:       j1,
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1024,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 1024,
					},
				},
			},
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
		TaskGroup:     "web",
	}
	allocPreempt := &structs.Allocation{
		ID:        uuid.Generate(),
		Namespace: structs.DefaultNamespace,
		EvalID:    uuid.Generate(),
		NodeID:    nodes[0].Node.ID,
		JobID:     j2.ID,
		Job:       j2,
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1024,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 1024,
					},
				},
			},
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
		TaskGroup:     "web",
	}
	allocPropose := &structs.Allocation{
		ID:        uuid.Generate(),
		Namespace: structs.DefaultNamespace,
		EvalID:    uuid.Generate(),
		NodeID:    nodes[0].Node.ID,
		JobID:     j3.ID,
		Job:       j3,
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 1024,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 1024,
					},
				},
			},
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		ClientStatus:  structs.AllocClientStatusPending,
		TaskGroup:     "web",
	}
	require.NoError(t, state.UpsertJobSummary(998, mock.JobSummary(allocEvict.JobID)))
	require.NoError(t, state.UpsertJobSummary(999, mock.JobSummary(allocPreempt.JobID)))
	require.NoError(t, state.UpsertJobSummary(999, mock.JobSummary(allocPropose.JobID)))
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{allocEvict, allocPreempt, allocPropose}))

	// Plan to evict one alloc and preempt another
	plan := ctx.Plan()
	plan.NodePreemptions[nodes[0].Node.ID] = []*structs.Allocation{allocEvict}
	plan.NodeUpdate[nodes[0].Node.ID] = []*structs.Allocation{allocPreempt}

	proposed, err := ctx.ProposedAllocs(nodes[0].Node.ID)
	require.NoError(t, err)
	require.Len(t, proposed, 1)
}

func TestEvalEligibility_JobStatus(t *testing.T) {
	ci.Parallel(t)

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
}

func TestEvalEligibility_TaskGroupStatus(t *testing.T) {
	ci.Parallel(t)

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
}

func TestEvalEligibility_SetJob(t *testing.T) {
	ci.Parallel(t)

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
	ci.Parallel(t)

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
		"v1:1": false,
		"v1:2": false,
		"v1:3": true,
		"v1:4": false,
		"v1:5": true,
	}

	actClasses := e.GetClasses()
	require.Equal(t, expClasses, actClasses)
}
func TestEvalEligibility_GetClasses_JobEligible_TaskGroupIneligible(t *testing.T) {
	ci.Parallel(t)

	e := NewEvalEligibility()
	e.SetJobEligibility(true, "v1:1")
	e.SetTaskGroupEligibility(false, "foo", "v1:1")

	e.SetJobEligibility(true, "v1:2")
	e.SetTaskGroupEligibility(false, "foo", "v1:2")
	e.SetTaskGroupEligibility(true, "bar", "v1:2")

	e.SetJobEligibility(true, "v1:3")
	e.SetTaskGroupEligibility(false, "foo", "v1:3")
	e.SetTaskGroupEligibility(false, "bar", "v1:3")

	expClasses := map[string]bool{
		"v1:1": false,
		"v1:2": true,
		"v1:3": false,
	}

	actClasses := e.GetClasses()
	require.Equal(t, expClasses, actClasses)
}

func TestPortCollisionEvent_Copy(t *testing.T) {
	ci.Parallel(t)

	ev := &PortCollisionEvent{
		Reason: "original",
		Node:   mock.Node(),
		Allocations: []*structs.Allocation{
			mock.Alloc(),
			mock.Alloc(),
		},
		NetIndex: structs.NewNetworkIndex(),
	}
	ev.NetIndex.SetNode(ev.Node)

	// Copy must be equal
	evCopy := ev.Copy()
	require.Equal(t, ev, evCopy)

	// Modifying the copy should not affect the original value
	evCopy.Reason = "copy"
	require.NotEqual(t, ev.Reason, evCopy.Reason)

	evCopy.Node.Attributes["test"] = "true"
	require.NotEqual(t, ev.Node, evCopy.Node)

	evCopy.Allocations = append(evCopy.Allocations, mock.Alloc())
	require.NotEqual(t, ev.Allocations, evCopy.Allocations)

	evCopy.NetIndex.AddAllocs(evCopy.Allocations)
	require.NotEqual(t, ev.NetIndex, evCopy.NetIndex)
}

func TestPortCollisionEvent_Sanitize(t *testing.T) {
	ci.Parallel(t)

	ev := &PortCollisionEvent{
		Reason: "original",
		Node:   mock.Node(),
		Allocations: []*structs.Allocation{
			mock.Alloc(),
		},
		NetIndex: structs.NewNetworkIndex(),
	}

	cleanEv := ev.Sanitize()
	require.Empty(t, cleanEv.Node.SecretID)
	require.Nil(t, cleanEv.Allocations[0].Job)
}
