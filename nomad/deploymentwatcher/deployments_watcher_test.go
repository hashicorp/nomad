package deploymentwatcher

import (
	"log"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	mocker "github.com/stretchr/testify/mock"
)

// TODO
// Test evaluations are batched between watchers
// Test allocation watcher
// Test that evaluation due to allocation changes are batched

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

type mockBackend struct {
	mocker.Mock
	index uint64
	state *state.StateStore
	l     sync.Mutex
}

func newMockBackend(t *testing.T) *mockBackend {
	state, err := state.NewStateStore(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if state == nil {
		t.Fatalf("missing state")
	}
	return &mockBackend{
		index: 10000,
		state: state,
	}
}

func (m *mockBackend) nextIndex() uint64 {
	m.l.Lock()
	defer m.l.Unlock()
	i := m.index
	m.index++
	return i
}

func (m *mockBackend) UpsertEvals(evals []*structs.Evaluation) (uint64, error) {
	m.Called(evals)
	i := m.nextIndex()
	return i, m.state.UpsertEvals(i, evals)
}

func (m *mockBackend) UpsertJob(job *structs.Job) (uint64, error) {
	m.Called(job)
	i := m.nextIndex()
	return i, m.state.UpsertJob(i, job)
}

func (m *mockBackend) UpsertDeploymentStatusUpdate(u *structs.DeploymentStatusUpdateRequest) (uint64, error) {
	m.Called(u)
	i := m.nextIndex()
	return i, m.state.UpsertDeploymentStatusUpdate(i, u)
}

// matchDeploymentStatusUpdateConfig is used to configure the matching
// function
type matchDeploymentStatusUpdateConfig struct {
	// DeploymentID is the expected ID
	DeploymentID string

	// Status is the desired status
	Status string

	// StatusDescription is the desired status description
	StatusDescription string

	// JobVersion marks whether we expect a roll back job at the given version
	JobVersion *uint64

	// Eval marks whether we expect an evaluation.
	Eval bool
}

// matchDeploymentStatusUpdateRequest is used to match an update request
func matchDeploymentStatusUpdateRequest(c *matchDeploymentStatusUpdateConfig) func(args *structs.DeploymentStatusUpdateRequest) bool {
	return func(args *structs.DeploymentStatusUpdateRequest) bool {
		if args.DeploymentUpdate.DeploymentID != c.DeploymentID {
			return false
		}

		if args.DeploymentUpdate.Status != c.Status && args.DeploymentUpdate.StatusDescription != c.StatusDescription {
			return false
		}

		if c.Eval && args.Eval == nil || !c.Eval && args.Eval != nil {
			return false
		}

		if (c.JobVersion != nil && (args.Job == nil || args.Job.Version != *c.JobVersion)) || c.JobVersion == nil && args.Job != nil {
			return false
		}

		return true
	}
}

func (m *mockBackend) UpsertDeploymentPromotion(req *structs.ApplyDeploymentPromoteRequest) (uint64, error) {
	m.Called(req)
	i := m.nextIndex()
	return i, m.state.UpsertDeploymentPromotion(i, req)
}

// matchDeploymentPromoteRequestConfig is used to configure the matching
// function
type matchDeploymentPromoteRequestConfig struct {
	// Promotion holds the expected promote request
	Promotion *structs.DeploymentPromoteRequest

	// Eval marks whether we expect an evaluation.
	Eval bool
}

// matchDeploymentPromoteRequest is used to match a promote request
func matchDeploymentPromoteRequest(c *matchDeploymentPromoteRequestConfig) func(args *structs.ApplyDeploymentPromoteRequest) bool {
	return func(args *structs.ApplyDeploymentPromoteRequest) bool {
		if !reflect.DeepEqual(*c.Promotion, args.DeploymentPromoteRequest) {
			return false
		}

		if c.Eval && args.Eval == nil || !c.Eval && args.Eval != nil {
			return false
		}

		return true
	}
}
func (m *mockBackend) UpsertDeploymentAllocHealth(req *structs.ApplyDeploymentAllocHealthRequest) (uint64, error) {
	m.Called(req)
	i := m.nextIndex()
	return i, m.state.UpsertDeploymentAllocHealth(i, req)
}

// matchDeploymentAllocHealthRequestConfig is used to configure the matching
// function
type matchDeploymentAllocHealthRequestConfig struct {
	// DeploymentID is the expected ID
	DeploymentID string

	// Healthy and Unhealthy contain the expected allocation IDs that are having
	// their health set
	Healthy, Unhealthy []string

	// DeploymentUpdate holds the expected values of status and description. We
	// don't check for exact match but string contains
	DeploymentUpdate *structs.DeploymentStatusUpdate

	// JobVersion marks whether we expect a roll back job at the given version
	JobVersion *uint64

	// Eval marks whether we expect an evaluation.
	Eval bool
}

// matchDeploymentAllocHealthRequest is used to match an update request
func matchDeploymentAllocHealthRequest(c *matchDeploymentAllocHealthRequestConfig) func(args *structs.ApplyDeploymentAllocHealthRequest) bool {
	return func(args *structs.ApplyDeploymentAllocHealthRequest) bool {
		if args.DeploymentID != c.DeploymentID {
			return false
		}

		if len(c.Healthy) != len(args.HealthyAllocationIDs) {
			return false
		}
		if len(c.Unhealthy) != len(args.UnhealthyAllocationIDs) {
			return false
		}

		hmap, umap := make(map[string]struct{}, len(c.Healthy)), make(map[string]struct{}, len(c.Unhealthy))
		for _, h := range c.Healthy {
			hmap[h] = struct{}{}
		}
		for _, u := range c.Unhealthy {
			umap[u] = struct{}{}
		}

		for _, h := range args.HealthyAllocationIDs {
			if _, ok := hmap[h]; !ok {
				return false
			}
		}
		for _, u := range args.UnhealthyAllocationIDs {
			if _, ok := umap[u]; !ok {
				return false
			}
		}

		if c.DeploymentUpdate != nil {
			if args.DeploymentUpdate == nil {
				return false
			}

			if !strings.Contains(args.DeploymentUpdate.Status, c.DeploymentUpdate.Status) {
				return false
			}
			if !strings.Contains(args.DeploymentUpdate.StatusDescription, c.DeploymentUpdate.StatusDescription) {
				return false
			}
		} else if args.DeploymentUpdate != nil {
			return false
		}

		if c.Eval && args.Eval == nil || !c.Eval && args.Eval != nil {
			return false
		}

		if (c.JobVersion != nil && (args.Job == nil || args.Job.Version != *c.JobVersion)) || c.JobVersion == nil && args.Job != nil {
			return false
		}

		return true
	}
}

func (m *mockBackend) Evaluations(args *structs.JobSpecificRequest, reply *structs.JobEvaluationsResponse) error {
	rargs := m.Called(args, reply)
	return rargs.Error(0)
}

func (m *mockBackend) evaluationsFromState(in mocker.Arguments) {
	args, reply := in.Get(0).(*structs.JobSpecificRequest), in.Get(1).(*structs.JobEvaluationsResponse)
	ws := memdb.NewWatchSet()
	evals, _ := m.state.EvalsByJob(ws, args.JobID)
	reply.Evaluations = evals
	reply.Index = m.nextIndex()
}

func (m *mockBackend) Allocations(args *structs.DeploymentSpecificRequest, reply *structs.AllocListResponse) error {
	rargs := m.Called(args, reply)
	return rargs.Error(0)
}

func (m *mockBackend) allocationsFromState(in mocker.Arguments) {
	args, reply := in.Get(0).(*structs.DeploymentSpecificRequest), in.Get(1).(*structs.AllocListResponse)
	ws := memdb.NewWatchSet()
	allocs, _ := m.state.AllocsByDeployment(ws, args.DeploymentID)

	var stubs []*structs.AllocListStub
	for _, a := range allocs {
		stubs = append(stubs, a.Stub())
	}

	reply.Allocations = stubs
	reply.Index = m.nextIndex()
}

func (m *mockBackend) List(args *structs.DeploymentListRequest, reply *structs.DeploymentListResponse) error {
	rargs := m.Called(args, reply)
	return rargs.Error(0)
}

func (m *mockBackend) listFromState(in mocker.Arguments) {
	reply := in.Get(1).(*structs.DeploymentListResponse)
	ws := memdb.NewWatchSet()
	iter, _ := m.state.Deployments(ws)

	var deploys []*structs.Deployment
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		deploys = append(deploys, raw.(*structs.Deployment))
	}

	reply.Deployments = deploys
	reply.Index = m.nextIndex()
}

