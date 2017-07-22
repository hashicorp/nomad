package deploymentwatcher

import (
	"fmt"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	mocker "github.com/stretchr/testify/mock"
)

func testDeploymentWatcher(t *testing.T, qps float64, batchDur time.Duration) (*Watcher, *mockBackend) {
	m := newMockBackend(t)
	w := NewDeploymentsWatcher(testLogger(), m, m, qps, batchDur)
	return w, m
}

func defaultTestDeploymentWatcher(t *testing.T) (*Watcher, *mockBackend) {
	return testDeploymentWatcher(t, LimitStateQueriesPerSecond, CrossDeploymentEvalBatchDuration)
}

// Tests that the watcher properly watches for deployments and reconciles them
func TestWatcher_WatchDeployments(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Return no allocations or evals
	m.On("Allocations", mocker.Anything, mocker.Anything).Return(nil).Run(func(args mocker.Arguments) {
		reply := args.Get(1).(*structs.AllocListResponse)
		reply.Index = m.nextIndex()
	})
	m.On("Evaluations", mocker.Anything, mocker.Anything).Return(nil).Run(func(args mocker.Arguments) {
		reply := args.Get(1).(*structs.JobEvaluationsResponse)
		reply.Index = m.nextIndex()
	})

	// Create three jobs
	j1, j2, j3 := mock.Job(), mock.Job(), mock.Job()
	jobs := map[string]*structs.Job{
		j1.ID: j1,
		j2.ID: j2,
		j3.ID: j3,
	}

	// Create three deployments all running
	d1, d2, d3 := mock.Deployment(), mock.Deployment(), mock.Deployment()
	d1.JobID = j1.ID
	d2.JobID = j2.ID
	d3.JobID = j3.ID

	m.On("GetJob", mocker.Anything, mocker.Anything).
		Return(nil).Run(func(args mocker.Arguments) {
		in := args.Get(0).(*structs.JobSpecificRequest)
		reply := args.Get(1).(*structs.SingleJobResponse)
		reply.Job = jobs[in.JobID]
		reply.Index = reply.Job.ModifyIndex
	})

	// Set up the calls for retrieving deployments
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(func(args mocker.Arguments) {
		reply := args.Get(1).(*structs.DeploymentListResponse)
		reply.Deployments = []*structs.Deployment{d1}
		reply.Index = m.nextIndex()
	}).Once()

	// Next list 3
	block1 := make(chan time.Time)
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(func(args mocker.Arguments) {
		reply := args.Get(1).(*structs.DeploymentListResponse)
		reply.Deployments = []*structs.Deployment{d1, d2, d3}
		reply.Index = m.nextIndex()
	}).Once().WaitUntil(block1)

	//// Next list 3 but have one be terminal
	block2 := make(chan time.Time)
	d3terminal := d3.Copy()
	d3terminal.Status = structs.DeploymentStatusFailed
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(func(args mocker.Arguments) {
		reply := args.Get(1).(*structs.DeploymentListResponse)
		reply.Deployments = []*structs.Deployment{d1, d2, d3terminal}
		reply.Index = m.nextIndex()
	}).WaitUntil(block2)

	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(func(args mocker.Arguments) {
		reply := args.Get(1).(*structs.DeploymentListResponse)
		reply.Deployments = []*structs.Deployment{d1, d2, d3terminal}
		reply.Index = m.nextIndex()
	})

	w.SetEnabled(true)
	testutil.WaitForResult(func() (bool, error) { return 1 == len(w.watchers), nil },
		func(err error) { assert.Equal(1, len(w.watchers), "1 deployment returned") })

	close(block1)
	testutil.WaitForResult(func() (bool, error) { return 3 == len(w.watchers), nil },
		func(err error) { assert.Equal(3, len(w.watchers), "3 deployment returned") })

	close(block2)
	testutil.WaitForResult(func() (bool, error) { return 2 == len(w.watchers), nil },
		func(err error) { assert.Equal(3, len(w.watchers), "3 deployment returned - 1 terminal") })
}

