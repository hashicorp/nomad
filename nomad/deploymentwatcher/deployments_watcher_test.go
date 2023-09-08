// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package deploymentwatcher

import (
	"fmt"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	mocker "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	m.On("UpdateDeploymentStatus", mocker.MatchedBy(func(args *structs.DeploymentStatusUpdateRequest) bool {
		return true
	})).Return(nil).Maybe()

	// Create three jobs
	j1, j2, j3 := mock.Job(), mock.Job(), mock.Job()
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, j1))
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, 101, nil, j2))
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, 102, nil, j3))

	// Create three deployments all running
	d1, d2, d3 := mock.Deployment(), mock.Deployment(), mock.Deployment()
	d1.JobID = j1.ID
	d2.JobID = j2.ID
	d3.JobID = j3.ID

	// Upsert the first deployment
	require.Nil(m.state.UpsertDeployment(103, d1))

	// Next list 3
	block1 := make(chan time.Time)
	go func() {
		<-block1
		require.Nil(m.state.UpsertDeployment(104, d2))
		require.Nil(m.state.UpsertDeployment(105, d3))
	}()

	//// Next list 3 but have one be terminal
	block2 := make(chan time.Time)
	d3terminal := d3.Copy()
	d3terminal.Status = structs.DeploymentStatusFailed
	go func() {
		<-block2
		require.Nil(m.state.UpsertDeployment(106, d3terminal))
	}()

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "1 deployment returned") })

	close(block1)
	testutil.WaitForResult(func() (bool, error) { return 3 == watchersCount(w), nil },
		func(err error) { require.Equal(3, watchersCount(w), "3 deployment returned") })

	close(block2)
	testutil.WaitForResult(func() (bool, error) { return 2 == watchersCount(w), nil },
		func(err error) { require.Equal(3, watchersCount(w), "3 deployment returned - 1 terminal") })
}

// Tests that calls against an unknown deployment fail
func TestWatcher_UnknownDeployment(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)
	w.SetEnabled(true, m.state)

	m.On("UpdateDeploymentStatus", mocker.MatchedBy(func(args *structs.DeploymentStatusUpdateRequest) bool {
		return true
	})).Return(nil).Maybe()

	// The expected error is that it should be an unknown deployment
	dID := uuid.Generate()
	expected := fmt.Sprintf("unknown deployment %q", dID)

	// Request setting the health against an unknown deployment
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         dID,
		HealthyAllocationIDs: []string{uuid.Generate()},
	}
	var resp structs.DeploymentUpdateResponse
	err := w.SetAllocHealth(req, &resp)
	if assert.NotNil(err, "should have error for unknown deployment") {
		require.Contains(err.Error(), expected)
	}

	// Request promoting against an unknown deployment
	req2 := &structs.DeploymentPromoteRequest{
		DeploymentID: dID,
		All:          true,
	}
	err = w.PromoteDeployment(req2, &resp)
	if assert.NotNil(err, "should have error for unknown deployment") {
		require.Contains(err.Error(), expected)
	}

	// Request pausing against an unknown deployment
	req3 := &structs.DeploymentPauseRequest{
		DeploymentID: dID,
		Pause:        true,
	}
	err = w.PauseDeployment(req3, &resp)
	if assert.NotNil(err, "should have error for unknown deployment") {
		require.Contains(err.Error(), expected)
	}

	// Request failing against an unknown deployment
	req4 := &structs.DeploymentFailRequest{
		DeploymentID: dID,
	}
	err = w.FailDeployment(req4, &resp)
	if assert.NotNil(err, "should have error for unknown deployment") {
		require.Contains(err.Error(), expected)
	}
}

// Test setting an unknown allocation's health
func TestWatcher_SetAllocHealth_Unknown(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	m.On("UpdateDeploymentStatus", mocker.MatchedBy(func(args *structs.DeploymentStatusUpdateRequest) bool {
		return true
	})).Return(nil).Maybe()

	// Create a job, and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")

	// require that we get a call to UpsertDeploymentAllocHealth
	a := mock.Alloc()
	matchConfig := &matchDeploymentAllocHealthRequestConfig{
		DeploymentID: d.ID,
		Healthy:      []string{a.ID},
		Eval:         true,
	}
	matcher := matchDeploymentAllocHealthRequest(matchConfig)
	m.On("UpdateDeploymentAllocHealth", mocker.MatchedBy(matcher)).Return(nil)

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Call SetAllocHealth
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         d.ID,
		HealthyAllocationIDs: []string{a.ID},
	}
	var resp structs.DeploymentUpdateResponse
	err := w.SetAllocHealth(req, &resp)
	if assert.NotNil(err, "Set health of unknown allocation") {
		require.Contains(err.Error(), "unknown")
	}
	require.Equal(1, watchersCount(w), "Deployment should still be active")
}

// Test setting allocation health
func TestWatcher_SetAllocHealth_Healthy(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	m.On("UpdateDeploymentStatus", mocker.MatchedBy(func(args *structs.DeploymentStatusUpdateRequest) bool {
		return true
	})).Return(nil).Maybe()

	// Create a job, alloc, and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	a.DeploymentID = d.ID
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// require that we get a call to UpsertDeploymentAllocHealth
	matchConfig := &matchDeploymentAllocHealthRequestConfig{
		DeploymentID: d.ID,
		Healthy:      []string{a.ID},
		Eval:         true,
	}
	matcher := matchDeploymentAllocHealthRequest(matchConfig)
	m.On("UpdateDeploymentAllocHealth", mocker.MatchedBy(matcher)).Return(nil)

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Call SetAllocHealth
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         d.ID,
		HealthyAllocationIDs: []string{a.ID},
	}
	var resp structs.DeploymentUpdateResponse
	err := w.SetAllocHealth(req, &resp)
	require.Nil(err, "SetAllocHealth")
	require.Equal(1, watchersCount(w), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentAllocHealth", mocker.MatchedBy(matcher))
}