func (m *mockBackend) GetJobVersions(args *structs.JobSpecificRequest, reply *structs.JobVersionsResponse) error {
	rargs := m.Called(args, reply)
	return rargs.Error(0)
}

func (m *mockBackend) getJobVersionsFromState(in mocker.Arguments) {
	args, reply := in.Get(0).(*structs.JobSpecificRequest), in.Get(1).(*structs.JobVersionsResponse)
	ws := memdb.NewWatchSet()
	versions, _ := m.state.JobVersionsByID(ws, args.JobID)
	reply.Versions = versions
	reply.Index = m.nextIndex()
}

func (m *mockBackend) GetJob(args *structs.JobSpecificRequest, reply *structs.SingleJobResponse) error {
	rargs := m.Called(args, reply)
	return rargs.Error(0)
}

func (m *mockBackend) getJobFromState(in mocker.Arguments) {
	args, reply := in.Get(0).(*structs.JobSpecificRequest), in.Get(1).(*structs.SingleJobResponse)
	ws := memdb.NewWatchSet()
	job, _ := m.state.JobByID(ws, args.JobID)
	reply.Job = job
	reply.Index = m.nextIndex()
}

// matchDeploymentSpecificRequest is used to match that a deployment specific
// request is for the passed deployment id
func matchDeploymentSpecificRequest(dID string) func(args *structs.DeploymentSpecificRequest) bool {
	return func(args *structs.DeploymentSpecificRequest) bool {
		return args.DeploymentID == dID
	}
}