// Tests that calls against an unknown deployment fail
func TestWatcher_UnknownDeployment(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	w, m := defaultTestDeploymentWatcher(t)
	w.SetEnabled(true)

	// Set up the calls for retrieving deployments
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(func(args mocker.Arguments) {
		reply := args.Get(1).(*structs.DeploymentListResponse)
		reply.Index = m.nextIndex()
	})
	m.On("GetDeployment", mocker.Anything, mocker.Anything).Return(nil).Run(func(args mocker.Arguments) {
		reply := args.Get(1).(*structs.SingleDeploymentResponse)
		reply.Index = m.nextIndex()
	})

	// The expected error is that it should be an unknown deployment
	dID := structs.GenerateUUID()
	expected := fmt.Sprintf("unknown deployment %q", dID)

	// Request setting the health against an unknown deployment
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         dID,
		HealthyAllocationIDs: []string{structs.GenerateUUID()},
	}
	var resp structs.DeploymentUpdateResponse
	err := w.SetAllocHealth(req, &resp)
	if assert.NotNil(err, "should have error for unknown deployment") {
		assert.Contains(err.Error(), expected)
	}

	// Request promoting against an unknown deployment
	req2 := &structs.DeploymentPromoteRequest{
		DeploymentID: dID,
		All:          true,
	}
	err = w.PromoteDeployment(req2, &resp)
	if assert.NotNil(err, "should have error for unknown deployment") {
		assert.Contains(err.Error(), expected)
	}

	// Request pausing against an unknown deployment
	req3 := &structs.DeploymentPauseRequest{
		DeploymentID: dID,
		Pause:        true,
	}
	err = w.PauseDeployment(req3, &resp)
	if assert.NotNil(err, "should have error for unknown deployment") {
		assert.Contains(err.Error(), expected)
	}

	// Request failing against an unknown deployment
	req4 := &structs.DeploymentFailRequest{
		DeploymentID: dID,
	}
	err = w.FailDeployment(req4, &resp)
	if assert.NotNil(err, "should have error for unknown deployment") {
		assert.Contains(err.Error(), expected)
	}
}

// Test setting an unknown allocation's health
func TestWatcher_SetAllocHealth_Unknown(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job, and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")

	// Assert the following methods will be called
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(m.listFromState)
	m.On("Allocations", mocker.MatchedBy(matchDeploymentSpecificRequest(d.ID)),
		mocker.Anything).Return(nil).Run(m.allocationsFromState)
	m.On("Evaluations", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.evaluationsFromState)
	m.On("GetJob", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.getJobFromState)

	w.SetEnabled(true)
	testutil.WaitForResult(func() (bool, error) { return 1 == len(w.watchers), nil },
		func(err error) { assert.Equal(1, len(w.watchers), "Should have 1 deployment") })

	// Assert that we get a call to UpsertDeploymentAllocHealth
	a := mock.Alloc()
	matchConfig := &matchDeploymentAllocHealthRequestConfig{
		DeploymentID: d.ID,
		Healthy:      []string{a.ID},
		Eval:         true,
	}
	matcher := matchDeploymentAllocHealthRequest(matchConfig)
	m.On("UpdateDeploymentAllocHealth", mocker.MatchedBy(matcher)).Return(nil)

	// Call SetAllocHealth
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         d.ID,
		HealthyAllocationIDs: []string{a.ID},
	}
	var resp structs.DeploymentUpdateResponse
	err := w.SetAllocHealth(req, &resp)
	if assert.NotNil(err, "Set health of unknown allocation") {
		assert.Contains(err.Error(), "unknown")
	}
	assert.Equal(1, len(w.watchers), "Deployment should still be active")
}

// Test setting allocation health
func TestWatcher_SetAllocHealth_Healthy(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	a.DeploymentID = d.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	assert.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// Assert the following methods will be called
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(m.listFromState)
	m.On("Allocations", mocker.MatchedBy(matchDeploymentSpecificRequest(d.ID)),
		mocker.Anything).Return(nil).Run(m.allocationsFromState)
	m.On("Evaluations", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.evaluationsFromState)
	m.On("GetJob", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.getJobFromState)

	w.SetEnabled(true)
	testutil.WaitForResult(func() (bool, error) { return 1 == len(w.watchers), nil },
		func(err error) { assert.Equal(1, len(w.watchers), "Should have 1 deployment") })

	// Assert that we get a call to UpsertDeploymentAllocHealth
	matchConfig := &matchDeploymentAllocHealthRequestConfig{
		DeploymentID: d.ID,
		Healthy:      []string{a.ID},
		Eval:         true,
	}
	matcher := matchDeploymentAllocHealthRequest(matchConfig)
	m.On("UpdateDeploymentAllocHealth", mocker.MatchedBy(matcher)).Return(nil)

	// Call SetAllocHealth
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         d.ID,
		HealthyAllocationIDs: []string{a.ID},
	}
	var resp structs.DeploymentUpdateResponse
	err := w.SetAllocHealth(req, &resp)
	assert.Nil(err, "SetAllocHealth")
	assert.Equal(1, len(w.watchers), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentAllocHealth", mocker.MatchedBy(matcher))
}