// Test setting allocation unhealthy
func TestWatcher_SetAllocHealth_Unhealthy(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	m.On("UpdateDeploymentStatus", mocker.MatchedBy(func(args *structs.DeploymentStatusUpdateRequest) bool {
		return true
	})).Return(nil).Maybe()

	// Create a job, alloc, and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	a.DeploymentID = d.ID
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// require that we get a call to UpsertDeploymentAllocHealth
	matchConfig := &matchDeploymentAllocHealthRequestConfig{
		DeploymentID: d.ID,
		Unhealthy:    []string{a.ID},
		Eval:         true,
		DeploymentUpdate: &structs.DeploymentStatusUpdate{
			DeploymentID:      d.ID,
			Status:            structs.DeploymentStatusFailed,
			StatusDescription: structs.DeploymentStatusDescriptionFailedAllocations,
		},
	}
	matcher := matchDeploymentAllocHealthRequest(matchConfig)
	m.On("UpdateDeploymentAllocHealth", mocker.MatchedBy(matcher)).Return(nil)

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Call SetAllocHealth
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:           d.ID,
		UnhealthyAllocationIDs: []string{a.ID},
	}
	var resp structs.DeploymentUpdateResponse
	err := w.SetAllocHealth(req, &resp)
	require.Nil(err, "SetAllocHealth")

	testutil.WaitForResult(func() (bool, error) { return 0 == watchersCount(w), nil },
		func(err error) { require.Equal(0, watchersCount(w), "Should have no deployment") })
	m.AssertNumberOfCalls(t, "UpdateDeploymentAllocHealth", 1)
}

// Test setting allocation unhealthy and that there should be a rollback
func TestWatcher_SetAllocHealth_Unhealthy_Rollback(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	m.On("UpdateDeploymentStatus", mocker.MatchedBy(func(args *structs.DeploymentStatusUpdateRequest) bool {
		return true
	})).Return(nil).Maybe()

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
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// Upsert the job again to get a new version
	j2 := j.Copy()
	j2.Stable = false
	// Modify the job to make its specification different
	j2.Meta["foo"] = "bar"

	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j2), "UpsertJob2")

	// require that we get a call to UpsertDeploymentAllocHealth
	matchConfig := &matchDeploymentAllocHealthRequestConfig{
		DeploymentID: d.ID,
		Unhealthy:    []string{a.ID},
		Eval:         true,
		DeploymentUpdate: &structs.DeploymentStatusUpdate{
			DeploymentID:      d.ID,
			Status:            structs.DeploymentStatusFailed,
			StatusDescription: structs.DeploymentStatusDescriptionFailedAllocations,
		},
		JobVersion: pointer.Of(uint64(0)),
	}
	matcher := matchDeploymentAllocHealthRequest(matchConfig)
	m.On("UpdateDeploymentAllocHealth", mocker.MatchedBy(matcher)).Return(nil)

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Call SetAllocHealth
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:           d.ID,
		UnhealthyAllocationIDs: []string{a.ID},
	}
	var resp structs.DeploymentUpdateResponse
	err := w.SetAllocHealth(req, &resp)
	require.Nil(err, "SetAllocHealth")

	testutil.WaitForResult(func() (bool, error) { return 0 == watchersCount(w), nil },
		func(err error) { require.Equal(0, watchersCount(w), "Should have no deployment") })
	m.AssertNumberOfCalls(t, "UpdateDeploymentAllocHealth", 1)
}

// Test setting allocation unhealthy on job with identical spec and there should be no rollback
func TestWatcher_SetAllocHealth_Unhealthy_NoRollback(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	m.On("UpdateDeploymentStatus", mocker.MatchedBy(func(args *structs.DeploymentStatusUpdateRequest) bool {
		return true
	})).Return(nil).Maybe()

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
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// Upsert the job again to get a new version
	j2 := j.Copy()
	j2.Stable = false

	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j2), "UpsertJob2")

	// require that we get a call to UpsertDeploymentAllocHealth
	matchConfig := &matchDeploymentAllocHealthRequestConfig{
		DeploymentID: d.ID,
		Unhealthy:    []string{a.ID},
		Eval:         true,
		DeploymentUpdate: &structs.DeploymentStatusUpdate{
			DeploymentID:      d.ID,
			Status:            structs.DeploymentStatusFailed,
			StatusDescription: structs.DeploymentStatusDescriptionFailedAllocations,
		},
		JobVersion: nil,
	}
	matcher := matchDeploymentAllocHealthRequest(matchConfig)
	m.On("UpdateDeploymentAllocHealth", mocker.MatchedBy(matcher)).Return(nil)

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Call SetAllocHealth
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:           d.ID,
		UnhealthyAllocationIDs: []string{a.ID},
	}
	var resp structs.DeploymentUpdateResponse
	err := w.SetAllocHealth(req, &resp)
	require.Nil(err, "SetAllocHealth")

	testutil.WaitForResult(func() (bool, error) { return 0 == watchersCount(w), nil },
		func(err error) { require.Equal(0, watchersCount(w), "Should have no deployment") })
	m.AssertNumberOfCalls(t, "UpdateDeploymentAllocHealth", 1)
}

