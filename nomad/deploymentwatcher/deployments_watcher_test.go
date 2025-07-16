// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package deploymentwatcher

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func testDeploymentWatcher(t *testing.T, qps float64, batchDur time.Duration) (*Watcher, *mockBackend) {
	m := newMockBackend(t)
	w := NewDeploymentsWatcher(testlog.HCLogger(t), m, nil, nil, qps, batchDur)
	return w, m
}

func defaultTestDeploymentWatcher(t *testing.T) (*Watcher, *mockBackend) {
	return testDeploymentWatcher(t, LimitStateQueriesPerSecond, CrossDeploymentUpdateBatchDuration)
}

// Tests that the watcher properly watches for deployments and reconciles them
func TestWatcher_WatchDeployments(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create three jobs
	j1, j2, j3 := mock.Job(), mock.Job(), mock.Job()
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, j1))
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, 101, nil, j2))
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, 102, nil, j3))

	// Create three deployments all running
	d1, d2, d3 := mock.Deployment(), mock.Deployment(), mock.Deployment()
	d1.JobID = j1.ID
	d2.JobID = j2.ID
	d3.JobID = j3.ID

	// Upsert the first deployment
	must.NoError(t, m.state.UpsertDeployment(103, d1))

	// Next list 3
	block1 := make(chan time.Time)
	go func() {
		<-block1
		must.NoError(t, m.state.UpsertDeployment(104, d2))
		must.NoError(t, m.state.UpsertDeployment(105, d3))
	}()

	//// Next list 3 but have one be terminal
	block2 := make(chan time.Time)
	d3terminal := d3.Copy()
	d3terminal.Status = structs.DeploymentStatusFailed
	go func() {
		<-block2
		must.NoError(t, m.state.UpsertDeployment(106, d3terminal))
	}()

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	close(block1)
	waitForWatchers(t, w, 3)

	close(block2)
	waitForWatchers(t, w, 2)
}

// Tests that calls against an unknown deployment fail
func TestWatcher_UnknownDeployment(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)
	w.SetEnabled(true, m.state)

	// The expected error is that it should be an unknown deployment
	dID := uuid.Generate()
	expectedErr := fmt.Sprintf("unknown deployment %q", dID)

	// Request setting the health against an unknown deployment
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         dID,
		HealthyAllocationIDs: []string{uuid.Generate()},
	}
	var resp structs.DeploymentUpdateResponse
	err := w.SetAllocHealth(req, &resp)
	must.ErrorContains(t, err, expectedErr)

	// Request promoting against an unknown deployment
	req2 := &structs.DeploymentPromoteRequest{
		DeploymentID: dID,
		All:          true,
	}
	err = w.PromoteDeployment(req2, &resp)
	must.ErrorContains(t, err, expectedErr)

	// Request pausing against an unknown deployment
	req3 := &structs.DeploymentPauseRequest{
		DeploymentID: dID,
		Pause:        true,
	}
	err = w.PauseDeployment(req3, &resp)
	must.ErrorContains(t, err, expectedErr)

	// Request failing against an unknown deployment
	req4 := &structs.DeploymentFailRequest{
		DeploymentID: dID,
	}
	err = w.FailDeployment(req4, &resp)
	must.ErrorContains(t, err, expectedErr)
}

// Test setting an unknown allocation's health
func TestWatcher_SetAllocHealth_Unknown(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job, and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))

	// require that we get a call to UpsertDeploymentAllocHealth
	a := mock.Alloc()

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// manually set an unknown alloc healthy
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         d.ID,
		HealthyAllocationIDs: []string{a.ID},
	}
	var resp structs.DeploymentUpdateResponse
	err := w.SetAllocHealth(req, &resp)
	must.ErrorContains(t, err, "unknown alloc")
	must.Eq(t, 1, watchersCount(w), must.Sprint("watcher should still be active"))
}

// Test setting allocation health
func TestWatcher_SetAllocHealth_Healthy(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	a.DeploymentID = d.ID
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// manually set the alloc healthy
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         d.ID,
		HealthyAllocationIDs: []string{a.ID},
	}
	var resp structs.DeploymentUpdateResponse
	must.NoError(t, w.SetAllocHealth(req, &resp))
	must.Eq(t, 1, watchersCount(w), must.Sprint("watcher should still be active"))

	d, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Eq(t, 1, d.TaskGroups["web"].HealthyAllocs)
	must.Eq(t, 0, d.TaskGroups["web"].UnhealthyAllocs)
}

// Test setting allocation unhealthy
func TestWatcher_SetAllocHealth_Unhealthy(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	a.DeploymentID = d.ID
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// manually set the alloc unhealthy
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:           d.ID,
		UnhealthyAllocationIDs: []string{a.ID},
	}
	var resp structs.DeploymentUpdateResponse
	must.NoError(t, w.SetAllocHealth(req, &resp))

	waitForWatchers(t, w, 0)

	d, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Eq(t, 0, d.TaskGroups["web"].HealthyAllocs)
	must.Eq(t, 1, d.TaskGroups["web"].UnhealthyAllocs)
}

// Test setting allocation unhealthy and that there should be a rollback
func TestWatcher_SetAllocHealth_Unhealthy_Rollback(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.AutoRevert = true
	j.TaskGroups[0].Update.ProgressDeadline = 0
	j.Stable = true
	d := mock.Deployment()
	d.JobID = j.ID
	d.TaskGroups["web"].AutoRevert = true
	a := mock.Alloc()
	a.DeploymentID = d.ID
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}))

	// Upsert the job again to get a new version
	j2 := j.Copy()
	j2.Stable = false
	// Modify the job to make its specification different
	j2.Meta["foo"] = "bar"

	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j2))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// manually set the alloc unhealthy
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:           d.ID,
		UnhealthyAllocationIDs: []string{a.ID},
	}
	var resp structs.DeploymentUpdateResponse
	must.NoError(t, w.SetAllocHealth(req, &resp))

	waitForWatchers(t, w, 0)

	d, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Eq(t, 0, d.TaskGroups["web"].HealthyAllocs)
	must.Eq(t, 1, d.TaskGroups["web"].UnhealthyAllocs)
	must.Eq(t, structs.DeploymentStatusFailed, d.Status)
	must.Eq(t, structs.DeploymentStatusDescriptionRollback(
		structs.DeploymentStatusDescriptionFailedAllocations, 0), d.StatusDescription)

	m.assertCalls(t, "UpdateDeploymentAllocHealth", 1)
}

