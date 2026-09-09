// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler/tests"
	"github.com/shoenig/test/must"
)

// The previous version must recover its own selected implementation while a
// different task group is being canaried. The previous jobspec also contains
// the target group, but that group was never its serving implementation.
func TestServiceSched_GroupSelection_OldCohortReschedule(t *testing.T) {
	ci.Parallel(t)
	reviewOldCohortReplacement(t, false)
}

// Draining the serving cohort during canary must migrate the old physical
// implementation and leave the unpromoted target allocation intact.
func TestServiceSched_GroupSelection_OldCohortDrain(t *testing.T) {
	ci.Parallel(t)
	reviewOldCohortReplacement(t, true)
}

func reviewOldCohortReplacement(t *testing.T, drain bool) {
	t.Helper()
	h := tests.NewHarness(t)
	selectionNode(t, h, "orin")
	selectionNode(t, h, "orin")
	selectionNode(t, h, "thor")
	job := selectionJob(1)
	for _, group := range job.TaskGroups {
		group.ReschedulePolicy = &structs.ReschedulePolicy{Unlimited: true, Delay: 5 * time.Second, DelayFunction: "constant"}
	}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	oldAllocs := selectionAllocs(t, h, job)
	must.Len(t, 1, oldAllocs)
	old := oldAllocs[0].Copy()
	must.Eq(t, "orin", old.TaskGroup)
	old.ClientStatus = structs.AllocClientStatusRunning
	old.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: new(true)}
	must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{old}}))
	deployment, err := h.State.LatestDeploymentByJobID(nil, job.Namespace, job.ID)
	must.NoError(t, err)
	deployment = deployment.Copy()
	deployment.Status = structs.DeploymentStatusSuccessful
	must.NoError(t, h.State.UpsertDeployment(h.NextIndex(), deployment))
	oldVersion := old.Job.Version
	oldCohort := old.GroupSelection.Cohort
	job = job.Copy()
	for _, group := range job.TaskGroups {
		group.Tasks[0].Config["command"] = "/bin/new-release"
	}
	job.LookupTaskGroup("orin").Constraints[1].RTarget = "2"
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	var canary *structs.Allocation
	for _, allocation := range selectionAllocs(t, h, job) {
		if allocation.DeploymentStatus.IsCanary() {
			canary = allocation
		}
	}
	must.NotNil(t, canary)
	must.Eq(t, "thor", canary.TaskGroup)
	if drain {
		must.NoError(t, h.State.UpdateNodeDrain(structs.MsgTypeTestSetup, h.NextIndex(), old.NodeID, &structs.DrainStrategy{DrainSpec: structs.DrainSpec{Deadline: time.Hour}}, false, time.Now().Unix(), nil, nil, ""))
		must.NoError(t, h.State.UpdateAllocsDesiredTransitions(structs.MsgTypeTestSetup, h.NextIndex(), map[string]*structs.DesiredTransition{old.ID: {Migrate: new(true)}}, nil))
	} else {
		old = old.Copy()
		old.ClientStatus = structs.AllocClientStatusFailed
		old.TaskStates = map[string]*structs.TaskState{old.Job.LookupTaskGroup("orin").Tasks[0].Name: {State: structs.TaskStateDead, Failed: true, FinishedAt: time.Now().Add(-time.Minute)}}
		must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{old}}))
	}
	selectionEval(t, h, job)
	var replacement *structs.Allocation
	for _, allocation := range selectionAllocs(t, h, job) {
		if allocation.ID != old.ID && allocation.ID != canary.ID && allocation.DesiredStatus == structs.AllocDesiredStatusRun && !allocation.DeploymentStatus.IsCanary() {
			replacement = allocation
		}
	}
	must.NotNil(t, replacement)
	must.Eq(t, "orin", replacement.TaskGroup)
	must.Eq(t, oldVersion, replacement.Job.Version)
	must.Eq(t, oldCohort, replacement.GroupSelection.Cohort)
	must.Eq(t, old.ID, replacement.PreviousAllocation)
	if drain {
		must.NotEq(t, old.NodeID, replacement.NodeID)
	}
	retainedCanary, err := h.State.AllocByID(nil, canary.ID)
	must.NoError(t, err)
	must.Eq(t, structs.AllocDesiredStatusRun, retainedCanary.DesiredStatus)
	must.True(t, retainedCanary.DeploymentStatus.IsCanary())
	must.Eq(t, int64(800), replacement.AllocatedResources.Tasks[old.Job.LookupTaskGroup("orin").Tasks[0].Name].Cpu.CpuShares)
	latestEval := h.Evals[len(h.Evals)-1]
	must.Eq(t, 0, latestEval.QueuedAllocations["orin"])
	must.Eq(t, 0, latestEval.QueuedAllocations["thor"])
}

