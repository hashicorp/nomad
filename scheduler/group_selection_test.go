// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler/tests"
	"github.com/shoenig/test/must"
)

func selectionJob(replicas int) *structs.Job {
	job := mock.Job()
	original := job.TaskGroups[0]
	job.TaskGroups = nil
	for i, name := range []string{"encoder", "orin", "thor"} {
		tg := original.Copy()
		tg.Name, tg.Count = name, replicas
		tg.Constraints = []*structs.Constraint{
			{LTarget: "${meta.profile}", Operand: "=", RTarget: name},
			{LTarget: "${meta.available}", Operand: "=", RTarget: "1"},
		}
		tg.Tasks[0].Resources.CPU = []int{1000, 800, 500}[i]
		tg.Tasks[0].Resources.MemoryMB = []int{1000, 800, 500}[i]
		tg.Update = &structs.UpdateStrategy{MaxParallel: 1, Canary: 1, AutoPromote: true,
			HealthCheck: structs.UpdateStrategyHealthCheck_Checks, HealthyDeadline: time.Minute, ProgressDeadline: 2 * time.Minute}
		job.TaskGroups = append(job.TaskGroups, tg)
	}
	job.GroupSelections = []*structs.TaskGroupSelection{{Name: "runtime", Count: 1, Groups: []string{"encoder", "orin", "thor"}}}
	return job
}

func selectionNode(t *testing.T, h *tests.Harness, profile string) *structs.Node {
	t.Helper()
	node := mock.Node()
	node.Meta["profile"], node.Meta["available"] = profile, "1"
	must.NoError(t, node.ComputeClass())
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	return node
}

func selectionEval(t *testing.T, h *tests.Harness, job *structs.Job) {
	t.Helper()
	eval := &structs.Evaluation{ID: uuid.Generate(), Namespace: job.Namespace, JobID: job.ID,
		Priority: job.Priority, TriggeredBy: structs.EvalTriggerJobRegister, Status: structs.EvalStatusPending}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))
	must.NoError(t, h.Process(NewServiceScheduler, eval))
}

func selectionAllocs(t *testing.T, h *tests.Harness, job *structs.Job) []*structs.Allocation {
	t.Helper()
	allocs, err := h.State.AllocsByJob(nil, job.Namespace, job.ID, false)
	must.NoError(t, err)
	return allocs
}

func TestServiceSched_GroupSelection_Placement(t *testing.T) {
	for _, replicas := range []int{1, 3} {
		t.Run(fmt.Sprint(replicas), func(t *testing.T) {
			h := tests.NewHarness(t)
			selectionNode(t, h, "thor")
			job := selectionJob(replicas)
			must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
			selectionEval(t, h, job)
			allocs := selectionAllocs(t, h, job)
			must.Len(t, replicas, allocs)
			for _, alloc := range allocs {
				must.Eq(t, "thor", alloc.TaskGroup)
				must.Eq(t, "runtime", alloc.GroupSelection.Name)
				must.Eq(t, 0, alloc.GroupSelection.Slot)
				must.Eq(t, int64(500), alloc.AllocatedResources.Tasks[job.TaskGroups[2].Tasks[0].Name].Cpu.CpuShares)
			}
			must.Len(t, 0, h.CreateEvals)
			before := allocs[0].ID
			selectionNode(t, h, "encoder")
			selectionEval(t, h, job)
			allocs = selectionAllocs(t, h, job)
			must.Len(t, replicas, allocs)
			found := false
			for _, alloc := range allocs {
				if alloc.ID == before {
					found = true
				}
			}
			must.True(t, found)
		})
	}
}

func TestServiceSched_GroupSelection_NoFit(t *testing.T) {
	h := tests.NewHarness(t)
	selectionNode(t, h, "unavailable")
	job := selectionJob(1)
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	must.Len(t, 0, selectionAllocs(t, h, job))
	must.Len(t, 1, h.CreateEvals)
	d, err := h.State.LatestDeploymentByJobID(nil, job.Namespace, job.ID)
	must.NoError(t, err)
	must.NotNil(t, d)
	must.NotEq(t, structs.DeploymentStatusSuccessful, d.Status)
}