// Test setting allocation unhealthy on job with identical spec and there should be no rollback
func TestWatcher_SetAllocHealth_Unhealthy_NoRollback(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.AutoRevert = true
	j.TaskGroups[0].Update.ProgressDeadline = 0
	j.Stable = true
	d := mock.Deployment()
	d.JobID = j.ID
	d.TaskGroups["web"].AutoRevert = true
	a := mock.Alloc()
	a.DeploymentID = d.ID
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}))

	// Upsert the job again to get a new version
	j2 := j.Copy()
	j2.Stable = false

	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j2))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// Call SetAllocHealth
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:           d.ID,
		UnhealthyAllocationIDs: []string{a.ID},
	}
	var resp structs.DeploymentUpdateResponse
	must.NoError(t, w.SetAllocHealth(req, &resp))

	waitForWatchers(t, w, 0)
	must.Eq(t, structs.DeploymentStatusRunning, d.Status)
	must.Eq(t, structs.DeploymentStatusDescriptionRunning, d.StatusDescription)

	m.assertCalls(t, "UpdateDeploymentAllocHealth", 1)
}

// Test promoting a deployment
func TestWatcher_PromoteDeployment_HealthyCanaries(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job, canary alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.Canary = 1
	j.TaskGroups[0].Update.ProgressDeadline = 0
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	d.TaskGroups[a.TaskGroup].DesiredCanaries = 1
	d.TaskGroups[a.TaskGroup].PlacedCanaries = []string{a.ID}
	a.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(true),
	}
	a.DeploymentID = d.ID
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// manually promote
	req := &structs.DeploymentPromoteRequest{
		DeploymentID: d.ID,
		All:          true,
	}
	var resp structs.DeploymentUpdateResponse
	must.NoError(t, w.PromoteDeployment(req, &resp))
	must.Eq(t, 1, watchersCount(w), must.Sprint("watcher should still be active"))

	d, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.True(t, d.TaskGroups["web"].Promoted)
}

// Test promoting a deployment with unhealthy canaries
func TestWatcher_PromoteDeployment_UnhealthyCanaries(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job, canary alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.Canary = 2
	j.TaskGroups[0].Update.ProgressDeadline = 0
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	d.TaskGroups[a.TaskGroup].PlacedCanaries = []string{a.ID}
	d.TaskGroups[a.TaskGroup].DesiredCanaries = 2
	a.DeploymentID = d.ID
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// manually promote
	req := &structs.DeploymentPromoteRequest{
		DeploymentID: d.ID,
		All:          true,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PromoteDeployment(req, &resp)
	// 0/2 because the old version has been stopped but the canary isn't marked healthy yet
	must.ErrorContains(t, err, `Task group "web" has 0/2 healthy allocations`,
		must.Sprint("Should error because canary isn't marked healthy"))

	must.Eq(t, 1, watchersCount(w), must.Sprint("watcher should still be active"))

	d, err = m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Eq(t, structs.DeploymentStatusRunning, d.Status)
	must.False(t, d.TaskGroups["web"].Promoted)
}

func TestWatcher_AutoPromoteDeployment(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)
	now := time.Now()

	// Create 1 UpdateStrategy, 1 job (2 TaskGroups), 2 canaries, and 1 deployment
	canaryUpd := structs.DefaultUpdateStrategy.Copy()
	canaryUpd.AutoPromote = true
	canaryUpd.MaxParallel = 2
	canaryUpd.Canary = 2
	canaryUpd.ProgressDeadline = 5 * time.Second

	rollingUpd := structs.DefaultUpdateStrategy.Copy()
	rollingUpd.ProgressDeadline = 5 * time.Second

	j := mock.MultiTaskGroupJob()
	j.TaskGroups[0].Update = canaryUpd
	j.TaskGroups[1].Update = rollingUpd

	d := mock.Deployment()
	d.JobID = j.ID
	// This is created in scheduler.computeGroup at runtime, where properties from the
	// UpdateStrategy are copied in
	d.TaskGroups = map[string]*structs.DeploymentState{
		"web": {
			AutoPromote:      canaryUpd.AutoPromote,
			AutoRevert:       canaryUpd.AutoRevert,
			ProgressDeadline: canaryUpd.ProgressDeadline,
			DesiredTotal:     2,
		},
		"api": {
			AutoPromote:      rollingUpd.AutoPromote,
			AutoRevert:       rollingUpd.AutoRevert,
			ProgressDeadline: rollingUpd.ProgressDeadline,
			DesiredTotal:     2,
		},
	}

	canaryAlloc := func() *structs.Allocation {
		a := mock.Alloc()
		a.DeploymentID = d.ID
		a.CreateTime = now.UnixNano()
		a.ModifyTime = now.UnixNano()
		a.DeploymentStatus = &structs.AllocDeploymentStatus{
			Canary: true,
		}
		return a
	}

	rollingAlloc := func() *structs.Allocation {
		a := mock.Alloc()
		a.DeploymentID = d.ID
		a.CreateTime = now.UnixNano()
		a.ModifyTime = now.UnixNano()
		a.TaskGroup = "api"
		a.AllocatedResources.Tasks["api"] = a.AllocatedResources.Tasks["web"].Copy()
		delete(a.AllocatedResources.Tasks, "web")
		a.TaskResources["api"] = a.TaskResources["web"].Copy()
		delete(a.TaskResources, "web")
		a.DeploymentStatus = &structs.AllocDeploymentStatus{
			Canary: false,
		}
		return a
	}

	// Web taskgroup (0)
	ca1 := canaryAlloc()
	ca2 := canaryAlloc()

	// Api taskgroup (1)
	ra1 := rollingAlloc()
	ra2 := rollingAlloc()

	d.TaskGroups[ca1.TaskGroup].PlacedCanaries = []string{ca1.ID, ca2.ID}
	d.TaskGroups[ca1.TaskGroup].DesiredCanaries = 2
	d.TaskGroups[ra1.TaskGroup].PlacedAllocs = 2
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{ca1, ca2, ra1, ra2}))

	// Start the deployment
	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// Mark the canaries healthy
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         d.ID,
		HealthyAllocationIDs: []string{ca1.ID, ca2.ID, ra1.ID, ra2.ID},
	}
	var resp structs.DeploymentUpdateResponse
	// Calls w.raft.UpdateDeploymentAllocHealth, which is implemented by StateStore in
	// state.UpdateDeploymentAllocHealth via a raft shim?
	must.NoError(t, w.SetAllocHealth(req, &resp))

	// Wait for the promotion
	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(func() error {
		ws := memdb.NewWatchSet()
		ds, err := m.state.DeploymentsByJobID(ws, j.Namespace, j.ID, true)
		if err != nil {
			return err
		}
		d = ds[0]
		if 2 != d.TaskGroups["web"].HealthyAllocs {
			return fmt.Errorf("expected 2 healthy allocs")
		}
		if !d.TaskGroups["web"].Promoted {
			return fmt.Errorf("expected task group to be promoted")
		}
		if d.Status != structs.DeploymentStatusRunning {
			return fmt.Errorf("expected deployment to be running")
		}
		return nil

	}),
		wait.Gap(10*time.Millisecond), wait.Timeout(time.Second)),
		must.Sprint("expected promotion request submitted"))

	must.Eq(t, 1, watchersCount(w), must.Sprint("watcher should still be active"))

	a1, _ := m.state.AllocByID(nil, ca1.ID)
	must.False(t, a1.DeploymentStatus.Canary)
	must.Eq(t, "pending", a1.ClientStatus)
	must.Eq(t, "run", a1.DesiredStatus)

	b1, _ := m.state.AllocByID(nil, ca2.ID)
	must.False(t, b1.DeploymentStatus.Canary)
}