// Removing the opt-in block restores ordinary required-group semantics, even
// for an allocation retained through an in-place update from the selection job.
func TestServiceSched_GroupSelection_RemoveSelection(t *testing.T) {
	ci.Parallel(t)
	h := tests.NewHarness(t)
	selectionNode(t, h, "orin")
	job := selectionJob(1)
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	allocs := selectionAllocs(t, h, job)
	must.Len(t, 1, allocs)
	old := allocs[0].Copy()
	old.ClientStatus = structs.AllocClientStatusRunning
	old.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: new(true)}
	must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{old}}))
	deployment, err := h.State.LatestDeploymentByJobID(nil, job.Namespace, job.ID)
	must.NoError(t, err)
	deployment = deployment.Copy()
	deployment.Status = structs.DeploymentStatusSuccessful
	must.NoError(t, h.State.UpsertDeployment(h.NextIndex(), deployment))
	selectionNode(t, h, "encoder")
	selectionNode(t, h, "thor")
	job = job.Copy()
	job.GroupSelections = nil
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	allocs = selectionAllocs(t, h, job)
	must.Len(t, 3, allocs)
	for _, allocation := range allocs {
		allocation = allocation.Copy()
		allocation.ClientStatus = structs.AllocClientStatusRunning
		allocation.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: new(true)}
		must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{allocation}}))
	}
	deployment, err = h.State.LatestDeploymentByJobID(nil, job.Namespace, job.ID)
	must.NoError(t, err)
	must.NotNil(t, deployment.TaskGroups["orin"])
	must.Eq(t, 1, deployment.TaskGroups["orin"].PlacedAllocs)
	must.Eq(t, 1, deployment.TaskGroups["orin"].HealthyAllocs)
	retained, err := h.State.AllocByID(nil, old.ID)
	must.NoError(t, err)
	must.Eq(t, structs.AllocDesiredStatusRun, retained.DesiredStatus)
}

// Alternative groups may have different replica counts. Promotion must converge
// to the selected target's count, not the source count or the sum of cohorts.
func TestServiceSched_GroupSelection_DifferentReplicaCounts(t *testing.T) {
	ci.Parallel(t)
	for _, pair := range [][2]int{{2, 3}, {3, 2}, {1, 3}, {3, 1}} {
		t.Run(fmt.Sprintf("%d_to_%d", pair[0], pair[1]), func(t *testing.T) {
			h := tests.NewHarness(t)
			selectionNode(t, h, "orin")
			selectionNode(t, h, "thor")
			job := selectionJob(1)
			job.LookupTaskGroup("orin").Count = pair[0]
			job.LookupTaskGroup("thor").Count = pair[1]
			must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
			selectionEval(t, h, job)
			old := selectionAllocs(t, h, job)
			must.Len(t, pair[0], old)
			for _, allocation := range old {
				allocation = allocation.Copy()
				allocation.ClientStatus = structs.AllocClientStatusRunning
				allocation.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: new(true)}
				must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{allocation}}))
			}
			deployment, err := h.State.LatestDeploymentByJobID(nil, job.Namespace, job.ID)
			must.NoError(t, err)
			deployment = deployment.Copy()
			deployment.Status = structs.DeploymentStatusSuccessful
			must.NoError(t, h.State.UpsertDeployment(h.NextIndex(), deployment))
			job = job.Copy()
			for _, group := range job.TaskGroups {
				group.Tasks[0].Config["command"] = "/bin/new-release"
			}
			job.LookupTaskGroup("orin").Constraints[1].RTarget = "2"
			must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
			selectionEval(t, h, job)
			var canary *structs.Allocation
			for _, allocation := range selectionAllocs(t, h, job) {
				if allocation.DeploymentStatus.IsCanary() {
					canary = allocation.Copy()
				}
			}
			must.NotNil(t, canary)
			must.Eq(t, "thor", canary.TaskGroup)
			for _, previous := range old {
				retained, err := h.State.AllocByID(nil, previous.ID)
				must.NoError(t, err)
				must.Eq(t, structs.AllocDesiredStatusRun, retained.DesiredStatus)
			}
			canary.ClientStatus = structs.AllocClientStatusRunning
			canary.DeploymentStatus.Healthy = new(true)
			must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{canary}}))
			must.NoError(t, h.State.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, h.NextIndex(), &structs.ApplyDeploymentPromoteRequest{DeploymentPromoteRequest: structs.DeploymentPromoteRequest{DeploymentID: canary.DeploymentID, All: true, PromotedAt: time.Now().UnixNano()}}))
			for iteration := 0; iteration < 8; iteration++ {
				selectionEval(t, h, job)
				for _, allocation := range selectionAllocs(t, h, job) {
					if allocation.DesiredStatus != structs.AllocDesiredStatusRun {
						continue
					}
					allocation = allocation.Copy()
					allocation.ClientStatus = structs.AllocClientStatusRunning
					if allocation.DeploymentStatus == nil {
						allocation.DeploymentStatus = &structs.AllocDeploymentStatus{}
					}
					allocation.DeploymentStatus.Healthy = new(true)
					must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{allocation}}))
				}
			}
			live := 0
			retained := false
			for _, allocation := range selectionAllocs(t, h, job) {
				if allocation.DesiredStatus == structs.AllocDesiredStatusRun {
					live++
					must.Eq(t, "thor", allocation.TaskGroup)
					must.Eq(t, canary.GroupSelection.Cohort, allocation.GroupSelection.Cohort)
					if allocation.ID == canary.ID {
						retained = true
					}
				}
			}
			must.Eq(t, pair[1], live)
			must.True(t, retained)
			deployment, err = h.State.LatestDeploymentByJobID(nil, job.Namespace, job.ID)
			must.NoError(t, err)
			must.Eq(t, pair[1], deployment.TaskGroups["thor"].DesiredTotal)
			must.Eq(t, pair[1], deployment.TaskGroups["thor"].HealthyAllocs)
		})
	}
}