func TestServiceSched_GroupSelection_CrossProfileCanary(t *testing.T) {
	for _, replicas := range []int{1, 3} {
		t.Run(fmt.Sprint(replicas), func(t *testing.T) {
			h := tests.NewHarness(t)
			oldNode := selectionNode(t, h, "orin")
			selectionNode(t, h, "thor")
			job := selectionJob(replicas)
			must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
			selectionEval(t, h, job)
			old := selectionAllocs(t, h, job)
			must.Len(t, replicas, old)
			healthy := true
			for _, alloc := range old {
				must.Eq(t, "orin", alloc.TaskGroup)
				alloc.ClientStatus = structs.AllocClientStatusRunning
				alloc.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: &healthy}
			}
			must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), old))
			d, err := h.State.LatestDeploymentByJobID(nil, job.Namespace, job.ID)
			must.NoError(t, err)
			d = d.Copy()
			d.Status = structs.DeploymentStatusSuccessful
			must.NoError(t, h.State.UpsertDeployment(h.NextIndex(), d))
			oldNode = oldNode.Copy()
			oldNode.Meta["available"] = "0"
			must.NoError(t, oldNode.ComputeClass())
			must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), oldNode))
			job = job.Copy()
			for _, tg := range job.TaskGroups {
				tg.Tasks[0].Config["command"] = "/bin/new-release"
			}
			must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
			selectionEval(t, h, job)
			allocs := selectionAllocs(t, h, job)
			must.Len(t, replicas+1, allocs)
			var canary *structs.Allocation
			for _, alloc := range allocs {
				if alloc.DeploymentStatus.IsCanary() {
					canary = alloc
				} else {
					must.Eq(t, structs.AllocDesiredStatusRun, alloc.DesiredStatus)
				}
			}
			must.NotNil(t, canary)
			must.Eq(t, "thor", canary.TaskGroup)
			must.Eq(t, "", canary.PreviousAllocation)
			must.NotEq(t, old[0].GroupSelection.Cohort, canary.GroupSelection.Cohort)
			canary = canary.Copy()
			canary.ClientStatus = structs.AllocClientStatusRunning
			canary.DeploymentStatus.Healthy = &healthy
			must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{canary}))
			must.NoError(t, h.State.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, h.NextIndex(), &structs.ApplyDeploymentPromoteRequest{
				DeploymentPromoteRequest: structs.DeploymentPromoteRequest{DeploymentID: canary.DeploymentID, All: true, PromotedAt: time.Now().UnixNano()},
			}))
			selectionEval(t, h, job)
			allocs = selectionAllocs(t, h, job)
			retained := false
			for _, alloc := range allocs {
				if alloc.ID == canary.ID && alloc.DesiredStatus == structs.AllocDesiredStatusRun {
					retained = true
				}
			}
			must.True(t, retained)
			for _, alloc := range allocs {
				if alloc.ID == old[0].ID && alloc.Index() == canary.Index() {
					must.Eq(t, structs.AllocDesiredStatusStop, alloc.DesiredStatus)
				}
			}
			for range replicas + 1 {
				for _, alloc := range selectionAllocs(t, h, job) {
					if alloc.TaskGroup == "thor" && alloc.DesiredStatus == structs.AllocDesiredStatusRun {
						copy := alloc.Copy()
						copy.ClientStatus = structs.AllocClientStatusRunning
						if copy.DeploymentStatus == nil {
							copy.DeploymentStatus = &structs.AllocDeploymentStatus{}
						}
						copy.DeploymentStatus.Healthy = &healthy
						must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{copy}))
					}
				}
				selectionEval(t, h, job)
			}
			live := 0
			for _, alloc := range selectionAllocs(t, h, job) {
				if alloc.DesiredStatus == structs.AllocDesiredStatusRun {
					live++
					must.Eq(t, "thor", alloc.TaskGroup)
				}
			}
			must.Eq(t, replicas, live)
			completed, err := h.State.LatestDeploymentByJobID(nil, job.Namespace, job.ID)
			must.NoError(t, err)
			must.Eq(t, structs.DeploymentStatusSuccessful, completed.Status)
		})
	}
}