func TestWatcher_AutoPromoteDeployment_UnhealthyCanaries(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)
	now := time.Now()

	// Create 1 UpdateStrategy, 1 job (2 TaskGroups), 2 canaries, and 1 deployment
	canaryUpd := structs.DefaultUpdateStrategy.Copy()
	canaryUpd.AutoPromote = true
	canaryUpd.MaxParallel = 2
	canaryUpd.Canary = 2
	canaryUpd.ProgressDeadline = 5 * time.Second

	j := mock.MultiTaskGroupJob()
	j.TaskGroups[0].Update = canaryUpd

	d := mock.Deployment()
	d.JobID = j.ID
	// This is created in scheduler.computeGroup at runtime, where properties from the
	// UpdateStrategy are copied in
	d.TaskGroups = map[string]*structs.DeploymentState{
		"web": {
			AutoPromote:      canaryUpd.AutoPromote,
			AutoRevert:       canaryUpd.AutoRevert,
			ProgressDeadline: canaryUpd.ProgressDeadline,
			DesiredTotal:     2,
		},
	}

	canaryAlloc := func() *structs.Allocation {
		a := mock.Alloc()
		a.DeploymentID = d.ID
		a.CreateTime = now.UnixNano()
		a.ModifyTime = now.UnixNano()
		a.DeploymentStatus = &structs.AllocDeploymentStatus{
			Canary: true,
		}
		return a
	}

	// Web taskgroup
	ca1 := canaryAlloc()
	ca2 := canaryAlloc()
	ca3 := canaryAlloc()

	d.TaskGroups[ca1.TaskGroup].PlacedCanaries = []string{ca1.ID, ca2.ID, ca3.ID}
	d.TaskGroups[ca1.TaskGroup].DesiredCanaries = 2
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{ca1, ca2, ca3}))

	// Start the deployment
	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// Mark only 2 canaries as healthy
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         d.ID,
		HealthyAllocationIDs: []string{ca1.ID, ca2.ID},
	}
	var resp structs.DeploymentUpdateResponse
	// Calls w.raft.UpdateDeploymentAllocHealth, which is implemented by StateStore in
	// state.UpdateDeploymentAllocHealth via a raft shim?
	must.NoError(t, w.SetAllocHealth(req, &resp))

	// Wait for the promotion
	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(func() error {
		ws := memdb.NewWatchSet()
		ds, _ := m.state.DeploymentsByJobID(ws, j.Namespace, j.ID, true)
		d = ds[0]
		if 2 != d.TaskGroups["web"].HealthyAllocs {
			return fmt.Errorf("expected 2 healthy allocs")
		}
		if !d.TaskGroups["web"].Promoted {
			return fmt.Errorf("expected task group to be promoted")
		}
		if d.Status != structs.DeploymentStatusRunning {
			return fmt.Errorf("expected deployment to be running")
		}
		return nil

	}),
		wait.Gap(10*time.Millisecond), wait.Timeout(time.Second)),
		must.Sprint("expected promotion request submitted"))

	must.Eq(t, 1, watchersCount(w), must.Sprint("watcher should still be active"))

	a1, _ := m.state.AllocByID(nil, ca1.ID)
	must.False(t, a1.DeploymentStatus.Canary)
	must.Eq(t, "pending", a1.ClientStatus)
	must.Eq(t, "run", a1.DesiredStatus)

	b1, _ := m.state.AllocByID(nil, ca2.ID)
	must.False(t, b1.DeploymentStatus.Canary)
}

