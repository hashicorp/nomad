// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestStateStore_GroupSelectionPromotion(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStore(t)
	j := mock.Job()
	j.TaskGroups[0].Count = 1
	j.GroupSelections = []*structs.TaskGroupSelection{{Name: "runtime", Count: 1, Groups: []string{"web"}}}
	must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 100, nil, j))

	d := structs.NewDeployment(j, 50, time.Now().UnixNano())
	d.GroupSelections["runtime"].Slots[0] = &structs.DeploymentGroupSelectionSlot{
		TaskGroup: "web", Cohort: "new", PreviousTaskGroup: "web", PreviousCohort: "old",
	}
	old, target := mock.Alloc(), mock.Alloc()
	for _, a := range []*structs.Allocation{old, target} {
		a.Job, a.JobID, a.DeploymentID = j, j.ID, d.ID
		a.ClientStatus = structs.AllocClientStatusPending
		a.DeploymentStatus = &structs.AllocDeploymentStatus{Canary: true}
		a.GroupSelection = &structs.AllocationGroupSelection{Name: "runtime", Slot: 0, Cohort: "new"}
	}
	old.GroupSelection.Cohort = "old"
	d.TaskGroups["web"] = &structs.DeploymentState{
		DesiredTotal: 1, DesiredCanaries: 1, PlacedCanaries: []string{old.ID, target.ID},
	}
	must.NoError(t, s.UpsertDeployment(101, d))
	must.NoError(t, s.UpsertAllocs(structs.MsgTypeTestSetup, 102, []*structs.Allocation{old, target}))

	old = old.Copy()
	old.ClientStatus = structs.AllocClientStatusRunning
	old.DeploymentStatus.Healthy = new(true)
	must.NoError(t, s.UpsertAllocs(structs.MsgTypeTestSetup, 103, []*structs.Allocation{old}))
	stored, err := s.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Eq(t, 1, stored.TaskGroups["web"].PlacedAllocs)
	must.Eq(t, 0, stored.TaskGroups["web"].HealthyAllocs)

	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID, All: true, PromotedAt: time.Now().UnixNano(),
		},
	}
	must.Error(t, s.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, 104, req))

	target = target.Copy()
	target.ClientStatus = structs.AllocClientStatusRunning
	target.DeploymentStatus.Healthy = new(true)
	must.NoError(t, s.UpsertAllocs(structs.MsgTypeTestSetup, 105, []*structs.Allocation{target}))
	must.NoError(t, s.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, 106, req))
	stored, err = s.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.True(t, stored.TaskGroups["web"].Promoted)
	must.Eq(t, 1, stored.TaskGroups["web"].HealthyAllocs)
	must.Eq(t, "old", stored.GroupSelections["runtime"].Slots[0].PreviousCohort)

	promoted, err := s.AllocByID(nil, target.ID)
	must.NoError(t, err)
	must.False(t, promoted.DeploymentStatus.Canary)
	must.Eq(t, "new", promoted.GroupSelection.Cohort)
	previous, err := s.AllocByID(nil, old.ID)
	must.NoError(t, err)
	must.True(t, previous.DeploymentStatus.Canary)
	must.Eq(t, structs.AllocDesiredStatusRun, previous.DesiredStatus)
}

func TestStateStore_GroupSelectionUnplaced(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStore(t)
	j := mock.Job()
	j.GroupSelections = []*structs.TaskGroupSelection{{Name: "runtime", Count: 1, Groups: []string{"web"}}}
	must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 100, nil, j))
	d := structs.NewDeployment(j, 50, time.Now().UnixNano())
	must.NoError(t, s.UpsertDeployment(101, d))

	must.ErrorContains(t, s.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, 102,
		&structs.ApplyDeploymentPromoteRequest{
			DeploymentPromoteRequest: structs.DeploymentPromoteRequest{DeploymentID: d.ID, All: true},
		}), "group selections are unplaced")

	complete := d.Copy()
	complete.Status = structs.DeploymentStatusSuccessful
	must.ErrorContains(t, s.UpsertDeployment(103, complete), "unplaced group selections")
	stored, err := s.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Eq(t, structs.DeploymentStatusRunning, stored.Status)
	job, err := s.JobByID(nil, j.Namespace, j.ID)
	must.NoError(t, err)
	must.False(t, job.Stable)
}