// matchJobSpecificRequest is used to match that a job specific
// request is for the passed job id
func matchJobSpecificRequest(jID string) func(args *structs.JobSpecificRequest) bool {
	return func(args *structs.JobSpecificRequest) bool {
		return args.JobID == jID
	}
}

// Tests that the watcher properly watches for deployments and reconciles them
func TestWatcher_WatchDeployments(t *testing.T) {
	assert := assert.New(t)
	m := newMockBackend(t)
	w := NewDeploymentsWatcher(testLogger(), m, m)

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
	assert := assert.New(t)
	m := newMockBackend(t)
	w := NewDeploymentsWatcher(testLogger(), m, m)
	w.SetEnabled(true)

	// Set up the calls for retrieving deployments
	m.On("List", mocker.Anything, mocker.Anything).Return(nil).Run(func(args mocker.Arguments) {
		reply := args.Get(1).(*structs.DeploymentListResponse)
		reply.Index = m.nextIndex()
	})

	// Request setting the health against an unknown deployment
	dID := structs.GenerateUUID()
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         dID,
		HealthyAllocationIDs: []string{structs.GenerateUUID()},
	}
	var resp structs.DeploymentUpdateResponse
	err := w.SetAllocHealth(req, &resp)
	if assert.NotNil(err, "should have error for unknown deployment") {
		assert.Contains(err.Error(), "not being watched")
	}

	// Request promoting against an unknown deployment
	req2 := &structs.DeploymentPromoteRequest{
		DeploymentID: dID,
		All:          true,
	}
	err = w.PromoteDeployment(req2, &resp)
	if assert.NotNil(err, "should have error for unknown deployment") {
		assert.Contains(err.Error(), "not being watched")
	}

	// Request pausing against an unknown deployment
	req3 := &structs.DeploymentPauseRequest{
		DeploymentID: dID,
		Pause:        true,
	}
	err = w.PauseDeployment(req3, &resp)
	if assert.NotNil(err, "should have error for unknown deployment") {
		assert.Contains(err.Error(), "not being watched")
	}
}

