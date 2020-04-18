package deploymentwatcher

import (
	"fmt"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	mocker "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testDeploymentWatcher(t *testing.T, qps float64, batchDur time.Duration) (*Watcher, *mockBackend) {
	m := newMockBackend(t)
	w := NewDeploymentsWatcher(testlog.HCLogger(t), m, qps, batchDur)
	return w, m
}

func defaultTestDeploymentWatcher(t *testing.T) (*Watcher, *mockBackend) {
	return testDeploymentWatcher(t, LimitStateQueriesPerSecond, CrossDeploymentUpdateBatchDuration)
}

// Tests that the watcher properly watches for deployments and reconciles them
func TestWatcher_WatchDeployments(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	m.On("UpdateDeploymentStatus", mocker.MatchedBy(func(args *structs.DeploymentStatusUpdateRequest) bool {
		return true
	})).Return(nil).Maybe()

	// Create three jobs
	j1, j2, j3 := mock.Job(), mock.Job(), mock.Job()
	require.Nil(m.state.UpsertJob(100, j1))
	require.Nil(m.state.UpsertJob(101, j2))
	require.Nil(m.state.UpsertJob(102, j3))

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
	t.Parallel()
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
	t.Parallel()
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
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
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
	t.Parallel()
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
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

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
	t.Parallel()
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
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

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
	t.Parallel()
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
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// Upsert the job again to get a new version
	j2 := j.Copy()
	j2.Stable = false
	// Modify the job to make its specification different
	j2.Meta["foo"] = "bar"

	require.Nil(m.state.UpsertJob(m.nextIndex(), j2), "UpsertJob2")

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
		JobVersion: helper.Uint64ToPtr(0),
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
	t.Parallel()
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
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// Upsert the job again to get a new version
	j2 := j.Copy()
	j2.Stable = false

	require.Nil(m.state.UpsertJob(m.nextIndex(), j2), "UpsertJob2")

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
	t.Parallel()
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
		Healthy: helper.BoolToPtr(true),
	}
	a.DeploymentID = d.ID
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

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
	t.Parallel()
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
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

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
	t.Parallel()
	w, m := defaultTestDeploymentWatcher(t)
	now := time.Now()

	// Create 1 UpdateStrategy, 1 job (1 TaskGroup), 2 canaries, and 1 deployment
	upd := structs.DefaultUpdateStrategy.Copy()
	upd.AutoPromote = true
	upd.MaxParallel = 2
	upd.Canary = 2
	upd.ProgressDeadline = 5 * time.Second

	j := mock.Job()
	j.TaskGroups[0].Update = upd

	d := mock.Deployment()
	d.JobID = j.ID
	// This is created in scheduler.computeGroup at runtime, where properties from the
	// UpdateStrategy are copied in
	d.TaskGroups = map[string]*structs.DeploymentState{
		"web": {
			AutoPromote:      upd.AutoPromote,
			AutoRevert:       upd.AutoRevert,
			ProgressDeadline: upd.ProgressDeadline,
			DesiredTotal:     2,
		},
	}

	alloc := func() *structs.Allocation {
		a := mock.Alloc()
		a.DeploymentID = d.ID
		a.CreateTime = now.UnixNano()
		a.ModifyTime = now.UnixNano()
		a.DeploymentStatus = &structs.AllocDeploymentStatus{
			Canary: true,
		}
		return a
	}

	a := alloc()
	b := alloc()

	d.TaskGroups[a.TaskGroup].PlacedCanaries = []string{a.ID, b.ID}
	d.TaskGroups[a.TaskGroup].DesiredCanaries = 2
	require.NoError(t, m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	require.NoError(t, m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.NoError(t, m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a, b}), "UpsertAllocs")

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
		Healthy:      []string{a.ID, b.ID},
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
	testutil.WaitForResult(func() (bool, error) { return 1 == len(w.watchers), nil },
		func(err error) { require.Equal(t, 1, len(w.watchers), "Should have 1 deployment") })

	// Mark the canaries healthy
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         d.ID,
		HealthyAllocationIDs: []string{a.ID, b.ID},
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

	a1, _ := m.state.AllocByID(ws, a.ID)
	require.False(t, a1.DeploymentStatus.Canary)
	require.Equal(t, "pending", a1.ClientStatus)
	require.Equal(t, "run", a1.DesiredStatus)

	b1, _ := m.state.AllocByID(ws, b.ID)
	require.False(t, b1.DeploymentStatus.Canary)
}