// Test setting allocation unhealthy
func TestWatcher_SetAllocHealth_Unhealthy(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	a.DeploymentID = d.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	assert.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// Assert the following methods will be called
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(m.listFromState)
	m.On("Allocations", mocker.MatchedBy(matchDeploymentSpecificRequest(d.ID)),
		mocker.Anything).Return(nil).Run(m.allocationsFromState)
	m.On("Evaluations", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.evaluationsFromState)
	m.On("GetJob", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.getJobFromState)

	w.SetEnabled(true)
	testutil.WaitForResult(func() (bool, error) { return 1 == len(w.watchers), nil },
		func(err error) { assert.Equal(1, len(w.watchers), "Should have 1 deployment") })

	// Assert that we get a call to UpsertDeploymentAllocHealth
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

	// Call SetAllocHealth
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:           d.ID,
		UnhealthyAllocationIDs: []string{a.ID},
	}
	var resp structs.DeploymentUpdateResponse
	err := w.SetAllocHealth(req, &resp)
	assert.Nil(err, "SetAllocHealth")

	testutil.WaitForResult(func() (bool, error) { return 0 == len(w.watchers), nil },
		func(err error) { assert.Equal(0, len(w.watchers), "Should have no deployment") })
	m.AssertNumberOfCalls(t, "UpdateDeploymentAllocHealth", 1)
}

// Test setting allocation unhealthy and that there should be a rollback
func TestWatcher_SetAllocHealth_Unhealthy_Rollback(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.AutoRevert = true
	j.Stable = true
	d := mock.Deployment()
	d.JobID = j.ID
	d.TaskGroups["web"].AutoRevert = true
	a := mock.Alloc()
	a.DeploymentID = d.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	assert.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// Upsert the job again to get a new version
	j2 := j.Copy()
	j2.Stable = false
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j2), "UpsertJob2")

	// Assert the following methods will be called
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(m.listFromState)
	m.On("Allocations", mocker.MatchedBy(matchDeploymentSpecificRequest(d.ID)),
		mocker.Anything).Return(nil).Run(m.allocationsFromState)
	m.On("Evaluations", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.evaluationsFromState)
	m.On("GetJob", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.getJobFromState)
	m.On("GetJobVersions", mocker.MatchedBy(matchJobVersionsRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.getJobVersionsFromState)

	w.SetEnabled(true)
	testutil.WaitForResult(func() (bool, error) { return 1 == len(w.watchers), nil },
		func(err error) { assert.Equal(1, len(w.watchers), "Should have 1 deployment") })

	// Assert that we get a call to UpsertDeploymentAllocHealth
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

	// Call SetAllocHealth
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:           d.ID,
		UnhealthyAllocationIDs: []string{a.ID},
	}
	var resp structs.DeploymentUpdateResponse
	err := w.SetAllocHealth(req, &resp)
	assert.Nil(err, "SetAllocHealth")

	testutil.WaitForResult(func() (bool, error) { return 0 == len(w.watchers), nil },
		func(err error) { assert.Equal(0, len(w.watchers), "Should have no deployment") })
	m.AssertNumberOfCalls(t, "UpdateDeploymentAllocHealth", 1)
}