func TestServiceSched_GroupSelection_CountAndControlChanges(t *testing.T) {
	h := tests.NewHarness(t)
	oldNode := selectionNode(t, h, "orin")
	selectionNode(t, h, "thor")
	job := selectionJob(1)
	for _, tg := range job.TaskGroups {
		tg.Update = &structs.UpdateStrategy{}
	}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	first := selectionAllocs(t, h, job)[0]
	must.Eq(t, "orin", first.TaskGroup)
	oldNode = oldNode.Copy()
	oldNode.Meta["available"] = "0"
	must.NoError(t, oldNode.ComputeClass())
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), oldNode))
	job = job.Copy()
	job.GroupSelections[0].Count = 2
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	allocs := selectionAllocs(t, h, job)
	must.Len(t, 2, allocs)
	for _, alloc := range allocs {
		must.Eq(t, structs.AllocDesiredStatusRun, alloc.DesiredStatus)
	}
	d, err := h.State.LatestDeploymentByJobID(nil, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Nil(t, d)
	job = job.Copy()
	job.GroupSelections[0].Count = 1
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	allocs = selectionAllocs(t, h, job)
	remaining := 0
	for _, alloc := range allocs {
		if alloc.DesiredStatus == structs.AllocDesiredStatusRun {
			remaining++
			must.Eq(t, first.ID, alloc.ID)
		}
	}
	must.Eq(t, 1, remaining)
	job = job.Copy()
	job.GroupSelections[0].Count = 0
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	for _, alloc := range selectionAllocs(t, h, job) {
		must.Eq(t, structs.AllocDesiredStatusStop, alloc.DesiredStatus)
	}
}

func TestServiceSched_GroupSelection_RequiredAndIndependentGroups(t *testing.T) {
	h := tests.NewHarness(t)
	selectionNode(t, h, "orin")
	selectionNode(t, h, "thor")
	job := selectionJob(1)
	required := job.TaskGroups[2].Copy()
	required.Name = "required"
	required.Count = 2
	left := job.TaskGroups[2].Copy()
	left.Name = "left"
	right := job.TaskGroups[0].Copy()
	right.Name = "right"
	job.TaskGroups = append(job.TaskGroups, required, left, right)
	job.GroupSelections = append(job.GroupSelections, &structs.TaskGroupSelection{Name: "second", Count: 1, Groups: []string{"right", "left"}})
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	counts := make(map[string]int)
	for _, alloc := range selectionAllocs(t, h, job) {
		counts[alloc.TaskGroup]++
	}
	must.Eq(t, map[string]int{"orin": 1, "required": 2, "left": 1}, counts)
	must.Len(t, 0, h.CreateEvals)
}

func TestServiceSched_GroupSelection_LostNode(t *testing.T) {
	h := tests.NewHarness(t)
	oldNode := selectionNode(t, h, "orin")
	selectionNode(t, h, "thor")
	job := selectionJob(1)
	for _, tg := range job.TaskGroups {
		tg.Update = &structs.UpdateStrategy{}
	}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	old := selectionAllocs(t, h, job)[0]
	oldNode = oldNode.Copy()
	oldNode.Status = structs.NodeStatusDown
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), oldNode))
	selectionEval(t, h, job)
	live := 0
	for _, alloc := range selectionAllocs(t, h, job) {
		if alloc.DesiredStatus == structs.AllocDesiredStatusRun {
			live++
			must.Eq(t, "thor", alloc.TaskGroup)
			must.Eq(t, old.ID, alloc.PreviousAllocation)
		}
	}
	must.Eq(t, 1, live)
}