// Test pausing a deployment that is running
func TestWatcher_PauseDeployment_Pause_Running(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// manually pause
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        true,
	}
	var resp structs.DeploymentUpdateResponse
	must.NoError(t, w.PauseDeployment(req, &resp))

	must.Eq(t, 1, watchersCount(w), must.Sprint("watcher should still be active"))

	d, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Eq(t, structs.DeploymentStatusPaused, d.Status)
	must.Eq(t, structs.DeploymentStatusDescriptionPaused, d.StatusDescription)
}

// Test pausing a deployment that is paused
func TestWatcher_PauseDeployment_Pause_Paused(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	d.Status = structs.DeploymentStatusPaused
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// manually pause
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        true,
	}
	var resp structs.DeploymentUpdateResponse
	must.NoError(t, w.PauseDeployment(req, &resp))

	must.Eq(t, 1, watchersCount(w), must.Sprint("watcher should still be active"))

	d, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Eq(t, structs.DeploymentStatusPaused, d.Status)
	must.Eq(t, structs.DeploymentStatusDescriptionPaused, d.StatusDescription)
}

// Test unpausing a deployment that is paused
func TestWatcher_PauseDeployment_Unpause_Paused(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	d.Status = structs.DeploymentStatusPaused
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// manually unpause
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        false,
	}
	var resp structs.DeploymentUpdateResponse
	must.NoError(t, w.PauseDeployment(req, &resp))

	must.Eq(t, 1, watchersCount(w), must.Sprint("watcher should still be active"))

	d, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Eq(t, structs.DeploymentStatusRunning, d.Status)
	must.Eq(t, structs.DeploymentStatusDescriptionRunning, d.StatusDescription)
}

// Test unpausing a deployment that is running
func TestWatcher_PauseDeployment_Unpause_Running(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// manually unpause the deployment
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        false,
	}
	var resp structs.DeploymentUpdateResponse
	must.NoError(t, w.PauseDeployment(req, &resp))

	must.Eq(t, 1, watchersCount(w), must.Sprint("watcher should still be active"))

	d, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Eq(t, structs.DeploymentStatusRunning, d.Status)
	must.Eq(t, structs.DeploymentStatusDescriptionRunning, d.StatusDescription)
}

// Test failing a deployment that is running
func TestWatcher_FailDeployment_Running(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// manually fail the deployment
	req := &structs.DeploymentFailRequest{
		DeploymentID: d.ID,
	}
	var resp structs.DeploymentUpdateResponse
	must.NoError(t, w.FailDeployment(req, &resp))

	must.Eq(t, 1, watchersCount(w), must.Sprint("watcher should still be active"))

	d, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Eq(t, structs.DeploymentStatusFailed, d.Status)
	must.Eq(t, structs.DeploymentStatusDescriptionFailedByUser, d.StatusDescription)
}

// Tests that the watcher properly watches for allocation changes and takes the
// proper actions
func TestDeploymentWatcher_Watch_NoProgressDeadline(t *testing.T) {
	ci.Parallel(t)
	w, m := testDeploymentWatcher(t, 1000.0, 1*time.Millisecond)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.AutoRevert = true
	j.TaskGroups[0].Update.ProgressDeadline = 0
	j.Stable = true
	d := mock.Deployment()
	d.JobID = j.ID
	d.TaskGroups["web"].AutoRevert = true
	a := mock.Alloc()
	a.DeploymentID = d.ID
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}))

	// Upsert the job again to get a new version
	j2 := j.Copy()
	// Modify the job to make its specification different
	j2.Meta["foo"] = "bar"
	j2.Stable = false
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j2))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// Update the allocs health to healthy which should create an evaluation
	for range 5 {
		req := &structs.ApplyDeploymentAllocHealthRequest{
			DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
				DeploymentID:         d.ID,
				HealthyAllocationIDs: []string{a.ID},
			},
		}
		must.NoError(t, m.state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, m.nextIndex(), req))
	}

	waitForEvals(t, m.state, j, 1)

	// Update the allocs health to unhealthy which should create a job rollback,
	// status update and eval
	req2 := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:           d.ID,
			UnhealthyAllocationIDs: []string{a.ID},
		},
	}
	must.NoError(t, m.state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, m.nextIndex(), req2))
	waitForEvals(t, m.state, j, 1)

	// Wait for the deployment to be failed
	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(func() error {
		d, _ := m.state.DeploymentByID(nil, d.ID)
		if d.Status != structs.DeploymentStatusFailed {
			return fmt.Errorf("bad status %q", d.Status)
		}
		if !strings.Contains(d.StatusDescription,
			structs.DeploymentStatusDescriptionFailedAllocations) {
			return fmt.Errorf("bad status description %q", d.StatusDescription)
		}
		return nil
	}),
		wait.Gap(10*time.Millisecond), wait.Timeout(time.Second)),
		must.Sprint("expected deployment to be failed"))

	waitForWatchers(t, w, 0)
	// verify that the watcher is batching allocation changes
	m.assertCalls(t, "UpdateAllocDesiredTransition", 1)

	d, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Eq(t, structs.DeploymentStatusFailed, d.Status)
	must.Eq(t, structs.DeploymentStatusDescriptionRollback(
		structs.DeploymentStatusDescriptionFailedAllocations, 0), d.StatusDescription)
}