// Test promoting a deployment
func TestWatcher_PromoteDeployment_HealthyCanaries(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job, canary alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.Canary = 2
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	d.TaskGroups[a.TaskGroup].PlacedCanaries = []string{a.ID}
	a.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
	}
	a.DeploymentID = d.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	assert.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// Assert the following methods will be called
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(m.listFromState)
	m.On("Allocations", mocker.MatchedBy(matchDeploymentSpecificRequest(d.ID)),
		mocker.Anything).Return(nil).Run(m.allocationsFromState)
	m.On("Evaluations", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.evaluationsFromState)
	m.On("GetJob", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.getJobFromState)

	w.SetEnabled(true)
	testutil.WaitForResult(func() (bool, error) { return 1 == len(w.watchers), nil },
		func(err error) { assert.Equal(1, len(w.watchers), "Should have 1 deployment") })

	// Assert that we get a call to UpsertDeploymentPromotion
	matchConfig := &matchDeploymentPromoteRequestConfig{
		Promotion: &structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
		Eval: true,
	}
	matcher := matchDeploymentPromoteRequest(matchConfig)
	m.On("UpdateDeploymentPromotion", mocker.MatchedBy(matcher)).Return(nil)

	// Call PromoteDeployment
	req := &structs.DeploymentPromoteRequest{
		DeploymentID: d.ID,
		All:          true,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PromoteDeployment(req, &resp)
	assert.Nil(err, "PromoteDeployment")
	assert.Equal(1, len(w.watchers), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentPromotion", mocker.MatchedBy(matcher))
}

// Test promoting a deployment with unhealthy canaries
func TestWatcher_PromoteDeployment_UnhealthyCanaries(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job, canary alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.Canary = 2
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	d.TaskGroups[a.TaskGroup].PlacedCanaries = []string{a.ID}
	a.DeploymentID = d.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	assert.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// Assert the following methods will be called
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(m.listFromState)
	m.On("Allocations", mocker.MatchedBy(matchDeploymentSpecificRequest(d.ID)),
		mocker.Anything).Return(nil).Run(m.allocationsFromState)
	m.On("Evaluations", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.evaluationsFromState)
	m.On("GetJob", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.getJobFromState)

	w.SetEnabled(true)
	testutil.WaitForResult(func() (bool, error) { return 1 == len(w.watchers), nil },
		func(err error) { assert.Equal(1, len(w.watchers), "Should have 1 deployment") })

	// Assert that we get a call to UpsertDeploymentPromotion
	matchConfig := &matchDeploymentPromoteRequestConfig{
		Promotion: &structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
		Eval: true,
	}
	matcher := matchDeploymentPromoteRequest(matchConfig)
	m.On("UpdateDeploymentPromotion", mocker.MatchedBy(matcher)).Return(nil)

	// Call SetAllocHealth
	req := &structs.DeploymentPromoteRequest{
		DeploymentID: d.ID,
		All:          true,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PromoteDeployment(req, &resp)
	if assert.NotNil(err, "PromoteDeployment") {
		assert.Contains(err.Error(), "is not healthy", "Should error because canary isn't marked healthy")
	}

	assert.Equal(1, len(w.watchers), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentPromotion", mocker.MatchedBy(matcher))
}

// Test pausing a deployment that is running
func TestWatcher_PauseDeployment_Pause_Running(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")

	// Assert the following methods will be called
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(m.listFromState)
	m.On("Allocations", mocker.MatchedBy(matchDeploymentSpecificRequest(d.ID)),
		mocker.Anything).Return(nil).Run(m.allocationsFromState)
	m.On("Evaluations", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.evaluationsFromState)
	m.On("GetJob", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.getJobFromState)

	w.SetEnabled(true)
	testutil.WaitForResult(func() (bool, error) { return 1 == len(w.watchers), nil },
		func(err error) { assert.Equal(1, len(w.watchers), "Should have 1 deployment") })

	// Assert that we get a call to UpsertDeploymentStatusUpdate
	matchConfig := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusPaused,
		StatusDescription: structs.DeploymentStatusDescriptionPaused,
	}
	matcher := matchDeploymentStatusUpdateRequest(matchConfig)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(matcher)).Return(nil)

	// Call PauseDeployment
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        true,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PauseDeployment(req, &resp)
	assert.Nil(err, "PauseDeployment")

	assert.Equal(1, len(w.watchers), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentStatus", mocker.MatchedBy(matcher))
}

// Test pausing a deployment that is paused
func TestWatcher_PauseDeployment_Pause_Paused(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	d.Status = structs.DeploymentStatusPaused
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")

	// Assert the following methods will be called
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(m.listFromState)
	m.On("Allocations", mocker.MatchedBy(matchDeploymentSpecificRequest(d.ID)),
		mocker.Anything).Return(nil).Run(m.allocationsFromState)
	m.On("Evaluations", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.evaluationsFromState)
	m.On("GetJob", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.getJobFromState)

	w.SetEnabled(true)
	testutil.WaitForResult(func() (bool, error) { return 1 == len(w.watchers), nil },
		func(err error) { assert.Equal(1, len(w.watchers), "Should have 1 deployment") })

	// Assert that we get a call to UpsertDeploymentStatusUpdate
	matchConfig := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusPaused,
		StatusDescription: structs.DeploymentStatusDescriptionPaused,
	}
	matcher := matchDeploymentStatusUpdateRequest(matchConfig)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(matcher)).Return(nil)

	// Call PauseDeployment
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        true,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PauseDeployment(req, &resp)
	assert.Nil(err, "PauseDeployment")

	assert.Equal(1, len(w.watchers), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentStatus", mocker.MatchedBy(matcher))
}

// Test unpausing a deployment that is paused
func TestWatcher_PauseDeployment_Unpause_Paused(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	d.Status = structs.DeploymentStatusPaused
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")

	// Assert the following methods will be called
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(m.listFromState)
	m.On("Allocations", mocker.MatchedBy(matchDeploymentSpecificRequest(d.ID)),
		mocker.Anything).Return(nil).Run(m.allocationsFromState)
	m.On("Evaluations", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.evaluationsFromState)
	m.On("GetJob", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.getJobFromState)

	w.SetEnabled(true)
	testutil.WaitForResult(func() (bool, error) { return 1 == len(w.watchers), nil },
		func(err error) { assert.Equal(1, len(w.watchers), "Should have 1 deployment") })

	// Assert that we get a call to UpsertDeploymentStatusUpdate
	matchConfig := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusRunning,
		StatusDescription: structs.DeploymentStatusDescriptionRunning,
		Eval:              true,
	}
	matcher := matchDeploymentStatusUpdateRequest(matchConfig)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(matcher)).Return(nil)

	// Call PauseDeployment
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        false,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PauseDeployment(req, &resp)
	assert.Nil(err, "PauseDeployment")

	assert.Equal(1, len(w.watchers), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentStatus", mocker.MatchedBy(matcher))
}