// A preview that preempts a reserved port must release that victim from later
// previews. Otherwise its port collides with the new preview and hides feasible
// alternatives for another slot.
func TestServiceSched_GroupSelection_PreemptionPreviewPorts(t *testing.T) {
	ci.Parallel(t)
	h := tests.NewHarness(t)
	node := mock.Node()
	node.NodeResources.Cpu, node.NodeResources.Processors = tests.CpuResources(1000)
	node.ReservedResources.Cpu.CpuShares = 0
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	victim := mock.Job()
	victim.Priority = 10
	victim.TaskGroups[0].Count = 1
	victim.TaskGroups[0].Tasks[0].Resources.CPU = 900
	victim.TaskGroups[0].Networks = []*structs.NetworkResource{{Mode: "host", ReservedPorts: []structs.Port{{Label: "http", Value: 80, HostNetwork: "default"}}}}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, victim))
	selectionEval(t, h, victim)
	victimAllocs := selectionAllocs(t, h, victim)
	must.Len(t, 1, victimAllocs)

	job := selectionJob(1)
	job.Priority = 100
	job.GroupSelections[0].Count = 2
	for i, group := range job.TaskGroups {
		group.Constraints = nil
		group.Networks = nil
		group.Tasks[0].Resources.CPU = []int{300, 800, 400}[i]
	}
	job.TaskGroups[0].Networks = []*structs.NetworkResource{{Mode: "host", ReservedPorts: []structs.Port{{Label: "http", Value: 80, HostNetwork: "default"}}}}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	allocs := selectionAllocs(t, h, job)
	must.Len(t, 2, allocs)
	groups := make(map[string]bool)
	for _, alloc := range allocs {
		groups[alloc.TaskGroup] = true
	}
	must.Eq(t, map[string]bool{"encoder": true, "thor": true}, groups)
}