func TestDeploymentWatcher_Watch_ProgressDeadline(t *testing.T) {
	ci.Parallel(t)
	w, m := testDeploymentWatcher(t, 1000.0, 1*time.Millisecond)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.ProgressDeadline = 500 * time.Millisecond
	j.Stable = true
	d := mock.Deployment()
	d.JobID = j.ID
	d.TaskGroups["web"].ProgressDeadline = 500 * time.Millisecond
	a := mock.Alloc()
	now := time.Now()
	a.CreateTime = now.UnixNano()
	a.ModifyTime = now.UnixNano()
	a.DeploymentID = d.ID
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// Update the alloc to be unhealthy and require that nothing happens.
	a2 := a.Copy()
	a2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy:   pointer.Of(false),
		Timestamp: now,
	}
	must.NoError(t, m.state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 100, []*structs.Allocation{a2}))

	// Wait for the deployment to be failed
	must.Wait(t, wait.InitialSuccess(wait.BoolFunc(func() bool {
		d, _ := m.state.DeploymentByID(nil, d.ID)
		return d.Status == structs.DeploymentStatusFailed &&
			d.StatusDescription == structs.DeploymentStatusDescriptionProgressDeadline
	}),
		wait.Gap(10*time.Millisecond), wait.Timeout(time.Second)),
		must.Sprint("expected deployment to be failed"))

	waitForEvals(t, m.state, j, 1)
}

// Test that progress deadline handling works when there are multiple groups
func TestDeploymentWatcher_ProgressCutoff(t *testing.T) {
	ci.Parallel(t)
	w, m := testDeploymentWatcher(t, 1000.0, 1*time.Millisecond)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Count = 1
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.ProgressDeadline = 500 * time.Millisecond
	j.TaskGroups = append(j.TaskGroups, j.TaskGroups[0].Copy())
	j.TaskGroups[1].Name = "foo"
	j.TaskGroups[1].Update.ProgressDeadline = 1 * time.Second
	j.Stable = true

	d := mock.Deployment()
	d.JobID = j.ID
	d.TaskGroups["web"].DesiredTotal = 1
	d.TaskGroups["foo"] = d.TaskGroups["web"].Copy()
	d.TaskGroups["web"].ProgressDeadline = 500 * time.Millisecond
	d.TaskGroups["foo"].ProgressDeadline = 1 * time.Second

	a := mock.Alloc()
	now := time.Now()
	a.CreateTime = now.UnixNano()
	a.ModifyTime = now.UnixNano()
	a.DeploymentID = d.ID

	a2 := mock.Alloc()
	a2.TaskGroup = "foo"
	a2.CreateTime = now.UnixNano()
	a2.ModifyTime = now.UnixNano()
	a2.DeploymentID = d.ID

	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a, a2}))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	watcher, err := w.getOrCreateWatcher(d.ID)
	must.NoError(t, err)
	must.NotNil(t, watcher)

	d1, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)

	done := watcher.doneGroups(d1)
	must.MapContainsKey(t, done, "web")
	must.False(t, done["web"])
	must.MapContainsKey(t, done, "foo")
	must.False(t, done["foo"])

	cutoff1 := watcher.getDeploymentProgressCutoff(d1)
	must.False(t, cutoff1.IsZero())

	// Update the first allocation to be healthy
	a3 := a.Copy()
	a3.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: pointer.Of(true)}
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a3}))

	// Get the updated deployment
	d2, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)

	done = watcher.doneGroups(d2)
	must.MapContainsKey(t, done, "web")
	must.True(t, done["web"])
	must.MapContainsKey(t, done, "foo")
	must.False(t, done["foo"])

	cutoff2 := watcher.getDeploymentProgressCutoff(d2)
	must.False(t, cutoff2.IsZero())
	must.True(t, cutoff1.UnixNano() < cutoff2.UnixNano())

	// Update the second allocation to be healthy
	a4 := a2.Copy()
	a4.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: pointer.Of(true)}
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a4}))

	// Get the updated deployment
	d3, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)

	done = watcher.doneGroups(d3)
	must.MapContainsKey(t, done, "web")
	must.True(t, done["web"])
	must.MapContainsKey(t, done, "foo")
	must.True(t, done["foo"])

	cutoff3 := watcher.getDeploymentProgressCutoff(d2)
	must.True(t, cutoff3.IsZero())
}

// Test that we will allow the progress deadline to be reached when the canaries
// are healthy but we haven't promoted
func TestDeploymentWatcher_Watch_ProgressDeadline_Canaries(t *testing.T) {
	ci.Parallel(t)
	w, m := testDeploymentWatcher(t, 1000.0, 1*time.Millisecond)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.Canary = 1
	j.TaskGroups[0].Update.MaxParallel = 1
	j.TaskGroups[0].Update.ProgressDeadline = 500 * time.Millisecond
	j.Stable = true
	d := mock.Deployment()
	d.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
	d.JobID = j.ID
	d.TaskGroups["web"].ProgressDeadline = 500 * time.Millisecond
	d.TaskGroups["web"].DesiredCanaries = 1
	a := mock.Alloc()
	now := time.Now()
	a.CreateTime = now.UnixNano()
	a.ModifyTime = now.UnixNano()
	a.DeploymentID = d.ID
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// Update the alloc to be unhealthy and require that nothing happens.
	a2 := a.Copy()
	a2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy:   pointer.Of(true),
		Timestamp: now,
	}
	must.NoError(t, m.state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a2}))

	// Wait for the deployment to cross the deadline
	dout, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.NotNil(t, dout)
	state := dout.TaskGroups["web"]
	must.NotNil(t, state)
	time.Sleep(state.RequireProgressBy.Add(time.Second).Sub(now))

	// Require the deployment is still running
	dout, err = m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.NotNil(t, dout)
	must.Eq(t, structs.DeploymentStatusRunning, dout.Status)
	must.Eq(t, structs.DeploymentStatusDescriptionRunningNeedsPromotion, dout.StatusDescription)

	waitForEvals(t, m.state, j, 1)
}