// Test promoting a deployment
func TestWatcher_PromoteDeployment_HealthyCanaries(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	m.On("UpdateDeploymentStatus", mocker.MatchedBy(func(args *structs.DeploymentStatusUpdateRequest) bool {
		return true
	})).Return(nil).Maybe()

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
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// require that we get a call to UpsertDeploymentPromotion
	matchConfig := &matchDeploymentPromoteRequestConfig{
		Promotion: &structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
		Eval: true,
	}
	matcher := matchDeploymentPromoteRequest(matchConfig)
	m.On("UpdateDeploymentPromotion", mocker.MatchedBy(matcher)).Return(nil)

	// We may get an update for the desired transition.
	m1 := matchUpdateAllocDesiredTransitions([]string{d.ID})
	m.On("UpdateAllocDesiredTransition", mocker.MatchedBy(m1)).Return(nil).Once()

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Call PromoteDeployment
	req := &structs.DeploymentPromoteRequest{
		DeploymentID: d.ID,
		All:          true,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PromoteDeployment(req, &resp)
	require.Nil(err, "PromoteDeployment")
	require.Equal(1, watchersCount(w), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentPromotion", mocker.MatchedBy(matcher))
}

// Test promoting a deployment with unhealthy canaries
func TestWatcher_PromoteDeployment_UnhealthyCanaries(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	m.On("UpdateDeploymentStatus", mocker.MatchedBy(func(args *structs.DeploymentStatusUpdateRequest) bool {
		return true
	})).Return(nil).Maybe()

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
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// require that we get a call to UpsertDeploymentPromotion
	matchConfig := &matchDeploymentPromoteRequestConfig{
		Promotion: &structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
		Eval: true,
	}
	matcher := matchDeploymentPromoteRequest(matchConfig)
	m.On("UpdateDeploymentPromotion", mocker.MatchedBy(matcher)).Return(nil)

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Call SetAllocHealth
	req := &structs.DeploymentPromoteRequest{
		DeploymentID: d.ID,
		All:          true,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PromoteDeployment(req, &resp)
	if assert.NotNil(t, err, "PromoteDeployment") {
		// 0/2 because the old version has been stopped but the canary isn't marked healthy yet
		require.Contains(err.Error(), `Task group "web" has 0/2 healthy allocations`, "Should error because canary isn't marked healthy")
	}

	require.Equal(1, watchersCount(w), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentPromotion", mocker.MatchedBy(matcher))
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
	require.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{ca1, ca2, ra1, ra2}), "UpsertAllocs")

	// =============================================================
	// Support method calls

	// clear UpdateDeploymentStatus default expectation
	m.Mock.ExpectedCalls = nil

	matchConfig0 := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusFailed,
		StatusDescription: structs.DeploymentStatusDescriptionProgressDeadline,
		Eval:              true,
	}
	matcher0 := matchDeploymentStatusUpdateRequest(matchConfig0)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(matcher0)).Return(nil)

	matchConfig1 := &matchDeploymentAllocHealthRequestConfig{
		DeploymentID: d.ID,
		Healthy:      []string{ca1.ID, ca2.ID, ra1.ID, ra2.ID},
		Eval:         true,
	}
	matcher1 := matchDeploymentAllocHealthRequest(matchConfig1)
	m.On("UpdateDeploymentAllocHealth", mocker.MatchedBy(matcher1)).Return(nil)

	matchConfig2 := &matchDeploymentPromoteRequestConfig{
		Promotion: &structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
		Eval: true,
	}
	matcher2 := matchDeploymentPromoteRequest(matchConfig2)
	m.On("UpdateDeploymentPromotion", mocker.MatchedBy(matcher2)).Return(nil)
	// =============================================================

	// Start the deployment
	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) {
		w.l.RLock()
		defer w.l.RUnlock()
		return 1 == len(w.watchers), nil
	},
		func(err error) {
			w.l.RLock()
			defer w.l.RUnlock()
			require.Equal(t, 1, len(w.watchers), "Should have 1 deployment")
		},
	)

	// Mark the canaries healthy
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         d.ID,
		HealthyAllocationIDs: []string{ca1.ID, ca2.ID, ra1.ID, ra2.ID},
	}
	var resp structs.DeploymentUpdateResponse
	// Calls w.raft.UpdateDeploymentAllocHealth, which is implemented by StateStore in
	// state.UpdateDeploymentAllocHealth via a raft shim?
	err := w.SetAllocHealth(req, &resp)
	require.NoError(t, err)

	ws := memdb.NewWatchSet()

	testutil.WaitForResult(
		func() (bool, error) {
			ds, _ := m.state.DeploymentsByJobID(ws, j.Namespace, j.ID, true)
			d = ds[0]
			return 2 == d.TaskGroups["web"].HealthyAllocs, nil
		},
		func(err error) { require.NoError(t, err) },
	)

	require.Equal(t, 1, len(w.watchers), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentPromotion", mocker.MatchedBy(matcher2))

	require.Equal(t, "running", d.Status)
	require.True(t, d.TaskGroups["web"].Promoted)

	a1, _ := m.state.AllocByID(ws, ca1.ID)
	require.False(t, a1.DeploymentStatus.Canary)
	require.Equal(t, "pending", a1.ClientStatus)
	require.Equal(t, "run", a1.DesiredStatus)

	b1, _ := m.state.AllocByID(ws, ca2.ID)
	require.False(t, b1.DeploymentStatus.Canary)
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
	require.NoError(t, m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.NoError(t, m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{ca1, ca2, ca3}), "UpsertAllocs")

	// =============================================================
	// Support method calls

	// clear UpdateDeploymentStatus default expectation
	m.Mock.ExpectedCalls = nil

	matchConfig0 := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusFailed,
		StatusDescription: structs.DeploymentStatusDescriptionProgressDeadline,
		Eval:              true,
	}
	matcher0 := matchDeploymentStatusUpdateRequest(matchConfig0)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(matcher0)).Return(nil)

	matchConfig1 := &matchDeploymentAllocHealthRequestConfig{
		DeploymentID: d.ID,
		Healthy:      []string{ca1.ID, ca2.ID},
		Eval:         true,
	}
	matcher1 := matchDeploymentAllocHealthRequest(matchConfig1)
	m.On("UpdateDeploymentAllocHealth", mocker.MatchedBy(matcher1)).Return(nil)

	matchConfig2 := &matchDeploymentPromoteRequestConfig{
		Promotion: &structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
		Eval: true,
	}
	matcher2 := matchDeploymentPromoteRequest(matchConfig2)
	m.On("UpdateDeploymentPromotion", mocker.MatchedBy(matcher2)).Return(nil)
	// =============================================================

	// Start the deployment
	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) {
		w.l.RLock()
		defer w.l.RUnlock()
		return 1 == len(w.watchers), nil
	},
		func(err error) {
			w.l.RLock()
			defer w.l.RUnlock()
			require.Equal(t, 1, len(w.watchers), "Should have 1 deployment")
		},
	)

	// Mark only 2 canaries as healthy
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         d.ID,
		HealthyAllocationIDs: []string{ca1.ID, ca2.ID},
	}
	var resp structs.DeploymentUpdateResponse
	// Calls w.raft.UpdateDeploymentAllocHealth, which is implemented by StateStore in
	// state.UpdateDeploymentAllocHealth via a raft shim?
	err := w.SetAllocHealth(req, &resp)
	require.NoError(t, err)

	ws := memdb.NewWatchSet()

	testutil.WaitForResult(
		func() (bool, error) {
			ds, _ := m.state.DeploymentsByJobID(ws, j.Namespace, j.ID, true)
			d = ds[0]
			return 2 == d.TaskGroups["web"].HealthyAllocs, nil
		},
		func(err error) { require.NoError(t, err) },
	)

	// Verify that a promotion request was submitted.
	require.Equal(t, 1, len(w.watchers), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentPromotion", mocker.MatchedBy(matcher2))

	require.Equal(t, "running", d.Status)
	require.True(t, d.TaskGroups["web"].Promoted)

	a1, _ := m.state.AllocByID(ws, ca1.ID)
	require.False(t, a1.DeploymentStatus.Canary)
	require.Equal(t, "pending", a1.ClientStatus)
	require.Equal(t, "run", a1.DesiredStatus)

	b1, _ := m.state.AllocByID(ws, ca2.ID)
	require.False(t, b1.DeploymentStatus.Canary)
}