// Test setting an unknown allocation's health
func TestWatcher_SetAllocHealth_Unknown(t *testing.T) {
	assert := assert.New(t)
	m := newMockBackend(t)
	w := NewDeploymentsWatcher(testLogger(), m, m)

	// Create a job, and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d, false), "UpsertDeployment")

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
	m.On("UpsertDeploymentAllocHealth", mocker.MatchedBy(matcher)).Return(nil)

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
	assert := assert.New(t)
	m := newMockBackend(t)
	w := NewDeploymentsWatcher(testLogger(), m, m)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	a.DeploymentID = d.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d, false), "UpsertDeployment")
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
	m.On("UpsertDeploymentAllocHealth", mocker.MatchedBy(matcher)).Return(nil)

	// Call SetAllocHealth
	req := &structs.DeploymentAllocHealthRequest{
		DeploymentID:         d.ID,
		HealthyAllocationIDs: []string{a.ID},
	}
	var resp structs.DeploymentUpdateResponse
	err := w.SetAllocHealth(req, &resp)
	assert.Nil(err, "SetAllocHealth")
	assert.Equal(1, len(w.watchers), "Deployment should still be active")
	m.AssertCalled(t, "UpsertDeploymentAllocHealth", mocker.MatchedBy(matcher))
}

// Test setting allocation unhealthy
func TestWatcher_SetAllocHealth_Unhealthy(t *testing.T) {
	assert := assert.New(t)
	m := newMockBackend(t)
	w := NewDeploymentsWatcher(testLogger(), m, m)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	a.DeploymentID = d.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d, false), "UpsertDeployment")
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
	m.On("UpsertDeploymentAllocHealth", mocker.MatchedBy(matcher)).Return(nil)

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
	m.AssertNumberOfCalls(t, "UpsertDeploymentAllocHealth", 1)
}

// Test setting allocation unhealthy and that there should be a rollback
func TestWatcher_SetAllocHealth_Unhealthy_Rollback(t *testing.T) {
	assert := assert.New(t)
	m := newMockBackend(t)
	w := NewDeploymentsWatcher(testLogger(), m, m)

	// Create a job, alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.AutoRevert = true
	j.Stable = true
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	a.DeploymentID = d.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d, false), "UpsertDeployment")
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
	m.On("GetJobVersions", mocker.MatchedBy(matchJobSpecificRequest(j.ID)),
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
	m.On("UpsertDeploymentAllocHealth", mocker.MatchedBy(matcher)).Return(nil)

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
	m.AssertNumberOfCalls(t, "UpsertDeploymentAllocHealth", 1)
}

// Test promoting a deployment
func TestWatcher_PromoteDeployment_HealthyCanaries(t *testing.T) {
	assert := assert.New(t)
	m := newMockBackend(t)
	w := NewDeploymentsWatcher(testLogger(), m, m)

	// Create a job, canary alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.Canary = 2
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	a.Canary = true
	a.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
	}
	a.DeploymentID = d.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d, false), "UpsertDeployment")
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
	m.On("UpsertDeploymentPromotion", mocker.MatchedBy(matcher)).Return(nil)

	// Call SetAllocHealth
	req := &structs.DeploymentPromoteRequest{
		DeploymentID: d.ID,
		All:          true,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PromoteDeployment(req, &resp)
	assert.Nil(err, "PromoteDeployment")
	assert.Equal(1, len(w.watchers), "Deployment should still be active")
	m.AssertCalled(t, "UpsertDeploymentPromotion", mocker.MatchedBy(matcher))
}