// Test that a promoted deployment with alloc healthy updates create
// evals to move the deployment forward
func TestDeploymentWatcher_PromotedCanary_UpdatedAllocs(t *testing.T) {
	ci.Parallel(t)
	w, m := testDeploymentWatcher(t, 1000.0, 1*time.Millisecond)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Count = 2
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.Canary = 1
	j.TaskGroups[0].Update.MaxParallel = 1
	j.TaskGroups[0].Update.ProgressDeadline = 50 * time.Millisecond
	j.Stable = true

	d := mock.Deployment()
	d.TaskGroups["web"].DesiredTotal = 2
	d.TaskGroups["web"].DesiredCanaries = 1
	d.TaskGroups["web"].HealthyAllocs = 1
	d.StatusDescription = structs.DeploymentStatusDescriptionRunning
	d.JobID = j.ID
	d.TaskGroups["web"].ProgressDeadline = 50 * time.Millisecond
	d.TaskGroups["web"].RequireProgressBy = time.Now().Add(50 * time.Millisecond)

	a := mock.Alloc()
	now := time.Now()
	a.CreateTime = now.UnixNano()
	a.ModifyTime = now.UnixNano()
	a.DeploymentID = d.ID
	a.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy:   pointer.Of(true),
		Timestamp: now,
	}
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// Create another alloc
	a2 := a.Copy()
	a2.ID = uuid.Generate()
	now = time.Now()
	a2.CreateTime = now.UnixNano()
	a2.ModifyTime = now.UnixNano()
	a2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy:   pointer.Of(true),
		Timestamp: now,
	}
	d.TaskGroups["web"].RequireProgressBy = time.Now().Add(2 * time.Second)
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))
	// Wait until batch eval period passes before updating another alloc
	time.Sleep(1 * time.Second)
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a2}))

	// Wait for the deployment to cross the deadline
	dout, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.NotNil(t, dout)
	state := dout.TaskGroups["web"]
	must.NotNil(t, state)
	time.Sleep(state.RequireProgressBy.Add(time.Second).Sub(now))

	waitForEvals(t, m.state, j, 2)
}

func TestDeploymentWatcher_ProgressDeadline_LatePromote(t *testing.T) {
	ci.Parallel(t)
	mtype := structs.MsgTypeTestSetup

	w, m := defaultTestDeploymentWatcher(t)
	w.SetEnabled(true, m.state)

	progressTimeout := time.Millisecond * 1000
	j := mock.Job()
	j.TaskGroups[0].Name = "group1"
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.AutoRevert = false
	j.TaskGroups[0].Update.ProgressDeadline = progressTimeout
	j.TaskGroups = append(j.TaskGroups, j.TaskGroups[0].Copy())
	j.TaskGroups[0].Name = "group2"

	d := mock.Deployment()
	d.JobID = j.ID
	d.TaskGroups = map[string]*structs.DeploymentState{
		"group1": {
			ProgressDeadline: progressTimeout,
			Promoted:         false,
			PlacedCanaries:   []string{},
			DesiredCanaries:  1,
			DesiredTotal:     3,
			PlacedAllocs:     0,
			HealthyAllocs:    0,
			UnhealthyAllocs:  0,
		},
		"group2": {
			ProgressDeadline: progressTimeout,
			Promoted:         false,
			PlacedCanaries:   []string{},
			DesiredCanaries:  1,
			DesiredTotal:     1,
			PlacedAllocs:     0,
			HealthyAllocs:    0,
			UnhealthyAllocs:  0,
		},
	}

	must.NoError(t, m.state.UpsertJob(mtype, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))

	// create canaries

	now := time.Now()

	canary1 := mock.Alloc()
	canary1.Job = j
	canary1.DeploymentID = d.ID
	canary1.TaskGroup = "group1"
	canary1.DesiredStatus = structs.AllocDesiredStatusRun
	canary1.ModifyTime = now.UnixNano()

	canary2 := mock.Alloc()
	canary2.Job = j
	canary2.DeploymentID = d.ID
	canary2.TaskGroup = "group2"
	canary2.DesiredStatus = structs.AllocDesiredStatusRun
	canary2.ModifyTime = now.UnixNano()

	allocs := []*structs.Allocation{canary1, canary2}
	must.NoError(t, m.state.UpsertAllocs(mtype, m.nextIndex(), allocs))

	// 2nd group's canary becomes healthy

	now = time.Now()

	canary2 = canary2.Copy()
	canary2.ModifyTime = now.UnixNano()
	canary2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Canary:    true,
		Healthy:   pointer.Of(true),
		Timestamp: now,
	}

	allocs = []*structs.Allocation{canary2}
	err := m.state.UpdateAllocsFromClient(mtype, m.nextIndex(), allocs)
	must.NoError(t, err)

	// wait for long enough to ensure we read deployment update channel
	// this sleep creates the race condition associated with #7058
	time.Sleep(50 * time.Millisecond)

	// 1st group's canary becomes healthy
	now = time.Now()

	canary1 = canary1.Copy()
	canary1.ModifyTime = now.UnixNano()
	canary1.DeploymentStatus = &structs.AllocDeploymentStatus{
		Canary:    true,
		Healthy:   pointer.Of(true),
		Timestamp: now,
	}

	allocs = []*structs.Allocation{canary1}
	err = m.state.UpdateAllocsFromClient(mtype, m.nextIndex(), allocs)
	must.NoError(t, err)

	// ensure progress_deadline has definitely expired
	time.Sleep(progressTimeout)

	// promote the deployment

	req := &structs.DeploymentPromoteRequest{
		DeploymentID: d.ID,
		All:          true,
	}
	err = w.PromoteDeployment(req, &structs.DeploymentUpdateResponse{})
	must.NoError(t, err)

	// wait for long enough to ensure we read deployment update channel
	time.Sleep(50 * time.Millisecond)

	// create new allocs for promoted deployment
	// (these come from plan_apply, not a client update)
	now = time.Now()

	alloc1a := mock.Alloc()
	alloc1a.Job = j
	alloc1a.DeploymentID = d.ID
	alloc1a.TaskGroup = "group1"
	alloc1a.ClientStatus = structs.AllocClientStatusPending
	alloc1a.DesiredStatus = structs.AllocDesiredStatusRun
	alloc1a.ModifyTime = now.UnixNano()

	alloc1b := mock.Alloc()
	alloc1b.Job = j
	alloc1b.DeploymentID = d.ID
	alloc1b.TaskGroup = "group1"
	alloc1b.ClientStatus = structs.AllocClientStatusPending
	alloc1b.DesiredStatus = structs.AllocDesiredStatusRun
	alloc1b.ModifyTime = now.UnixNano()

	allocs = []*structs.Allocation{alloc1a, alloc1b}
	err = m.state.UpsertAllocs(mtype, m.nextIndex(), allocs)
	must.NoError(t, err)

	// allocs become healthy

	now = time.Now()

	alloc1a = alloc1a.Copy()
	alloc1a.ClientStatus = structs.AllocClientStatusRunning
	alloc1a.ModifyTime = now.UnixNano()
	alloc1a.DeploymentStatus = &structs.AllocDeploymentStatus{
		Canary:    false,
		Healthy:   pointer.Of(true),
		Timestamp: now,
	}

	alloc1b = alloc1b.Copy()
	alloc1b.ClientStatus = structs.AllocClientStatusRunning
	alloc1b.ModifyTime = now.UnixNano()
	alloc1b.DeploymentStatus = &structs.AllocDeploymentStatus{
		Canary:    false,
		Healthy:   pointer.Of(true),
		Timestamp: now,
	}

	allocs = []*structs.Allocation{alloc1a, alloc1b}
	err = m.state.UpdateAllocsFromClient(mtype, m.nextIndex(), allocs)
	must.NoError(t, err)

	// ensure any progress deadline has expired
	time.Sleep(progressTimeout)

	// without a scheduler running we'll never mark the deployment as
	// successful, so test that healthy == desired and that we haven't failed
	deployment, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Eq(t, structs.DeploymentStatusRunning, deployment.Status)

	group1 := deployment.TaskGroups["group1"]
	must.Eq(t, group1.DesiredTotal, group1.HealthyAllocs, must.Sprint("not enough healthy"))
	must.Eq(t, group1.DesiredTotal, group1.PlacedAllocs, must.Sprint("not enough placed"))
	must.Eq(t, 0, group1.UnhealthyAllocs)
	must.True(t, group1.Promoted)

	group2 := deployment.TaskGroups["group2"]
	must.Eq(t, group2.DesiredTotal, group2.HealthyAllocs, must.Sprint("not enough healthy"))
	must.Eq(t, group2.DesiredTotal, group2.PlacedAllocs, must.Sprint("not enough placed"))
	must.Eq(t, 0, group2.UnhealthyAllocs)
}