// Candidate previews account for placeable replicas. Repeated evaluations
// must converge when all selected groups fit together without preemption.
func TestServiceSched_GroupSelection_ReplicaCapacity(t *testing.T) {
	ci.Parallel(t)
	h := tests.NewHarness(t)
	node := mock.Node()
	node.NodeResources.Cpu, node.NodeResources.Processors = tests.CpuResources(1000)
	node.ReservedResources.Cpu.CpuShares = 0
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	job := selectionJob(1)
	job.GroupSelections[0].Count = 2
	job.TaskGroups[0].Count = 2
	for i, group := range job.TaskGroups {
		group.Constraints = nil
		group.Networks = nil
		group.Tasks[0].Resources.CPU = []int{300, 500, 300}[i]
	}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	for iteration := 0; iteration < 3; iteration++ {
		selectionEval(t, h, job)
		for _, allocation := range selectionAllocs(t, h, job) {
			allocation = allocation.Copy()
			allocation.ClientStatus = structs.AllocClientStatusRunning
			allocation.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: new(true)}
			must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{allocation}}))
		}
	}
	groups := make(map[string]int)
	for _, allocation := range selectionAllocs(t, h, job) {
		if allocation.DesiredStatus == structs.AllocDesiredStatusRun {
			groups[allocation.TaskGroup]++
		}
	}
	must.Eq(t, map[string]int{"encoder": 2, "thor": 1}, groups)
}

// All previewed alternatives contribute to the blocked evaluation's class
// eligibility. Capacity returning to a non-fallback profile must wake the job.
func TestServiceSched_GroupSelection_BlockedAlternativeClass(t *testing.T) {
	ci.Parallel(t)
	h := tests.NewHarness(t)
	node := selectionNode(t, h, "thor").Copy()
	node.NodeResources.Cpu, node.NodeResources.Processors = tests.CpuResources(100)
	node.ReservedResources.Cpu.CpuShares = 0
	must.NoError(t, node.ComputeClass())
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	job := selectionJob(1)
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	must.Len(t, 0, selectionAllocs(t, h, job))
	must.Len(t, 1, h.CreateEvals)
	blocked := h.CreateEvals[0]
	must.Eq(t, structs.EvalStatusBlocked, blocked.Status)
	must.True(t, blocked.EscapedComputedClass || blocked.ClassEligibility[node.ComputedClass])

	// Changing capacity does not change the profile's computed class.
	previousClass := node.ComputedClass
	node = node.Copy()
	node.NodeResources.Cpu, node.NodeResources.Processors = tests.CpuResources(1000)
	must.NoError(t, node.ComputeClass())
	must.Eq(t, previousClass, node.ComputedClass)
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{blocked}))
	must.NoError(t, h.Process(NewServiceScheduler, blocked))
	allocs := selectionAllocs(t, h, job)
	must.Len(t, 1, allocs)
	must.Eq(t, "thor", allocs[0].TaskGroup)
}

// Replacing one slot must not steal a healthy implementation assigned to a
// later slot, even when it precedes the unused alternatives in preference order.
func TestServiceSched_GroupSelection_PreserveOtherSlot(t *testing.T) {
	ci.Parallel(t)
	h := tests.NewHarness(t)
	failedNode := selectionNode(t, h, "encoder")
	selectionNode(t, h, "orin")
	selectionNode(t, h, "thor")
	job := selectionJob(1)
	job.GroupSelections[0].Count = 2
	for _, group := range job.TaskGroups {
		group.Update = &structs.UpdateStrategy{}
	}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	initial := selectionAllocs(t, h, job)
	must.Len(t, 2, initial)
	var incumbent *structs.Allocation
	for _, allocation := range initial {
		allocation = allocation.Copy()
		allocation.ClientStatus = structs.AllocClientStatusRunning
		must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{allocation}}))
		if allocation.TaskGroup == "orin" {
			incumbent = allocation
		}
	}
	must.NotNil(t, incumbent)
	must.Eq(t, 1, incumbent.GroupSelection.Slot)
	failedNode = failedNode.Copy()
	failedNode.Status = structs.NodeStatusDown
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), failedNode))
	selectionEval(t, h, job)
	retained, err := h.State.AllocByID(nil, incumbent.ID)
	must.NoError(t, err)
	must.Eq(t, structs.AllocDesiredStatusRun, retained.DesiredStatus)
	must.Eq(t, incumbent.GroupSelection, retained.GroupSelection)
	live := make(map[string]int)
	for _, allocation := range selectionAllocs(t, h, job) {
		if allocation.DesiredStatus == structs.AllocDesiredStatusRun {
			live[allocation.TaskGroup]++
			if allocation.TaskGroup == "thor" {
				must.Eq(t, 0, allocation.GroupSelection.Slot)
			}
		}
	}
	must.Eq(t, map[string]int{"orin": 1, "thor": 1}, live)
}