// Test unpausing a deployment that is running
func TestWatcher_PauseDeployment_Unpause_Running(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")

	// Assert the following methods will be called
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(m.listFromState)
	m.On("Allocations", mocker.MatchedBy(matchDeploymentSpecificRequest(d.ID)),
		mocker.Anything).Return(nil).Run(m.allocationsFromState)
	m.On("Evaluations", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.evaluationsFromState)
	m.On("GetJob", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.getJobFromState)

	w.SetEnabled(true)
	testutil.WaitForResult(func() (bool, error) { return 1 == len(w.watchers), nil },
		func(err error) { assert.Equal(1, len(w.watchers), "Should have 1 deployment") })

	// Assert that we get a call to UpsertDeploymentStatusUpdate
	matchConfig := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusRunning,
		StatusDescription: structs.DeploymentStatusDescriptionRunning,
		Eval:              true,
	}
	matcher := matchDeploymentStatusUpdateRequest(matchConfig)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(matcher)).Return(nil)

	// Call PauseDeployment
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        false,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PauseDeployment(req, &resp)
	assert.Nil(err, "PauseDeployment")

	assert.Equal(1, len(w.watchers), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentStatus", mocker.MatchedBy(matcher))
}

// Test failing a deployment that is running
func TestWatcher_FailDeployment_Running(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	w, m := defaultTestDeploymentWatcher(t)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")

	// Assert the following methods will be called
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(m.listFromState)
	m.On("Allocations", mocker.MatchedBy(matchDeploymentSpecificRequest(d.ID)),
		mocker.Anything).Return(nil).Run(m.allocationsFromState)
	m.On("Evaluations", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.evaluationsFromState)
	m.On("GetJob", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.getJobFromState)

	w.SetEnabled(true)
	testutil.WaitForResult(func() (bool, error) { return 1 == len(w.watchers), nil },
		func(err error) { assert.Equal(1, len(w.watchers), "Should have 1 deployment") })

	// Assert that we get a call to UpsertDeploymentStatusUpdate
	matchConfig := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusFailed,
		StatusDescription: structs.DeploymentStatusDescriptionFailedByUser,
		Eval:              true,
	}
	matcher := matchDeploymentStatusUpdateRequest(matchConfig)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(matcher)).Return(nil)

	// Call PauseDeployment
	req := &structs.DeploymentFailRequest{
		DeploymentID: d.ID,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.FailDeployment(req, &resp)
	assert.Nil(err, "FailDeployment")

	assert.Equal(1, len(w.watchers), "Deployment should still be active")
	m.AssertCalled(t, "UpdateDeploymentStatus", mocker.MatchedBy(matcher))
}