func TestServiceSched_GroupSelection_ReschedulePolicy(t *testing.T) {
	for _, attempts := range []int{0, 1} {
		t.Run(fmt.Sprint(attempts), func(t *testing.T) {
			h := tests.NewHarness(t)
			oldNode := selectionNode(t, h, "orin")
			selectionNode(t, h, "thor")
			job := selectionJob(1)
			for _, tg := range job.TaskGroups {
				tg.Update = &structs.UpdateStrategy{}
				tg.ReschedulePolicy = &structs.ReschedulePolicy{Attempts: attempts, Interval: time.Hour, Delay: time.Minute, DelayFunction: "constant"}
			}
			must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
			selectionEval(t, h, job)
			old := selectionAllocs(t, h, job)[0]
			old.ClientStatus = structs.AllocClientStatusFailed
			old.TaskStates = map[string]*structs.TaskState{job.TaskGroups[1].Tasks[0].Name: {State: structs.TaskStateDead, Failed: true, FinishedAt: time.Now()}}
			must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{old}))
			oldNode = oldNode.Copy()
			oldNode.Meta["available"] = "0"
			must.NoError(t, oldNode.ComputeClass())
			must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), oldNode))
			selectionEval(t, h, job)
			must.Len(t, 1, selectionAllocs(t, h, job))
			if attempts == 0 {
				return
			}
			old, err := h.State.AllocByID(nil, old.ID)
			must.NoError(t, err)
			old = old.Copy()
			old.TaskStates[job.TaskGroups[1].Tasks[0].Name].FinishedAt = time.Now().Add(-2 * time.Minute)
			must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{old}}))
			eval := &structs.Evaluation{ID: old.FollowupEvalID, Namespace: job.Namespace, JobID: job.ID, JobModifyIndex: job.JobModifyIndex, Priority: job.Priority, Type: job.Type, TriggeredBy: structs.EvalTriggerRetryFailedAlloc, Status: structs.EvalStatusPending}
			must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))
			must.NoError(t, h.Process(NewServiceScheduler, eval))
			live := 0
			for _, alloc := range selectionAllocs(t, h, job) {
				if alloc.DesiredStatus == structs.AllocDesiredStatusRun && !alloc.ClientTerminalStatus() {
					live++
					must.Eq(t, "thor", alloc.TaskGroup)
				}
			}
			must.Eq(t, 1, live)
		})
	}
}

func TestServiceSched_GroupSelection_CanaryFailureAndNewRelease(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			h := tests.NewHarness(t)
			selectionNode(t, h, "orin")
			selectionNode(t, h, "thor")
			job := selectionJob(2)
			must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
			selectionEval(t, h, job)
			old := selectionAllocs(t, h, job)
			for _, alloc := range old {
				alloc = alloc.Copy()
				alloc.ClientStatus = structs.AllocClientStatusRunning
				alloc.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: new(true)}
				must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{alloc}}))
			}
			d, err := h.State.LatestDeploymentByJobID(nil, job.Namespace, job.ID)
			must.NoError(t, err)
			d = d.Copy()
			d.Status = structs.DeploymentStatusSuccessful
			must.NoError(t, h.State.UpsertDeployment(h.NextIndex(), d))
			job = job.Copy()
			for _, tg := range job.TaskGroups {
				tg.Tasks[0].Config["command"] = "/bin/new-release"
			}
			job.LookupTaskGroup("orin").Constraints[1].RTarget = "2"
			must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
			selectionEval(t, h, job)
			var firstCanary *structs.Allocation
			for _, alloc := range selectionAllocs(t, h, job) {
				if alloc.DeploymentStatus.IsCanary() {
					firstCanary = alloc
				}
			}
			must.NotNil(t, firstCanary)
			if fail {
				d, err = h.State.LatestDeploymentByJobID(nil, job.Namespace, job.ID)
				must.NoError(t, err)
				d = d.Copy()
				d.Status = structs.DeploymentStatusFailed
				must.NoError(t, h.State.UpsertDeployment(h.NextIndex(), d))
				selectionEval(t, h, job)
				for _, previous := range old {
					alloc, err := h.State.AllocByID(nil, previous.ID)
					must.NoError(t, err)
					must.Eq(t, structs.AllocDesiredStatusRun, alloc.DesiredStatus)
				}
			}
			job = job.Copy()
			for _, tg := range job.TaskGroups {
				tg.Tasks[0].Config["command"] = "/bin/third-release"
			}
			must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
			selectionEval(t, h, job)
			activeCanaries := 0
			for _, alloc := range selectionAllocs(t, h, job) {
				if alloc.ID == firstCanary.ID {
					must.Eq(t, structs.AllocDesiredStatusStop, alloc.DesiredStatus)
				}
				if alloc.DesiredStatus == structs.AllocDesiredStatusRun && alloc.DeploymentStatus.IsCanary() {
					activeCanaries++
					must.Eq(t, job.Version, alloc.Job.Version)
					must.Eq(t, "thor", alloc.TaskGroup)
				}
			}
			must.Eq(t, 1, activeCanaries)
			for _, previous := range old {
				alloc, err := h.State.AllocByID(nil, previous.ID)
				must.NoError(t, err)
				must.Eq(t, structs.AllocDesiredStatusRun, alloc.DesiredStatus)
			}
		})
	}
}