// Test promoting a deployment with unhealthy canaries
func TestWatcher_PromoteDeployment_UnhealthyCanaries(t *testing.T) {
	assert := assert.New(t)
	m := newMockBackend(t)
	w := NewDeploymentsWatcher(testLogger(), m, m)

	// Create a job, canary alloc, and a deployment
	j := mock.Job()
	j.TaskGroups[0].Update = structs.DefaultUpdateStrategy.Copy()
	j.TaskGroups[0].Update.MaxParallel = 2
	j.TaskGroups[0].Update.Canary = 2
	d := mock.Deployment()
	d.JobID = j.ID
	a := mock.Alloc()
	a.Canary = true
	a.DeploymentID = d.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d, false), "UpsertDeployment")
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
	m.On("UpsertDeploymentPromotion", mocker.MatchedBy(matcher)).Return(nil)

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
	m.AssertCalled(t, "UpsertDeploymentPromotion", mocker.MatchedBy(matcher))
}

// Test pausing a deployment that is running
func TestWatcher_PauseDeployment_Pause_Running(t *testing.T) {
	assert := assert.New(t)
	m := newMockBackend(t)
	w := NewDeploymentsWatcher(testLogger(), m, m)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d, false), "UpsertDeployment")

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
	m.On("UpsertDeploymentStatusUpdate", mocker.MatchedBy(matcher)).Return(nil)

	// Call PauseDeployment
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        true,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PauseDeployment(req, &resp)
	assert.Nil(err, "PauseDeployment")

	assert.Equal(1, len(w.watchers), "Deployment should still be active")
	m.AssertCalled(t, "UpsertDeploymentStatusUpdate", mocker.MatchedBy(matcher))
}

// Test pausing a deployment that is paused
func TestWatcher_PauseDeployment_Pause_Paused(t *testing.T) {
	assert := assert.New(t)
	m := newMockBackend(t)
	w := NewDeploymentsWatcher(testLogger(), m, m)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	d.Status = structs.DeploymentStatusPaused
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d, false), "UpsertDeployment")

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
	m.On("UpsertDeploymentStatusUpdate", mocker.MatchedBy(matcher)).Return(nil)

	// Call PauseDeployment
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        true,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PauseDeployment(req, &resp)
	assert.Nil(err, "PauseDeployment")

	assert.Equal(1, len(w.watchers), "Deployment should still be active")
	m.AssertCalled(t, "UpsertDeploymentStatusUpdate", mocker.MatchedBy(matcher))
}

// Test unpausing a deployment that is paused
func TestWatcher_PauseDeployment_Unpause_Paused(t *testing.T) {
	assert := assert.New(t)
	m := newMockBackend(t)
	w := NewDeploymentsWatcher(testLogger(), m, m)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	d.Status = structs.DeploymentStatusPaused
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d, false), "UpsertDeployment")

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
	m.On("UpsertDeploymentStatusUpdate", mocker.MatchedBy(matcher)).Return(nil)

	// Call PauseDeployment
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        false,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PauseDeployment(req, &resp)
	assert.Nil(err, "PauseDeployment")

	assert.Equal(1, len(w.watchers), "Deployment should still be active")
	m.AssertCalled(t, "UpsertDeploymentStatusUpdate", mocker.MatchedBy(matcher))
}

// Test unpausing a deployment that is running
func TestWatcher_PauseDeployment_Unpause_Running(t *testing.T) {
	assert := assert.New(t)
	m := newMockBackend(t)
	w := NewDeploymentsWatcher(testLogger(), m, m)

	// Create a job and a deployment
	j := mock.Job()
	d := mock.Deployment()
	d.JobID = j.ID
	assert.Nil(m.state.UpsertJob(m.nextIndex(), j), "UpsertJob")
	assert.Nil(m.state.UpsertDeployment(m.nextIndex(), d, false), "UpsertDeployment")

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
	m.On("UpsertDeploymentStatusUpdate", mocker.MatchedBy(matcher)).Return(nil)

	// Call PauseDeployment
	req := &structs.DeploymentPauseRequest{
		DeploymentID: d.ID,
		Pause:        false,
	}
	var resp structs.DeploymentUpdateResponse
	err := w.PauseDeployment(req, &resp)
	assert.Nil(err, "PauseDeployment")

	assert.Equal(1, len(w.watchers), "Deployment should still be active")
	m.AssertCalled(t, "UpsertDeploymentStatusUpdate", mocker.MatchedBy(matcher))
}