// Tests that the watcher properly watches for allocation changes and takes the
// proper actions
func TestDeploymentWatcher_Watch(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	w, m := testDeploymentWatcher(t, 1000.0, 1*time.Millisecond)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.AutoRevert = true
	j.Stable = true
	d := mock.Deployment()
	d.JobID = j.ID
	d.TaskGroups["web"].AutoRevert = true
	a := mock.Alloc()
	a.DeploymentID = d.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d), "UpsertDeployment")
	assert.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a}), "UpsertAllocs")

	// Upsert the job again to get a new version
	j2 := j.Copy()
	j2.Stable = false
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j2), "UpsertJob2")

	// Assert the following methods will be called
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(m.listFromState)
	m.On("Allocations", mocker.MatchedBy(matchDeploymentSpecificRequest(d.ID)),
		mocker.Anything).Return(nil).Run(m.allocationsFromState)
	m.On("Evaluations", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.evaluationsFromState)
	m.On("GetJob", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.getJobFromState)
	m.On("GetJobVersions", mocker.MatchedBy(matchJobVersionsRequest(j.ID)),
		mocker.Anything).Return(nil).Run(m.getJobVersionsFromState)

	w.SetEnabled(true)
	testutil.WaitForResult(func() (bool, error) { return 1 == len(w.watchers), nil },
		func(err error) { assert.Equal(1, len(w.watchers), "Should have 1 deployment") })

	// Assert that we will get a createEvaluation call only once. This will
	// verify that the watcher is batching allocation changes
	m1 := matchUpsertEvals([]string{d.ID})
	m.On("UpsertEvals", mocker.MatchedBy(m1)).Return(nil).Once()

	// Update the allocs health to healthy which should create an evaluation
	for i := 0; i < 5; i++ {
		req := &structs.ApplyDeploymentAllocHealthRequest{
			DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
				DeploymentID:         d.ID,
				HealthyAllocationIDs: []string{a.ID},
			},
		}
		assert.Nil(m.state.UpdateDeploymentAllocHealth(m.nextIndex(), req), "UpsertDeploymentAllocHealth")
	}

	// Wait for there to be one eval
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		evals, err := m.state.EvalsByJob(ws, j.ID)
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

	// Assert that we get a call to UpsertDeploymentStatusUpdate
	c := &matchDeploymentStatusUpdateConfig{
		DeploymentID:      d.ID,
		Status:            structs.DeploymentStatusFailed,
		StatusDescription: structs.DeploymentStatusDescriptionRollback(structs.DeploymentStatusDescriptionFailedAllocations, 0),
		JobVersion:        helper.Uint64ToPtr(0),
		Eval:              true,
	}
	m2 := matchDeploymentStatusUpdateRequest(c)
	m.On("UpdateDeploymentStatus", mocker.MatchedBy(m2)).Return(nil)

	// Update the allocs health to unhealthy which should create a job rollback,
	// status update and eval
	req2 := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:           d.ID,
			UnhealthyAllocationIDs: []string{a.ID},
		},
	}
	assert.Nil(m.state.UpdateDeploymentAllocHealth(m.nextIndex(), req2), "UpsertDeploymentAllocHealth")

	// Wait for there to be one eval
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		evals, err := m.state.EvalsByJob(ws, j.ID)
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

	m.AssertCalled(t, "UpsertEvals", mocker.MatchedBy(m1))

	// After we upsert the job version will go to 2. So use this to assert the
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
	testutil.WaitForResult(func() (bool, error) { return 0 == len(w.watchers), nil },
		func(err error) { assert.Equal(0, len(w.watchers), "Should have no deployment") })
}