func TestServiceSched_GroupSelection_LostDuringInitialDeployment(t *testing.T) {
	h := tests.NewHarness(t)
	node := selectionNode(t, h, "orin")
	selectionNode(t, h, "thor")
	job := selectionJob(1)
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	node = node.Copy()
	node.Status = structs.NodeStatusDown
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	selectionEval(t, h, job)
	live := 0
	for _, alloc := range selectionAllocs(t, h, job) {
		if alloc.DesiredStatus == structs.AllocDesiredStatusRun {
			live++
			must.Eq(t, "thor", alloc.TaskGroup)
		}
	}
	must.Eq(t, 1, live)
}

func TestServiceSched_GroupSelection_DisconnectDelay(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprint(replace), func(t *testing.T) {
			h := tests.NewHarness(t)
			node := selectionNode(t, h, "orin")
			selectionNode(t, h, "thor")
			job := selectionJob(1)
			for _, tg := range job.TaskGroups {
				tg.Update = &structs.UpdateStrategy{}
				tg.Disconnect = &structs.DisconnectStrategy{LostAfter: time.Hour, Replace: &replace, Reconcile: structs.ReconcileOptionKeepReplacement}
				tg.ReschedulePolicy = &structs.ReschedulePolicy{Unlimited: true, Delay: time.Minute, DelayFunction: "constant"}
			}
			must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
			selectionEval(t, h, job)
			old := selectionAllocs(t, h, job)[0].Copy()
			old.ClientStatus = structs.AllocClientStatusRunning
			must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{old}}))
			node = node.Copy()
			node.Status = structs.NodeStatusDisconnected
			must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
			selectionEval(t, h, job)
			must.Len(t, 1, selectionAllocs(t, h, job))
			old, err := h.State.AllocByID(nil, old.ID)
			must.NoError(t, err)
			must.Eq(t, structs.AllocClientStatusUnknown, old.ClientStatus)
			old = old.Copy()
			for _, state := range old.AllocStates {
				if state.Field == structs.AllocStateFieldClientStatus && state.Value == structs.AllocClientStatusUnknown {
					state.Time = time.Now().Add(-2 * time.Minute)
				}
			}
			must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{old}))
			eval := &structs.Evaluation{ID: old.FollowupEvalID, Namespace: job.Namespace, JobID: job.ID, Priority: job.Priority, TriggeredBy: structs.EvalTriggerRetryFailedAlloc, Status: structs.EvalStatusPending}
			must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))
			must.NoError(t, h.Process(NewServiceScheduler, eval))
			allocs := selectionAllocs(t, h, job)
			if !replace {
				must.Len(t, 1, allocs)
				return
			}
			must.Len(t, 2, allocs)
			for _, alloc := range allocs {
				if alloc.ID != old.ID {
					must.Eq(t, "thor", alloc.TaskGroup)
					must.Eq(t, old.ID, alloc.PreviousAllocation)
				}
			}
		})
	}
}