// Test pausing a deployment that is running
func TestWatcher_PauseDeployment_Pause_Running(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// clear UpdateDeploymentStatus default expectation
	m.Mock.ExpectedCalls = nil

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
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
	t.Parallel()
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// clear UpdateDeploymentStatus default expectation
	m.Mock.ExpectedCalls = nil

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	d.Status = structs.DeploymentStatusPaused
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
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
	t.Parallel()
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	d.Status = structs.DeploymentStatusPaused
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
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
	t.Parallel()
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
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
	t.Parallel()
	require := require.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
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
	t.Parallel()
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
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// Upsert the job again to get a new version
	j2 := j.Copy()
	// Modify the job to make its specification different
	j2.Meta["foo"] = "bar"
	j2.Stable = false
	require.Nil(m.state.UpsertJob(m.nextIndex(), j2), "UpsertJob2")

	// require that we will get a update allocation call only once. This will
	// verify that the watcher is batching allocation changes
	m1 := matchUpdateAllocDesiredTransitions([]string{d.ID})
	m.On("UpdateAllocDesiredTransition", mocker.MatchedBy(m1)).Return(nil).Once()

	// require that we get a call to UpsertDeploymentStatusUpdate
	c := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusFailed,
		StatusDescription: structs.DeploymentStatusDescriptionRollback(structs.DeploymentStatusDescriptionFailedAllocations, 0),
		JobVersion:        helper.Uint64ToPtr(0),
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
		require.Nil(m.state.UpdateDeploymentAllocHealth(m.nextIndex(), req), "UpsertDeploymentAllocHealth")
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
	require.Nil(m.state.UpdateDeploymentAllocHealth(m.nextIndex(), req2), "UpsertDeploymentAllocHealth")

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
		JobVersion:        helper.Uint64ToPtr(2),
		Eval:              true,
	}
	m3 := matchDeploymentStatusUpdateRequest(c2)
	m.AssertCalled(t, "UpdateDeploymentStatus", mocker.MatchedBy(m3))
	testutil.WaitForResult(func() (bool, error) { return 0 == watchersCount(w), nil },
		func(err error) { require.Equal(0, watchersCount(w), "Should have no deployment") })
}

func TestDeploymentWatcher_Watch_ProgressDeadline(t *testing.T) {
	t.Parallel()
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
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

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
		Healthy:   helper.BoolToPtr(false),
		Timestamp: now,
	}
	require.Nil(m.state.UpdateAllocsFromClient(100, []*structs.Allocation{a2}))

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
	t.Parallel()
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

	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a, a2}), "UpsertAllocs")

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
	a3.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: helper.BoolToPtr(true)}
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a3}), "UpsertAllocs")

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
	a4.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: helper.BoolToPtr(true)}
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a4}), "UpsertAllocs")

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
	t.Parallel()
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
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

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
		Healthy:   helper.BoolToPtr(true),
		Timestamp: now,
	}
	require.Nil(m.state.UpdateAllocsFromClient(m.nextIndex(), []*structs.Allocation{a2}))

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
	t.Parallel()
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
		Healthy:   helper.BoolToPtr(true),
		Timestamp: now,
	}
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

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
		Healthy:   helper.BoolToPtr(true),
		Timestamp: now,
	}
	d.TaskGroups["web"].RequireProgressBy = time.Now().Add(2 * time.Second)
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	// Wait until batch eval period passes before updating another alloc
	time.Sleep(1 * time.Second)
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a2}), "UpsertAllocs")

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

// Test scenario where deployment initially has no progress deadline
// After the deployment is updated, a failed alloc's DesiredTransition should be set
func TestDeploymentWatcher_Watch_StartWithoutProgressDeadline(t *testing.T) {
	t.Parallel()
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

	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")

	a := mock.Alloc()
	a.CreateTime = time.Now().UnixNano()
	a.DeploymentID = d.ID

	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

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
		Healthy:   helper.BoolToPtr(false),
		Timestamp: time.Now(),
	}
	require.Nil(m.state.UpdateAllocsFromClient(m.nextIndex(), []*structs.Allocation{a2}))

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

// Tests that the watcher fails rollback when the spec hasn't changed
func TestDeploymentWatcher_RollbackFailed(t *testing.T) {
	t.Parallel()
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
	require.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// Upsert the job again to get a new version
	j2 := j.Copy()
	// Modify the job to make its specification different
	j2.Stable = false
	require.Nil(m.state.UpsertJob(m.nextIndex(), j2), "UpsertJob2")

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
		require.Nil(m.state.UpdateDeploymentAllocHealth(m.nextIndex(), req), "UpsertDeploymentAllocHealth")
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
	require.Nil(m.state.UpdateDeploymentAllocHealth(m.nextIndex(), req2), "UpsertDeploymentAllocHealth")

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
	t.Parallel()
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

	require.Nil(m.state.UpsertJob(m.nextIndex(), j1), "UpsertJob")
	require.Nil(m.state.UpsertJob(m.nextIndex(), j2), "UpsertJob")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d1), "UpsertDeployment")
	require.Nil(m.state.UpsertDeployment(m.nextIndex(), d2), "UpsertDeployment")
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a1}), "UpsertAllocs")
	require.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a2}), "UpsertAllocs")

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
	require.Nil(m.state.UpdateDeploymentAllocHealth(m.nextIndex(), req), "UpsertDeploymentAllocHealth")

	req2 := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:         d2.ID,
			HealthyAllocationIDs: []string{a2.ID},
		},
	}
	require.Nil(m.state.UpdateDeploymentAllocHealth(m.nextIndex(), req2), "UpsertDeploymentAllocHealth")

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