// Test evaluations are batched between watchers
func TestWatcher_BatchEvals(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	w, m := testDeploymentWatcher(t, 1000.0, 1*time.Second)

	// Create a job, alloc, for two deployments
	j1 := mock.Job()
	d1 := mock.Deployment()
	d1.JobID = j1.ID
	a1 := mock.Alloc()
	a1.DeploymentID = d1.ID

	j2 := mock.Job()
	d2 := mock.Deployment()
	d2.JobID = j2.ID
	a2 := mock.Alloc()
	a2.DeploymentID = d2.ID

	assert.Nil(m.state.UpsertJob(m.nextIndex(), j1), "UpsertJob")
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j2), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d1), "UpsertDeployment")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d2), "UpsertDeployment")
	assert.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a1}), "UpsertAllocs")
	assert.Nil(m.state.UpsertAllocs(m.nextIndex(), []*structs.Allocation{a2}), "UpsertAllocs")

	// Assert the following methods will be called
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(m.listFromState)

	m.On("Allocations", mocker.MatchedBy(matchDeploymentSpecificRequest(d1.ID)),
		mocker.Anything).Return(nil).Run(m.allocationsFromState)
	m.On("Allocations", mocker.MatchedBy(matchDeploymentSpecificRequest(d2.ID)),
		mocker.Anything).Return(nil).Run(m.allocationsFromState)

	m.On("Evaluations", mocker.MatchedBy(matchJobSpecificRequest(j1.ID)),
		mocker.Anything).Return(nil).Run(m.evaluationsFromState)
	m.On("Evaluations", mocker.MatchedBy(matchJobSpecificRequest(j2.ID)),
		mocker.Anything).Return(nil).Run(m.evaluationsFromState)

	m.On("GetJob", mocker.MatchedBy(matchJobSpecificRequest(j1.ID)),
		mocker.Anything).Return(nil).Run(m.getJobFromState)
	m.On("GetJob", mocker.MatchedBy(matchJobSpecificRequest(j2.ID)),
		mocker.Anything).Return(nil).Run(m.getJobFromState)

	m.On("GetJobVersions", mocker.MatchedBy(matchJobVersionsRequest(j1.ID)),
		mocker.Anything).Return(nil).Run(m.getJobVersionsFromState)
	m.On("GetJobVersions", mocker.MatchedBy(matchJobVersionsRequest(j2.ID)),
		mocker.Anything).Return(nil).Run(m.getJobVersionsFromState)

	w.SetEnabled(true)
	testutil.WaitForResult(func() (bool, error) { return 2 == len(w.watchers), nil },
		func(err error) { assert.Equal(2, len(w.watchers), "Should have 2 deployment") })

	// Assert that we will get a createEvaluation call only once and it contains
	// both deployments. This will verify that the watcher is batching
	// allocation changes
	m1 := matchUpsertEvals([]string{d1.ID, d2.ID})
	m.On("UpsertEvals", mocker.MatchedBy(m1)).Return(nil).Once()

	// Update the allocs health to healthy which should create an evaluation
	req := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:         d1.ID,
			HealthyAllocationIDs: []string{a1.ID},
		},
	}
	assert.Nil(m.state.UpdateDeploymentAllocHealth(m.nextIndex(), req), "UpsertDeploymentAllocHealth")

	req2 := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:         d2.ID,
			HealthyAllocationIDs: []string{a2.ID},
		},
	}
	assert.Nil(m.state.UpdateDeploymentAllocHealth(m.nextIndex(), req2), "UpsertDeploymentAllocHealth")

	// Wait for there to be one eval for each job
	testutil.WaitForResult(func() (bool, error) {
		ws := memdb.NewWatchSet()
		evals1, err := m.state.EvalsByJob(ws, j1.ID)
		if err != nil {
			return false, err
		}

		evals2, err := m.state.EvalsByJob(ws, j2.ID)
		if err != nil {
			return false, err
		}

		if l := len(evals1); l != 1 {
			return false, fmt.Errorf("Got %d evals; want 1", l)
		}

		if l := len(evals2); l != 1 {
			return false, fmt.Errorf("Got %d evals; want 1", l)
		}

		return true, nil
	}, func(err error) {
		t.Fatal(err)
	})

	m.AssertCalled(t, "UpsertEvals", mocker.MatchedBy(m1))
	testutil.WaitForResult(func() (bool, error) { return 2 == len(w.watchers), nil },
		func(err error) { assert.Equal(2, len(w.watchers), "Should have 2 deployment") })
}