func TestServiceSched_GroupSelection_ReconnectWholeCohort(t *testing.T) {
	for _, strategy := range []string{structs.ReconcileOptionKeepOriginal, structs.ReconcileOptionKeepReplacement} {
		t.Run(strategy, func(t *testing.T) {
			h := tests.NewHarness(t)
			node := selectionNode(t, h, "orin")
			selectionNode(t, h, "thor")
			job := selectionJob(2)
			for _, tg := range job.TaskGroups {
				tg.Update = &structs.UpdateStrategy{}
				tg.Disconnect = &structs.DisconnectStrategy{LostAfter: time.Hour, Replace: new(true), Reconcile: strategy}
				tg.ReschedulePolicy = &structs.ReschedulePolicy{Unlimited: true, Delay: time.Second, DelayFunction: "constant"}
			}
			must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
			selectionEval(t, h, job)
			for _, alloc := range selectionAllocs(t, h, job) {
				alloc = alloc.Copy()
				alloc.ClientStatus = structs.AllocClientStatusRunning
				must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{alloc}}))
			}
			node = node.Copy()
			node.Status = structs.NodeStatusDisconnected
			must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
			selectionEval(t, h, job)
			allocs := selectionAllocs(t, h, job)
			must.Len(t, 4, allocs)
			for _, alloc := range allocs {
				alloc = alloc.Copy()
				alloc.ClientStatus = structs.AllocClientStatusRunning
				must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{alloc}}))
			}
			node = node.Copy()
			node.Status = structs.NodeStatusReady
			must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
			selectionEval(t, h, job)
			expected := "orin"
			if strategy == structs.ReconcileOptionKeepReplacement {
				expected = "thor"
			}
			live := 0
			for _, alloc := range selectionAllocs(t, h, job) {
				if alloc.DesiredStatus == structs.AllocDesiredStatusRun {
					live++
					must.Eq(t, expected, alloc.TaskGroup)
				}
			}
			must.Eq(t, 2, live)
		})
	}
}

