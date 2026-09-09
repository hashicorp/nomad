// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package deploymentwatcher

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestDeploymentWatcher_GroupSelectionDemand(t *testing.T) {
	ci.Parallel(t)
	s := state.TestStateStore(t)
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.AutoRevert = true
	j.GroupSelections = []*structs.TaskGroupSelection{{Name: "runtime", Count: 1, Groups: []string{"web"}}}
	must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 100, nil, j))
	d := structs.NewDeployment(j, 50, time.Now().UnixNano())
	must.NoError(t, s.UpsertDeployment(101, d))
	w := &deploymentWatcher{state: s, j: j, d: d, deploymentID: d.ID}

	// An empty TaskGroups map is not a completed deployment when a selection
	// still needs a target. Its deadline is tracked without a fictitious group.
	must.Eq(t, d.GroupSelections["runtime"].RequireProgressBy, w.getDeploymentProgressCutoff(d))
	fail, rollback, err := w.shouldFail()
	must.NoError(t, err)
	must.True(t, fail)
	must.True(t, rollback)

	d = d.Copy()
	d.GroupSelections["runtime"].Slots[0] = &structs.DeploymentGroupSelectionSlot{TaskGroup: "web", Cohort: "target"}
	deadline := time.Now().Add(20 * time.Minute)
	d.TaskGroups["web"] = &structs.DeploymentState{DesiredTotal: 1, RequireProgressBy: deadline}
	must.NoError(t, s.UpsertDeployment(102, d))
	must.Eq(t, deadline, w.getDeploymentProgressCutoff(d))

	d.Status = structs.DeploymentStatusPaused
	must.NoError(t, s.UpsertDeployment(103, d))
	fail, rollback, err = w.shouldFail()
	must.NoError(t, err)
	must.False(t, fail)
	must.False(t, rollback)
}

func TestDeploymentWatcher_GroupSelectionTargetHealth(t *testing.T) {
	ci.Parallel(t)
	s := state.TestStateStore(t)
	j := mock.Job()
	j.GroupSelections = []*structs.TaskGroupSelection{{Name: "runtime", Count: 1, Groups: []string{"web"}}}
	must.NoError(t, s.UpsertJob(structs.MsgTypeTestSetup, 100, nil, j))
	d := structs.NewDeployment(j, 50, time.Now().UnixNano())
	d.GroupSelections["runtime"].Slots[0] = &structs.DeploymentGroupSelectionSlot{
		TaskGroup: "web", Cohort: "new", PreviousTaskGroup: "web", PreviousCohort: "old",
	}
	d.TaskGroups["web"] = &structs.DeploymentState{DesiredTotal: 1, Promoted: true}
	must.NoError(t, s.UpsertDeployment(101, d))
	old := mock.Alloc()
	old.Job, old.JobID, old.DeploymentID = j, j.ID, d.ID
	old.ClientStatus = structs.AllocClientStatusPending
	old.GroupSelection = &structs.AllocationGroupSelection{Name: "runtime", Cohort: "old"}
	must.NoError(t, s.UpsertAllocs(structs.MsgTypeTestSetup, 102, []*structs.Allocation{old}))
	old = old.Copy()
	old.ClientStatus = structs.AllocClientStatusRunning
	old.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: new(true)}
	must.NoError(t, s.UpsertAllocs(structs.MsgTypeTestSetup, 103, []*structs.Allocation{old}))
	w := &deploymentWatcher{state: s, j: j, d: d, deploymentID: d.ID}
	must.False(t, w.doneGroups(d)["web"])

	target := mock.Alloc()
	target.Job, target.JobID, target.DeploymentID = j, j.ID, d.ID
	target.ClientStatus = structs.AllocClientStatusPending
	target.GroupSelection = &structs.AllocationGroupSelection{Name: "runtime", Cohort: "new"}
	must.NoError(t, s.UpsertAllocs(structs.MsgTypeTestSetup, 104, []*structs.Allocation{target}))
	target = target.Copy()
	target.ClientStatus = structs.AllocClientStatusRunning
	target.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: new(true)}
	must.NoError(t, s.UpsertAllocs(structs.MsgTypeTestSetup, 105, []*structs.Allocation{target}))
	must.True(t, w.doneGroups(d)["web"])
}

func TestDeploymentWatcher_GroupSelectionAutoRevert(t *testing.T) {
	ci.Parallel(t)
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.AutoRevert = true
	alternative := j.TaskGroups[0].Copy()
	alternative.Name = "alternative"
	j.TaskGroups = append(j.TaskGroups, alternative)
	j.GroupSelections = []*structs.TaskGroupSelection{{Name: "runtime", Count: 1, Groups: []string{"web", "alternative"}}}
	w := &deploymentWatcher{j: j}
	must.True(t, w.unplacedSelectionAutoRevert("runtime"))
	alternative.Update.AutoRevert = false
	must.False(t, w.unplacedSelectionAutoRevert("runtime"))
	alternative.Count = 0
	must.True(t, w.unplacedSelectionAutoRevert("runtime"))
	must.False(t, w.unplacedSelectionAutoRevert("unknown"))
}