// Test scenario where deployment initially has no progress deadline
// After the deployment is updated, a failed alloc's DesiredTransition should be set
func TestDeploymentWatcher_Watch_StartWithoutProgressDeadline(t *testing.T) {
	ci.Parallel(t)
	w, m := testDeploymentWatcher(t, 1000.0, 1*time.Millisecond)

	// Create a job, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.ProgressDeadline = 500 * time.Millisecond
	j.Stable = true
	d := mock.Deployment()
	d.JobID = j.ID

	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))

	a := mock.Alloc()
	a.CreateTime = time.Now().UnixNano()
	a.DeploymentID = d.ID

	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}))

	d.TaskGroups["web"].ProgressDeadline = 500 * time.Millisecond
	// Update the deployment with a progress deadline
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// Update the alloc to be unhealthy
	a2 := a.Copy()
	a2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy:   pointer.Of(false),
		Timestamp: time.Now(),
	}
	must.NoError(t, m.state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a2}))

	// Wait for the alloc's DesiredState to set reschedule
	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(func() error {
		a, err := m.state.AllocByID(nil, a.ID)
		if err != nil {
			return err
		}
		dt := a.DesiredTransition
		if dt.Reschedule == nil || !*dt.Reschedule {
			return fmt.Errorf("Desired Transition Reschedule should be set: %+v", dt)
		}
		return nil
	}),
		wait.Gap(10*time.Millisecond),
		wait.Timeout(3*time.Second)))

}

// Test that we exit before hitting the Progress Deadline when we run out of reschedule attempts
// for a failing deployment
func TestDeploymentWatcher_Watch_FailEarly(t *testing.T) {
	ci.Parallel(t)
	w, m := testDeploymentWatcher(t, 1000.0, 1*time.Millisecond)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.ProgressDeadline = 500 * time.Millisecond
	// Allow only 1 allocation for that deployment
	j.TaskGroups[0].ReschedulePolicy.Attempts = 0
	j.TaskGroups[0].ReschedulePolicy.Unlimited = false
	j.Stable = true
	d := mock.Deployment()
	d.JobID = j.ID
	d.TaskGroups["web"].ProgressDeadline = 500 * time.Millisecond
	d.TaskGroups["web"].RequireProgressBy = time.Now().Add(d.TaskGroups["web"].ProgressDeadline)
	a := mock.Alloc()
	now := time.Now()
	a.CreateTime = now.UnixNano()
	a.ModifyTime = now.UnixNano()
	a.DeploymentID = d.ID
	must.Nil(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), must.Sprint("UpsertJob"))
	must.Nil(t, m.state.UpsertDeployment(m.nextIndex(), d), must.Sprint("UpsertDeployment"))
	must.Nil(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}), must.Sprint("UpsertAllocs"))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// Update the alloc to be unhealthy
	a2 := a.Copy()
	a2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy:   pointer.Of(false),
		Timestamp: now,
	}
	must.Nil(t, m.state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a2}))

	// Wait for the deployment to be failed
	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(func() error {
		d, _ := m.state.DeploymentByID(nil, d.ID)
		if d.Status != structs.DeploymentStatusFailed {
			return fmt.Errorf("bad status %q", d.Status)
		}
		if d.StatusDescription != structs.DeploymentStatusDescriptionFailedAllocations {
			return fmt.Errorf("bad status description %q", d.StatusDescription)
		}
		return nil
	}),
		wait.Gap(10*time.Millisecond), wait.Timeout(time.Second)),
		must.Sprint("expected deployment to be failed"))

	waitForEvals(t, m.state, j, 1)
}