// Test pausing a deployment that is running
func TestWatcher_PauseDeployment_Pause_Running(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// clear UpdateDeploymentStatus default expectation
	m.Mock.ExpectedCalls = nil

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")

	// require that we get a call to UpsertDeploymentStatusUpdate
	matchConfig := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusPaused,
		StatusDescription: structs.DeploymentStatusDescriptionPaused,
	}
	matcher := matchDeploymentStatusUpdateRequest(matchConfig)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(matcher)).Return(nil)

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Call PauseDeployment
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        true,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PauseDeployment(req, &resp)
	require.Nil(err, "PauseDeployment")

	require.Equal(1, watchersCount(w), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentStatus", mocker.MatchedBy(matcher))
}

// Test pausing a deployment that is paused
func TestWatcher_PauseDeployment_Pause_Paused(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// clear UpdateDeploymentStatus default expectation
	m.Mock.ExpectedCalls = nil

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	d.Status = structs.DeploymentStatusPaused
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")

	// require that we get a call to UpsertDeploymentStatusUpdate
	matchConfig := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusPaused,
		StatusDescription: structs.DeploymentStatusDescriptionPaused,
	}
	matcher := matchDeploymentStatusUpdateRequest(matchConfig)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(matcher)).Return(nil)

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Call PauseDeployment
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        true,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PauseDeployment(req, &resp)
	require.Nil(err, "PauseDeployment")

	require.Equal(1, watchersCount(w), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentStatus", mocker.MatchedBy(matcher))
}

// Test unpausing a deployment that is paused
func TestWatcher_PauseDeployment_Unpause_Paused(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	d.Status = structs.DeploymentStatusPaused
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")

	// require that we get a call to UpsertDeploymentStatusUpdate
	matchConfig := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusRunning,
		StatusDescription: structs.DeploymentStatusDescriptionRunning,
		Eval:              true,
	}
	matcher := matchDeploymentStatusUpdateRequest(matchConfig)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(matcher)).Return(nil)

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Call PauseDeployment
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        false,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PauseDeployment(req, &resp)
	require.Nil(err, "PauseDeployment")

	require.Equal(1, watchersCount(w), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentStatus", mocker.MatchedBy(matcher))
}

// Test unpausing a deployment that is running
func TestWatcher_PauseDeployment_Unpause_Running(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")

	// require that we get a call to UpsertDeploymentStatusUpdate
	matchConfig := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusRunning,
		StatusDescription: structs.DeploymentStatusDescriptionRunning,
		Eval:              true,
	}
	matcher := matchDeploymentStatusUpdateRequest(matchConfig)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(matcher)).Return(nil)

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Call PauseDeployment
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        false,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PauseDeployment(req, &resp)
	require.Nil(err, "PauseDeployment")

	require.Equal(1, watchersCount(w), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentStatus", mocker.MatchedBy(matcher))
}