func TestServiceSched_GroupSelection_ScaleDownCanary(t *testing.T) {
	h := tests.NewHarness(t)
	selectionNode(t, h, "encoder")
	selectionNode(t, h, "orin")
	selectionNode(t, h, "thor")
	job := selectionJob(1)
	job.GroupSelections[0].Count = 2
	for _, group := range job.TaskGroups {
		group.ReschedulePolicy = &structs.ReschedulePolicy{Unlimited: true, Delay: time.Second, DelayFunction: "constant"}
	}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	old := selectionAllocs(t, h, job)
	must.Len(t, 2, old)
	for _, alloc := range old {
		alloc = alloc.Copy()
		alloc.ClientStatus = structs.AllocClientStatusRunning
		alloc.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: new(true)}
		must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{alloc}}))
	}
	deployment, err := h.State.LatestDeploymentByJobID(nil, job.Namespace, job.ID)
	must.NoError(t, err)
	deployment = deployment.Copy()
	deployment.Status = structs.DeploymentStatusSuccessful
	must.NoError(t, h.State.UpsertDeployment(h.NextIndex(), deployment))
	job = job.Copy()
	job.GroupSelections[0].Count = 1
	for _, tg := range job.TaskGroups {
		tg.Tasks[0].Config["command"] = "/bin/new-release"
		if tg.Name != "thor" {
			tg.Constraints[1].RTarget = "2"
		}
	}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	selectionEval(t, h, job)
	var canary *structs.Allocation
	for _, alloc := range selectionAllocs(t, h, job) {
		must.Eq(t, structs.AllocDesiredStatusRun, alloc.DesiredStatus)
		if alloc.DeploymentStatus.IsCanary() {
			canary = alloc.Copy()
		}
	}
	must.NotNil(t, canary)
	deployment, err = h.State.LatestDeploymentByJobID(nil, job.Namespace, job.ID)
	must.NoError(t, err)
	state := deployment.GroupSelections["runtime"]
	must.Eq(t, 1, state.Count)
	must.NotNil(t, state.Slots[1])
	must.Eq(t, "", state.Slots[1].TaskGroup)
	must.NotEq(t, "", state.Slots[1].PreviousCohort)
	// A soon-to-be-retired serving slot still recovers using its original
	// profile and job version while the surviving target is unpromoted.
	var failed *structs.Allocation
	for _, allocation := range old {
		if allocation.GroupSelection.Slot == 1 {
			failed = allocation.Copy()
		}
	}
	must.NotNil(t, failed)
	failed.ClientStatus = structs.AllocClientStatusFailed
	failed.TaskStates = map[string]*structs.TaskState{failed.Job.LookupTaskGroup(failed.TaskGroup).Tasks[0].Name: {State: structs.TaskStateDead, Failed: true, FinishedAt: time.Now().Add(-time.Minute)}}
	must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{failed}}))
	selectionEval(t, h, job)
	var repair *structs.Allocation
	for _, allocation := range selectionAllocs(t, h, job) {
		if allocation.PreviousAllocation == failed.ID {
			repair = allocation
		}
	}
	must.NotNil(t, repair)
	must.Eq(t, failed.TaskGroup, repair.TaskGroup)
	must.Eq(t, failed.Job.Version, repair.Job.Version)
	must.Eq(t, failed.GroupSelection, repair.GroupSelection)
	canary.ClientStatus = structs.AllocClientStatusRunning
	canary.DeploymentStatus.Healthy = new(true)
	must.NoError(t, h.State.UpdateAllocsFromClient(structs.MsgTypeTestSetup, h.NextIndex(), structs.AllocUpdateRequest{Alloc: []*structs.Allocation{canary}}))
	must.NoError(t, h.State.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, h.NextIndex(), &structs.ApplyDeploymentPromoteRequest{DeploymentPromoteRequest: structs.DeploymentPromoteRequest{DeploymentID: canary.DeploymentID, All: true, PromotedAt: time.Now().UnixNano()}}))
	selectionEval(t, h, job)
	live := 0
	for _, alloc := range selectionAllocs(t, h, job) {
		if alloc.DesiredStatus == structs.AllocDesiredStatusRun {
			live++
			must.Eq(t, canary.ID, alloc.ID)
		}
	}
	must.Eq(t, 1, live)
}

func TestServiceSched_GroupSelection_ConstraintOnlyUpdate(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(fmt.Sprint(invalid), func(t *testing.T) {
			h := tests.NewHarness(t)
			selectionNode(t, h, "orin")
			selectionNode(t, h, "thor")
			job := selectionJob(1)
			for _, group := range job.TaskGroups {
				group.Update = &structs.UpdateStrategy{}
			}
			must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
			selectionEval(t, h, job)
			old := selectionAllocs(t, h, job)[0]
			job = job.Copy()
			desired := "1"
			if invalid {
				desired = "2"
			}
			job.LookupTaskGroup("orin").Constraints = append(job.LookupTaskGroup("orin").Constraints, &structs.Constraint{LTarget: "${meta.available}", Operand: "=", RTarget: desired})
			must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
			selectionEval(t, h, job)
			live := 0
			for _, alloc := range selectionAllocs(t, h, job) {
				if alloc.DesiredStatus == structs.AllocDesiredStatusRun {
					live++
					if invalid {
						must.Eq(t, "thor", alloc.TaskGroup)
					} else {
						must.Eq(t, old.ID, alloc.ID)
						must.Eq(t, old.GroupSelection, alloc.GroupSelection)
					}
				}
			}
			must.Eq(t, 1, live)
		})
	}
}