// Tests that the watcher fails rollback when the spec hasn't changed
func TestDeploymentWatcher_RollbackFailed(t *testing.T) {
	ci.Parallel(t)
	w, m := testDeploymentWatcher(t, 1000.0, 1*time.Millisecond)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.AutoRevert = true
	j.TaskGroups[0].Update.ProgressDeadline = 0
	j.Stable = true
	d := mock.Deployment()
	d.JobID = j.ID
	d.TaskGroups["web"].AutoRevert = true
	a := mock.Alloc()
	a.DeploymentID = d.ID
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}))

	// Upsert the job again to get a new version
	j2 := j.Copy()
	// Modify the job to make its specification different
	j2.Stable = false
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j2))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	// Update the allocs health to healthy which should create an evaluation
	for range 5 {
		req := &structs.ApplyDeploymentAllocHealthRequest{
			DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
				DeploymentID:         d.ID,
				HealthyAllocationIDs: []string{a.ID},
			},
		}
		must.NoError(t, m.state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, m.nextIndex(), req))
	}

	waitForEvals(t, m.state, j, 1)

	// Update the allocs health to unhealthy which will cause attempting a rollback,
	// fail in that step, do status update and eval
	req2 := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:           d.ID,
			UnhealthyAllocationIDs: []string{a.ID},
		},
	}
	must.NoError(t, m.state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, m.nextIndex(), req2))

	waitForEvals(t, m.state, j, 2)

	// verify that the watcher is batching allocation changes
	m.assertCalls(t, "UpdateAllocDesiredTransition", 1)

	// verify that the job version hasn't changed after upsert
	m.state.JobByID(nil, structs.DefaultNamespace, j.ID)
	must.Eq(t, uint64(0), j.Version)

	d, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Eq(t, structs.DeploymentStatusFailed, d.Status)
	must.Eq(t, structs.DeploymentStatusDescriptionRollbackNoop(
		structs.DeploymentStatusDescriptionFailedAllocations, 0), d.StatusDescription)
}

// Test allocation updates and evaluation creation is batched between watchers
func TestWatcher_BatchAllocUpdates(t *testing.T) {
	ci.Parallel(t)
	w, m := testDeploymentWatcher(t, 1000.0, 1*time.Second)

	// Create a job, alloc, for two deployments
	j1 := mock.Job()
	j1.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j1.TaskGroups[0].Update.ProgressDeadline = 0
	d1 := mock.Deployment()
	d1.JobID = j1.ID
	a1 := mock.Alloc()
	a1.Job = j1
	a1.JobID = j1.ID
	a1.DeploymentID = d1.ID

	j2 := mock.Job()
	j2.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j2.TaskGroups[0].Update.ProgressDeadline = 0
	d2 := mock.Deployment()
	d2.JobID = j2.ID
	a2 := mock.Alloc()
	a2.Job = j2
	a2.JobID = j2.ID
	a2.DeploymentID = d2.ID

	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j1))
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j2))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d1))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d2))
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a1}))
	must.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a2}))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 2)

	// Update the allocs health to healthy which should create an evaluation
	req := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:         d1.ID,
			HealthyAllocationIDs: []string{a1.ID},
		},
	}
	must.NoError(t, m.state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, m.nextIndex(), req))

	req2 := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:         d2.ID,
			HealthyAllocationIDs: []string{a2.ID},
		},
	}
	must.NoError(t, m.state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, m.nextIndex(), req2))

	waitForEvals(t, m.state, j1, 1)
	waitForEvals(t, m.state, j2, 1)
	waitForWatchers(t, w, 2)

	// verify that the watcher is batching allocation changes
	m.assertCalls(t, "UpdateAllocDesiredTransition", 1)
}

func watchersCount(w *Watcher) int {
	w.l.RLock()
	defer w.l.RUnlock()
	return len(w.watchers)
}

// TestWatcher_PurgeDeployment tests that we don't leak watchers if a job is purged
func TestWatcher_PurgeDeployment(t *testing.T) {
	ci.Parallel(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	must.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j))
	must.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d))

	w.SetEnabled(true, m.state)
	waitForWatchers(t, w, 1)

	must.NoError(t, m.state.DeleteJob(m.nextIndex(), j.Namespace, j.ID))
	waitForWatchers(t, w, 0)

	d, err := m.state.DeploymentByID(nil, d.ID)
	must.NoError(t, err)
	must.Nil(t, d)
}

func waitForWatchers(t *testing.T, w *Watcher, expect int) {
	t.Helper()
	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(func() bool { return expect == watchersCount(w) }),
		wait.Gap(10*time.Millisecond),
		wait.Timeout(time.Second)), must.Sprintf("expected %d deployments", expect))
}

func waitForEvals(t *testing.T, store *state.StateStore, job *structs.Job, expect int) {
	t.Helper()
	must.Wait(t, wait.InitialSuccess(wait.BoolFunc(func() bool {
		ws := memdb.NewWatchSet()
		evals, _ := store.EvalsByJob(ws, job.Namespace, job.ID)
		return len(evals) == expect
	}),
		wait.Gap(10*time.Millisecond),
		wait.Timeout(5*time.Second), // some of these need to wait quite a while
	), must.Sprintf("expected %d evals before timeout", expect))
}