// Test failing a deployment that is running
func TestWatcher_FailDeployment_Running(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")

	// require that we get a call to UpsertDeploymentStatusUpdate
	matchConfig := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusFailed,
		StatusDescription: structs.DeploymentStatusDescriptionFailedByUser,
		Eval:              true,
	}
	matcher := matchDeploymentStatusUpdateRequest(matchConfig)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(matcher)).Return(nil)

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Call PauseDeployment
	req := &structs.DeploymentFailRequest{
		DeploymentID: d.ID,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.FailDeployment(req, &resp)
	require.Nil(err, "FailDeployment")

	require.Equal(1, watchersCount(w), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentStatus", mocker.MatchedBy(matcher))
}

// Tests that the watcher properly watches for allocation changes and takes the
// proper actions
func TestDeploymentWatcher_Watch_NoProgressDeadline(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
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
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// Upsert the job again to get a new version
	j2 := j.Copy()
	// Modify the job to make its specification different
	j2.Meta["foo"] = "bar"
	j2.Stable = false
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j2), "UpsertJob2")

	// require that we will get a update allocation call only once. This will
	// verify that the watcher is batching allocation changes
	m1 := matchUpdateAllocDesiredTransitions([]string{d.ID})
	m.On("UpdateAllocDesiredTransition", mocker.MatchedBy(m1)).Return(nil).Once()

	// require that we get a call to UpsertDeploymentStatusUpdate
	c := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusFailed,
		StatusDescription: structs.DeploymentStatusDescriptionRollback(structs.DeploymentStatusDescriptionFailedAllocations, 0),
		JobVersion:        pointer.Of(uint64(0)),
		Eval:              true,
	}
	m2 := matchDeploymentStatusUpdateRequest(c)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(m2)).Return(nil)

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Update the allocs health to healthy which should create an evaluation
	for i := 0; i < 5; i++ {
		req := &structs.ApplyDeploymentAllocHealthRequest{
			DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
				DeploymentID:         d.ID,
				HealthyAllocationIDs: []string{a.ID},
			},
		}
		require.Nil(m.state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, m.nextIndex(), req), "UpsertDeploymentAllocHealth")
	}

	// Wait for there to be one eval
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		evals, err := m.state.EvalsByJob(ws, j.Namespace, j.ID)
		if err != nil {
			return false, err
		}

		if l := len(evals); l != 1 {
			return false, fmt.Errorf("Got %d evals; want 1", l)
		}

		return true, nil
	}, func(err error) {
		t.Fatal(err)
	})

	// Update the allocs health to unhealthy which should create a job rollback,
	// status update and eval
	req2 := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:           d.ID,
			UnhealthyAllocationIDs: []string{a.ID},
		},
	}
	require.Nil(m.state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, m.nextIndex(), req2), "UpsertDeploymentAllocHealth")

	// Wait for there to be one eval
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		evals, err := m.state.EvalsByJob(ws, j.Namespace, j.ID)
		if err != nil {
			return false, err
		}

		if l := len(evals); l != 2 {
			return false, fmt.Errorf("Got %d evals; want 1", l)
		}

		return true, nil
	}, func(err error) {
		t.Fatal(err)
	})

	m.AssertCalled(t, "UpdateAllocDesiredTransition", mocker.MatchedBy(m1))

	// After we upsert the job version will go to 2. So use this to require the
	// original call happened.
	c2 := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusFailed,
		StatusDescription: structs.DeploymentStatusDescriptionRollback(structs.DeploymentStatusDescriptionFailedAllocations, 0),
		JobVersion:        pointer.Of(uint64(2)),
		Eval:              true,
	}
	m3 := matchDeploymentStatusUpdateRequest(c2)
	m.AssertCalled(t, "UpdateDeploymentStatus", mocker.MatchedBy(m3))
	testutil.WaitForResult(func() (bool, error) { return 0 == watchersCount(w), nil },
		func(err error) { require.Equal(0, watchersCount(w), "Should have no deployment") })
}

func TestDeploymentWatcher_Watch_ProgressDeadline(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
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
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// require that we get a call to UpsertDeploymentStatusUpdate
	c := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusFailed,
		StatusDescription: structs.DeploymentStatusDescriptionProgressDeadline,
		Eval:              true,
	}
	m2 := matchDeploymentStatusUpdateRequest(c)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(m2)).Return(nil)

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Update the alloc to be unhealthy and require that nothing happens.
	a2 := a.Copy()
	a2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy:   pointer.Of(false),
		Timestamp: now,
	}
	require.Nil(m.state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 100, []*structs.Allocation{a2}))

	// Wait for the deployment to be failed
	testutil.WaitForResult(func() (bool, error) {
		d, err := m.state.DeploymentByID(nil, d.ID)
		if err != nil {
			return false, err
		}

		return d.Status == structs.DeploymentStatusFailed, fmt.Errorf("bad status %q", d.Status)
	}, func(err error) {
		t.Fatal(err)
	})

	// require there are is only one evaluation
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		evals, err := m.state.EvalsByJob(ws, j.Namespace, j.ID)
		if err != nil {
			return false, err
		}

		if l := len(evals); l != 1 {
			return false, fmt.Errorf("Got %d evals; want 1", l)
		}

		return true, nil
	}, func(err error) {
		t.Fatal(err)
	})
}

