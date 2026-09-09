// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestFSM_SnapshotRestore_GroupSelectionCanary(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	s := fsm.State()
	j := mock.Job()
	j.TaskGroups[0].Count = 2
	alternative := j.TaskGroups[0].Copy()
	alternative.Name, alternative.Count = "alternative", 3
	j.TaskGroups = append(j.TaskGroups, alternative)
	j.GroupSelections = []*structs.TaskGroupSelection{{Name: "runtime", Count: 1, Groups: []string{"web", "alternative"}}}
	must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 100, nil, j))

	d := structs.NewDeployment(j, 50, time.Now().UnixNano())
	d.GroupSelections["runtime"].Slots[0] = &structs.DeploymentGroupSelectionSlot{
		TaskGroup: "alternative", Cohort: "target", PreviousTaskGroup: "web", PreviousCohort: "previous",
	}
	previous, target := mock.Alloc(), mock.Alloc()
	for _, allocation := range []*structs.Allocation{previous, target} {
		allocation.Job, allocation.JobID = j, j.ID
		allocation.ClientStatus = structs.AllocClientStatusPending
		allocation.GroupSelection = &structs.AllocationGroupSelection{Name: "runtime", Slot: 0, Cohort: "previous"}
	}
	target.TaskGroup, target.Name = "alternative", structs.AllocName(j.ID, "alternative", 0)
	target.GroupSelection.Cohort = "target"
	target.DeploymentID = d.ID
	target.DeploymentStatus = &structs.AllocDeploymentStatus{Canary: true}
	d.TaskGroups["alternative"] = &structs.DeploymentState{
		DesiredTotal: 3, DesiredCanaries: 1, PlacedCanaries: []string{target.ID},
	}
	must.NoError(t, s.UpsertDeployment(101, d))
	must.NoError(t, s.UpsertAllocs(structs.MsgTypeTestSetup, 102, []*structs.Allocation{previous, target}))
	target = target.Copy()
	target.ClientStatus = structs.AllocClientStatusRunning
	target.DeploymentStatus.Healthy = new(true)
	must.NoError(t, s.UpsertAllocs(structs.MsgTypeTestSetup, 103, []*structs.Allocation{target}))

	restored := testSnapshotRestore(t, fsm).State()
	job, err := restored.JobByID(nil, j.Namespace, j.ID)
	must.NoError(t, err)
	must.Eq(t, j.GroupSelections, job.GroupSelections)
	must.Eq(t, 2, job.LookupTaskGroup("web").Count)
	must.Eq(t, 3, job.LookupTaskGroup("alternative").Count)
	deployment, err := restored.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Eq(t, d.GroupSelections, deployment.GroupSelections)
	must.Eq(t, 1, deployment.TaskGroups["alternative"].HealthyAllocs)

	// Promotion after recovery retains the tested allocation and still knows
	// which cohort must retire. It must not turn the previous job's replicas
	// into targets or erase the original jobspec counts.
	must.NoError(t, restored.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, 104,
		&structs.ApplyDeploymentPromoteRequest{DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID, All: true, PromotedAt: time.Now().UnixNano(),
		}}))
	promoted, err := restored.AllocByID(nil, target.ID)
	must.NoError(t, err)
	must.False(t, promoted.DeploymentStatus.Canary)
	must.Eq(t, target.GroupSelection, promoted.GroupSelection)
	must.Eq(t, "", promoted.PreviousAllocation)
	old, err := restored.AllocByID(nil, previous.ID)
	must.NoError(t, err)
	must.Eq(t, previous.GroupSelection, old.GroupSelection)
	must.Eq(t, structs.AllocDesiredStatusRun, old.DesiredStatus)
	must.NoError(t, restored.ReconcileJobSummaries(105))
	deployment, err = restored.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Eq(t, "previous", deployment.GroupSelections["runtime"].Slots[0].PreviousCohort)
}