// Test that progress deadline handling works when there are multiple groups
func TestDeploymentWatcher_ProgressCutoff(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	w, m := testDeploymentWatcher(t, 1000.0, 1*time.Millisecond)

	m.On("UpdateDeploymentStatus", mocker.MatchedBy(func(args *structs.DeploymentStatusUpdateRequest) bool {
		return true
	})).Return(nil).Maybe()

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

	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a, a2}), "UpsertAllocs")

	// We may get an update for the desired transition.
	m1 := matchUpdateAllocDesiredTransitions([]string{d.ID})
	m.On("UpdateAllocDesiredTransition", mocker.MatchedBy(m1)).Return(nil).Once()

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	watcher, err := w.getOrCreateWatcher(d.ID)
	require.NoError(err)
	require.NotNil(watcher)

	d1, err := m.state.DeploymentByID(nil, d.ID)
	require.NoError(err)

	done := watcher.doneGroups(d1)
	require.Contains(done, "web")
	require.False(done["web"])
	require.Contains(done, "foo")
	require.False(done["foo"])

	cutoff1 := watcher.getDeploymentProgressCutoff(d1)
	require.False(cutoff1.IsZero())

	// Update the first allocation to be healthy
	a3 := a.Copy()
	a3.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: pointer.Of(true)}
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a3}), "UpsertAllocs")

	// Get the updated deployment
	d2, err := m.state.DeploymentByID(nil, d.ID)
	require.NoError(err)

	done = watcher.doneGroups(d2)
	require.Contains(done, "web")
	require.True(done["web"])
	require.Contains(done, "foo")
	require.False(done["foo"])

	cutoff2 := watcher.getDeploymentProgressCutoff(d2)
	require.False(cutoff2.IsZero())
	require.True(cutoff1.UnixNano() < cutoff2.UnixNano())

	// Update the second allocation to be healthy
	a4 := a2.Copy()
	a4.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: pointer.Of(true)}
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a4}), "UpsertAllocs")

	// Get the updated deployment
	d3, err := m.state.DeploymentByID(nil, d.ID)
	require.NoError(err)

	done = watcher.doneGroups(d3)
	require.Contains(done, "web")
	require.True(done["web"])
	require.Contains(done, "foo")
	require.True(done["foo"])

	cutoff3 := watcher.getDeploymentProgressCutoff(d2)
	require.True(cutoff3.IsZero())
}

// Test that we will allow the progress deadline to be reached when the canaries
// are healthy but we haven't promoted
func TestDeploymentWatcher_Watch_ProgressDeadline_Canaries(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	w, m := testDeploymentWatcher(t, 1000.0, 1*time.Millisecond)

	m.On("UpdateDeploymentStatus", mocker.MatchedBy(func(args *structs.DeploymentStatusUpdateRequest) bool {
		return true
	})).Return(nil).Maybe()

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
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// require that we will get a createEvaluation call only once. This will
	// verify that the watcher is batching allocation changes
	m1 := matchUpdateAllocDesiredTransitions([]string{d.ID})
	m.On("UpdateAllocDesiredTransition", mocker.MatchedBy(m1)).Return(nil).Once()

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Update the alloc to be unhealthy and require that nothing happens.
	a2 := a.Copy()
	a2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy:   pointer.Of(true),
		Timestamp: now,
	}
	require.Nil(m.state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a2}))

	// Wait for the deployment to cross the deadline
	dout, err := m.state.DeploymentByID(nil, d.ID)
	require.NoError(err)
	require.NotNil(dout)
	state := dout.TaskGroups["web"]
	require.NotNil(state)
	time.Sleep(state.RequireProgressBy.Add(time.Second).Sub(now))

	// Require the deployment is still running
	dout, err = m.state.DeploymentByID(nil, d.ID)
	require.NoError(err)
	require.NotNil(dout)
	require.Equal(structs.DeploymentStatusRunning, dout.Status)
	require.Equal(structs.DeploymentStatusDescriptionRunningNeedsPromotion, dout.StatusDescription)

	// require there are is only one evaluation
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		evals, err := m.state.EvalsByJob(ws, j.Namespace, j.ID)
		if err != nil {
			return false, err
		}

		if l := len(evals); l != 1 {
			return false, fmt.Errorf("Got %d evals; want 1", l)
		}

		return true, nil
	}, func(err error) {
		t.Fatal(err)
	})
}

// Test that a promoted deployment with alloc healthy updates create
// evals to move the deployment forward
func TestDeploymentWatcher_PromotedCanary_UpdatedAllocs(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	w, m := testDeploymentWatcher(t, 1000.0, 1*time.Millisecond)

	m.On("UpdateDeploymentStatus", mocker.MatchedBy(func(args *structs.DeploymentStatusUpdateRequest) bool {
		return true
	})).Return(nil).Maybe()

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
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	m1 := matchUpdateAllocDesiredTransitions([]string{d.ID})
	m.On("UpdateAllocDesiredTransition", mocker.MatchedBy(m1)).Return(nil).Twice()

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
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	// Wait until batch eval period passes before updating another alloc
	time.Sleep(1 * time.Second)
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a2}), "UpsertAllocs")

	// Wait for the deployment to cross the deadline
	dout, err := m.state.DeploymentByID(nil, d.ID)
	require.NoError(err)
	require.NotNil(dout)
	state := dout.TaskGroups["web"]
	require.NotNil(state)
	time.Sleep(state.RequireProgressBy.Add(time.Second).Sub(now))

	// There should be two evals
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		evals, err := m.state.EvalsByJob(ws, j.Namespace, j.ID)
		if err != nil {
			return false, err
		}

		if l := len(evals); l != 2 {
			return false, fmt.Errorf("Got %d evals; want 2", l)
		}

		return true, nil
	}, func(err error) {
		t.Fatal(err)
	})
}

func TestDeploymentWatcher_ProgressDeadline_LatePromote(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	mtype := structs.MsgTypeTestSetup

	w, m := defaultTestDeploymentWatcher(t)
	w.SetEnabled(true, m.state)

	m.On("UpdateDeploymentStatus", mocker.MatchedBy(func(args *structs.DeploymentStatusUpdateRequest) bool {
		return true
	})).Return(nil).Maybe()

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

	require.NoError(m.state.UpsertJob(mtype, m.nextIndex(), nil, j))
	require.NoError(m.state.UpsertDeployment(m.nextIndex(), d))

	// require that we get a call to UpsertDeploymentPromotion
	matchConfig := &matchDeploymentPromoteRequestConfig{
		Promotion: &structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
		Eval: true,
	}
	matcher := matchDeploymentPromoteRequest(matchConfig)
	m.On("UpdateDeploymentPromotion", mocker.MatchedBy(matcher)).Return(nil)
	m1 := matchUpdateAllocDesiredTransitions([]string{d.ID})
	m.On("UpdateAllocDesiredTransition", mocker.MatchedBy(m1)).Return(nil)

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
	err := m.state.UpsertAllocs(mtype, m.nextIndex(), allocs)
	require.NoError(err)

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
	err = m.state.UpdateAllocsFromClient(mtype, m.nextIndex(), allocs)
	require.NoError(err)

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
	require.NoError(err)

	// ensure progress_deadline has definitely expired
	time.Sleep(progressTimeout)

	// promote the deployment

	req := &structs.DeploymentPromoteRequest{
		DeploymentID: d.ID,
		All:          true,
	}
	err = w.PromoteDeployment(req, &structs.DeploymentUpdateResponse{})
	require.NoError(err)

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
	require.NoError(err)

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
	require.NoError(err)

	// ensure any progress deadline has expired
	time.Sleep(progressTimeout)

	// without a scheduler running we'll never mark the deployment as
	// successful, so test that healthy == desired and that we haven't failed
	deployment, err := m.state.DeploymentByID(nil, d.ID)
	require.NoError(err)
	require.Equal(structs.DeploymentStatusRunning, deployment.Status)

	group1 := deployment.TaskGroups["group1"]

	require.Equal(group1.DesiredTotal, group1.HealthyAllocs, "not enough healthy")
	require.Equal(group1.DesiredTotal, group1.PlacedAllocs, "not enough placed")
	require.Equal(0, group1.UnhealthyAllocs)

	group2 := deployment.TaskGroups["group2"]
	require.Equal(group2.DesiredTotal, group2.HealthyAllocs, "not enough healthy")
	require.Equal(group2.DesiredTotal, group2.PlacedAllocs, "not enough placed")
	require.Equal(0, group2.UnhealthyAllocs)
}

// Test scenario where deployment initially has no progress deadline
// After the deployment is updated, a failed alloc's DesiredTransition should be set
func TestDeploymentWatcher_Watch_StartWithoutProgressDeadline(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	w, m := testDeploymentWatcher(t, 1000.0, 1*time.Millisecond)

	m.On("UpdateDeploymentStatus", mocker.MatchedBy(func(args *structs.DeploymentStatusUpdateRequest) bool {
		return true
	})).Return(nil).Maybe()

	// Create a job, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.ProgressDeadline = 500 * time.Millisecond
	j.Stable = true
	d := mock.Deployment()
	d.JobID = j.ID

	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")

	a := mock.Alloc()
	a.CreateTime = time.Now().UnixNano()
	a.DeploymentID = d.ID

	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	d.TaskGroups["web"].ProgressDeadline = 500 * time.Millisecond
	// Update the deployment with a progress deadline
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")

	// Match on DesiredTransition set to Reschedule for the failed alloc
	m1 := matchUpdateAllocDesiredTransitionReschedule([]string{a.ID})
	m.On("UpdateAllocDesiredTransition", mocker.MatchedBy(m1)).Return(nil).Once()

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Update the alloc to be unhealthy
	a2 := a.Copy()
	a2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy:   pointer.Of(false),
		Timestamp: time.Now(),
	}
	require.Nil(m.state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a2}))

	// Wait for the alloc's DesiredState to set reschedule
	testutil.WaitForResult(func() (bool, error) {
		a, err := m.state.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		dt := a.DesiredTransition
		shouldReschedule := dt.Reschedule != nil && *dt.Reschedule
		return shouldReschedule, fmt.Errorf("Desired Transition Reschedule should be set but got %v", shouldReschedule)
	}, func(err error) {
		t.Fatal(err)
	})
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

	// require that we get a call to UpsertDeploymentStatusUpdate
	c := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusFailed,
		StatusDescription: structs.DeploymentStatusDescriptionFailedAllocations,
		Eval:              true,
	}
	m2 := matchDeploymentStatusUpdateRequest(c)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(m2)).Return(nil)

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { must.Eq(t, 1, watchersCount(w), must.Sprint("Should have 1 deployment")) })

	// Update the alloc to be unhealthy
	a2 := a.Copy()
	a2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy:   pointer.Of(false),
		Timestamp: now,
	}
	must.Nil(t, m.state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a2}))

	// Wait for the deployment to be failed
	testutil.WaitForResult(func() (bool, error) {
		d, err := m.state.DeploymentByID(nil, d.ID)
		if err != nil {
			return false, err
		}

		if d.Status != structs.DeploymentStatusFailed {
			return false, fmt.Errorf("bad status %q", d.Status)
		}

		return d.StatusDescription == structs.DeploymentStatusDescriptionFailedAllocations, fmt.Errorf("bad status description %q", d.StatusDescription)
	}, func(err error) {
		t.Fatal(err)
	})

	// require there are is only one evaluation
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		evals, err := m.state.EvalsByJob(ws, j.Namespace, j.ID)
		if err != nil {
			return false, err
		}

		if l := len(evals); l != 1 {
			return false, fmt.Errorf("Got %d evals; want 1", l)
		}

		return true, nil
	}, func(err error) {
		t.Fatal(err)
	})
}

// Tests that the watcher fails rollback when the spec hasn't changed
func TestDeploymentWatcher_RollbackFailed(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
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
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// Upsert the job again to get a new version
	j2 := j.Copy()
	// Modify the job to make its specification different
	j2.Stable = false
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j2), "UpsertJob2")

	// require that we will get a createEvaluation call only once. This will
	// verify that the watcher is batching allocation changes
	m1 := matchUpdateAllocDesiredTransitions([]string{d.ID})
	m.On("UpdateAllocDesiredTransition", mocker.MatchedBy(m1)).Return(nil).Once()

	// require that we get a call to UpsertDeploymentStatusUpdate with roll back failed as the status
	c := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusFailed,
		StatusDescription: structs.DeploymentStatusDescriptionRollbackNoop(structs.DeploymentStatusDescriptionFailedAllocations, 0),
		JobVersion:        nil,
		Eval:              true,
	}
	m2 := matchDeploymentStatusUpdateRequest(c)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(m2)).Return(nil)

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 1 == watchersCount(w), nil },
		func(err error) { require.Equal(1, watchersCount(w), "Should have 1 deployment") })

	// Update the allocs health to healthy which should create an evaluation
	for i := 0; i < 5; i++ {
		req := &structs.ApplyDeploymentAllocHealthRequest{
			DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
				DeploymentID:         d.ID,
				HealthyAllocationIDs: []string{a.ID},
			},
		}
		require.Nil(m.state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, m.nextIndex(), req), "UpsertDeploymentAllocHealth")
	}

	// Wait for there to be one eval
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		evals, err := m.state.EvalsByJob(ws, j.Namespace, j.ID)
		if err != nil {
			return false, err
		}

		if l := len(evals); l != 1 {
			return false, fmt.Errorf("Got %d evals; want 1", l)
		}

		return true, nil
	}, func(err error) {
		t.Fatal(err)
	})

	// Update the allocs health to unhealthy which will cause attempting a rollback,
	// fail in that step, do status update and eval
	req2 := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:           d.ID,
			UnhealthyAllocationIDs: []string{a.ID},
		},
	}
	require.Nil(m.state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, m.nextIndex(), req2), "UpsertDeploymentAllocHealth")

	// Wait for there to be one eval
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		evals, err := m.state.EvalsByJob(ws, j.Namespace, j.ID)
		if err != nil {
			return false, err
		}

		if l := len(evals); l != 2 {
			return false, fmt.Errorf("Got %d evals; want 1", l)
		}

		return true, nil
	}, func(err error) {
		t.Fatal(err)
	})

	m.AssertCalled(t, "UpdateAllocDesiredTransition", mocker.MatchedBy(m1))

	// verify that the job version hasn't changed after upsert
	m.state.JobByID(nil, structs.DefaultNamespace, j.ID)
	require.Equal(uint64(0), j.Version, "Expected job version 0 but got ", j.Version)
}

// Test allocation updates and evaluation creation is batched between watchers
func TestWatcher_BatchAllocUpdates(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	w, m := testDeploymentWatcher(t, 1000.0, 1*time.Second)

	m.On("UpdateDeploymentStatus", mocker.MatchedBy(func(args *structs.DeploymentStatusUpdateRequest) bool {
		return true
	})).Return(nil).Maybe()

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

	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j1), "UpsertJob")
	require.Nil(m.state.UpsertJob(structs.MsgTypeTestSetup, m.nextIndex(), nil, j2), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d1), "UpsertDeployment")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d2), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a1}), "UpsertAllocs")
	require.Nil(m.state.UpsertAllocs(structs.MsgTypeTestSetup, m.nextIndex(), []*structs.Allocation{a2}), "UpsertAllocs")

	// require that we will get a createEvaluation call only once and it contains
	// both deployments. This will verify that the watcher is batching
	// allocation changes
	m1 := matchUpdateAllocDesiredTransitions([]string{d1.ID, d2.ID})
	m.On("UpdateAllocDesiredTransition", mocker.MatchedBy(m1)).Return(nil).Once()

	w.SetEnabled(true, m.state)
	testutil.WaitForResult(func() (bool, error) { return 2 == watchersCount(w), nil },
		func(err error) { require.Equal(2, watchersCount(w), "Should have 2 deployment") })

	// Update the allocs health to healthy which should create an evaluation
	req := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:         d1.ID,
			HealthyAllocationIDs: []string{a1.ID},
		},
	}
	require.Nil(m.state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, m.nextIndex(), req), "UpsertDeploymentAllocHealth")

	req2 := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:         d2.ID,
			HealthyAllocationIDs: []string{a2.ID},
		},
	}
	require.Nil(m.state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, m.nextIndex(), req2), "UpsertDeploymentAllocHealth")

	// Wait for there to be one eval for each job
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		evals1, err := m.state.EvalsByJob(ws, j1.Namespace, j1.ID)
		if err != nil {
			return false, err
		}

		evals2, err := m.state.EvalsByJob(ws, j2.Namespace, j2.ID)
		if err != nil {
			return false, err
		}

		if l := len(evals1); l != 1 {
			return false, fmt.Errorf("Got %d evals for job %v; want 1", l, j1.ID)
		}

		if l := len(evals2); l != 1 {
			return false, fmt.Errorf("Got %d evals for job 2; want 1", l)
		}

		return true, nil
	}, func(err error) {
		t.Fatal(err)
	})

	m.AssertCalled(t, "UpdateAllocDesiredTransition", mocker.MatchedBy(m1))
	testutil.WaitForResult(func() (bool, error) { return 2 == watchersCount(w), nil },
		func(err error) { require.Equal(2, watchersCount(w), "Should have 2 deployment") })
}

func watchersCount(w *Watcher) int {
	w.l.Lock()
	defer w.l.Unlock()

	return len(w.watchers)
}
