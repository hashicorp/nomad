package state

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func testStateStore(t *testing.T) *StateStore {
	return TestStateStore(t)
}

func TestStateStore_Blocking_Error(t *testing.T) {
	t.Parallel()

	expected := fmt.Errorf("test error")
	errFn := func(memdb.WatchSet, *StateStore) (interface{}, uint64, error) {
		return nil, 0, expected
	}

	state := testStateStore(t)
	_, idx, err := state.BlockingQuery(errFn, 10, context.Background())
	assert.EqualError(t, err, expected.Error())
	assert.Zero(t, idx)
}

func TestStateStore_Blocking_Timeout(t *testing.T) {
	t.Parallel()

	noopFn := func(memdb.WatchSet, *StateStore) (interface{}, uint64, error) {
		return nil, 5, nil
	}

	state := testStateStore(t)
	timeout := time.Now().Add(250 * time.Millisecond)
	deadlineCtx, cancel := context.WithDeadline(context.Background(), timeout)
	defer cancel()

	_, idx, err := state.BlockingQuery(noopFn, 10, deadlineCtx)
	assert.EqualError(t, err, context.DeadlineExceeded.Error())
	assert.EqualValues(t, 5, idx)
	assert.WithinDuration(t, timeout, time.Now(), 100*time.Millisecond)
}

func TestStateStore_Blocking_MinQuery(t *testing.T) {
	t.Parallel()

	node := mock.Node()
	count := 0
	queryFn := func(ws memdb.WatchSet, s *StateStore) (interface{}, uint64, error) {
		_, err := s.NodeByID(ws, node.ID)
		if err != nil {
			return nil, 0, err
		}

		count++
		if count == 1 {
			return false, 5, nil
		} else if count > 2 {
			return false, 20, fmt.Errorf("called too many times")
		}

		return true, 11, nil
	}

	state := testStateStore(t)
	timeout := time.Now().Add(100 * time.Millisecond)
	deadlineCtx, cancel := context.WithDeadline(context.Background(), timeout)
	defer cancel()

	time.AfterFunc(5*time.Millisecond, func() {
		state.UpsertNode(structs.MsgTypeTestSetup, 11, node)
	})

	resp, idx, err := state.BlockingQuery(queryFn, 10, deadlineCtx)
	if assert.Nil(t, err) {
		assert.Equal(t, 2, count)
		assert.EqualValues(t, 11, idx)
		assert.True(t, resp.(bool))
	}
}

// COMPAT 0.11: Uses AllocUpdateRequest.Alloc
// This test checks that:
// 1) The job is denormalized
// 2) Allocations are created
func TestStateStore_UpsertPlanResults_AllocationsCreated_Denormalized(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	alloc := mock.Alloc()
	job := alloc.Job
	alloc.Job = nil

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 999, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	eval := mock.Eval()
	eval.JobID = job.ID

	// Create an eval
	if err := state.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a plan result
	res := structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Alloc: []*structs.Allocation{alloc},
			Job:   job,
		},
		EvalID: eval.ID,
	}
	assert := assert.New(t)
	err := state.UpsertPlanResults(structs.MsgTypeTestSetup, 1000, &res)
	assert.Nil(err)

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	assert.Nil(err)
	assert.Equal(alloc, out)

	index, err := state.Index("allocs")
	assert.Nil(err)
	assert.EqualValues(1000, index)

	if watchFired(ws) {
		t.Fatalf("bad")
	}

	evalOut, err := state.EvalByID(ws, eval.ID)
	assert.Nil(err)
	assert.NotNil(evalOut)
	assert.EqualValues(1000, evalOut.ModifyIndex)
}

// This test checks that:
// 1) The job is denormalized
// 2) Allocations are denormalized and updated with the diff
// That stopped allocs Job is unmodified
func TestStateStore_UpsertPlanResults_AllocationsDenormalized(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	alloc := mock.Alloc()
	job := alloc.Job
	alloc.Job = nil

	stoppedAlloc := mock.Alloc()
	stoppedAlloc.Job = job
	stoppedAllocDiff := &structs.AllocationDiff{
		ID:                 stoppedAlloc.ID,
		DesiredDescription: "desired desc",
		ClientStatus:       structs.AllocClientStatusLost,
	}
	preemptedAlloc := mock.Alloc()
	preemptedAlloc.Job = job
	preemptedAllocDiff := &structs.AllocationDiff{
		ID:                    preemptedAlloc.ID,
		PreemptedByAllocation: alloc.ID,
	}

	require := require.New(t)
	require.NoError(state.UpsertAllocs(structs.MsgTypeTestSetup, 900, []*structs.Allocation{stoppedAlloc, preemptedAlloc}))
	require.NoError(state.UpsertJob(structs.MsgTypeTestSetup, 999, job))

	// modify job and ensure that stopped and preempted alloc point to original Job
	mJob := job.Copy()
	mJob.TaskGroups[0].Name = "other"

	require.NoError(state.UpsertJob(structs.MsgTypeTestSetup, 1001, mJob))

	eval := mock.Eval()
	eval.JobID = job.ID

	// Create an eval
	require.NoError(state.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval}))

	// Create a plan result
	res := structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			AllocsUpdated: []*structs.Allocation{alloc},
			AllocsStopped: []*structs.AllocationDiff{stoppedAllocDiff},
			Job:           mJob,
		},
		EvalID:          eval.ID,
		AllocsPreempted: []*structs.AllocationDiff{preemptedAllocDiff},
	}
	assert := assert.New(t)
	planModifyIndex := uint64(1000)
	err := state.UpsertPlanResults(structs.MsgTypeTestSetup, planModifyIndex, &res)
	require.NoError(err)

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	require.NoError(err)
	assert.Equal(alloc, out)

	outJob, err := state.JobByID(ws, job.Namespace, job.ID)
	require.NoError(err)
	require.Equal(mJob.TaskGroups, outJob.TaskGroups)
	require.NotEmpty(job.TaskGroups, outJob.TaskGroups)

	updatedStoppedAlloc, err := state.AllocByID(ws, stoppedAlloc.ID)
	require.NoError(err)
	assert.Equal(stoppedAllocDiff.DesiredDescription, updatedStoppedAlloc.DesiredDescription)
	assert.Equal(structs.AllocDesiredStatusStop, updatedStoppedAlloc.DesiredStatus)
	assert.Equal(stoppedAllocDiff.ClientStatus, updatedStoppedAlloc.ClientStatus)
	assert.Equal(planModifyIndex, updatedStoppedAlloc.AllocModifyIndex)
	assert.Equal(planModifyIndex, updatedStoppedAlloc.AllocModifyIndex)
	assert.Equal(job.TaskGroups, updatedStoppedAlloc.Job.TaskGroups)

	updatedPreemptedAlloc, err := state.AllocByID(ws, preemptedAlloc.ID)
	require.NoError(err)
	assert.Equal(structs.AllocDesiredStatusEvict, updatedPreemptedAlloc.DesiredStatus)
	assert.Equal(preemptedAllocDiff.PreemptedByAllocation, updatedPreemptedAlloc.PreemptedByAllocation)
	assert.Equal(planModifyIndex, updatedPreemptedAlloc.AllocModifyIndex)
	assert.Equal(planModifyIndex, updatedPreemptedAlloc.AllocModifyIndex)
	assert.Equal(job.TaskGroups, updatedPreemptedAlloc.Job.TaskGroups)

	index, err := state.Index("allocs")
	require.NoError(err)
	assert.EqualValues(planModifyIndex, index)

	require.False(watchFired(ws))

	evalOut, err := state.EvalByID(ws, eval.ID)
	require.NoError(err)
	require.NotNil(evalOut)
	assert.EqualValues(planModifyIndex, evalOut.ModifyIndex)

}

// This test checks that the deployment is created and allocations count towards
// the deployment
func TestStateStore_UpsertPlanResults_Deployment(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	alloc := mock.Alloc()
	alloc2 := mock.Alloc()
	job := alloc.Job
	alloc.Job = nil
	alloc2.Job = nil

	d := mock.Deployment()
	alloc.DeploymentID = d.ID
	alloc2.DeploymentID = d.ID

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 999, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	eval := mock.Eval()
	eval.JobID = job.ID

	// Create an eval
	if err := state.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a plan result
	res := structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Alloc: []*structs.Allocation{alloc, alloc2},
			Job:   job,
		},
		Deployment: d,
		EvalID:     eval.ID,
	}

	err := state.UpsertPlanResults(structs.MsgTypeTestSetup, 1000, &res)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	assert := assert.New(t)
	out, err := state.AllocByID(ws, alloc.ID)
	assert.Nil(err)
	assert.Equal(alloc, out)

	dout, err := state.DeploymentByID(ws, d.ID)
	assert.Nil(err)
	assert.NotNil(dout)

	tg, ok := dout.TaskGroups[alloc.TaskGroup]
	assert.True(ok)
	assert.NotNil(tg)
	assert.Equal(2, tg.PlacedAllocs)

	evalOut, err := state.EvalByID(ws, eval.ID)
	assert.Nil(err)
	assert.NotNil(evalOut)
	assert.EqualValues(1000, evalOut.ModifyIndex)

	if watchFired(ws) {
		t.Fatalf("bad")
	}

	// Update the allocs to be part of a new deployment
	d2 := d.Copy()
	d2.ID = uuid.Generate()

	allocNew := alloc.Copy()
	allocNew.DeploymentID = d2.ID
	allocNew2 := alloc2.Copy()
	allocNew2.DeploymentID = d2.ID

	// Create another plan
	res = structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Alloc: []*structs.Allocation{allocNew, allocNew2},
			Job:   job,
		},
		Deployment: d2,
		EvalID:     eval.ID,
	}

	err = state.UpsertPlanResults(structs.MsgTypeTestSetup, 1001, &res)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	dout, err = state.DeploymentByID(ws, d2.ID)
	assert.Nil(err)
	assert.NotNil(dout)

	tg, ok = dout.TaskGroups[alloc.TaskGroup]
	assert.True(ok)
	assert.NotNil(tg)
	assert.Equal(2, tg.PlacedAllocs)

	evalOut, err = state.EvalByID(ws, eval.ID)
	assert.Nil(err)
	assert.NotNil(evalOut)
	assert.EqualValues(1001, evalOut.ModifyIndex)
}

// This test checks that:
// 1) Preempted allocations in plan results are updated
// 2) Evals are inserted for preempted jobs
func TestStateStore_UpsertPlanResults_PreemptedAllocs(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)
	alloc := mock.Alloc()
	job := alloc.Job
	alloc.Job = nil

	// Insert job
	err := state.UpsertJob(structs.MsgTypeTestSetup, 999, job)
	require.NoError(err)

	// Create an eval
	eval := mock.Eval()
	eval.JobID = job.ID
	err = state.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval})
	require.NoError(err)

	// Insert alloc that will be preempted in the plan
	preemptedAlloc := mock.Alloc()
	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 2, []*structs.Allocation{preemptedAlloc})
	require.NoError(err)

	minimalPreemptedAlloc := &structs.Allocation{
		ID:                    preemptedAlloc.ID,
		PreemptedByAllocation: alloc.ID,
		ModifyTime:            time.Now().Unix(),
	}

	// Create eval for preempted job
	eval2 := mock.Eval()
	eval2.JobID = preemptedAlloc.JobID

	// Create a plan result
	res := structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Alloc: []*structs.Allocation{alloc},
			Job:   job,
		},
		EvalID:          eval.ID,
		NodePreemptions: []*structs.Allocation{minimalPreemptedAlloc},
		PreemptionEvals: []*structs.Evaluation{eval2},
	}

	err = state.UpsertPlanResults(structs.MsgTypeTestSetup, 1000, &res)
	require.NoError(err)

	ws := memdb.NewWatchSet()

	// Verify alloc and eval created by plan
	out, err := state.AllocByID(ws, alloc.ID)
	require.NoError(err)
	require.Equal(alloc, out)

	index, err := state.Index("allocs")
	require.NoError(err)
	require.EqualValues(1000, index)

	evalOut, err := state.EvalByID(ws, eval.ID)
	require.NoError(err)
	require.NotNil(evalOut)
	require.EqualValues(1000, evalOut.ModifyIndex)

	// Verify preempted alloc
	preempted, err := state.AllocByID(ws, preemptedAlloc.ID)
	require.NoError(err)
	require.Equal(preempted.DesiredStatus, structs.AllocDesiredStatusEvict)
	require.Equal(preempted.DesiredDescription, fmt.Sprintf("Preempted by alloc ID %v", alloc.ID))
	require.Equal(preempted.Job.ID, preemptedAlloc.Job.ID)
	require.Equal(preempted.Job, preemptedAlloc.Job)

	// Verify eval for preempted job
	preemptedJobEval, err := state.EvalByID(ws, eval2.ID)
	require.NoError(err)
	require.NotNil(preemptedJobEval)
	require.EqualValues(1000, preemptedJobEval.ModifyIndex)

}

// This test checks that deployment updates are applied correctly
func TestStateStore_UpsertPlanResults_DeploymentUpdates(t *testing.T) {
	t.Parallel()
	state := testStateStore(t)

	// Create a job that applies to all
	job := mock.Job()
	if err := state.UpsertJob(structs.MsgTypeTestSetup, 998, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a deployment that we will update its status
	doutstanding := mock.Deployment()
	doutstanding.JobID = job.ID

	if err := state.UpsertDeployment(1000, doutstanding); err != nil {
		t.Fatalf("err: %v", err)
	}

	eval := mock.Eval()
	eval.JobID = job.ID

	// Create an eval
	if err := state.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("err: %v", err)
	}
	alloc := mock.Alloc()
	alloc.Job = nil

	dnew := mock.Deployment()
	dnew.JobID = job.ID
	alloc.DeploymentID = dnew.ID

	// Update the old deployment
	update := &structs.DeploymentStatusUpdate{
		DeploymentID:      doutstanding.ID,
		Status:            "foo",
		StatusDescription: "bar",
	}

	// Create a plan result
	res := structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Alloc: []*structs.Allocation{alloc},
			Job:   job,
		},
		Deployment:        dnew,
		DeploymentUpdates: []*structs.DeploymentStatusUpdate{update},
		EvalID:            eval.ID,
	}

	err := state.UpsertPlanResults(structs.MsgTypeTestSetup, 1000, &res)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assert := assert.New(t)
	ws := memdb.NewWatchSet()

	// Check the deployments are correctly updated.
	dout, err := state.DeploymentByID(ws, dnew.ID)
	assert.Nil(err)
	assert.NotNil(dout)

	tg, ok := dout.TaskGroups[alloc.TaskGroup]
	assert.True(ok)
	assert.NotNil(tg)
	assert.Equal(1, tg.PlacedAllocs)

	doutstandingout, err := state.DeploymentByID(ws, doutstanding.ID)
	assert.Nil(err)
	assert.NotNil(doutstandingout)
	assert.Equal(update.Status, doutstandingout.Status)
	assert.Equal(update.StatusDescription, doutstandingout.StatusDescription)
	assert.EqualValues(1000, doutstandingout.ModifyIndex)

	evalOut, err := state.EvalByID(ws, eval.ID)
	assert.Nil(err)
	assert.NotNil(evalOut)
	assert.EqualValues(1000, evalOut.ModifyIndex)
	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpsertDeployment(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	deployment := mock.Deployment()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.DeploymentsByJobID(ws, deployment.Namespace, deployment.ID, true)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	err = state.UpsertDeployment(1000, deployment)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.DeploymentByID(ws, deployment.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(deployment, out) {
		t.Fatalf("bad: %#v %#v", deployment, out)
	}

	index, err := state.Index("deployment")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

// Tests that deployments of older create index and same job id are not returned
func TestStateStore_OldDeployment(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	job := mock.Job()
	job.ID = "job1"
	state.UpsertJob(structs.MsgTypeTestSetup, 1000, job)

	deploy1 := mock.Deployment()
	deploy1.JobID = job.ID
	deploy1.JobCreateIndex = job.CreateIndex

	deploy2 := mock.Deployment()
	deploy2.JobID = job.ID
	deploy2.JobCreateIndex = 11

	require := require.New(t)

	// Insert both deployments
	err := state.UpsertDeployment(1001, deploy1)
	require.Nil(err)

	err = state.UpsertDeployment(1002, deploy2)
	require.Nil(err)

	ws := memdb.NewWatchSet()
	// Should return both deployments
	deploys, err := state.DeploymentsByJobID(ws, deploy1.Namespace, job.ID, true)
	require.Nil(err)
	require.Len(deploys, 2)

	// Should only return deploy1
	deploys, err = state.DeploymentsByJobID(ws, deploy1.Namespace, job.ID, false)
	require.Nil(err)
	require.Len(deploys, 1)
	require.Equal(deploy1.ID, deploys[0].ID)
}

func TestStateStore_DeleteDeployment(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	d1 := mock.Deployment()
	d2 := mock.Deployment()

	err := state.UpsertDeployment(1000, d1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := state.UpsertDeployment(1001, d2); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.DeploymentByID(ws, d1.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	err = state.DeleteDeployment(1002, []string{d1.ID, d2.ID})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.DeploymentByID(ws, d1.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", d1, out)
	}

	index, err := state.Index("deployment")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1002 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_Deployments(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	var deployments []*structs.Deployment

	for i := 0; i < 10; i++ {
		deployment := mock.Deployment()
		deployments = append(deployments, deployment)

		err := state.UpsertDeployment(1000+uint64(i), deployment)
		require.NoError(t, err)
	}

	ws := memdb.NewWatchSet()
	it, err := state.Deployments(ws, true)
	require.NoError(t, err)

	var out []*structs.Deployment
	for {
		raw := it.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Deployment))
	}

	require.Equal(t, deployments, out)
	require.False(t, watchFired(ws))
}

func TestStateStore_DeploymentsByIDPrefix(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	deploy := mock.Deployment()

	deploy.ID = "11111111-662e-d0ab-d1c9-3e434af7bdb4"
	err := state.UpsertDeployment(1000, deploy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watchset so we can test that getters don't cause it to fire
	ws := memdb.NewWatchSet()
	iter, err := state.DeploymentsByIDPrefix(ws, deploy.Namespace, deploy.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	gatherDeploys := func(iter memdb.ResultIterator) []*structs.Deployment {
		var deploys []*structs.Deployment
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			deploy := raw.(*structs.Deployment)
			deploys = append(deploys, deploy)
		}
		return deploys
	}

	deploys := gatherDeploys(iter)
	if len(deploys) != 1 {
		t.Fatalf("err: %v", err)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}

	iter, err = state.DeploymentsByIDPrefix(ws, deploy.Namespace, "11")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	deploys = gatherDeploys(iter)
	if len(deploys) != 1 {
		t.Fatalf("err: %v", err)
	}

	deploy = mock.Deployment()
	deploy.ID = "11222222-662e-d0ab-d1c9-3e434af7bdb4"
	err = state.UpsertDeployment(1001, deploy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	iter, err = state.DeploymentsByIDPrefix(ws, deploy.Namespace, "11")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	deploys = gatherDeploys(iter)
	if len(deploys) != 2 {
		t.Fatalf("err: %v", err)
	}

	iter, err = state.DeploymentsByIDPrefix(ws, deploy.Namespace, "1111")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	deploys = gatherDeploys(iter)
	if len(deploys) != 1 {
		t.Fatalf("err: %v", err)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpsertNode_Node(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	state := testStateStore(t)
	node := mock.Node()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NodeByID(ws, node.ID)
	require.NoError(err)

	require.NoError(state.UpsertNode(structs.MsgTypeTestSetup, 1000, node))
	require.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	require.NoError(err)

	out2, err := state.NodeBySecretID(ws, node.SecretID)
	require.NoError(err)
	require.EqualValues(node, out)
	require.EqualValues(node, out2)
	require.Len(out.Events, 1)
	require.Equal(NodeRegisterEventRegistered, out.Events[0].Message)

	index, err := state.Index("nodes")
	require.NoError(err)
	require.EqualValues(1000, index)
	require.False(watchFired(ws))

	// Transition the node to down and then up and ensure we get a re-register
	// event
	down := out.Copy()
	down.Status = structs.NodeStatusDown
	require.NoError(state.UpsertNode(structs.MsgTypeTestSetup, 1001, down))
	require.NoError(state.UpsertNode(structs.MsgTypeTestSetup, 1002, out))

	out, err = state.NodeByID(ws, node.ID)
	require.NoError(err)
	require.Len(out.Events, 2)
	require.Equal(NodeRegisterEventReregistered, out.Events[1].Message)
}

func TestStateStore_DeleteNode_Node(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Create and insert two nodes, which we'll delete
	node0 := mock.Node()
	node1 := mock.Node()
	err := state.UpsertNode(structs.MsgTypeTestSetup, 1000, node0)
	require.NoError(t, err)
	err = state.UpsertNode(structs.MsgTypeTestSetup, 1001, node1)
	require.NoError(t, err)

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()

	// Check that both nodes are not nil
	out, err := state.NodeByID(ws, node0.ID)
	require.NoError(t, err)
	require.NotNil(t, out)
	out, err = state.NodeByID(ws, node1.ID)
	require.NoError(t, err)
	require.NotNil(t, out)

	// Delete both nodes in a batch, fires the watch
	err = state.DeleteNode(structs.MsgTypeTestSetup, 1002, []string{node0.ID, node1.ID})
	require.NoError(t, err)
	require.True(t, watchFired(ws))

	// Check that both nodes are nil
	ws = memdb.NewWatchSet()
	out, err = state.NodeByID(ws, node0.ID)
	require.NoError(t, err)
	require.Nil(t, out)
	out, err = state.NodeByID(ws, node1.ID)
	require.NoError(t, err)
	require.Nil(t, out)

	// Ensure that the index is still at 1002, from DeleteNode
	index, err := state.Index("nodes")
	require.NoError(t, err)
	require.Equal(t, uint64(1002), index)
	require.False(t, watchFired(ws))
}

func TestStateStore_UpdateNodeStatus_Node(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)
	node := mock.Node()

	require.NoError(state.UpsertNode(structs.MsgTypeTestSetup, 800, node))

	// Create a watchset so we can test that update node status fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NodeByID(ws, node.ID)
	require.NoError(err)

	event := &structs.NodeEvent{
		Message:   "Node ready foo",
		Subsystem: structs.NodeEventSubsystemCluster,
		Timestamp: time.Now(),
	}

	require.NoError(state.UpdateNodeStatus(structs.MsgTypeTestSetup, 801, node.ID, structs.NodeStatusReady, 70, event))
	require.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	require.NoError(err)
	require.Equal(structs.NodeStatusReady, out.Status)
	require.EqualValues(801, out.ModifyIndex)
	require.EqualValues(70, out.StatusUpdatedAt)
	require.Len(out.Events, 2)
	require.Equal(event.Message, out.Events[1].Message)

	index, err := state.Index("nodes")
	require.NoError(err)
	require.EqualValues(801, index)
	require.False(watchFired(ws))
}

func TestStateStore_BatchUpdateNodeDrain(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)

	n1, n2 := mock.Node(), mock.Node()
	require.Nil(state.UpsertNode(structs.MsgTypeTestSetup, 1000, n1))
	require.Nil(state.UpsertNode(structs.MsgTypeTestSetup, 1001, n2))

	// Create a watchset so we can test that update node drain fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NodeByID(ws, n1.ID)
	require.Nil(err)

	expectedDrain := &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: -1 * time.Second,
		},
	}

	update := map[string]*structs.DrainUpdate{
		n1.ID: {
			DrainStrategy: expectedDrain,
		},
		n2.ID: {
			DrainStrategy: expectedDrain,
		},
	}

	event := &structs.NodeEvent{
		Message:   "Drain strategy enabled",
		Subsystem: structs.NodeEventSubsystemDrain,
		Timestamp: time.Now(),
	}
	events := map[string]*structs.NodeEvent{
		n1.ID: event,
		n2.ID: event,
	}

	require.Nil(state.BatchUpdateNodeDrain(structs.MsgTypeTestSetup, 1002, 7, update, events))
	require.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	for _, id := range []string{n1.ID, n2.ID} {
		out, err := state.NodeByID(ws, id)
		require.Nil(err)
		require.NotNil(out.DrainStrategy)
		require.Equal(out.DrainStrategy, expectedDrain)
		require.NotNil(out.LastDrain)
		require.Equal(structs.DrainStatusDraining, out.LastDrain.Status)
		require.Len(out.Events, 2)
		require.EqualValues(1002, out.ModifyIndex)
		require.EqualValues(7, out.StatusUpdatedAt)
	}

	index, err := state.Index("nodes")
	require.Nil(err)
	require.EqualValues(1002, index)
	require.False(watchFired(ws))
}

func TestStateStore_UpdateNodeDrain_Node(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)
	node := mock.Node()

	require.Nil(state.UpsertNode(structs.MsgTypeTestSetup, 1000, node))

	// Create a watchset so we can test that update node drain fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NodeByID(ws, node.ID)
	require.Nil(err)

	expectedDrain := &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: -1 * time.Second,
		},
	}

	event := &structs.NodeEvent{
		Message:   "Drain strategy enabled",
		Subsystem: structs.NodeEventSubsystemDrain,
		Timestamp: time.Now(),
	}
	require.Nil(state.UpdateNodeDrain(structs.MsgTypeTestSetup, 1001, node.ID, expectedDrain, false, 7, event, nil, ""))
	require.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	require.Nil(err)
	require.NotNil(out.DrainStrategy)
	require.NotNil(out.LastDrain)
	require.Equal(structs.DrainStatusDraining, out.LastDrain.Status)
	require.Equal(out.DrainStrategy, expectedDrain)
	require.Len(out.Events, 2)
	require.EqualValues(1001, out.ModifyIndex)
	require.EqualValues(7, out.StatusUpdatedAt)

	index, err := state.Index("nodes")
	require.Nil(err)
	require.EqualValues(1001, index)
	require.False(watchFired(ws))
}

func TestStateStore_AddSingleNodeEvent(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)

	node := mock.Node()

	// We create a new node event every time we register a node
	err := state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	require.Nil(err)

	require.Equal(1, len(node.Events))
	require.Equal(structs.NodeEventSubsystemCluster, node.Events[0].Subsystem)
	require.Equal(NodeRegisterEventRegistered, node.Events[0].Message)

	// Create a watchset so we can test that AddNodeEvent fires the watch
	ws := memdb.NewWatchSet()
	_, err = state.NodeByID(ws, node.ID)
	require.Nil(err)

	nodeEvent := &structs.NodeEvent{
		Message:   "failed",
		Subsystem: "Driver",
		Timestamp: time.Now(),
	}
	nodeEvents := map[string][]*structs.NodeEvent{
		node.ID: {nodeEvent},
	}
	err = state.UpsertNodeEvents(structs.MsgTypeTestSetup, uint64(1001), nodeEvents)
	require.Nil(err)

	require.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	require.Nil(err)

	require.Equal(2, len(out.Events))
	require.Equal(nodeEvent, out.Events[1])
}

// To prevent stale node events from accumulating, we limit the number of
// stored node events to 10.
func TestStateStore_NodeEvents_RetentionWindow(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)

	node := mock.Node()

	err := state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	require.Equal(1, len(node.Events))
	require.Equal(structs.NodeEventSubsystemCluster, node.Events[0].Subsystem)
	require.Equal(NodeRegisterEventRegistered, node.Events[0].Message)

	var out *structs.Node
	for i := 1; i <= 20; i++ {
		ws := memdb.NewWatchSet()
		out, err = state.NodeByID(ws, node.ID)
		require.Nil(err)

		nodeEvent := &structs.NodeEvent{
			Message:   fmt.Sprintf("%dith failed", i),
			Subsystem: "Driver",
			Timestamp: time.Now(),
		}

		nodeEvents := map[string][]*structs.NodeEvent{
			out.ID: {nodeEvent},
		}
		err := state.UpsertNodeEvents(structs.MsgTypeTestSetup, uint64(i), nodeEvents)
		require.Nil(err)

		require.True(watchFired(ws))
		ws = memdb.NewWatchSet()
		out, err = state.NodeByID(ws, node.ID)
		require.Nil(err)
	}

	ws := memdb.NewWatchSet()
	out, err = state.NodeByID(ws, node.ID)
	require.Nil(err)

	require.Equal(10, len(out.Events))
	require.Equal(uint64(11), out.Events[0].CreateIndex)
	require.Equal(uint64(20), out.Events[len(out.Events)-1].CreateIndex)
}

func TestStateStore_UpdateNodeDrain_ResetEligiblity(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)
	node := mock.Node()
	require.Nil(state.UpsertNode(structs.MsgTypeTestSetup, 1000, node))

	// Create a watchset so we can test that update node drain fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NodeByID(ws, node.ID)
	require.Nil(err)

	drain := &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: -1 * time.Second,
		},
	}

	event1 := &structs.NodeEvent{
		Message:   "Drain strategy enabled",
		Subsystem: structs.NodeEventSubsystemDrain,
		Timestamp: time.Now(),
	}
	require.Nil(state.UpdateNodeDrain(structs.MsgTypeTestSetup, 1001, node.ID, drain, false, 7, event1, nil, ""))
	require.True(watchFired(ws))

	// Remove the drain
	event2 := &structs.NodeEvent{
		Message:   "Drain strategy disabled",
		Subsystem: structs.NodeEventSubsystemDrain,
		Timestamp: time.Now(),
	}
	require.Nil(state.UpdateNodeDrain(structs.MsgTypeTestSetup, 1002, node.ID, nil, true, 9, event2, nil, ""))

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	require.Nil(err)
	require.Nil(out.DrainStrategy)
	require.Equal(out.SchedulingEligibility, structs.NodeSchedulingEligible)
	require.NotNil(out.LastDrain)
	require.Equal(structs.DrainStatusCanceled, out.LastDrain.Status)
	require.Equal(time.Unix(7, 0), out.LastDrain.StartedAt)
	require.Equal(time.Unix(9, 0), out.LastDrain.UpdatedAt)
	require.Len(out.Events, 3)
	require.EqualValues(1002, out.ModifyIndex)
	require.EqualValues(9, out.StatusUpdatedAt)

	index, err := state.Index("nodes")
	require.Nil(err)
	require.EqualValues(1002, index)
	require.False(watchFired(ws))
}

func TestStateStore_UpdateNodeEligibility(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)
	node := mock.Node()

	err := state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	expectedEligibility := structs.NodeSchedulingIneligible

	// Create a watchset so we can test that update node drain fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.NodeByID(ws, node.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	event := &structs.NodeEvent{
		Message:   "Node marked as ineligible",
		Subsystem: structs.NodeEventSubsystemCluster,
		Timestamp: time.Now(),
	}
	require.Nil(state.UpdateNodeEligibility(structs.MsgTypeTestSetup, 1001, node.ID, expectedEligibility, 7, event))
	require.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	require.Nil(err)
	require.Equal(out.SchedulingEligibility, expectedEligibility)
	require.Len(out.Events, 2)
	require.Equal(out.Events[1], event)
	require.EqualValues(1001, out.ModifyIndex)
	require.EqualValues(7, out.StatusUpdatedAt)

	index, err := state.Index("nodes")
	require.Nil(err)
	require.EqualValues(1001, index)
	require.False(watchFired(ws))

	// Set a drain strategy
	expectedDrain := &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: -1 * time.Second,
		},
	}
	require.Nil(state.UpdateNodeDrain(structs.MsgTypeTestSetup, 1002, node.ID, expectedDrain, false, 7, nil, nil, ""))

	// Try to set the node to eligible
	err = state.UpdateNodeEligibility(structs.MsgTypeTestSetup, 1003, node.ID, structs.NodeSchedulingEligible, 9, nil)
	require.NotNil(err)
	require.Contains(err.Error(), "while it is draining")
}

func TestStateStore_Nodes(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	var nodes []*structs.Node

	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)

		err := state.UpsertNode(structs.MsgTypeTestSetup, 1000+uint64(i), node)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Create a watchset so we can test that getters don't cause it to fire
	ws := memdb.NewWatchSet()
	iter, err := state.Nodes(ws)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	var out []*structs.Node
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Node))
	}

	sort.Sort(NodeIDSort(nodes))
	sort.Sort(NodeIDSort(out))

	if !reflect.DeepEqual(nodes, out) {
		t.Fatalf("bad: %#v %#v", nodes, out)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_NodesByIDPrefix(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	node := mock.Node()

	node.ID = "11111111-662e-d0ab-d1c9-3e434af7bdb4"
	err := state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watchset so we can test that getters don't cause it to fire
	ws := memdb.NewWatchSet()
	iter, err := state.NodesByIDPrefix(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	gatherNodes := func(iter memdb.ResultIterator) []*structs.Node {
		var nodes []*structs.Node
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			node := raw.(*structs.Node)
			nodes = append(nodes, node)
		}
		return nodes
	}

	nodes := gatherNodes(iter)
	if len(nodes) != 1 {
		t.Fatalf("err: %v", err)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}

	iter, err = state.NodesByIDPrefix(ws, "11")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	nodes = gatherNodes(iter)
	if len(nodes) != 1 {
		t.Fatalf("err: %v", err)
	}

	node = mock.Node()
	node.ID = "11222222-662e-d0ab-d1c9-3e434af7bdb4"
	err = state.UpsertNode(structs.MsgTypeTestSetup, 1001, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	iter, err = state.NodesByIDPrefix(ws, "11")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	nodes = gatherNodes(iter)
	if len(nodes) != 2 {
		t.Fatalf("err: %v", err)
	}

	iter, err = state.NodesByIDPrefix(ws, "1111")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	nodes = gatherNodes(iter)
	if len(nodes) != 1 {
		t.Fatalf("err: %v", err)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpsertJob_Job(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	job := mock.Job()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, job); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(job, out) {
		t.Fatalf("bad: %#v %#v", job, out)
	}

	index, err := state.Index("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	summary, err := state.JobSummaryByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if summary == nil {
		t.Fatalf("nil summary")
	}
	if summary.JobID != job.ID {
		t.Fatalf("bad summary id: %v", summary.JobID)
	}
	_, ok := summary.Summary["web"]
	if !ok {
		t.Fatalf("nil summary for task group")
	}
	if watchFired(ws) {
		t.Fatalf("bad")
	}

	// Check the job versions
	allVersions, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(allVersions) != 1 {
		t.Fatalf("got %d; want 1", len(allVersions))
	}

	if a := allVersions[0]; a.ID != job.ID || a.Version != 0 {
		t.Fatalf("bad: %v", a)
	}

	// Test the looking up the job by version returns the same results
	vout, err := state.JobByIDAndVersion(ws, job.Namespace, job.ID, 0)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, vout) {
		t.Fatalf("bad: %#v %#v", out, vout)
	}
}

func TestStateStore_UpdateUpsertJob_Job(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	job := mock.Job()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	job2 := mock.Job()
	job2.ID = job.ID
	job2.AllAtOnce = true
	err = state.UpsertJob(structs.MsgTypeTestSetup, 1001, job2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(job2, out) {
		t.Fatalf("bad: %#v %#v", job2, out)
	}

	if out.CreateIndex != 1000 {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1001 {
		t.Fatalf("bad: %#v", out)
	}
	if out.Version != 1 {
		t.Fatalf("bad: %#v", out)
	}

	index, err := state.Index("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	// Test the looking up the job by version returns the same results
	vout, err := state.JobByIDAndVersion(ws, job.Namespace, job.ID, 1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, vout) {
		t.Fatalf("bad: %#v %#v", out, vout)
	}

	// Test that the job summary remains the same if the job is updated but
	// count remains same
	summary, err := state.JobSummaryByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if summary == nil {
		t.Fatalf("nil summary")
	}
	if summary.JobID != job.ID {
		t.Fatalf("bad summary id: %v", summary.JobID)
	}
	_, ok := summary.Summary["web"]
	if !ok {
		t.Fatalf("nil summary for task group")
	}

	// Check the job versions
	allVersions, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(allVersions) != 2 {
		t.Fatalf("got %d; want 1", len(allVersions))
	}

	if a := allVersions[0]; a.ID != job.ID || a.Version != 1 || !a.AllAtOnce {
		t.Fatalf("bad: %+v", a)
	}
	if a := allVersions[1]; a.ID != job.ID || a.Version != 0 || a.AllAtOnce {
		t.Fatalf("bad: %+v", a)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpdateUpsertJob_PeriodicJob(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	job := mock.PeriodicJob()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a child and an evaluation
	job2 := job.Copy()
	job2.Periodic = nil
	job2.ID = fmt.Sprintf("%v/%s-1490635020", job.ID, structs.PeriodicLaunchSuffix)
	err = state.UpsertJob(structs.MsgTypeTestSetup, 1001, job2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	eval := mock.Eval()
	eval.JobID = job2.ID
	err = state.UpsertEvals(structs.MsgTypeTestSetup, 1002, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	job3 := job.Copy()
	job3.TaskGroups[0].Tasks[0].Name = "new name"
	err = state.UpsertJob(structs.MsgTypeTestSetup, 1003, job3)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if s, e := out.Status, structs.JobStatusRunning; s != e {
		t.Fatalf("got status %v; want %v", s, e)
	}

}

func TestStateStore_UpsertJob_BadNamespace(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	state := testStateStore(t)
	job := mock.Job()
	job.Namespace = "foo"

	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, job)
	assert.Contains(err.Error(), "nonexistent namespace")

	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	assert.Nil(err)
	assert.Nil(out)
}

// Upsert a job that is the child of a parent job and ensures its summary gets
// updated.
func TestStateStore_UpsertJob_ChildJob(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Create a watchset so we can test that upsert fires the watch
	parent := mock.Job()
	ws := memdb.NewWatchSet()
	_, err := state.JobByID(ws, parent.Namespace, parent.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, parent); err != nil {
		t.Fatalf("err: %v", err)
	}

	child := mock.Job()
	child.Status = ""
	child.ParentID = parent.ID
	if err := state.UpsertJob(structs.MsgTypeTestSetup, 1001, child); err != nil {
		t.Fatalf("err: %v", err)
	}

	summary, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if summary == nil {
		t.Fatalf("nil summary")
	}
	if summary.JobID != parent.ID {
		t.Fatalf("bad summary id: %v", parent.ID)
	}
	if summary.Children == nil {
		t.Fatalf("nil children summary")
	}
	if summary.Children.Pending != 1 || summary.Children.Running != 0 || summary.Children.Dead != 0 {
		t.Fatalf("bad children summary: %v", summary.Children)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpdateUpsertJob_JobVersion(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Create a job and mark it as stable
	job := mock.Job()
	job.Stable = true
	job.Name = "0"

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	var finalJob *structs.Job
	for i := 1; i < 300; i++ {
		finalJob = mock.Job()
		finalJob.ID = job.ID
		finalJob.Name = fmt.Sprintf("%d", i)
		err = state.UpsertJob(structs.MsgTypeTestSetup, uint64(1000+i), finalJob)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	ws = memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(finalJob, out) {
		t.Fatalf("bad: %#v %#v", finalJob, out)
	}

	if out.CreateIndex != 1000 {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1299 {
		t.Fatalf("bad: %#v", out)
	}
	if out.Version != 299 {
		t.Fatalf("bad: %#v", out)
	}

	index, err := state.Index("job_version")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1299 {
		t.Fatalf("bad: %d", index)
	}

	// Check the job versions
	allVersions, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(allVersions) != structs.JobTrackedVersions {
		t.Fatalf("got %d; want %d", len(allVersions), structs.JobTrackedVersions)
	}

	if a := allVersions[0]; a.ID != job.ID || a.Version != 299 || a.Name != "299" {
		t.Fatalf("bad: %+v", a)
	}
	if a := allVersions[1]; a.ID != job.ID || a.Version != 298 || a.Name != "298" {
		t.Fatalf("bad: %+v", a)
	}

	// Ensure we didn't delete the stable job
	if a := allVersions[structs.JobTrackedVersions-1]; a.ID != job.ID ||
		a.Version != 0 || a.Name != "0" || !a.Stable {
		t.Fatalf("bad: %+v", a)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_DeleteJob_Job(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	job := mock.Job()

	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.JobByID(ws, job.Namespace, job.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	err = state.DeleteJob(1001, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", job, out)
	}

	index, err := state.Index("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	summary, err := state.JobSummaryByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if summary != nil {
		t.Fatalf("expected summary to be nil, but got: %v", summary)
	}

	index, err = state.Index("job_summary")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	versions, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(versions) != 0 {
		t.Fatalf("expected no job versions")
	}

	index, err = state.Index("job_summary")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_DeleteJobTxn_BatchDeletes(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	const testJobCount = 10
	const jobVersionCount = 4

	stateIndex := uint64(1000)

	jobs := make([]*structs.Job, testJobCount)
	for i := 0; i < testJobCount; i++ {
		stateIndex++
		job := mock.BatchJob()

		err := state.UpsertJob(structs.MsgTypeTestSetup, stateIndex, job)
		require.NoError(t, err)

		jobs[i] = job

		// Create some versions
		for vi := 1; vi < jobVersionCount; vi++ {
			stateIndex++

			job := job.Copy()
			job.TaskGroups[0].Tasks[0].Env = map[string]string{
				"Version": fmt.Sprintf("%d", vi),
			}

			require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, stateIndex, job))
		}
	}

	ws := memdb.NewWatchSet()

	// Check that jobs are present in DB
	job, err := state.JobByID(ws, jobs[0].Namespace, jobs[0].ID)
	require.NoError(t, err)
	require.Equal(t, jobs[0].ID, job.ID)

	jobVersions, err := state.JobVersionsByID(ws, jobs[0].Namespace, jobs[0].ID)
	require.NoError(t, err)
	require.Equal(t, jobVersionCount, len(jobVersions))

	// Actually delete
	const deletionIndex = uint64(10001)
	err = state.WithWriteTransaction(structs.MsgTypeTestSetup, deletionIndex, func(txn Txn) error {
		for i, job := range jobs {
			err := state.DeleteJobTxn(deletionIndex, job.Namespace, job.ID, txn)
			require.NoError(t, err, "failed at %d %e", i, err)
		}
		return nil
	})
	assert.NoError(t, err)

	assert.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.JobByID(ws, jobs[0].Namespace, jobs[0].ID)
	require.NoError(t, err)
	require.Nil(t, out)

	jobVersions, err = state.JobVersionsByID(ws, jobs[0].Namespace, jobs[0].ID)
	require.NoError(t, err)
	require.Empty(t, jobVersions)

	index, err := state.Index("jobs")
	require.NoError(t, err)
	require.Equal(t, deletionIndex, index)
}

func TestStateStore_DeleteJob_MultipleVersions(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	assert := assert.New(t)

	// Create a job and mark it as stable
	job := mock.Job()
	job.Stable = true
	job.Priority = 0

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	assert.Nil(err)
	assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 1000, job))
	assert.True(watchFired(ws))

	var finalJob *structs.Job
	for i := 1; i < 20; i++ {
		finalJob = mock.Job()
		finalJob.ID = job.ID
		finalJob.Priority = i
		assert.Nil(state.UpsertJob(structs.MsgTypeTestSetup, uint64(1000+i), finalJob))
	}

	assert.Nil(state.DeleteJob(1020, job.Namespace, job.ID))
	assert.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	assert.Nil(err)
	assert.Nil(out)

	index, err := state.Index("jobs")
	assert.Nil(err)
	assert.EqualValues(1020, index)

	summary, err := state.JobSummaryByID(ws, job.Namespace, job.ID)
	assert.Nil(err)
	assert.Nil(summary)

	index, err = state.Index("job_version")
	assert.Nil(err)
	assert.EqualValues(1020, index)

	versions, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	assert.Nil(err)
	assert.Len(versions, 0)

	index, err = state.Index("job_summary")
	assert.Nil(err)
	assert.EqualValues(1020, index)

	assert.False(watchFired(ws))
}

func TestStateStore_DeleteJob_ChildJob(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	parent := mock.Job()
	if err := state.UpsertJob(structs.MsgTypeTestSetup, 998, parent); err != nil {
		t.Fatalf("err: %v", err)
	}

	child := mock.Job()
	child.Status = ""
	child.ParentID = parent.ID

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 999, child); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	err := state.DeleteJob(1001, child.Namespace, child.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	summary, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if summary == nil {
		t.Fatalf("nil summary")
	}
	if summary.JobID != parent.ID {
		t.Fatalf("bad summary id: %v", parent.ID)
	}
	if summary.Children == nil {
		t.Fatalf("nil children summary")
	}
	if summary.Children.Pending != 0 || summary.Children.Running != 0 || summary.Children.Dead != 1 {
		t.Fatalf("bad children summary: %v", summary.Children)
	}
	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_Jobs(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	var jobs []*structs.Job

	for i := 0; i < 10; i++ {
		job := mock.Job()
		jobs = append(jobs, job)

		err := state.UpsertJob(structs.MsgTypeTestSetup, 1000+uint64(i), job)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	ws := memdb.NewWatchSet()
	iter, err := state.Jobs(ws)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out []*structs.Job
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Job))
	}

	sort.Sort(JobIDSort(jobs))
	sort.Sort(JobIDSort(out))

	if !reflect.DeepEqual(jobs, out) {
		t.Fatalf("bad: %#v %#v", jobs, out)
	}
	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_JobVersions(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	var jobs []*structs.Job

	for i := 0; i < 10; i++ {
		job := mock.Job()
		jobs = append(jobs, job)

		err := state.UpsertJob(structs.MsgTypeTestSetup, 1000+uint64(i), job)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	ws := memdb.NewWatchSet()
	iter, err := state.JobVersions(ws)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out []*structs.Job
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Job))
	}

	sort.Sort(JobIDSort(jobs))
	sort.Sort(JobIDSort(out))

	if !reflect.DeepEqual(jobs, out) {
		t.Fatalf("bad: %#v %#v", jobs, out)
	}
	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_JobsByIDPrefix(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	job := mock.Job()

	job.ID = "redis"
	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	iter, err := state.JobsByIDPrefix(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	gatherJobs := func(iter memdb.ResultIterator) []*structs.Job {
		var jobs []*structs.Job
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			jobs = append(jobs, raw.(*structs.Job))
		}
		return jobs
	}

	jobs := gatherJobs(iter)
	if len(jobs) != 1 {
		t.Fatalf("err: %v", err)
	}

	iter, err = state.JobsByIDPrefix(ws, job.Namespace, "re")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	jobs = gatherJobs(iter)
	if len(jobs) != 1 {
		t.Fatalf("err: %v", err)
	}
	if watchFired(ws) {
		t.Fatalf("bad")
	}

	job = mock.Job()
	job.ID = "riak"
	err = state.UpsertJob(structs.MsgTypeTestSetup, 1001, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	iter, err = state.JobsByIDPrefix(ws, job.Namespace, "r")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	jobs = gatherJobs(iter)
	if len(jobs) != 2 {
		t.Fatalf("err: %v", err)
	}

	iter, err = state.JobsByIDPrefix(ws, job.Namespace, "ri")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	jobs = gatherJobs(iter)
	if len(jobs) != 1 {
		t.Fatalf("err: %v", err)
	}
	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_JobsByPeriodic(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	var periodic, nonPeriodic []*structs.Job

	for i := 0; i < 10; i++ {
		job := mock.Job()
		nonPeriodic = append(nonPeriodic, job)

		err := state.UpsertJob(structs.MsgTypeTestSetup, 1000+uint64(i), job)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	for i := 0; i < 10; i++ {
		job := mock.PeriodicJob()
		periodic = append(periodic, job)

		err := state.UpsertJob(structs.MsgTypeTestSetup, 2000+uint64(i), job)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	ws := memdb.NewWatchSet()
	iter, err := state.JobsByPeriodic(ws, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var outPeriodic []*structs.Job
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		outPeriodic = append(outPeriodic, raw.(*structs.Job))
	}

	iter, err = state.JobsByPeriodic(ws, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var outNonPeriodic []*structs.Job
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		outNonPeriodic = append(outNonPeriodic, raw.(*structs.Job))
	}

	sort.Sort(JobIDSort(periodic))
	sort.Sort(JobIDSort(nonPeriodic))
	sort.Sort(JobIDSort(outPeriodic))
	sort.Sort(JobIDSort(outNonPeriodic))

	if !reflect.DeepEqual(periodic, outPeriodic) {
		t.Fatalf("bad: %#v %#v", periodic, outPeriodic)
	}

	if !reflect.DeepEqual(nonPeriodic, outNonPeriodic) {
		t.Fatalf("bad: %#v %#v", nonPeriodic, outNonPeriodic)
	}
	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_JobsByScheduler(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	var serviceJobs []*structs.Job
	var sysJobs []*structs.Job

	for i := 0; i < 10; i++ {
		job := mock.Job()
		serviceJobs = append(serviceJobs, job)

		err := state.UpsertJob(structs.MsgTypeTestSetup, 1000+uint64(i), job)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	for i := 0; i < 10; i++ {
		job := mock.SystemJob()
		job.Status = structs.JobStatusRunning
		sysJobs = append(sysJobs, job)

		err := state.UpsertJob(structs.MsgTypeTestSetup, 2000+uint64(i), job)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	ws := memdb.NewWatchSet()
	iter, err := state.JobsByScheduler(ws, "service")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var outService []*structs.Job
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		outService = append(outService, raw.(*structs.Job))
	}

	iter, err = state.JobsByScheduler(ws, "system")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var outSystem []*structs.Job
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		outSystem = append(outSystem, raw.(*structs.Job))
	}

	sort.Sort(JobIDSort(serviceJobs))
	sort.Sort(JobIDSort(sysJobs))
	sort.Sort(JobIDSort(outService))
	sort.Sort(JobIDSort(outSystem))

	if !reflect.DeepEqual(serviceJobs, outService) {
		t.Fatalf("bad: %#v %#v", serviceJobs, outService)
	}

	if !reflect.DeepEqual(sysJobs, outSystem) {
		t.Fatalf("bad: %#v %#v", sysJobs, outSystem)
	}
	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_JobsByGC(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	gc, nonGc := make(map[string]struct{}), make(map[string]struct{})

	for i := 0; i < 20; i++ {
		var job *structs.Job
		if i%2 == 0 {
			job = mock.Job()
		} else {
			job = mock.PeriodicJob()
		}
		nonGc[job.ID] = struct{}{}

		if err := state.UpsertJob(structs.MsgTypeTestSetup, 1000+uint64(i), job); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	for i := 0; i < 20; i += 2 {
		job := mock.Job()
		job.Type = structs.JobTypeBatch
		gc[job.ID] = struct{}{}

		if err := state.UpsertJob(structs.MsgTypeTestSetup, 2000+uint64(i), job); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Create an eval for it
		eval := mock.Eval()
		eval.JobID = job.ID
		eval.Status = structs.EvalStatusComplete
		if err := state.UpsertEvals(structs.MsgTypeTestSetup, 2000+uint64(i+1), []*structs.Evaluation{eval}); err != nil {
			t.Fatalf("err: %v", err)
		}

	}

	ws := memdb.NewWatchSet()
	iter, err := state.JobsByGC(ws, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	outGc := make(map[string]struct{})
	for i := iter.Next(); i != nil; i = iter.Next() {
		j := i.(*structs.Job)
		outGc[j.ID] = struct{}{}
	}

	iter, err = state.JobsByGC(ws, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	outNonGc := make(map[string]struct{})
	for i := iter.Next(); i != nil; i = iter.Next() {
		j := i.(*structs.Job)
		outNonGc[j.ID] = struct{}{}
	}

	if !reflect.DeepEqual(gc, outGc) {
		t.Fatalf("bad: %#v %#v", gc, outGc)
	}

	if !reflect.DeepEqual(nonGc, outNonGc) {
		t.Fatalf("bad: %#v %#v", nonGc, outNonGc)
	}
	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpsertPeriodicLaunch(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	job := mock.Job()
	launch := &structs.PeriodicLaunch{
		ID:        job.ID,
		Namespace: job.Namespace,
		Launch:    time.Now(),
	}

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.PeriodicLaunchByID(ws, job.Namespace, launch.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	err := state.UpsertPeriodicLaunch(1000, launch)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.PeriodicLaunchByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.CreateIndex != 1000 {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1000 {
		t.Fatalf("bad: %#v", out)
	}

	if !reflect.DeepEqual(launch, out) {
		t.Fatalf("bad: %#v %#v", job, out)
	}

	index, err := state.Index("periodic_launch")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpdateUpsertPeriodicLaunch(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	job := mock.Job()
	launch := &structs.PeriodicLaunch{
		ID:        job.ID,
		Namespace: job.Namespace,
		Launch:    time.Now(),
	}

	err := state.UpsertPeriodicLaunch(1000, launch)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.PeriodicLaunchByID(ws, job.Namespace, launch.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	launch2 := &structs.PeriodicLaunch{
		ID:        job.ID,
		Namespace: job.Namespace,
		Launch:    launch.Launch.Add(1 * time.Second),
	}
	err = state.UpsertPeriodicLaunch(1001, launch2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.PeriodicLaunchByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.CreateIndex != 1000 {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1001 {
		t.Fatalf("bad: %#v", out)
	}

	if !reflect.DeepEqual(launch2, out) {
		t.Fatalf("bad: %#v %#v", launch2, out)
	}

	index, err := state.Index("periodic_launch")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_DeletePeriodicLaunch(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	job := mock.Job()
	launch := &structs.PeriodicLaunch{
		ID:        job.ID,
		Namespace: job.Namespace,
		Launch:    time.Now(),
	}

	err := state.UpsertPeriodicLaunch(1000, launch)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.PeriodicLaunchByID(ws, job.Namespace, launch.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	err = state.DeletePeriodicLaunch(1001, launch.Namespace, launch.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.PeriodicLaunchByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", job, out)
	}

	index, err := state.Index("periodic_launch")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_PeriodicLaunches(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	var launches []*structs.PeriodicLaunch

	for i := 0; i < 10; i++ {
		job := mock.Job()
		launch := &structs.PeriodicLaunch{
			ID:        job.ID,
			Namespace: job.Namespace,
			Launch:    time.Now(),
		}
		launches = append(launches, launch)

		err := state.UpsertPeriodicLaunch(1000+uint64(i), launch)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	ws := memdb.NewWatchSet()
	iter, err := state.PeriodicLaunches(ws)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out := make(map[string]*structs.PeriodicLaunch, 10)
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		launch := raw.(*structs.PeriodicLaunch)
		if _, ok := out[launch.ID]; ok {
			t.Fatalf("duplicate: %v", launch.ID)
		}

		out[launch.ID] = launch
	}

	for _, launch := range launches {
		l, ok := out[launch.ID]
		if !ok {
			t.Fatalf("bad %v", launch.ID)
		}

		if !reflect.DeepEqual(launch, l) {
			t.Fatalf("bad: %#v %#v", launch, l)
		}

		delete(out, launch.ID)
	}

	if len(out) != 0 {
		t.Fatalf("leftover: %#v", out)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

// TestStateStore_CSIVolume checks register, list and deregister for csi_volumes
func TestStateStore_CSIVolume(t *testing.T) {
	state := testStateStore(t)
	index := uint64(1000)

	// Volume IDs
	vol0, vol1 := uuid.Generate(), uuid.Generate()

	// Create a node running a healthy instance of the plugin
	node := mock.Node()
	pluginID := "minnie"
	alloc := mock.Alloc()
	alloc.DesiredStatus = "run"
	alloc.ClientStatus = "running"
	alloc.NodeID = node.ID
	alloc.Job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{
		"foo": {
			Name:   "foo",
			Source: vol0,
			Type:   "csi",
		},
	}

	node.CSINodePlugins = map[string]*structs.CSIInfo{
		pluginID: {
			PluginID:                 pluginID,
			AllocID:                  alloc.ID,
			Healthy:                  true,
			HealthDescription:        "healthy",
			RequiresControllerPlugin: false,
			RequiresTopologies:       false,
			NodeInfo: &structs.CSINodeInfo{
				ID:                      node.ID,
				MaxVolumes:              64,
				RequiresNodeStageVolume: true,
			},
		},
	}

	index++
	err := state.UpsertNode(structs.MsgTypeTestSetup, index, node)
	require.NoError(t, err)
	defer state.DeleteNode(structs.MsgTypeTestSetup, 9999, []string{pluginID})

	index++
	err = state.UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc})
	require.NoError(t, err)

	ns := structs.DefaultNamespace

	v0 := structs.NewCSIVolume("foo", index)
	v0.ID = vol0
	v0.Namespace = ns
	v0.PluginID = "minnie"
	v0.Schedulable = true
	v0.AccessMode = structs.CSIVolumeAccessModeMultiNodeSingleWriter
	v0.AttachmentMode = structs.CSIVolumeAttachmentModeFilesystem

	index++
	v1 := structs.NewCSIVolume("foo", index)
	v1.ID = vol1
	v1.Namespace = ns
	v1.PluginID = "adam"
	v1.Schedulable = true
	v1.AccessMode = structs.CSIVolumeAccessModeMultiNodeSingleWriter
	v1.AttachmentMode = structs.CSIVolumeAttachmentModeFilesystem

	index++
	err = state.CSIVolumeRegister(index, []*structs.CSIVolume{v0, v1})
	require.NoError(t, err)

	// volume registration is idempotent, unless identies are changed
	index++
	err = state.CSIVolumeRegister(index, []*structs.CSIVolume{v0, v1})
	require.NoError(t, err)

	index++
	v2 := v0.Copy()
	v2.PluginID = "new-id"
	err = state.CSIVolumeRegister(index, []*structs.CSIVolume{v2})
	require.Error(t, err, fmt.Sprintf("volume exists: %s", v0.ID))

	ws := memdb.NewWatchSet()
	iter, err := state.CSIVolumesByNamespace(ws, ns, "")
	require.NoError(t, err)

	slurp := func(iter memdb.ResultIterator) (vs []*structs.CSIVolume) {
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			vol := raw.(*structs.CSIVolume)
			vs = append(vs, vol)
		}
		return vs
	}

	vs := slurp(iter)
	require.Equal(t, 2, len(vs))

	ws = memdb.NewWatchSet()
	iter, err = state.CSIVolumesByPluginID(ws, ns, "", "minnie")
	require.NoError(t, err)
	vs = slurp(iter)
	require.Equal(t, 1, len(vs))

	ws = memdb.NewWatchSet()
	iter, err = state.CSIVolumesByNodeID(ws, "", node.ID)
	require.NoError(t, err)
	vs = slurp(iter)
	require.Equal(t, 1, len(vs))

	// Allocs
	a0 := mock.Alloc()
	a1 := mock.Alloc()
	index++
	err = state.UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{a0, a1})
	require.NoError(t, err)

	// Claims
	r := structs.CSIVolumeClaimRead
	w := structs.CSIVolumeClaimWrite
	u := structs.CSIVolumeClaimGC
	claim0 := &structs.CSIVolumeClaim{
		AllocationID: a0.ID,
		NodeID:       node.ID,
		Mode:         r,
	}
	claim1 := &structs.CSIVolumeClaim{
		AllocationID: a1.ID,
		NodeID:       node.ID,
		Mode:         w,
	}

	index++
	err = state.CSIVolumeClaim(index, ns, vol0, claim0)
	require.NoError(t, err)
	index++
	err = state.CSIVolumeClaim(index, ns, vol0, claim1)
	require.NoError(t, err)

	ws = memdb.NewWatchSet()
	iter, err = state.CSIVolumesByPluginID(ws, ns, "", "minnie")
	require.NoError(t, err)
	vs = slurp(iter)
	require.False(t, vs[0].HasFreeWriteClaims())

	claim0.Mode = u
	err = state.CSIVolumeClaim(2, ns, vol0, claim0)
	require.NoError(t, err)
	ws = memdb.NewWatchSet()
	iter, err = state.CSIVolumesByPluginID(ws, ns, "", "minnie")
	require.NoError(t, err)
	vs = slurp(iter)
	require.True(t, vs[0].ReadSchedulable())

	// registration is an error when the volume is in use
	index++
	err = state.CSIVolumeRegister(index, []*structs.CSIVolume{v0})
	require.Error(t, err, "volume re-registered while in use")
	// as is deregistration
	index++
	err = state.CSIVolumeDeregister(index, ns, []string{vol0}, false)
	require.Error(t, err, "volume deregistered while in use")

	// even if forced, because we have a non-terminal claim
	index++
	err = state.CSIVolumeDeregister(index, ns, []string{vol0}, true)
	require.Error(t, err, "volume force deregistered while in use")

	// we use the ID, not a prefix
	index++
	err = state.CSIVolumeDeregister(index, ns, []string{"fo"}, true)
	require.Error(t, err, "volume deregistered by prefix")

	// release claims to unblock deregister
	index++
	claim0.State = structs.CSIVolumeClaimStateReadyToFree
	err = state.CSIVolumeClaim(index, ns, vol0, claim0)
	require.NoError(t, err)
	index++
	claim1.Mode = u
	claim1.State = structs.CSIVolumeClaimStateReadyToFree
	err = state.CSIVolumeClaim(index, ns, vol0, claim1)
	require.NoError(t, err)

	index++
	err = state.CSIVolumeDeregister(index, ns, []string{vol0}, false)
	require.NoError(t, err)

	// List, now omitting the deregistered volume
	ws = memdb.NewWatchSet()
	iter, err = state.CSIVolumesByPluginID(ws, ns, "", "minnie")
	require.NoError(t, err)
	vs = slurp(iter)
	require.Equal(t, 0, len(vs))

	ws = memdb.NewWatchSet()
	iter, err = state.CSIVolumesByNamespace(ws, ns, "")
	require.NoError(t, err)
	vs = slurp(iter)
	require.Equal(t, 1, len(vs))
}

func TestStateStore_CSIPlugin_Lifecycle(t *testing.T) {
	t.Parallel()

	store := testStateStore(t)
	plugID := "foo"
	var err error
	var controllerJobID string
	var nodeJobID string
	allocIDs := []string{}

	type pluginCounts struct {
		controllerFingerprints int
		nodeFingerprints       int
		controllersHealthy     int
		nodesHealthy           int
		controllersExpected    int
		nodesExpected          int
	}

	// helper function for test assertions
	checkPlugin := func(counts pluginCounts) *structs.CSIPlugin {
		plug, err := store.CSIPluginByID(memdb.NewWatchSet(), plugID)
		require.NotNil(t, plug, "plugin was nil")
		require.NoError(t, err)
		require.Equal(t, counts.controllerFingerprints, len(plug.Controllers), "controllers fingerprinted")
		require.Equal(t, counts.nodeFingerprints, len(plug.Nodes), "nodes fingerprinted")
		require.Equal(t, counts.controllersHealthy, plug.ControllersHealthy, "controllers healthy")
		require.Equal(t, counts.nodesHealthy, plug.NodesHealthy, "nodes healthy")
		require.Equal(t, counts.controllersExpected, plug.ControllersExpected, "controllers expected")
		require.Equal(t, counts.nodesExpected, plug.NodesExpected, "nodes expected")
		return plug
	}

	type allocUpdateKind int
	const (
		SERVER allocUpdateKind = iota
		CLIENT
	)

	// helper function calling client-side update with with
	// UpsertAllocs and/or UpdateAllocsFromClient, depending on which
	// status(es) are set
	updateAllocsFn := func(allocIDs []string, kind allocUpdateKind,
		transform func(alloc *structs.Allocation)) []*structs.Allocation {
		allocs := []*structs.Allocation{}
		ws := memdb.NewWatchSet()
		for _, id := range allocIDs {
			alloc, err := store.AllocByID(ws, id)
			require.NoError(t, err)
			alloc = alloc.Copy()
			transform(alloc)
			allocs = append(allocs, alloc)
		}
		switch kind {
		case SERVER:
			err = store.UpsertAllocs(structs.MsgTypeTestSetup, nextIndex(store), allocs)
		case CLIENT:
			// this is somewhat artificial b/c we get alloc updates
			// from multiple nodes concurrently but not in a single
			// RPC call. But this guarantees we'll trigger any nested
			// transaction setup bugs
			err = store.UpdateAllocsFromClient(structs.MsgTypeTestSetup, nextIndex(store), allocs)
		}
		require.NoError(t, err)
		return allocs
	}

	// helper function calling UpsertNode for fingerprinting
	updateNodeFn := func(nodeID string, transform func(node *structs.Node)) {
		ws := memdb.NewWatchSet()
		node, _ := store.NodeByID(ws, nodeID)
		node = node.Copy()
		transform(node)
		err = store.UpsertNode(structs.MsgTypeTestSetup, nextIndex(store), node)
		require.NoError(t, err)
	}

	nodes := []*structs.Node{mock.Node(), mock.Node(), mock.Node()}
	for _, node := range nodes {
		err = store.UpsertNode(structs.MsgTypeTestSetup, nextIndex(store), node)
		require.NoError(t, err)
	}

	// Note: these are all subtests for clarity but are expected to be
	// ordered, because they walk through all the phases of plugin
	// instance registration and deregistration

	t.Run("register plugin jobs", func(t *testing.T) {

		controllerJob := mock.CSIPluginJob(structs.CSIPluginTypeController, plugID)
		controllerJobID = controllerJob.ID
		err = store.UpsertJob(structs.MsgTypeTestSetup, nextIndex(store), controllerJob)

		nodeJob := mock.CSIPluginJob(structs.CSIPluginTypeNode, plugID)
		nodeJobID = nodeJob.ID
		err = store.UpsertJob(structs.MsgTypeTestSetup, nextIndex(store), nodeJob)

		// plugins created, but no fingerprints or allocs yet
		// note: there's no job summary yet, but we know the task
		// group count for the non-system job
		//
		// TODO: that's the current code but we really should be able
		// to figure out the system jobs too
		plug := checkPlugin(pluginCounts{
			controllerFingerprints: 0,
			nodeFingerprints:       0,
			controllersHealthy:     0,
			nodesHealthy:           0,
			controllersExpected:    2,
			nodesExpected:          0,
		})
		require.False(t, plug.ControllerRequired)
	})

	t.Run("plan apply upserts allocations", func(t *testing.T) {

		allocForJob := func(job *structs.Job) *structs.Allocation {
			alloc := mock.Alloc()
			alloc.Job = job.Copy()
			alloc.JobID = job.ID
			alloc.TaskGroup = job.TaskGroups[0].Name
			alloc.DesiredStatus = structs.AllocDesiredStatusRun
			alloc.ClientStatus = structs.AllocClientStatusPending
			return alloc
		}

		ws := memdb.NewWatchSet()
		controllerJob, _ := store.JobByID(ws, structs.DefaultNamespace, controllerJobID)
		controllerAlloc0 := allocForJob(controllerJob)
		controllerAlloc0.NodeID = nodes[0].ID
		allocIDs = append(allocIDs, controllerAlloc0.ID)

		controllerAlloc1 := allocForJob(controllerJob)
		controllerAlloc1.NodeID = nodes[1].ID
		allocIDs = append(allocIDs, controllerAlloc1.ID)

		allocs := []*structs.Allocation{controllerAlloc0, controllerAlloc1}

		nodeJob, _ := store.JobByID(ws, structs.DefaultNamespace, nodeJobID)
		for _, node := range nodes {
			nodeAlloc := allocForJob(nodeJob)
			nodeAlloc.NodeID = node.ID
			allocIDs = append(allocIDs, nodeAlloc.ID)
			allocs = append(allocs, nodeAlloc)
		}
		err = store.UpsertAllocs(structs.MsgTypeTestSetup, nextIndex(store), allocs)
		require.NoError(t, err)

		// node plugin now has expected counts too
		plug := checkPlugin(pluginCounts{
			controllerFingerprints: 0,
			nodeFingerprints:       0,
			controllersHealthy:     0,
			nodesHealthy:           0,
			controllersExpected:    2,
			nodesExpected:          3,
		})
		require.False(t, plug.ControllerRequired)
	})

	t.Run("client upserts alloc status", func(t *testing.T) {

		updateAllocsFn(allocIDs, CLIENT, func(alloc *structs.Allocation) {
			alloc.ClientStatus = structs.AllocClientStatusRunning
		})

		// plugin still has allocs but no fingerprints
		plug := checkPlugin(pluginCounts{
			controllerFingerprints: 0,
			nodeFingerprints:       0,
			controllersHealthy:     0,
			nodesHealthy:           0,
			controllersExpected:    2,
			nodesExpected:          3,
		})
		require.False(t, plug.ControllerRequired)
	})

	t.Run("client upserts node fingerprints", func(t *testing.T) {

		nodeFingerprint := map[string]*structs.CSIInfo{
			plugID: {
				PluginID:                 plugID,
				Healthy:                  true,
				UpdateTime:               time.Now(),
				RequiresControllerPlugin: true,
				RequiresTopologies:       false,
				NodeInfo:                 &structs.CSINodeInfo{},
			},
		}
		for _, node := range nodes {
			updateNodeFn(node.ID, func(node *structs.Node) {
				node.CSINodePlugins = nodeFingerprint
			})
		}

		controllerFingerprint := map[string]*structs.CSIInfo{
			plugID: {
				PluginID:                 plugID,
				Healthy:                  true,
				UpdateTime:               time.Now(),
				RequiresControllerPlugin: true,
				RequiresTopologies:       false,
				ControllerInfo: &structs.CSIControllerInfo{
					SupportsReadOnlyAttach: true,
					SupportsListVolumes:    true,
				},
			},
		}
		for n := 0; n < 2; n++ {
			updateNodeFn(nodes[n].ID, func(node *structs.Node) {
				node.CSIControllerPlugins = controllerFingerprint
			})
		}

		// plugins have been fingerprinted so we have healthy counts
		plug := checkPlugin(pluginCounts{
			controllerFingerprints: 2,
			nodeFingerprints:       3,
			controllersHealthy:     2,
			nodesHealthy:           3,
			controllersExpected:    2,
			nodesExpected:          3,
		})
		require.True(t, plug.ControllerRequired)
	})

	t.Run("node marked for drain", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		nodeAllocs, err := store.AllocsByNode(ws, nodes[0].ID)
		require.NoError(t, err)
		require.Len(t, nodeAllocs, 2)

		updateAllocsFn([]string{nodeAllocs[0].ID, nodeAllocs[1].ID},
			SERVER, func(alloc *structs.Allocation) {
				alloc.DesiredStatus = structs.AllocDesiredStatusStop
			})

		plug := checkPlugin(pluginCounts{
			controllerFingerprints: 2,
			nodeFingerprints:       3,
			controllersHealthy:     2,
			nodesHealthy:           3,
			controllersExpected:    2, // job summary hasn't changed
			nodesExpected:          3, // job summary hasn't changed
		})
		require.True(t, plug.ControllerRequired)
	})

	t.Run("client removes fingerprints after node drain", func(t *testing.T) {
		updateNodeFn(nodes[0].ID, func(node *structs.Node) {
			node.CSIControllerPlugins = nil
			node.CSINodePlugins = nil
		})

		plug := checkPlugin(pluginCounts{
			controllerFingerprints: 1,
			nodeFingerprints:       2,
			controllersHealthy:     1,
			nodesHealthy:           2,
			controllersExpected:    2,
			nodesExpected:          3,
		})
		require.True(t, plug.ControllerRequired)
	})

	t.Run("client updates alloc status to stopped after node drain", func(t *testing.T) {
		nodeAllocs, err := store.AllocsByNode(memdb.NewWatchSet(), nodes[0].ID)
		require.NoError(t, err)
		require.Len(t, nodeAllocs, 2)

		updateAllocsFn([]string{nodeAllocs[0].ID, nodeAllocs[1].ID}, CLIENT,
			func(alloc *structs.Allocation) {
				alloc.ClientStatus = structs.AllocClientStatusComplete
			})

		plug := checkPlugin(pluginCounts{
			controllerFingerprints: 1,
			nodeFingerprints:       2,
			controllersHealthy:     1,
			nodesHealthy:           2,
			controllersExpected:    2, // still 2 because count=2
			nodesExpected:          2, // has to use nodes we're actually placed on
		})
		require.True(t, plug.ControllerRequired)
	})

	t.Run("job stop with purge", func(t *testing.T) {

		vol := &structs.CSIVolume{
			ID:        uuid.Generate(),
			Namespace: structs.DefaultNamespace,
			PluginID:  plugID,
		}
		err = store.CSIVolumeRegister(nextIndex(store), []*structs.CSIVolume{vol})
		require.NoError(t, err)

		err = store.DeleteJob(nextIndex(store), structs.DefaultNamespace, controllerJobID)
		require.NoError(t, err)

		err = store.DeleteJob(nextIndex(store), structs.DefaultNamespace, nodeJobID)
		require.NoError(t, err)

		plug := checkPlugin(pluginCounts{
			controllerFingerprints: 1, // no changes till we get fingerprint
			nodeFingerprints:       2,
			controllersHealthy:     1,
			nodesHealthy:           2,
			controllersExpected:    0,
			nodesExpected:          0,
		})
		require.True(t, plug.ControllerRequired)
		require.False(t, plug.IsEmpty())

		updateAllocsFn(allocIDs, SERVER,
			func(alloc *structs.Allocation) {
				alloc.DesiredStatus = structs.AllocDesiredStatusStop
			})

		updateAllocsFn(allocIDs, CLIENT,
			func(alloc *structs.Allocation) {
				alloc.ClientStatus = structs.AllocClientStatusComplete
			})

		plug = checkPlugin(pluginCounts{
			controllerFingerprints: 1,
			nodeFingerprints:       2,
			controllersHealthy:     1,
			nodesHealthy:           2,
			controllersExpected:    0,
			nodesExpected:          0,
		})
		require.True(t, plug.ControllerRequired)
		require.False(t, plug.IsEmpty())

		for _, node := range nodes {
			updateNodeFn(node.ID, func(node *structs.Node) {
				node.CSIControllerPlugins = nil
			})
		}

		plug = checkPlugin(pluginCounts{
			controllerFingerprints: 0,
			nodeFingerprints:       2, // haven't removed fingerprints yet
			controllersHealthy:     0,
			nodesHealthy:           2,
			controllersExpected:    0,
			nodesExpected:          0,
		})
		require.True(t, plug.ControllerRequired)
		require.False(t, plug.IsEmpty())

		for _, node := range nodes {
			updateNodeFn(node.ID, func(node *structs.Node) {
				node.CSINodePlugins = nil
			})
		}

		ws := memdb.NewWatchSet()
		plug, err := store.CSIPluginByID(ws, plugID)
		require.NoError(t, err)
		require.Nil(t, plug, "plugin was not deleted")

		vol, err = store.CSIVolumeByID(ws, vol.Namespace, vol.ID)
		require.NoError(t, err)
		require.NotNil(t, vol, "volume should be queryable even if plugin is deleted")
		require.False(t, vol.Schedulable)
	})
}

func TestStateStore_Indexes(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	node := mock.Node()

	err := state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	iter, err := state.Indexes()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out []*IndexEntry
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*IndexEntry))
	}

	expect := &IndexEntry{"nodes", 1000}
	if l := len(out); l < 1 {
		t.Fatalf("unexpected number of index entries: %v", pretty.Sprint(out))
	}

	for _, index := range out {
		if index.Key != expect.Key {
			continue
		}
		if index.Value != expect.Value {
			t.Fatalf("bad index; got %d; want %d", index.Value, expect.Value)
		}

		// We matched
		return
	}

	t.Fatal("did not find expected index entry")
}

func TestStateStore_LatestIndex(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	if err := state.UpsertNode(structs.MsgTypeTestSetup, 1000, mock.Node()); err != nil {
		t.Fatalf("err: %v", err)
	}

	exp := uint64(2000)
	if err := state.UpsertJob(structs.MsgTypeTestSetup, exp, mock.Job()); err != nil {
		t.Fatalf("err: %v", err)
	}

	latest, err := state.LatestIndex()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if latest != exp {
		t.Fatalf("LatestIndex() returned %d; want %d", latest, exp)
	}
}

func TestStateStore_UpsertEvals_Eval(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	eval := mock.Eval()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.EvalByID(ws, eval.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(eval, out) {
		t.Fatalf("bad: %#v %#v", eval, out)
	}

	index, err := state.Index("evals")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpsertEvals_CancelBlocked(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Create two blocked evals for the same job
	j := "test-job"
	b1, b2 := mock.Eval(), mock.Eval()
	b1.JobID = j
	b1.Status = structs.EvalStatusBlocked
	b2.JobID = j
	b2.Status = structs.EvalStatusBlocked

	err := state.UpsertEvals(structs.MsgTypeTestSetup, 999, []*structs.Evaluation{b1, b2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create one complete and successful eval for the job
	eval := mock.Eval()
	eval.JobID = j
	eval.Status = structs.EvalStatusComplete

	// Create a watchset so we can test that the upsert of the complete eval
	// fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.EvalByID(ws, b1.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.EvalByID(ws, b2.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	if err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(eval, out) {
		t.Fatalf("bad: %#v %#v", eval, out)
	}

	index, err := state.Index("evals")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	// Get b1/b2 and check they are cancelled
	out1, err := state.EvalByID(ws, b1.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out2, err := state.EvalByID(ws, b2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out1.Status != structs.EvalStatusCancelled || out2.Status != structs.EvalStatusCancelled {
		t.Fatalf("bad: %#v %#v", out1, out2)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_Update_UpsertEvals_Eval(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	eval := mock.Eval()

	err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	ws2 := memdb.NewWatchSet()
	if _, err := state.EvalByID(ws, eval.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	if _, err := state.EvalsByJob(ws2, eval.Namespace, eval.JobID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	eval2 := mock.Eval()
	eval2.ID = eval.ID
	eval2.JobID = eval.JobID
	err = state.UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{eval2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}
	if !watchFired(ws2) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(eval2, out) {
		t.Fatalf("bad: %#v %#v", eval2, out)
	}

	if out.CreateIndex != 1000 {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1001 {
		t.Fatalf("bad: %#v", out)
	}

	index, err := state.Index("evals")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpsertEvals_Eval_ChildJob(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	parent := mock.Job()
	if err := state.UpsertJob(structs.MsgTypeTestSetup, 998, parent); err != nil {
		t.Fatalf("err: %v", err)
	}

	child := mock.Job()
	child.Status = ""
	child.ParentID = parent.ID

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 999, child); err != nil {
		t.Fatalf("err: %v", err)
	}

	eval := mock.Eval()
	eval.Status = structs.EvalStatusComplete
	eval.JobID = child.ID

	// Create watchsets so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	ws2 := memdb.NewWatchSet()
	ws3 := memdb.NewWatchSet()
	if _, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.EvalByID(ws2, eval.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.EvalsByJob(ws3, eval.Namespace, eval.JobID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}
	if !watchFired(ws2) {
		t.Fatalf("bad")
	}
	if !watchFired(ws3) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(eval, out) {
		t.Fatalf("bad: %#v %#v", eval, out)
	}

	index, err := state.Index("evals")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	summary, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if summary == nil {
		t.Fatalf("nil summary")
	}
	if summary.JobID != parent.ID {
		t.Fatalf("bad summary id: %v", parent.ID)
	}
	if summary.Children == nil {
		t.Fatalf("nil children summary")
	}
	if summary.Children.Pending != 0 || summary.Children.Running != 0 || summary.Children.Dead != 1 {
		t.Fatalf("bad children summary: %v", summary.Children)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_DeleteEval_Eval(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	eval1 := mock.Eval()
	eval2 := mock.Eval()
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()

	// Create watchsets so we can test that upsert fires the watch
	watches := make([]memdb.WatchSet, 12)
	for i := 0; i < 12; i++ {
		watches[i] = memdb.NewWatchSet()
	}
	if _, err := state.EvalByID(watches[0], eval1.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.EvalByID(watches[1], eval2.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.EvalsByJob(watches[2], eval1.Namespace, eval1.JobID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.EvalsByJob(watches[3], eval2.Namespace, eval2.JobID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocByID(watches[4], alloc1.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocByID(watches[5], alloc2.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByEval(watches[6], alloc1.EvalID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByEval(watches[7], alloc2.EvalID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByJob(watches[8], alloc1.Namespace, alloc1.JobID, false); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByJob(watches[9], alloc2.Namespace, alloc2.JobID, false); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByNode(watches[10], alloc1.NodeID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByNode(watches[11], alloc2.NodeID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	state.UpsertJobSummary(900, mock.JobSummary(eval1.JobID))
	state.UpsertJobSummary(901, mock.JobSummary(eval2.JobID))
	state.UpsertJobSummary(902, mock.JobSummary(alloc1.JobID))
	state.UpsertJobSummary(903, mock.JobSummary(alloc2.JobID))
	err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1, eval2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc1, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.DeleteEval(1002, []string{eval1.ID, eval2.ID}, []string{alloc1.ID, alloc2.ID})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	for i, ws := range watches {
		if !watchFired(ws) {
			t.Fatalf("bad %d", i)
		}
	}

	ws := memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval1.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", eval1, out)
	}

	out, err = state.EvalByID(ws, eval2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", eval1, out)
	}

	outA, err := state.AllocByID(ws, alloc1.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", alloc1, outA)
	}

	outA, err = state.AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", alloc1, outA)
	}

	index, err := state.Index("evals")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1002 {
		t.Fatalf("bad: %d", index)
	}

	index, err = state.Index("allocs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1002 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_DeleteEval_ChildJob(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	parent := mock.Job()
	if err := state.UpsertJob(structs.MsgTypeTestSetup, 998, parent); err != nil {
		t.Fatalf("err: %v", err)
	}

	child := mock.Job()
	child.Status = ""
	child.ParentID = parent.ID

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 999, child); err != nil {
		t.Fatalf("err: %v", err)
	}

	eval1 := mock.Eval()
	eval1.JobID = child.ID
	alloc1 := mock.Alloc()
	alloc1.JobID = child.ID

	err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc1})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create watchsets so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	err = state.DeleteEval(1002, []string{eval1.ID}, []string{alloc1.ID})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	summary, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if summary == nil {
		t.Fatalf("nil summary")
	}
	if summary.JobID != parent.ID {
		t.Fatalf("bad summary id: %v", parent.ID)
	}
	if summary.Children == nil {
		t.Fatalf("nil children summary")
	}
	if summary.Children.Pending != 0 || summary.Children.Running != 0 || summary.Children.Dead != 1 {
		t.Fatalf("bad children summary: %v", summary.Children)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_EvalsByJob(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	eval1 := mock.Eval()
	eval2 := mock.Eval()
	eval2.JobID = eval1.JobID
	eval3 := mock.Eval()
	evals := []*structs.Evaluation{eval1, eval2}

	err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, evals)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	err = state.UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{eval3})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	out, err := state.EvalsByJob(ws, eval1.Namespace, eval1.JobID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	sort.Sort(EvalIDSort(evals))
	sort.Sort(EvalIDSort(out))

	if !reflect.DeepEqual(evals, out) {
		t.Fatalf("bad: %#v %#v", evals, out)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_Evals(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	var evals []*structs.Evaluation

	for i := 0; i < 10; i++ {
		eval := mock.Eval()
		evals = append(evals, eval)

		err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000+uint64(i), []*structs.Evaluation{eval})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	ws := memdb.NewWatchSet()
	iter, err := state.Evals(ws, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out []*structs.Evaluation
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Evaluation))
	}

	sort.Sort(EvalIDSort(evals))
	sort.Sort(EvalIDSort(out))

	if !reflect.DeepEqual(evals, out) {
		t.Fatalf("bad: %#v %#v", evals, out)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_EvalsByIDPrefix(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	var evals []*structs.Evaluation

	ids := []string{
		"aaaaaaaa-7bfb-395d-eb95-0685af2176b2",
		"aaaaaaab-7bfb-395d-eb95-0685af2176b2",
		"aaaaaabb-7bfb-395d-eb95-0685af2176b2",
		"aaaaabbb-7bfb-395d-eb95-0685af2176b2",
		"aaaabbbb-7bfb-395d-eb95-0685af2176b2",
		"aaabbbbb-7bfb-395d-eb95-0685af2176b2",
		"aabbbbbb-7bfb-395d-eb95-0685af2176b2",
		"abbbbbbb-7bfb-395d-eb95-0685af2176b2",
		"bbbbbbbb-7bfb-395d-eb95-0685af2176b2",
	}
	for i := 0; i < 9; i++ {
		eval := mock.Eval()
		eval.ID = ids[i]
		evals = append(evals, eval)
	}

	err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, evals)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	iter, err := state.EvalsByIDPrefix(ws, structs.DefaultNamespace, "aaaa")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	gatherEvals := func(iter memdb.ResultIterator) []*structs.Evaluation {
		var evals []*structs.Evaluation
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			evals = append(evals, raw.(*structs.Evaluation))
		}
		return evals
	}

	out := gatherEvals(iter)
	if len(out) != 5 {
		t.Fatalf("bad: expected five evaluations, got: %#v", out)
	}

	sort.Sort(EvalIDSort(evals))

	for index, eval := range out {
		if ids[index] != eval.ID {
			t.Fatalf("bad: got unexpected id: %s", eval.ID)
		}
	}

	iter, err = state.EvalsByIDPrefix(ws, structs.DefaultNamespace, "b-a7bfb")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out = gatherEvals(iter)
	if len(out) != 0 {
		t.Fatalf("bad: unexpected zero evaluations, got: %#v", out)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpdateAllocsFromClient(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	parent := mock.Job()
	if err := state.UpsertJob(structs.MsgTypeTestSetup, 998, parent); err != nil {
		t.Fatalf("err: %v", err)
	}

	child := mock.Job()
	child.Status = ""
	child.ParentID = parent.ID
	if err := state.UpsertJob(structs.MsgTypeTestSetup, 999, child); err != nil {
		t.Fatalf("err: %v", err)
	}

	alloc := mock.Alloc()
	alloc.JobID = child.ID
	alloc.Job = child

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	summary, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if summary == nil {
		t.Fatalf("nil summary")
	}
	if summary.JobID != parent.ID {
		t.Fatalf("bad summary id: %v", parent.ID)
	}
	if summary.Children == nil {
		t.Fatalf("nil children summary")
	}
	if summary.Children.Pending != 0 || summary.Children.Running != 1 || summary.Children.Dead != 0 {
		t.Fatalf("bad children summary: %v", summary.Children)
	}

	// Create watchsets so we can test that update fires the watch
	ws = memdb.NewWatchSet()
	if _, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create the delta updates
	ts := map[string]*structs.TaskState{"web": {State: structs.TaskStateRunning}}
	update := &structs.Allocation{
		ID:           alloc.ID,
		ClientStatus: structs.AllocClientStatusComplete,
		TaskStates:   ts,
		JobID:        alloc.JobID,
		TaskGroup:    alloc.TaskGroup,
	}
	err = state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{update})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	summary, err = state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if summary == nil {
		t.Fatalf("nil summary")
	}
	if summary.JobID != parent.ID {
		t.Fatalf("bad summary id: %v", parent.ID)
	}
	if summary.Children == nil {
		t.Fatalf("nil children summary")
	}
	if summary.Children.Pending != 0 || summary.Children.Running != 0 || summary.Children.Dead != 1 {
		t.Fatalf("bad children summary: %v", summary.Children)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpdateAllocsFromClient_ChildJob(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 999, alloc1.Job); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := state.UpsertJob(structs.MsgTypeTestSetup, 999, alloc2.Job); err != nil {
		t.Fatalf("err: %v", err)
	}

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc1, alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create watchsets so we can test that update fires the watch
	watches := make([]memdb.WatchSet, 8)
	for i := 0; i < 8; i++ {
		watches[i] = memdb.NewWatchSet()
	}
	if _, err := state.AllocByID(watches[0], alloc1.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocByID(watches[1], alloc2.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByEval(watches[2], alloc1.EvalID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByEval(watches[3], alloc2.EvalID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByJob(watches[4], alloc1.Namespace, alloc1.JobID, false); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByJob(watches[5], alloc2.Namespace, alloc2.JobID, false); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByNode(watches[6], alloc1.NodeID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByNode(watches[7], alloc2.NodeID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create the delta updates
	ts := map[string]*structs.TaskState{"web": {State: structs.TaskStatePending}}
	update := &structs.Allocation{
		ID:           alloc1.ID,
		ClientStatus: structs.AllocClientStatusFailed,
		TaskStates:   ts,
		JobID:        alloc1.JobID,
		TaskGroup:    alloc1.TaskGroup,
	}
	update2 := &structs.Allocation{
		ID:           alloc2.ID,
		ClientStatus: structs.AllocClientStatusRunning,
		TaskStates:   ts,
		JobID:        alloc2.JobID,
		TaskGroup:    alloc2.TaskGroup,
	}

	err = state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{update, update2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	for i, ws := range watches {
		if !watchFired(ws) {
			t.Fatalf("bad %d", i)
		}
	}

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc1.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	alloc1.CreateIndex = 1000
	alloc1.ModifyIndex = 1001
	alloc1.TaskStates = ts
	alloc1.ClientStatus = structs.AllocClientStatusFailed
	if !reflect.DeepEqual(alloc1, out) {
		t.Fatalf("bad: %#v %#v", alloc1, out)
	}

	out, err = state.AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	alloc2.ModifyIndex = 1000
	alloc2.ModifyIndex = 1001
	alloc2.ClientStatus = structs.AllocClientStatusRunning
	alloc2.TaskStates = ts
	if !reflect.DeepEqual(alloc2, out) {
		t.Fatalf("bad: %#v %#v", alloc2, out)
	}

	index, err := state.Index("allocs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	// Ensure summaries have been updated
	summary, err := state.JobSummaryByID(ws, alloc1.Namespace, alloc1.JobID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	tgSummary := summary.Summary["web"]
	if tgSummary.Failed != 1 {
		t.Fatalf("expected failed: %v, actual: %v, summary: %#v", 1, tgSummary.Failed, tgSummary)
	}

	summary2, err := state.JobSummaryByID(ws, alloc2.Namespace, alloc2.JobID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	tgSummary2 := summary2.Summary["web"]
	if tgSummary2.Running != 1 {
		t.Fatalf("expected running: %v, actual: %v", 1, tgSummary2.Running)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpdateMultipleAllocsFromClient(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	alloc := mock.Alloc()

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 999, alloc.Job); err != nil {
		t.Fatalf("err: %v", err)
	}
	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create the delta updates
	ts := map[string]*structs.TaskState{"web": {State: structs.TaskStatePending}}
	update := &structs.Allocation{
		ID:           alloc.ID,
		ClientStatus: structs.AllocClientStatusRunning,
		TaskStates:   ts,
		JobID:        alloc.JobID,
		TaskGroup:    alloc.TaskGroup,
	}
	update2 := &structs.Allocation{
		ID:           alloc.ID,
		ClientStatus: structs.AllocClientStatusPending,
		TaskStates:   ts,
		JobID:        alloc.JobID,
		TaskGroup:    alloc.TaskGroup,
	}

	err = state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{update, update2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	alloc.CreateIndex = 1000
	alloc.ModifyIndex = 1001
	alloc.TaskStates = ts
	alloc.ClientStatus = structs.AllocClientStatusPending
	if !reflect.DeepEqual(alloc, out) {
		t.Fatalf("bad: %#v , actual:%#v", alloc, out)
	}

	summary, err := state.JobSummaryByID(ws, alloc.Namespace, alloc.JobID)
	expectedSummary := &structs.JobSummary{
		JobID:     alloc.JobID,
		Namespace: alloc.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {
				Starting: 1,
			},
		},
		Children:    new(structs.JobChildrenSummary),
		CreateIndex: 999,
		ModifyIndex: 1001,
	}
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !reflect.DeepEqual(summary, expectedSummary) {
		t.Fatalf("expected: %#v, actual: %#v", expectedSummary, summary)
	}
}

func TestStateStore_UpdateAllocsFromClient_Deployment(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)

	alloc := mock.Alloc()
	now := time.Now()
	alloc.CreateTime = now.UnixNano()
	pdeadline := 5 * time.Minute
	deployment := mock.Deployment()
	deployment.TaskGroups[alloc.TaskGroup].ProgressDeadline = pdeadline
	alloc.DeploymentID = deployment.ID

	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, alloc.Job))
	require.Nil(state.UpsertDeployment(1000, deployment))
	require.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}))

	healthy := now.Add(time.Second)
	update := &structs.Allocation{
		ID:           alloc.ID,
		ClientStatus: structs.AllocClientStatusRunning,
		JobID:        alloc.JobID,
		TaskGroup:    alloc.TaskGroup,
		DeploymentStatus: &structs.AllocDeploymentStatus{
			Healthy:   helper.BoolToPtr(true),
			Timestamp: healthy,
		},
	}
	require.Nil(state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{update}))

	// Check that the deployment state was updated because the healthy
	// deployment
	dout, err := state.DeploymentByID(nil, deployment.ID)
	require.Nil(err)
	require.NotNil(dout)
	require.Len(dout.TaskGroups, 1)
	dstate := dout.TaskGroups[alloc.TaskGroup]
	require.NotNil(dstate)
	require.Equal(1, dstate.PlacedAllocs)
	require.True(healthy.Add(pdeadline).Equal(dstate.RequireProgressBy))
}

// This tests that the deployment state is merged correctly
func TestStateStore_UpdateAllocsFromClient_DeploymentStateMerges(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)
	alloc := mock.Alloc()
	now := time.Now()
	alloc.CreateTime = now.UnixNano()
	pdeadline := 5 * time.Minute
	deployment := mock.Deployment()
	deployment.TaskGroups[alloc.TaskGroup].ProgressDeadline = pdeadline
	alloc.DeploymentID = deployment.ID
	alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
		Canary: true,
	}

	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, alloc.Job))
	require.Nil(state.UpsertDeployment(1000, deployment))
	require.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}))

	update := &structs.Allocation{
		ID:           alloc.ID,
		ClientStatus: structs.AllocClientStatusRunning,
		JobID:        alloc.JobID,
		TaskGroup:    alloc.TaskGroup,
		DeploymentStatus: &structs.AllocDeploymentStatus{
			Healthy: helper.BoolToPtr(true),
			Canary:  false,
		},
	}
	require.Nil(state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{update}))

	// Check that the merging of the deployment status was correct
	out, err := state.AllocByID(nil, alloc.ID)
	require.Nil(err)
	require.NotNil(out)
	require.True(out.DeploymentStatus.Canary)
	require.NotNil(out.DeploymentStatus.Healthy)
	require.True(*out.DeploymentStatus.Healthy)
}

func TestStateStore_UpsertAlloc_Alloc(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	alloc := mock.Alloc()

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 999, alloc.Job); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create watchsets so we can test that update fires the watch
	watches := make([]memdb.WatchSet, 4)
	for i := 0; i < 4; i++ {
		watches[i] = memdb.NewWatchSet()
	}
	if _, err := state.AllocByID(watches[0], alloc.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByEval(watches[1], alloc.EvalID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByJob(watches[2], alloc.Namespace, alloc.JobID, false); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByNode(watches[3], alloc.NodeID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	for i, ws := range watches {
		if !watchFired(ws) {
			t.Fatalf("bad %d", i)
		}
	}

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(alloc, out) {
		t.Fatalf("bad: %#v %#v", alloc, out)
	}

	index, err := state.Index("allocs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	summary, err := state.JobSummaryByID(ws, alloc.Namespace, alloc.JobID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	tgSummary, ok := summary.Summary["web"]
	if !ok {
		t.Fatalf("no summary for task group web")
	}
	if tgSummary.Starting != 1 {
		t.Fatalf("expected queued: %v, actual: %v", 1, tgSummary.Starting)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpsertAlloc_Deployment(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)
	alloc := mock.Alloc()
	now := time.Now()
	alloc.CreateTime = now.UnixNano()
	alloc.ModifyTime = now.UnixNano()
	pdeadline := 5 * time.Minute
	deployment := mock.Deployment()
	deployment.TaskGroups[alloc.TaskGroup].ProgressDeadline = pdeadline
	alloc.DeploymentID = deployment.ID

	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, alloc.Job))
	require.Nil(state.UpsertDeployment(1000, deployment))

	// Create a watch set so we can test that update fires the watch
	ws := memdb.NewWatchSet()
	require.Nil(state.AllocsByDeployment(ws, alloc.DeploymentID))

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc})
	require.Nil(err)

	if !watchFired(ws) {
		t.Fatalf("watch not fired")
	}

	ws = memdb.NewWatchSet()
	allocs, err := state.AllocsByDeployment(ws, alloc.DeploymentID)
	require.Nil(err)
	require.Len(allocs, 1)
	require.EqualValues(alloc, allocs[0])

	index, err := state.Index("allocs")
	require.Nil(err)
	require.EqualValues(1001, index)
	if watchFired(ws) {
		t.Fatalf("bad")
	}

	// Check that the deployment state was updated
	dout, err := state.DeploymentByID(nil, deployment.ID)
	require.Nil(err)
	require.NotNil(dout)
	require.Len(dout.TaskGroups, 1)
	dstate := dout.TaskGroups[alloc.TaskGroup]
	require.NotNil(dstate)
	require.Equal(1, dstate.PlacedAllocs)
	require.True(now.Add(pdeadline).Equal(dstate.RequireProgressBy))
}

// Testing to ensure we keep issue
// https://github.com/hashicorp/nomad/issues/2583 fixed
func TestStateStore_UpsertAlloc_No_Job(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	alloc := mock.Alloc()
	alloc.Job = nil

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 999, []*structs.Allocation{alloc})
	if err == nil || !strings.Contains(err.Error(), "without a job") {
		t.Fatalf("expect err: %v", err)
	}
}

func TestStateStore_UpsertAlloc_ChildJob(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	parent := mock.Job()
	require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 998, parent))

	child := mock.Job()
	child.Status = ""
	child.ParentID = parent.ID

	require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, child))

	alloc := mock.Alloc()
	alloc.JobID = child.ID
	alloc.Job = child

	// Create watchsets so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	require.NoError(t, err)

	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc})
	require.NoError(t, err)

	require.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	summary, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	require.NoError(t, err)
	require.NotNil(t, summary)

	require.Equal(t, parent.ID, summary.JobID)
	require.NotNil(t, summary.Children)

	require.Equal(t, int64(0), summary.Children.Pending)
	require.Equal(t, int64(1), summary.Children.Running)
	require.Equal(t, int64(0), summary.Children.Dead)

	require.False(t, watchFired(ws))
}

func TestStateStore_UpdateAlloc_Alloc(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	alloc := mock.Alloc()

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 999, alloc.Job); err != nil {
		t.Fatalf("err: %v", err)
	}

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	summary, err := state.JobSummaryByID(ws, alloc.Namespace, alloc.JobID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	tgSummary := summary.Summary["web"]
	if tgSummary.Starting != 1 {
		t.Fatalf("expected starting: %v, actual: %v", 1, tgSummary.Starting)
	}

	alloc2 := mock.Alloc()
	alloc2.ID = alloc.ID
	alloc2.NodeID = alloc.NodeID + ".new"
	state.UpsertJobSummary(1001, mock.JobSummary(alloc2.JobID))

	// Create watchsets so we can test that update fires the watch
	watches := make([]memdb.WatchSet, 4)
	for i := 0; i < 4; i++ {
		watches[i] = memdb.NewWatchSet()
	}
	if _, err := state.AllocByID(watches[0], alloc2.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByEval(watches[1], alloc2.EvalID); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByJob(watches[2], alloc2.Namespace, alloc2.JobID, false); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if _, err := state.AllocsByNode(watches[3], alloc2.NodeID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	for i, ws := range watches {
		if !watchFired(ws) {
			t.Fatalf("bad %d", i)
		}
	}

	ws = memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(alloc2, out) {
		t.Fatalf("bad: %#v %#v", alloc2, out)
	}

	if out.CreateIndex != 1000 {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1002 {
		t.Fatalf("bad: %#v", out)
	}

	index, err := state.Index("allocs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1002 {
		t.Fatalf("bad: %d", index)
	}

	// Ensure that summary hasb't changed
	summary, err = state.JobSummaryByID(ws, alloc.Namespace, alloc.JobID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	tgSummary = summary.Summary["web"]
	if tgSummary.Starting != 1 {
		t.Fatalf("expected starting: %v, actual: %v", 1, tgSummary.Starting)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

// This test ensures that the state store will mark the clients status as lost
// when set rather than preferring the existing status.
func TestStateStore_UpdateAlloc_Lost(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	alloc := mock.Alloc()
	alloc.ClientStatus = "foo"

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 999, alloc.Job); err != nil {
		t.Fatalf("err: %v", err)
	}

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	alloc2 := new(structs.Allocation)
	*alloc2 = *alloc
	alloc2.ClientStatus = structs.AllocClientStatusLost
	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc2}); err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out.ClientStatus != structs.AllocClientStatusLost {
		t.Fatalf("bad: %#v", out)
	}
}

// This test ensures an allocation can be updated when there is no job
// associated with it. This will happen when a job is stopped by an user which
// has non-terminal allocations on clients
func TestStateStore_UpdateAlloc_NoJob(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	alloc := mock.Alloc()

	// Upsert a job
	state.UpsertJobSummary(998, mock.JobSummary(alloc.JobID))
	if err := state.UpsertJob(structs.MsgTypeTestSetup, 999, alloc.Job); err != nil {
		t.Fatalf("err: %v", err)
	}

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := state.DeleteJob(1001, alloc.Namespace, alloc.JobID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the desired state of the allocation to stop
	allocCopy := alloc.Copy()
	allocCopy.DesiredStatus = structs.AllocDesiredStatusStop
	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{allocCopy}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the client state of the allocation to complete
	allocCopy1 := allocCopy.Copy()
	allocCopy1.ClientStatus = structs.AllocClientStatusComplete
	if err := state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1003, []*structs.Allocation{allocCopy1}); err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	out, _ := state.AllocByID(ws, alloc.ID)
	// Update the modify index of the alloc before comparing
	allocCopy1.ModifyIndex = 1003
	if !reflect.DeepEqual(out, allocCopy1) {
		t.Fatalf("expected: %#v \n actual: %#v", allocCopy1, out)
	}
}

func TestStateStore_UpdateAllocDesiredTransition(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)
	alloc := mock.Alloc()

	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 999, alloc.Job))
	require.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc}))

	t1 := &structs.DesiredTransition{
		Migrate: helper.BoolToPtr(true),
	}
	t2 := &structs.DesiredTransition{
		Migrate: helper.BoolToPtr(false),
	}
	eval := &structs.Evaluation{
		ID:             uuid.Generate(),
		Namespace:      alloc.Namespace,
		Priority:       alloc.Job.Priority,
		Type:           alloc.Job.Type,
		TriggeredBy:    structs.EvalTriggerNodeDrain,
		JobID:          alloc.Job.ID,
		JobModifyIndex: alloc.Job.ModifyIndex,
		Status:         structs.EvalStatusPending,
	}
	evals := []*structs.Evaluation{eval}

	m := map[string]*structs.DesiredTransition{alloc.ID: t1}
	require.Nil(state.UpdateAllocsDesiredTransitions(structs.MsgTypeTestSetup, 1001, m, evals))

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	require.Nil(err)
	require.NotNil(out.DesiredTransition.Migrate)
	require.True(*out.DesiredTransition.Migrate)
	require.EqualValues(1000, out.CreateIndex)
	require.EqualValues(1001, out.ModifyIndex)

	index, err := state.Index("allocs")
	require.Nil(err)
	require.EqualValues(1001, index)

	// Check the eval is created
	eout, err := state.EvalByID(nil, eval.ID)
	require.Nil(err)
	require.NotNil(eout)

	m = map[string]*structs.DesiredTransition{alloc.ID: t2}
	require.Nil(state.UpdateAllocsDesiredTransitions(structs.MsgTypeTestSetup, 1002, m, evals))

	ws = memdb.NewWatchSet()
	out, err = state.AllocByID(ws, alloc.ID)
	require.Nil(err)
	require.NotNil(out.DesiredTransition.Migrate)
	require.False(*out.DesiredTransition.Migrate)
	require.EqualValues(1000, out.CreateIndex)
	require.EqualValues(1002, out.ModifyIndex)

	index, err = state.Index("allocs")
	require.Nil(err)
	require.EqualValues(1002, index)

	// Try with a bogus alloc id
	m = map[string]*structs.DesiredTransition{uuid.Generate(): t2}
	require.Nil(state.UpdateAllocsDesiredTransitions(structs.MsgTypeTestSetup, 1003, m, evals))
}

func TestStateStore_JobSummary(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Add a job
	job := mock.Job()
	state.UpsertJob(structs.MsgTypeTestSetup, 900, job)

	// Get the job back
	ws := memdb.NewWatchSet()
	outJob, _ := state.JobByID(ws, job.Namespace, job.ID)
	if outJob.CreateIndex != 900 {
		t.Fatalf("bad create index: %v", outJob.CreateIndex)
	}
	summary, _ := state.JobSummaryByID(ws, job.Namespace, job.ID)
	if summary.CreateIndex != 900 {
		t.Fatalf("bad create index: %v", summary.CreateIndex)
	}

	// Upsert an allocation
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	alloc.Job = job
	state.UpsertAllocs(structs.MsgTypeTestSetup, 910, []*structs.Allocation{alloc})

	// Update the alloc from client
	alloc1 := alloc.Copy()
	alloc1.ClientStatus = structs.AllocClientStatusPending
	alloc1.DesiredStatus = ""
	state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 920, []*structs.Allocation{alloc})

	alloc3 := alloc.Copy()
	alloc3.ClientStatus = structs.AllocClientStatusRunning
	alloc3.DesiredStatus = ""
	state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 930, []*structs.Allocation{alloc3})

	// Upsert the alloc
	alloc4 := alloc.Copy()
	alloc4.ClientStatus = structs.AllocClientStatusPending
	alloc4.DesiredStatus = structs.AllocDesiredStatusRun
	state.UpsertAllocs(structs.MsgTypeTestSetup, 950, []*structs.Allocation{alloc4})

	// Again upsert the alloc
	alloc5 := alloc.Copy()
	alloc5.ClientStatus = structs.AllocClientStatusPending
	alloc5.DesiredStatus = structs.AllocDesiredStatusRun
	state.UpsertAllocs(structs.MsgTypeTestSetup, 970, []*structs.Allocation{alloc5})

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	expectedSummary := structs.JobSummary{
		JobID:     job.ID,
		Namespace: job.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {
				Running: 1,
			},
		},
		Children:    new(structs.JobChildrenSummary),
		CreateIndex: 900,
		ModifyIndex: 930,
	}

	summary, _ = state.JobSummaryByID(ws, job.Namespace, job.ID)
	if !reflect.DeepEqual(&expectedSummary, summary) {
		t.Fatalf("expected: %#v, actual: %v", expectedSummary, summary)
	}

	// De-register the job.
	state.DeleteJob(980, job.Namespace, job.ID)

	// Shouldn't have any effect on the summary
	alloc6 := alloc.Copy()
	alloc6.ClientStatus = structs.AllocClientStatusRunning
	alloc6.DesiredStatus = ""
	state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 990, []*structs.Allocation{alloc6})

	// We shouldn't have any summary at this point
	summary, _ = state.JobSummaryByID(ws, job.Namespace, job.ID)
	if summary != nil {
		t.Fatalf("expected nil, actual: %#v", summary)
	}

	// Re-register the same job
	job1 := mock.Job()
	job1.ID = job.ID
	state.UpsertJob(structs.MsgTypeTestSetup, 1000, job1)
	outJob2, _ := state.JobByID(ws, job1.Namespace, job1.ID)
	if outJob2.CreateIndex != 1000 {
		t.Fatalf("bad create index: %v", outJob2.CreateIndex)
	}
	summary, _ = state.JobSummaryByID(ws, job1.Namespace, job1.ID)
	if summary.CreateIndex != 1000 {
		t.Fatalf("bad create index: %v", summary.CreateIndex)
	}

	// Upsert an allocation
	alloc7 := alloc.Copy()
	alloc7.JobID = outJob.ID
	alloc7.Job = outJob
	alloc7.ClientStatus = structs.AllocClientStatusComplete
	alloc7.DesiredStatus = structs.AllocDesiredStatusRun
	state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1020, []*structs.Allocation{alloc7})

	expectedSummary = structs.JobSummary{
		JobID:     job.ID,
		Namespace: job.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {},
		},
		Children:    new(structs.JobChildrenSummary),
		CreateIndex: 1000,
		ModifyIndex: 1000,
	}

	summary, _ = state.JobSummaryByID(ws, job1.Namespace, job1.ID)
	if !reflect.DeepEqual(&expectedSummary, summary) {
		t.Fatalf("expected: %#v, actual: %#v", expectedSummary, summary)
	}
}

func TestStateStore_ReconcileJobSummary(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Create an alloc
	alloc := mock.Alloc()

	// Add another task group to the job
	tg2 := alloc.Job.TaskGroups[0].Copy()
	tg2.Name = "db"
	alloc.Job.TaskGroups = append(alloc.Job.TaskGroups, tg2)
	state.UpsertJob(structs.MsgTypeTestSetup, 100, alloc.Job)

	// Create one more alloc for the db task group
	alloc2 := mock.Alloc()
	alloc2.TaskGroup = "db"
	alloc2.JobID = alloc.JobID
	alloc2.Job = alloc.Job

	// Upserts the alloc
	state.UpsertAllocs(structs.MsgTypeTestSetup, 110, []*structs.Allocation{alloc, alloc2})

	// Change the state of the first alloc to running
	alloc3 := alloc.Copy()
	alloc3.ClientStatus = structs.AllocClientStatusRunning
	state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 120, []*structs.Allocation{alloc3})

	//Add some more allocs to the second tg
	alloc4 := mock.Alloc()
	alloc4.JobID = alloc.JobID
	alloc4.Job = alloc.Job
	alloc4.TaskGroup = "db"
	alloc5 := alloc4.Copy()
	alloc5.ClientStatus = structs.AllocClientStatusRunning

	alloc6 := mock.Alloc()
	alloc6.JobID = alloc.JobID
	alloc6.Job = alloc.Job
	alloc6.TaskGroup = "db"
	alloc7 := alloc6.Copy()
	alloc7.ClientStatus = structs.AllocClientStatusComplete

	alloc8 := mock.Alloc()
	alloc8.JobID = alloc.JobID
	alloc8.Job = alloc.Job
	alloc8.TaskGroup = "db"
	alloc9 := alloc8.Copy()
	alloc9.ClientStatus = structs.AllocClientStatusFailed

	alloc10 := mock.Alloc()
	alloc10.JobID = alloc.JobID
	alloc10.Job = alloc.Job
	alloc10.TaskGroup = "db"
	alloc11 := alloc10.Copy()
	alloc11.ClientStatus = structs.AllocClientStatusLost

	state.UpsertAllocs(structs.MsgTypeTestSetup, 130, []*structs.Allocation{alloc4, alloc6, alloc8, alloc10})

	state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 150, []*structs.Allocation{alloc5, alloc7, alloc9, alloc11})

	// DeleteJobSummary is a helper method and doesn't modify the indexes table
	state.DeleteJobSummary(130, alloc.Namespace, alloc.Job.ID)

	state.ReconcileJobSummaries(120)

	ws := memdb.NewWatchSet()
	summary, _ := state.JobSummaryByID(ws, alloc.Namespace, alloc.Job.ID)
	expectedSummary := structs.JobSummary{
		JobID:     alloc.Job.ID,
		Namespace: alloc.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {
				Running: 1,
			},
			"db": {
				Starting: 1,
				Running:  1,
				Failed:   1,
				Complete: 1,
				Lost:     1,
			},
		},
		CreateIndex: 100,
		ModifyIndex: 120,
	}
	if !reflect.DeepEqual(&expectedSummary, summary) {
		t.Fatalf("expected: %v, actual: %v", expectedSummary, summary)
	}
}

func TestStateStore_ReconcileParentJobSummary(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)

	// Add a node
	node := mock.Node()
	state.UpsertNode(structs.MsgTypeTestSetup, 80, node)

	// Make a parameterized job
	job1 := mock.BatchJob()
	job1.ID = "test"
	job1.ParameterizedJob = &structs.ParameterizedJobConfig{
		Payload: "random",
	}
	job1.TaskGroups[0].Count = 1
	state.UpsertJob(structs.MsgTypeTestSetup, 100, job1)

	// Make a child job
	childJob := job1.Copy()
	childJob.ID = job1.ID + "dispatch-23423423"
	childJob.ParentID = job1.ID
	childJob.Dispatched = true
	childJob.Status = structs.JobStatusRunning

	// Make some allocs for child job
	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	alloc.Job = childJob
	alloc.JobID = childJob.ID
	alloc.ClientStatus = structs.AllocClientStatusRunning

	alloc2 := mock.Alloc()
	alloc2.NodeID = node.ID
	alloc2.Job = childJob
	alloc2.JobID = childJob.ID
	alloc2.ClientStatus = structs.AllocClientStatusFailed

	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 110, childJob))
	require.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 111, []*structs.Allocation{alloc, alloc2}))

	// Make the summary incorrect in the state store
	summary, err := state.JobSummaryByID(nil, job1.Namespace, job1.ID)
	require.Nil(err)

	summary.Children = nil
	summary.Summary = make(map[string]structs.TaskGroupSummary)
	summary.Summary["web"] = structs.TaskGroupSummary{
		Queued: 1,
	}

	// Delete the child job summary
	state.DeleteJobSummary(125, childJob.Namespace, childJob.ID)

	state.ReconcileJobSummaries(120)

	ws := memdb.NewWatchSet()

	// Verify parent summary is corrected
	summary, _ = state.JobSummaryByID(ws, alloc.Namespace, job1.ID)
	expectedSummary := structs.JobSummary{
		JobID:     job1.ID,
		Namespace: job1.Namespace,
		Summary:   make(map[string]structs.TaskGroupSummary),
		Children: &structs.JobChildrenSummary{
			Running: 1,
		},
		CreateIndex: 100,
		ModifyIndex: 120,
	}
	require.Equal(&expectedSummary, summary)

	// Verify child job summary is also correct
	childSummary, _ := state.JobSummaryByID(ws, childJob.Namespace, childJob.ID)
	expectedChildSummary := structs.JobSummary{
		JobID:     childJob.ID,
		Namespace: childJob.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {
				Running: 1,
				Failed:  1,
			},
		},
		CreateIndex: 110,
		ModifyIndex: 120,
	}
	require.Equal(&expectedChildSummary, childSummary)
}

func TestStateStore_UpdateAlloc_JobNotPresent(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	alloc := mock.Alloc()
	state.UpsertJob(structs.MsgTypeTestSetup, 100, alloc.Job)
	state.UpsertAllocs(structs.MsgTypeTestSetup, 200, []*structs.Allocation{alloc})

	// Delete the job
	state.DeleteJob(300, alloc.Namespace, alloc.Job.ID)

	// Update the alloc
	alloc1 := alloc.Copy()
	alloc1.ClientStatus = structs.AllocClientStatusRunning

	// Updating allocation should not throw any error
	if err := state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 400, []*structs.Allocation{alloc1}); err != nil {
		t.Fatalf("expect err: %v", err)
	}

	// Re-Register the job
	state.UpsertJob(structs.MsgTypeTestSetup, 500, alloc.Job)

	// Update the alloc again
	alloc2 := alloc.Copy()
	alloc2.ClientStatus = structs.AllocClientStatusComplete
	if err := state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 400, []*structs.Allocation{alloc1}); err != nil {
		t.Fatalf("expect err: %v", err)
	}

	// Job Summary of the newly registered job shouldn't account for the
	// allocation update for the older job
	expectedSummary := structs.JobSummary{
		JobID:     alloc1.JobID,
		Namespace: alloc1.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {},
		},
		Children:    new(structs.JobChildrenSummary),
		CreateIndex: 500,
		ModifyIndex: 500,
	}

	ws := memdb.NewWatchSet()
	summary, _ := state.JobSummaryByID(ws, alloc.Namespace, alloc.Job.ID)
	if !reflect.DeepEqual(&expectedSummary, summary) {
		t.Fatalf("expected: %v, actual: %v", expectedSummary, summary)
	}
}

func TestStateStore_EvictAlloc_Alloc(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	alloc := mock.Alloc()

	state.UpsertJobSummary(999, mock.JobSummary(alloc.JobID))
	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	alloc2 := new(structs.Allocation)
	*alloc2 = *alloc
	alloc2.DesiredStatus = structs.AllocDesiredStatusEvict
	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out.DesiredStatus != structs.AllocDesiredStatusEvict {
		t.Fatalf("bad: %#v %#v", alloc, out)
	}

	index, err := state.Index("allocs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_AllocsByNode(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	var allocs []*structs.Allocation

	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.NodeID = "foo"
		allocs = append(allocs, alloc)
	}

	for idx, alloc := range allocs {
		state.UpsertJobSummary(uint64(900+idx), mock.JobSummary(alloc.JobID))
	}

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	out, err := state.AllocsByNode(ws, "foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	sort.Sort(AllocIDSort(allocs))
	sort.Sort(AllocIDSort(out))

	if !reflect.DeepEqual(allocs, out) {
		t.Fatalf("bad: %#v %#v", allocs, out)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_AllocsByNodeTerminal(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	var allocs, term, nonterm []*structs.Allocation

	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.NodeID = "foo"
		if i%2 == 0 {
			alloc.DesiredStatus = structs.AllocDesiredStatusStop
			term = append(term, alloc)
		} else {
			nonterm = append(nonterm, alloc)
		}
		allocs = append(allocs, alloc)
	}

	for idx, alloc := range allocs {
		state.UpsertJobSummary(uint64(900+idx), mock.JobSummary(alloc.JobID))
	}

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify the terminal allocs
	ws := memdb.NewWatchSet()
	out, err := state.AllocsByNodeTerminal(ws, "foo", true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	sort.Sort(AllocIDSort(term))
	sort.Sort(AllocIDSort(out))

	if !reflect.DeepEqual(term, out) {
		t.Fatalf("bad: %#v %#v", term, out)
	}

	// Verify the non-terminal allocs
	out, err = state.AllocsByNodeTerminal(ws, "foo", false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	sort.Sort(AllocIDSort(nonterm))
	sort.Sort(AllocIDSort(out))

	if !reflect.DeepEqual(nonterm, out) {
		t.Fatalf("bad: %#v %#v", nonterm, out)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_AllocsByJob(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	var allocs []*structs.Allocation

	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.JobID = "foo"
		allocs = append(allocs, alloc)
	}

	for i, alloc := range allocs {
		state.UpsertJobSummary(uint64(900+i), mock.JobSummary(alloc.JobID))
	}

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	out, err := state.AllocsByJob(ws, mock.Alloc().Namespace, "foo", false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	sort.Sort(AllocIDSort(allocs))
	sort.Sort(AllocIDSort(out))

	if !reflect.DeepEqual(allocs, out) {
		t.Fatalf("bad: %#v %#v", allocs, out)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_AllocsForRegisteredJob(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	var allocs []*structs.Allocation
	var allocs1 []*structs.Allocation

	job := mock.Job()
	job.ID = "foo"
	state.UpsertJob(structs.MsgTypeTestSetup, 100, job)
	for i := 0; i < 3; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		allocs = append(allocs, alloc)
	}
	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 200, allocs); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := state.DeleteJob(250, job.Namespace, job.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	job1 := mock.Job()
	job1.ID = "foo"
	job1.CreateIndex = 50
	state.UpsertJob(structs.MsgTypeTestSetup, 300, job1)
	for i := 0; i < 4; i++ {
		alloc := mock.Alloc()
		alloc.Job = job1
		alloc.JobID = job1.ID
		allocs1 = append(allocs1, alloc)
	}

	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs1); err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	out, err := state.AllocsByJob(ws, job1.Namespace, job1.ID, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	expected := len(allocs) + len(allocs1)
	if len(out) != expected {
		t.Fatalf("expected: %v, actual: %v", expected, len(out))
	}

	out1, err := state.AllocsByJob(ws, job1.Namespace, job1.ID, false)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	expected = len(allocs1)
	if len(out1) != expected {
		t.Fatalf("expected: %v, actual: %v", expected, len(out1))
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_AllocsByIDPrefix(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	var allocs []*structs.Allocation

	ids := []string{
		"aaaaaaaa-7bfb-395d-eb95-0685af2176b2",
		"aaaaaaab-7bfb-395d-eb95-0685af2176b2",
		"aaaaaabb-7bfb-395d-eb95-0685af2176b2",
		"aaaaabbb-7bfb-395d-eb95-0685af2176b2",
		"aaaabbbb-7bfb-395d-eb95-0685af2176b2",
		"aaabbbbb-7bfb-395d-eb95-0685af2176b2",
		"aabbbbbb-7bfb-395d-eb95-0685af2176b2",
		"abbbbbbb-7bfb-395d-eb95-0685af2176b2",
		"bbbbbbbb-7bfb-395d-eb95-0685af2176b2",
	}
	for i := 0; i < 9; i++ {
		alloc := mock.Alloc()
		alloc.ID = ids[i]
		allocs = append(allocs, alloc)
	}

	for i, alloc := range allocs {
		state.UpsertJobSummary(uint64(900+i), mock.JobSummary(alloc.JobID))
	}

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	iter, err := state.AllocsByIDPrefix(ws, structs.DefaultNamespace, "aaaa")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	gatherAllocs := func(iter memdb.ResultIterator) []*structs.Allocation {
		var allocs []*structs.Allocation
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			allocs = append(allocs, raw.(*structs.Allocation))
		}
		return allocs
	}

	out := gatherAllocs(iter)
	if len(out) != 5 {
		t.Fatalf("bad: expected five allocations, got: %#v", out)
	}

	sort.Sort(AllocIDSort(allocs))

	for index, alloc := range out {
		if ids[index] != alloc.ID {
			t.Fatalf("bad: got unexpected id: %s", alloc.ID)
		}
	}

	iter, err = state.AllocsByIDPrefix(ws, structs.DefaultNamespace, "b-a7bfb")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out = gatherAllocs(iter)
	if len(out) != 0 {
		t.Fatalf("bad: unexpected zero allocations, got: %#v", out)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_Allocs(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	var allocs []*structs.Allocation

	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		allocs = append(allocs, alloc)
	}
	for i, alloc := range allocs {
		state.UpsertJobSummary(uint64(900+i), mock.JobSummary(alloc.JobID))
	}

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	iter, err := state.Allocs(ws)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out []*structs.Allocation
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Allocation))
	}

	sort.Sort(AllocIDSort(allocs))
	sort.Sort(AllocIDSort(out))

	if !reflect.DeepEqual(allocs, out) {
		t.Fatalf("bad: %#v %#v", allocs, out)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_Allocs_PrevAlloc(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	var allocs []*structs.Allocation

	require := require.New(t)
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		allocs = append(allocs, alloc)
	}
	for i, alloc := range allocs {
		state.UpsertJobSummary(uint64(900+i), mock.JobSummary(alloc.JobID))
	}
	// Set some previous alloc ids
	allocs[1].PreviousAllocation = allocs[0].ID
	allocs[2].PreviousAllocation = allocs[1].ID

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs)
	require.Nil(err)

	ws := memdb.NewWatchSet()
	iter, err := state.Allocs(ws)
	require.Nil(err)

	var out []*structs.Allocation
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Allocation))
	}

	// Set expected NextAllocation fields
	allocs[0].NextAllocation = allocs[1].ID
	allocs[1].NextAllocation = allocs[2].ID

	sort.Sort(AllocIDSort(allocs))
	sort.Sort(AllocIDSort(out))

	require.Equal(allocs, out)
	require.False(watchFired(ws))

	// Insert another alloc, verify index of previous alloc also got updated
	alloc := mock.Alloc()
	alloc.PreviousAllocation = allocs[0].ID
	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc})
	require.Nil(err)
	alloc0, err := state.AllocByID(nil, allocs[0].ID)
	require.Nil(err)
	require.Equal(alloc0.ModifyIndex, uint64(1001))
}

func TestStateStore_SetJobStatus_ForceStatus(t *testing.T) {
	t.Parallel()

	index := uint64(0)
	state := testStateStore(t)
	txn := state.db.WriteTxn(index)

	// Create and insert a mock job.
	job := mock.Job()
	job.Status = ""
	job.ModifyIndex = index
	if err := txn.Insert("jobs", job); err != nil {
		t.Fatalf("job insert failed: %v", err)
	}

	exp := "foobar"
	index = uint64(1000)
	if err := state.setJobStatus(index, txn, job, false, exp); err != nil {
		t.Fatalf("setJobStatus() failed: %v", err)
	}

	i, err := txn.First("jobs", "id", job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("job lookup failed: %v", err)
	}
	updated := i.(*structs.Job)

	if updated.Status != exp {
		t.Fatalf("setJobStatus() set %v; expected %v", updated.Status, exp)
	}

	if updated.ModifyIndex != index {
		t.Fatalf("setJobStatus() set %d; expected %d", updated.ModifyIndex, index)
	}
}

func TestStateStore_SetJobStatus_NoOp(t *testing.T) {
	t.Parallel()

	index := uint64(0)
	state := testStateStore(t)
	txn := state.db.WriteTxn(index)

	// Create and insert a mock job that should be pending.
	job := mock.Job()
	job.Status = structs.JobStatusPending
	job.ModifyIndex = 10
	if err := txn.Insert("jobs", job); err != nil {
		t.Fatalf("job insert failed: %v", err)
	}

	index = uint64(1000)
	if err := state.setJobStatus(index, txn, job, false, ""); err != nil {
		t.Fatalf("setJobStatus() failed: %v", err)
	}

	i, err := txn.First("jobs", "id", job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("job lookup failed: %v", err)
	}
	updated := i.(*structs.Job)

	if updated.ModifyIndex == index {
		t.Fatalf("setJobStatus() should have been a no-op")
	}
}

func TestStateStore_SetJobStatus(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	txn := state.db.WriteTxn(uint64(0))

	// Create and insert a mock job that should be pending but has an incorrect
	// status.
	job := mock.Job()
	job.Status = "foobar"
	job.ModifyIndex = 10
	if err := txn.Insert("jobs", job); err != nil {
		t.Fatalf("job insert failed: %v", err)
	}

	index := uint64(1000)
	if err := state.setJobStatus(index, txn, job, false, ""); err != nil {
		t.Fatalf("setJobStatus() failed: %v", err)
	}

	i, err := txn.First("jobs", "id", job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("job lookup failed: %v", err)
	}
	updated := i.(*structs.Job)

	if updated.Status != structs.JobStatusPending {
		t.Fatalf("setJobStatus() set %v; expected %v", updated.Status, structs.JobStatusPending)
	}

	if updated.ModifyIndex != index {
		t.Fatalf("setJobStatus() set %d; expected %d", updated.ModifyIndex, index)
	}
}

func TestStateStore_GetJobStatus_NoEvalsOrAllocs(t *testing.T) {
	t.Parallel()

	job := mock.Job()
	state := testStateStore(t)
	txn := state.db.ReadTxn()
	status, err := state.getJobStatus(txn, job, false)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if status != structs.JobStatusPending {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, structs.JobStatusPending)
	}
}

func TestStateStore_GetJobStatus_NoEvalsOrAllocs_Periodic(t *testing.T) {
	t.Parallel()

	job := mock.PeriodicJob()
	state := testStateStore(t)
	txn := state.db.ReadTxn()
	status, err := state.getJobStatus(txn, job, false)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if status != structs.JobStatusRunning {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, structs.JobStatusRunning)
	}
}

func TestStateStore_GetJobStatus_NoEvalsOrAllocs_EvalDelete(t *testing.T) {
	t.Parallel()

	job := mock.Job()
	state := testStateStore(t)
	txn := state.db.ReadTxn()
	status, err := state.getJobStatus(txn, job, true)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if status != structs.JobStatusDead {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, structs.JobStatusDead)
	}
}

func TestStateStore_GetJobStatus_DeadEvalsAndAllocs(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	job := mock.Job()

	// Create a mock alloc that is dead.
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusStop
	state.UpsertJobSummary(999, mock.JobSummary(alloc.JobID))
	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a mock eval that is complete
	eval := mock.Eval()
	eval.JobID = job.ID
	eval.Status = structs.EvalStatusComplete
	if err := state.UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("err: %v", err)
	}

	txn := state.db.ReadTxn()
	status, err := state.getJobStatus(txn, job, false)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if status != structs.JobStatusDead {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, structs.JobStatusDead)
	}
}

func TestStateStore_GetJobStatus_RunningAlloc(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	job := mock.Job()

	// Create a mock alloc that is running.
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusRun
	state.UpsertJobSummary(999, mock.JobSummary(alloc.JobID))
	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	txn := state.db.ReadTxn()
	status, err := state.getJobStatus(txn, job, true)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if status != structs.JobStatusRunning {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, structs.JobStatusRunning)
	}
}

func TestStateStore_GetJobStatus_PeriodicJob(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	job := mock.PeriodicJob()

	txn := state.db.ReadTxn()
	status, err := state.getJobStatus(txn, job, false)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if status != structs.JobStatusRunning {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, structs.JobStatusRunning)
	}

	// Mark it as stopped
	job.Stop = true
	status, err = state.getJobStatus(txn, job, false)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if status != structs.JobStatusDead {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, structs.JobStatusDead)
	}
}

func TestStateStore_GetJobStatus_ParameterizedJob(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	job := mock.Job()
	job.ParameterizedJob = &structs.ParameterizedJobConfig{}

	txn := state.db.ReadTxn()
	status, err := state.getJobStatus(txn, job, false)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if status != structs.JobStatusRunning {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, structs.JobStatusRunning)
	}

	// Mark it as stopped
	job.Stop = true
	status, err = state.getJobStatus(txn, job, false)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if status != structs.JobStatusDead {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, structs.JobStatusDead)
	}
}

func TestStateStore_SetJobStatus_PendingEval(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	job := mock.Job()

	// Create a mock eval that is pending.
	eval := mock.Eval()
	eval.JobID = job.ID
	eval.Status = structs.EvalStatusPending
	if err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("err: %v", err)
	}

	txn := state.db.ReadTxn()
	status, err := state.getJobStatus(txn, job, true)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if status != structs.JobStatusPending {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, structs.JobStatusPending)
	}
}

// TestStateStore_SetJobStatus_SystemJob asserts that system jobs are still
// considered running until explicitly stopped.
func TestStateStore_SetJobStatus_SystemJob(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	job := mock.SystemJob()

	// Create a mock eval that is pending.
	eval := mock.Eval()
	eval.JobID = job.ID
	eval.Type = job.Type
	eval.Status = structs.EvalStatusComplete
	if err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("err: %v", err)
	}

	txn := state.db.ReadTxn()
	status, err := state.getJobStatus(txn, job, true)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if expected := structs.JobStatusRunning; status != expected {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, expected)
	}

	// Stop the job
	job.Stop = true
	status, err = state.getJobStatus(txn, job, true)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if expected := structs.JobStatusDead; status != expected {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, expected)
	}
}

func TestStateJobSummary_UpdateJobCount(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	alloc := mock.Alloc()
	job := alloc.Job
	job.TaskGroups[0].Count = 3

	// Create watchsets so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.JobSummaryByID(ws, job.Namespace, job.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	if err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	summary, _ := state.JobSummaryByID(ws, job.Namespace, job.ID)
	expectedSummary := structs.JobSummary{
		JobID:     job.ID,
		Namespace: job.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {
				Starting: 1,
			},
		},
		Children:    new(structs.JobChildrenSummary),
		CreateIndex: 1000,
		ModifyIndex: 1001,
	}
	if !reflect.DeepEqual(summary, &expectedSummary) {
		t.Fatalf("expected: %v, actual: %v", expectedSummary, summary)
	}

	// Create watchsets so we can test that upsert fires the watch
	ws2 := memdb.NewWatchSet()
	if _, err := state.JobSummaryByID(ws2, job.Namespace, job.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	alloc2 := mock.Alloc()
	alloc2.Job = job
	alloc2.JobID = job.ID

	alloc3 := mock.Alloc()
	alloc3.Job = job
	alloc3.JobID = job.ID

	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{alloc2, alloc3}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws2) {
		t.Fatalf("bad")
	}

	outA, _ := state.AllocByID(ws, alloc3.ID)

	summary, _ = state.JobSummaryByID(ws, job.Namespace, job.ID)
	expectedSummary = structs.JobSummary{
		JobID:     job.ID,
		Namespace: job.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {
				Starting: 3,
			},
		},
		Children:    new(structs.JobChildrenSummary),
		CreateIndex: job.CreateIndex,
		ModifyIndex: outA.ModifyIndex,
	}
	if !reflect.DeepEqual(summary, &expectedSummary) {
		t.Fatalf("expected summary: %v, actual: %v", expectedSummary, summary)
	}

	// Create watchsets so we can test that upsert fires the watch
	ws3 := memdb.NewWatchSet()
	if _, err := state.JobSummaryByID(ws3, job.Namespace, job.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	alloc4 := mock.Alloc()
	alloc4.ID = alloc2.ID
	alloc4.Job = alloc2.Job
	alloc4.JobID = alloc2.JobID
	alloc4.ClientStatus = structs.AllocClientStatusComplete

	alloc5 := mock.Alloc()
	alloc5.ID = alloc3.ID
	alloc5.Job = alloc3.Job
	alloc5.JobID = alloc3.JobID
	alloc5.ClientStatus = structs.AllocClientStatusComplete

	if err := state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1004, []*structs.Allocation{alloc4, alloc5}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws2) {
		t.Fatalf("bad")
	}

	outA, _ = state.AllocByID(ws, alloc5.ID)
	summary, _ = state.JobSummaryByID(ws, job.Namespace, job.ID)
	expectedSummary = structs.JobSummary{
		JobID:     job.ID,
		Namespace: job.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {
				Complete: 2,
				Starting: 1,
			},
		},
		Children:    new(structs.JobChildrenSummary),
		CreateIndex: job.CreateIndex,
		ModifyIndex: outA.ModifyIndex,
	}
	if !reflect.DeepEqual(summary, &expectedSummary) {
		t.Fatalf("expected: %v, actual: %v", expectedSummary, summary)
	}
}

func TestJobSummary_UpdateClientStatus(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	alloc := mock.Alloc()
	job := alloc.Job
	job.TaskGroups[0].Count = 3

	alloc2 := mock.Alloc()
	alloc2.Job = job
	alloc2.JobID = job.ID

	alloc3 := mock.Alloc()
	alloc3.Job = job
	alloc3.JobID = job.ID

	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc, alloc2, alloc3}); err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	summary, _ := state.JobSummaryByID(ws, job.Namespace, job.ID)
	if summary.Summary["web"].Starting != 3 {
		t.Fatalf("bad job summary: %v", summary)
	}

	alloc4 := mock.Alloc()
	alloc4.ID = alloc2.ID
	alloc4.Job = alloc2.Job
	alloc4.JobID = alloc2.JobID
	alloc4.ClientStatus = structs.AllocClientStatusComplete

	alloc5 := mock.Alloc()
	alloc5.ID = alloc3.ID
	alloc5.Job = alloc3.Job
	alloc5.JobID = alloc3.JobID
	alloc5.ClientStatus = structs.AllocClientStatusFailed

	alloc6 := mock.Alloc()
	alloc6.ID = alloc.ID
	alloc6.Job = alloc.Job
	alloc6.JobID = alloc.JobID
	alloc6.ClientStatus = structs.AllocClientStatusRunning

	if err := state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{alloc4, alloc5, alloc6}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	summary, _ = state.JobSummaryByID(ws, job.Namespace, job.ID)
	if summary.Summary["web"].Running != 1 || summary.Summary["web"].Failed != 1 || summary.Summary["web"].Complete != 1 {
		t.Fatalf("bad job summary: %v", summary)
	}

	alloc7 := mock.Alloc()
	alloc7.Job = alloc.Job
	alloc7.JobID = alloc.JobID

	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1003, []*structs.Allocation{alloc7}); err != nil {
		t.Fatalf("err: %v", err)
	}
	summary, _ = state.JobSummaryByID(ws, job.Namespace, job.ID)
	if summary.Summary["web"].Starting != 1 || summary.Summary["web"].Running != 1 || summary.Summary["web"].Failed != 1 || summary.Summary["web"].Complete != 1 {
		t.Fatalf("bad job summary: %v", summary)
	}
}

// Test that nonexistent deployment can't be updated
func TestStateStore_UpsertDeploymentStatusUpdate_Nonexistent(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Update the nonexistent deployment
	req := &structs.DeploymentStatusUpdateRequest{
		DeploymentUpdate: &structs.DeploymentStatusUpdate{
			DeploymentID: uuid.Generate(),
			Status:       structs.DeploymentStatusRunning,
		},
	}
	err := state.UpdateDeploymentStatus(structs.MsgTypeTestSetup, 2, req)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected error updating the status because the deployment doesn't exist")
	}
}

// Test that terminal deployment can't be updated
func TestStateStore_UpsertDeploymentStatusUpdate_Terminal(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Insert a terminal deployment
	d := mock.Deployment()
	d.Status = structs.DeploymentStatusFailed

	if err := state.UpsertDeployment(1, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Update the deployment
	req := &structs.DeploymentStatusUpdateRequest{
		DeploymentUpdate: &structs.DeploymentStatusUpdate{
			DeploymentID: d.ID,
			Status:       structs.DeploymentStatusRunning,
		},
	}
	err := state.UpdateDeploymentStatus(structs.MsgTypeTestSetup, 2, req)
	if err == nil || !strings.Contains(err.Error(), "has terminal status") {
		t.Fatalf("expected error updating the status because the deployment is terminal")
	}
}

// Test that a non terminal deployment is updated and that a job and eval are
// created.
func TestStateStore_UpsertDeploymentStatusUpdate_NonTerminal(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Insert a deployment
	d := mock.Deployment()
	if err := state.UpsertDeployment(1, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create an eval and a job
	e := mock.Eval()
	j := mock.Job()

	// Update the deployment
	status, desc := structs.DeploymentStatusFailed, "foo"
	req := &structs.DeploymentStatusUpdateRequest{
		DeploymentUpdate: &structs.DeploymentStatusUpdate{
			DeploymentID:      d.ID,
			Status:            status,
			StatusDescription: desc,
		},
		Job:  j,
		Eval: e,
	}
	err := state.UpdateDeploymentStatus(structs.MsgTypeTestSetup, 2, req)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check that the status was updated properly
	ws := memdb.NewWatchSet()
	dout, err := state.DeploymentByID(ws, d.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if dout.Status != status || dout.StatusDescription != desc {
		t.Fatalf("bad: %#v", dout)
	}

	// Check that the evaluation was created
	eout, _ := state.EvalByID(ws, e.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if eout == nil {
		t.Fatalf("bad: %#v", eout)
	}

	// Check that the job was created
	jout, _ := state.JobByID(ws, j.Namespace, j.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if jout == nil {
		t.Fatalf("bad: %#v", jout)
	}
}

// Test that when a deployment is updated to successful the job is updated to
// stable
func TestStateStore_UpsertDeploymentStatusUpdate_Successful(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Insert a job
	job := mock.Job()
	if err := state.UpsertJob(structs.MsgTypeTestSetup, 1, job); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Insert a deployment
	d := structs.NewDeployment(job, 50)
	if err := state.UpsertDeployment(2, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Update the deployment
	req := &structs.DeploymentStatusUpdateRequest{
		DeploymentUpdate: &structs.DeploymentStatusUpdate{
			DeploymentID:      d.ID,
			Status:            structs.DeploymentStatusSuccessful,
			StatusDescription: structs.DeploymentStatusDescriptionSuccessful,
		},
	}
	err := state.UpdateDeploymentStatus(structs.MsgTypeTestSetup, 3, req)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check that the status was updated properly
	ws := memdb.NewWatchSet()
	dout, err := state.DeploymentByID(ws, d.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if dout.Status != structs.DeploymentStatusSuccessful ||
		dout.StatusDescription != structs.DeploymentStatusDescriptionSuccessful {
		t.Fatalf("bad: %#v", dout)
	}

	// Check that the job was created
	jout, _ := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if jout == nil {
		t.Fatalf("bad: %#v", jout)
	}
	if !jout.Stable {
		t.Fatalf("job not marked stable %#v", jout)
	}
	if jout.Version != d.JobVersion {
		t.Fatalf("job version changed; got %d; want %d", jout.Version, d.JobVersion)
	}
}

func TestStateStore_UpdateJobStability(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Insert a job twice to get two versions
	job := mock.Job()
	require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1, job))

	require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 2, job.Copy()))

	// Update the stability to true
	err := state.UpdateJobStability(3, job.Namespace, job.ID, 0, true)
	require.NoError(t, err)

	// Check that the job was updated properly
	ws := memdb.NewWatchSet()
	jout, err := state.JobByIDAndVersion(ws, job.Namespace, job.ID, 0)
	require.NoError(t, err)
	require.NotNil(t, jout)
	require.True(t, jout.Stable, "job not marked as stable")

	// Update the stability to false
	err = state.UpdateJobStability(3, job.Namespace, job.ID, 0, false)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check that the job was updated properly
	jout, err = state.JobByIDAndVersion(ws, job.Namespace, job.ID, 0)
	require.NoError(t, err)
	require.NotNil(t, jout)
	require.False(t, jout.Stable)
}

// Test that nonexistent deployment can't be promoted
func TestStateStore_UpsertDeploymentPromotion_Nonexistent(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Promote the nonexistent deployment
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: uuid.Generate(),
			All:          true,
		},
	}
	err := state.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, 2, req)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected error promoting because the deployment doesn't exist")
	}
}

// Test that terminal deployment can't be updated
func TestStateStore_UpsertDeploymentPromotion_Terminal(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Insert a terminal deployment
	d := mock.Deployment()
	d.Status = structs.DeploymentStatusFailed

	if err := state.UpsertDeployment(1, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Promote the deployment
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
	}
	err := state.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, 2, req)
	if err == nil || !strings.Contains(err.Error(), "has terminal status") {
		t.Fatalf("expected error updating the status because the deployment is terminal: %v", err)
	}
}

// Test promoting unhealthy canaries in a deployment.
func TestStateStore_UpsertDeploymentPromotion_Unhealthy(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	require := require.New(t)

	// Create a job
	j := mock.Job()
	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 1, j))

	// Create a deployment
	d := mock.Deployment()
	d.JobID = j.ID
	d.TaskGroups["web"].DesiredCanaries = 2
	require.Nil(state.UpsertDeployment(2, d))

	// Create a set of allocations
	c1 := mock.Alloc()
	c1.JobID = j.ID
	c1.DeploymentID = d.ID
	d.TaskGroups[c1.TaskGroup].PlacedCanaries = append(d.TaskGroups[c1.TaskGroup].PlacedCanaries, c1.ID)
	c2 := mock.Alloc()
	c2.JobID = j.ID
	c2.DeploymentID = d.ID
	d.TaskGroups[c2.TaskGroup].PlacedCanaries = append(d.TaskGroups[c2.TaskGroup].PlacedCanaries, c2.ID)

	// Create a healthy but terminal alloc
	c3 := mock.Alloc()
	c3.JobID = j.ID
	c3.DeploymentID = d.ID
	c3.DesiredStatus = structs.AllocDesiredStatusStop
	c3.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: helper.BoolToPtr(true)}
	d.TaskGroups[c3.TaskGroup].PlacedCanaries = append(d.TaskGroups[c3.TaskGroup].PlacedCanaries, c3.ID)

	require.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 3, []*structs.Allocation{c1, c2, c3}))

	// Promote the canaries
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
	}
	err := state.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, 4, req)
	require.NotNil(err)
	require.Contains(err.Error(), `Task group "web" has 0/2 healthy allocations`)
}

// Test promoting a deployment with no canaries
func TestStateStore_UpsertDeploymentPromotion_NoCanaries(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	require := require.New(t)

	// Create a job
	j := mock.Job()
	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 1, j))

	// Create a deployment
	d := mock.Deployment()
	d.TaskGroups["web"].DesiredCanaries = 2
	d.JobID = j.ID
	require.Nil(state.UpsertDeployment(2, d))

	// Promote the canaries
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
	}
	err := state.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, 4, req)
	require.NotNil(err)
	require.Contains(err.Error(), `Task group "web" has 0/2 healthy allocations`)
}

// Test promoting all canaries in a deployment.
func TestStateStore_UpsertDeploymentPromotion_All(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Create a job with two task groups
	j := mock.Job()
	tg1 := j.TaskGroups[0]
	tg2 := tg1.Copy()
	tg2.Name = "foo"
	j.TaskGroups = append(j.TaskGroups, tg2)
	if err := state.UpsertJob(structs.MsgTypeTestSetup, 1, j); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create a deployment
	d := mock.Deployment()
	d.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
	d.JobID = j.ID
	d.TaskGroups = map[string]*structs.DeploymentState{
		"web": {
			DesiredTotal:    10,
			DesiredCanaries: 1,
		},
		"foo": {
			DesiredTotal:    10,
			DesiredCanaries: 1,
		},
	}
	if err := state.UpsertDeployment(2, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create a set of allocations
	c1 := mock.Alloc()
	c1.JobID = j.ID
	c1.DeploymentID = d.ID
	d.TaskGroups[c1.TaskGroup].PlacedCanaries = append(d.TaskGroups[c1.TaskGroup].PlacedCanaries, c1.ID)
	c1.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
	}
	c2 := mock.Alloc()
	c2.JobID = j.ID
	c2.DeploymentID = d.ID
	d.TaskGroups[c2.TaskGroup].PlacedCanaries = append(d.TaskGroups[c2.TaskGroup].PlacedCanaries, c2.ID)
	c2.TaskGroup = tg2.Name
	c2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
	}

	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 3, []*structs.Allocation{c1, c2}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create an eval
	e := mock.Eval()

	// Promote the canaries
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
		Eval: e,
	}
	err := state.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, 4, req)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check that the status per task group was updated properly
	ws := memdb.NewWatchSet()
	dout, err := state.DeploymentByID(ws, d.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if dout.StatusDescription != structs.DeploymentStatusDescriptionRunning {
		t.Fatalf("status description not updated: got %v; want %v", dout.StatusDescription, structs.DeploymentStatusDescriptionRunning)
	}
	if len(dout.TaskGroups) != 2 {
		t.Fatalf("bad: %#v", dout.TaskGroups)
	}
	for tg, state := range dout.TaskGroups {
		if !state.Promoted {
			t.Fatalf("bad: group %q not promoted %#v", tg, state)
		}
	}

	// Check that the evaluation was created
	eout, _ := state.EvalByID(ws, e.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if eout == nil {
		t.Fatalf("bad: %#v", eout)
	}
}

// Test promoting a subset of canaries in a deployment.
func TestStateStore_UpsertDeploymentPromotion_Subset(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)

	// Create a job with two task groups
	j := mock.Job()
	tg1 := j.TaskGroups[0]
	tg2 := tg1.Copy()
	tg2.Name = "foo"
	j.TaskGroups = append(j.TaskGroups, tg2)
	require.Nil(state.UpsertJob(structs.MsgTypeTestSetup, 1, j))

	// Create a deployment
	d := mock.Deployment()
	d.JobID = j.ID
	d.TaskGroups = map[string]*structs.DeploymentState{
		"web": {
			DesiredTotal:    10,
			DesiredCanaries: 1,
		},
		"foo": {
			DesiredTotal:    10,
			DesiredCanaries: 1,
		},
	}
	require.Nil(state.UpsertDeployment(2, d))

	// Create a set of allocations for both groups, including an unhealthy one
	c1 := mock.Alloc()
	c1.JobID = j.ID
	c1.DeploymentID = d.ID
	d.TaskGroups[c1.TaskGroup].PlacedCanaries = append(d.TaskGroups[c1.TaskGroup].PlacedCanaries, c1.ID)
	c1.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
		Canary:  true,
	}

	// Should still be a canary
	c2 := mock.Alloc()
	c2.JobID = j.ID
	c2.DeploymentID = d.ID
	d.TaskGroups[c2.TaskGroup].PlacedCanaries = append(d.TaskGroups[c2.TaskGroup].PlacedCanaries, c2.ID)
	c2.TaskGroup = tg2.Name
	c2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
		Canary:  true,
	}

	c3 := mock.Alloc()
	c3.JobID = j.ID
	c3.DeploymentID = d.ID
	d.TaskGroups[c3.TaskGroup].PlacedCanaries = append(d.TaskGroups[c3.TaskGroup].PlacedCanaries, c3.ID)
	c3.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(false),
		Canary:  true,
	}

	require.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 3, []*structs.Allocation{c1, c2, c3}))

	// Create an eval
	e := mock.Eval()

	// Promote the canaries
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			Groups:       []string{"web"},
		},
		Eval: e,
	}
	require.Nil(state.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, 4, req))

	// Check that the status per task group was updated properly
	ws := memdb.NewWatchSet()
	dout, err := state.DeploymentByID(ws, d.ID)
	require.Nil(err)
	require.Len(dout.TaskGroups, 2)
	require.Contains(dout.TaskGroups, "web")
	require.True(dout.TaskGroups["web"].Promoted)

	// Check that the evaluation was created
	eout, err := state.EvalByID(ws, e.ID)
	require.Nil(err)
	require.NotNil(eout)

	// Check the canary field was set properly
	aout1, err1 := state.AllocByID(ws, c1.ID)
	aout2, err2 := state.AllocByID(ws, c2.ID)
	aout3, err3 := state.AllocByID(ws, c3.ID)
	require.Nil(err1)
	require.Nil(err2)
	require.Nil(err3)
	require.NotNil(aout1)
	require.NotNil(aout2)
	require.NotNil(aout3)
	require.False(aout1.DeploymentStatus.Canary)
	require.True(aout2.DeploymentStatus.Canary)
	require.True(aout3.DeploymentStatus.Canary)
}

// Test that allocation health can't be set against a nonexistent deployment
func TestStateStore_UpsertDeploymentAllocHealth_Nonexistent(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Set health against the nonexistent deployment
	req := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:         uuid.Generate(),
			HealthyAllocationIDs: []string{uuid.Generate()},
		},
	}
	err := state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, 2, req)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected error because the deployment doesn't exist: %v", err)
	}
}

// Test that allocation health can't be set against a terminal deployment
func TestStateStore_UpsertDeploymentAllocHealth_Terminal(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Insert a terminal deployment
	d := mock.Deployment()
	d.Status = structs.DeploymentStatusFailed

	if err := state.UpsertDeployment(1, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Set health against the terminal deployment
	req := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:         d.ID,
			HealthyAllocationIDs: []string{uuid.Generate()},
		},
	}
	err := state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, 2, req)
	if err == nil || !strings.Contains(err.Error(), "has terminal status") {
		t.Fatalf("expected error because the deployment is terminal: %v", err)
	}
}

// Test that allocation health can't be set against a nonexistent alloc
func TestStateStore_UpsertDeploymentAllocHealth_BadAlloc_Nonexistent(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Insert a deployment
	d := mock.Deployment()
	if err := state.UpsertDeployment(1, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Set health against the terminal deployment
	req := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:         d.ID,
			HealthyAllocationIDs: []string{uuid.Generate()},
		},
	}
	err := state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, 2, req)
	if err == nil || !strings.Contains(err.Error(), "unknown alloc") {
		t.Fatalf("expected error because the alloc doesn't exist: %v", err)
	}
}

// Test that a deployments PlacedCanaries is properly updated
func TestStateStore_UpsertDeploymentAlloc_Canaries(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Create a deployment
	d1 := mock.Deployment()
	require.NoError(t, state.UpsertDeployment(2, d1))

	// Create a Job
	job := mock.Job()
	require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 3, job))

	// Create alloc with canary status
	a := mock.Alloc()
	a.JobID = job.ID
	a.DeploymentID = d1.ID
	a.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(false),
		Canary:  true,
	}
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 4, []*structs.Allocation{a}))

	// Pull the deployment from state
	ws := memdb.NewWatchSet()
	deploy, err := state.DeploymentByID(ws, d1.ID)
	require.NoError(t, err)

	// Ensure that PlacedCanaries is accurate
	require.Equal(t, 1, len(deploy.TaskGroups[job.TaskGroups[0].Name].PlacedCanaries))

	// Create alloc without canary status
	b := mock.Alloc()
	b.JobID = job.ID
	b.DeploymentID = d1.ID
	b.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(false),
		Canary:  false,
	}
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 4, []*structs.Allocation{b}))

	// Pull the deployment from state
	ws = memdb.NewWatchSet()
	deploy, err = state.DeploymentByID(ws, d1.ID)
	require.NoError(t, err)

	// Ensure that PlacedCanaries is accurate
	require.Equal(t, 1, len(deploy.TaskGroups[job.TaskGroups[0].Name].PlacedCanaries))

	// Create a second deployment
	d2 := mock.Deployment()
	require.NoError(t, state.UpsertDeployment(5, d2))

	c := mock.Alloc()
	c.JobID = job.ID
	c.DeploymentID = d2.ID
	c.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(false),
		Canary:  true,
	}
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 6, []*structs.Allocation{c}))

	ws = memdb.NewWatchSet()
	deploy2, err := state.DeploymentByID(ws, d2.ID)
	require.NoError(t, err)

	// Ensure that PlacedCanaries is accurate
	require.Equal(t, 1, len(deploy2.TaskGroups[job.TaskGroups[0].Name].PlacedCanaries))
}

func TestStateStore_UpsertDeploymentAlloc_NoCanaries(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Create a deployment
	d1 := mock.Deployment()
	require.NoError(t, state.UpsertDeployment(2, d1))

	// Create a Job
	job := mock.Job()
	require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 3, job))

	// Create alloc with canary status
	a := mock.Alloc()
	a.JobID = job.ID
	a.DeploymentID = d1.ID
	a.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
		Canary:  false,
	}
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 4, []*structs.Allocation{a}))

	// Pull the deployment from state
	ws := memdb.NewWatchSet()
	deploy, err := state.DeploymentByID(ws, d1.ID)
	require.NoError(t, err)

	// Ensure that PlacedCanaries is accurate
	require.Equal(t, 0, len(deploy.TaskGroups[job.TaskGroups[0].Name].PlacedCanaries))
}

// Test that allocation health can't be set for an alloc with mismatched
// deployment ids
func TestStateStore_UpsertDeploymentAllocHealth_BadAlloc_MismatchDeployment(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Insert two  deployment
	d1 := mock.Deployment()
	d2 := mock.Deployment()
	if err := state.UpsertDeployment(1, d1); err != nil {
		t.Fatalf("bad: %v", err)
	}
	if err := state.UpsertDeployment(2, d2); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Insert an alloc for a random deployment
	a := mock.Alloc()
	a.DeploymentID = d1.ID
	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 3, []*structs.Allocation{a}); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Set health against the terminal deployment
	req := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:         d2.ID,
			HealthyAllocationIDs: []string{a.ID},
		},
	}
	err := state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, 4, req)
	if err == nil || !strings.Contains(err.Error(), "not part of deployment") {
		t.Fatalf("expected error because the alloc isn't part of the deployment: %v", err)
	}
}

// Test that allocation health is properly set
func TestStateStore_UpsertDeploymentAllocHealth(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)

	// Insert a deployment
	d := mock.Deployment()
	d.TaskGroups["web"].ProgressDeadline = 5 * time.Minute
	if err := state.UpsertDeployment(1, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Insert two allocations
	a1 := mock.Alloc()
	a1.DeploymentID = d.ID
	a2 := mock.Alloc()
	a2.DeploymentID = d.ID
	if err := state.UpsertAllocs(structs.MsgTypeTestSetup, 2, []*structs.Allocation{a1, a2}); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create a job to roll back to
	j := mock.Job()

	// Create an eval that should be upserted
	e := mock.Eval()

	// Create a status update for the deployment
	status, desc := structs.DeploymentStatusFailed, "foo"
	u := &structs.DeploymentStatusUpdate{
		DeploymentID:      d.ID,
		Status:            status,
		StatusDescription: desc,
	}

	// Capture the time for the update
	ts := time.Now()

	// Set health against the deployment
	req := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:           d.ID,
			HealthyAllocationIDs:   []string{a1.ID},
			UnhealthyAllocationIDs: []string{a2.ID},
		},
		Job:              j,
		Eval:             e,
		DeploymentUpdate: u,
		Timestamp:        ts,
	}
	err := state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, 3, req)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check that the status was updated properly
	ws := memdb.NewWatchSet()
	dout, err := state.DeploymentByID(ws, d.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if dout.Status != status || dout.StatusDescription != desc {
		t.Fatalf("bad: %#v", dout)
	}

	// Check that the evaluation was created
	eout, _ := state.EvalByID(ws, e.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if eout == nil {
		t.Fatalf("bad: %#v", eout)
	}

	// Check that the job was created
	jout, _ := state.JobByID(ws, j.Namespace, j.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if jout == nil {
		t.Fatalf("bad: %#v", jout)
	}

	// Check the status of the allocs
	out1, err := state.AllocByID(ws, a1.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	out2, err := state.AllocByID(ws, a2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !out1.DeploymentStatus.IsHealthy() {
		t.Fatalf("bad: alloc %q not healthy", out1.ID)
	}
	if !out2.DeploymentStatus.IsUnhealthy() {
		t.Fatalf("bad: alloc %q not unhealthy", out2.ID)
	}

	if !out1.DeploymentStatus.Timestamp.Equal(ts) {
		t.Fatalf("bad: alloc %q had timestamp %v; want %v", out1.ID, out1.DeploymentStatus.Timestamp, ts)
	}
	if !out2.DeploymentStatus.Timestamp.Equal(ts) {
		t.Fatalf("bad: alloc %q had timestamp %v; want %v", out2.ID, out2.DeploymentStatus.Timestamp, ts)
	}
}

func TestStateStore_UpsertVaultAccessors(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	a := mock.VaultAccessor()
	a2 := mock.VaultAccessor()

	ws := memdb.NewWatchSet()
	if _, err := state.VaultAccessor(ws, a.Accessor); err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, err := state.VaultAccessor(ws, a2.Accessor); err != nil {
		t.Fatalf("err: %v", err)
	}

	err := state.UpsertVaultAccessor(1000, []*structs.VaultAccessor{a, a2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.VaultAccessor(ws, a.Accessor)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(a, out) {
		t.Fatalf("bad: %#v %#v", a, out)
	}

	out, err = state.VaultAccessor(ws, a2.Accessor)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(a2, out) {
		t.Fatalf("bad: %#v %#v", a2, out)
	}

	iter, err := state.VaultAccessors(ws)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	count := 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		count++
		accessor := raw.(*structs.VaultAccessor)

		if !reflect.DeepEqual(accessor, a) && !reflect.DeepEqual(accessor, a2) {
			t.Fatalf("bad: %#v", accessor)
		}
	}

	if count != 2 {
		t.Fatalf("bad: %d", count)
	}

	index, err := state.Index("vault_accessors")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_DeleteVaultAccessors(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	a1 := mock.VaultAccessor()
	a2 := mock.VaultAccessor()
	accessors := []*structs.VaultAccessor{a1, a2}

	err := state.UpsertVaultAccessor(1000, accessors)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	if _, err := state.VaultAccessor(ws, a1.Accessor); err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.DeleteVaultAccessors(1001, accessors)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.VaultAccessor(ws, a1.Accessor)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %#v %#v", a1, out)
	}
	out, err = state.VaultAccessor(ws, a2.Accessor)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != nil {
		t.Fatalf("bad: %#v %#v", a2, out)
	}

	index, err := state.Index("vault_accessors")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_VaultAccessorsByAlloc(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	alloc := mock.Alloc()
	var accessors []*structs.VaultAccessor
	var expected []*structs.VaultAccessor

	for i := 0; i < 5; i++ {
		accessor := mock.VaultAccessor()
		accessor.AllocID = alloc.ID
		expected = append(expected, accessor)
		accessors = append(accessors, accessor)
	}

	for i := 0; i < 10; i++ {
		accessor := mock.VaultAccessor()
		accessors = append(accessors, accessor)
	}

	err := state.UpsertVaultAccessor(1000, accessors)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	out, err := state.VaultAccessorsByAlloc(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(expected) != len(out) {
		t.Fatalf("bad: %#v %#v", len(expected), len(out))
	}

	index, err := state.Index("vault_accessors")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_VaultAccessorsByNode(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	node := mock.Node()
	var accessors []*structs.VaultAccessor
	var expected []*structs.VaultAccessor

	for i := 0; i < 5; i++ {
		accessor := mock.VaultAccessor()
		accessor.NodeID = node.ID
		expected = append(expected, accessor)
		accessors = append(accessors, accessor)
	}

	for i := 0; i < 10; i++ {
		accessor := mock.VaultAccessor()
		accessors = append(accessors, accessor)
	}

	err := state.UpsertVaultAccessor(1000, accessors)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	out, err := state.VaultAccessorsByNode(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(expected) != len(out) {
		t.Fatalf("bad: %#v %#v", len(expected), len(out))
	}

	index, err := state.Index("vault_accessors")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpsertSITokenAccessors(t *testing.T) {
	t.Parallel()
	r := require.New(t)

	state := testStateStore(t)
	a1 := mock.SITokenAccessor()
	a2 := mock.SITokenAccessor()

	ws := memdb.NewWatchSet()
	var err error

	_, err = state.SITokenAccessor(ws, a1.AccessorID)
	r.NoError(err)

	_, err = state.SITokenAccessor(ws, a2.AccessorID)
	r.NoError(err)

	err = state.UpsertSITokenAccessors(1000, []*structs.SITokenAccessor{a1, a2})
	r.NoError(err)

	wsFired := watchFired(ws)
	r.True(wsFired)

	noInsertWS := memdb.NewWatchSet()
	result1, err := state.SITokenAccessor(noInsertWS, a1.AccessorID)
	r.NoError(err)
	r.Equal(a1, result1)

	result2, err := state.SITokenAccessor(noInsertWS, a2.AccessorID)
	r.NoError(err)
	r.Equal(a2, result2)

	iter, err := state.SITokenAccessors(noInsertWS)
	r.NoError(err)

	count := 0
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		count++
		accessor := raw.(*structs.SITokenAccessor)
		// iterator is sorted by dynamic UUID
		matches := reflect.DeepEqual(a1, accessor) || reflect.DeepEqual(a2, accessor)
		r.True(matches)
	}
	r.Equal(2, count)

	index, err := state.Index(siTokenAccessorTable)
	r.NoError(err)
	r.Equal(uint64(1000), index)

	noInsertWSFired := watchFired(noInsertWS)
	r.False(noInsertWSFired)
}

func TestStateStore_DeleteSITokenAccessors(t *testing.T) {
	t.Parallel()
	r := require.New(t)

	state := testStateStore(t)
	a1 := mock.SITokenAccessor()
	a2 := mock.SITokenAccessor()
	accessors := []*structs.SITokenAccessor{a1, a2}
	var err error

	err = state.UpsertSITokenAccessors(1000, accessors)
	r.NoError(err)

	ws := memdb.NewWatchSet()
	_, err = state.SITokenAccessor(ws, a1.AccessorID)
	r.NoError(err)

	err = state.DeleteSITokenAccessors(1001, accessors)
	r.NoError(err)

	wsFired := watchFired(ws)
	r.True(wsFired)

	wsPostDelete := memdb.NewWatchSet()

	result1, err := state.SITokenAccessor(wsPostDelete, a1.AccessorID)
	r.NoError(err)
	r.Nil(result1) // was deleted

	result2, err := state.SITokenAccessor(wsPostDelete, a2.AccessorID)
	r.NoError(err)
	r.Nil(result2) // was deleted

	index, err := state.Index(siTokenAccessorTable)
	r.NoError(err)
	r.Equal(uint64(1001), index)

	wsPostDeleteFired := watchFired(wsPostDelete)
	r.False(wsPostDeleteFired)
}

func TestStateStore_SITokenAccessorsByAlloc(t *testing.T) {
	t.Parallel()
	r := require.New(t)

	state := testStateStore(t)
	alloc := mock.Alloc()
	var accessors []*structs.SITokenAccessor
	var expected []*structs.SITokenAccessor

	for i := 0; i < 5; i++ {
		accessor := mock.SITokenAccessor()
		accessor.AllocID = alloc.ID
		expected = append(expected, accessor)
		accessors = append(accessors, accessor)
	}

	for i := 0; i < 10; i++ {
		accessor := mock.SITokenAccessor()
		accessor.AllocID = uuid.Generate() // does not belong to alloc
		accessors = append(accessors, accessor)
	}

	err := state.UpsertSITokenAccessors(1000, accessors)
	r.NoError(err)

	ws := memdb.NewWatchSet()
	result, err := state.SITokenAccessorsByAlloc(ws, alloc.ID)
	r.NoError(err)
	r.ElementsMatch(expected, result)

	index, err := state.Index(siTokenAccessorTable)
	r.NoError(err)
	r.Equal(uint64(1000), index)

	wsFired := watchFired(ws)
	r.False(wsFired)
}

func TestStateStore_SITokenAccessorsByNode(t *testing.T) {
	t.Parallel()
	r := require.New(t)

	state := testStateStore(t)
	node := mock.Node()
	var accessors []*structs.SITokenAccessor
	var expected []*structs.SITokenAccessor
	var err error

	for i := 0; i < 5; i++ {
		accessor := mock.SITokenAccessor()
		accessor.NodeID = node.ID
		expected = append(expected, accessor)
		accessors = append(accessors, accessor)
	}

	for i := 0; i < 10; i++ {
		accessor := mock.SITokenAccessor()
		accessor.NodeID = uuid.Generate() // does not belong to node
		accessors = append(accessors, accessor)
	}

	err = state.UpsertSITokenAccessors(1000, accessors)
	r.NoError(err)

	ws := memdb.NewWatchSet()
	result, err := state.SITokenAccessorsByNode(ws, node.ID)
	r.NoError(err)
	r.ElementsMatch(expected, result)

	index, err := state.Index(siTokenAccessorTable)
	r.NoError(err)
	r.Equal(uint64(1000), index)

	wsFired := watchFired(ws)
	r.False(wsFired)
}

func TestStateStore_UpsertACLPolicy(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	policy := mock.ACLPolicy()
	policy2 := mock.ACLPolicy()

	ws := memdb.NewWatchSet()
	if _, err := state.ACLPolicyByName(ws, policy.Name); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := state.ACLPolicyByName(ws, policy2.Name); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := state.UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{policy, policy2}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.ACLPolicyByName(ws, policy.Name)
	assert.Equal(t, nil, err)
	assert.Equal(t, policy, out)

	out, err = state.ACLPolicyByName(ws, policy2.Name)
	assert.Equal(t, nil, err)
	assert.Equal(t, policy2, out)

	iter, err := state.ACLPolicies(ws)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we see both policies
	count := 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
	}
	if count != 2 {
		t.Fatalf("bad: %d", count)
	}

	index, err := state.Index("acl_policy")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_DeleteACLPolicy(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	policy := mock.ACLPolicy()
	policy2 := mock.ACLPolicy()

	// Create the policy
	if err := state.UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{policy, policy2}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watcher
	ws := memdb.NewWatchSet()
	if _, err := state.ACLPolicyByName(ws, policy.Name); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Delete the policy
	if err := state.DeleteACLPolicies(structs.MsgTypeTestSetup, 1001, []string{policy.Name, policy2.Name}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure watching triggered
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Ensure we don't get the object back
	ws = memdb.NewWatchSet()
	out, err := state.ACLPolicyByName(ws, policy.Name)
	assert.Equal(t, nil, err)
	if out != nil {
		t.Fatalf("bad: %#v", out)
	}

	iter, err := state.ACLPolicies(ws)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we see neither policy
	count := 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
	}
	if count != 0 {
		t.Fatalf("bad: %d", count)
	}

	index, err := state.Index("acl_policy")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_ACLPolicyByNamePrefix(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	names := []string{
		"foo",
		"bar",
		"foobar",
		"foozip",
		"zip",
	}

	// Create the policies
	var baseIndex uint64 = 1000
	for _, name := range names {
		p := mock.ACLPolicy()
		p.Name = name
		if err := state.UpsertACLPolicies(structs.MsgTypeTestSetup, baseIndex, []*structs.ACLPolicy{p}); err != nil {
			t.Fatalf("err: %v", err)
		}
		baseIndex++
	}

	// Scan by prefix
	iter, err := state.ACLPolicyByNamePrefix(nil, "foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we see both policies
	count := 0
	out := []string{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
		out = append(out, raw.(*structs.ACLPolicy).Name)
	}
	if count != 3 {
		t.Fatalf("bad: %d %v", count, out)
	}
	sort.Strings(out)

	expect := []string{"foo", "foobar", "foozip"}
	assert.Equal(t, expect, out)
}

func TestStateStore_BootstrapACLTokens(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	tk1 := mock.ACLToken()
	tk2 := mock.ACLToken()

	ok, resetIdx, err := state.CanBootstrapACLToken()
	assert.Nil(t, err)
	assert.Equal(t, true, ok)
	assert.EqualValues(t, 0, resetIdx)

	if err := state.BootstrapACLTokens(structs.MsgTypeTestSetup, 1000, 0, tk1); err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.ACLTokenByAccessorID(nil, tk1.AccessorID)
	assert.Equal(t, nil, err)
	assert.Equal(t, tk1, out)

	ok, resetIdx, err = state.CanBootstrapACLToken()
	assert.Nil(t, err)
	assert.Equal(t, false, ok)
	assert.EqualValues(t, 1000, resetIdx)

	if err := state.BootstrapACLTokens(structs.MsgTypeTestSetup, 1001, 0, tk2); err == nil {
		t.Fatalf("expected error")
	}

	iter, err := state.ACLTokens(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we see both policies
	count := 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
	}
	if count != 1 {
		t.Fatalf("bad: %d", count)
	}

	index, err := state.Index("acl_token")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}
	index, err = state.Index("acl_token_bootstrap")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	// Should allow bootstrap with reset index
	if err := state.BootstrapACLTokens(structs.MsgTypeTestSetup, 1001, 1000, tk2); err != nil {
		t.Fatalf("err %v", err)
	}

	// Check we've modified the index
	index, err = state.Index("acl_token")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}
	index, err = state.Index("acl_token_bootstrap")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}
}

func TestStateStore_UpsertACLTokens(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	tk1 := mock.ACLToken()
	tk2 := mock.ACLToken()

	ws := memdb.NewWatchSet()
	if _, err := state.ACLTokenByAccessorID(ws, tk1.AccessorID); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := state.ACLTokenByAccessorID(ws, tk2.AccessorID); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := state.UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{tk1, tk2}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.ACLTokenByAccessorID(ws, tk1.AccessorID)
	assert.Equal(t, nil, err)
	assert.Equal(t, tk1, out)

	out, err = state.ACLTokenByAccessorID(ws, tk2.AccessorID)
	assert.Equal(t, nil, err)
	assert.Equal(t, tk2, out)

	out, err = state.ACLTokenBySecretID(ws, tk1.SecretID)
	assert.Equal(t, nil, err)
	assert.Equal(t, tk1, out)

	out, err = state.ACLTokenBySecretID(ws, tk2.SecretID)
	assert.Equal(t, nil, err)
	assert.Equal(t, tk2, out)

	iter, err := state.ACLTokens(ws)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we see both policies
	count := 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
	}
	if count != 2 {
		t.Fatalf("bad: %d", count)
	}

	index, err := state.Index("acl_token")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_DeleteACLTokens(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	tk1 := mock.ACLToken()
	tk2 := mock.ACLToken()

	// Create the tokens
	if err := state.UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{tk1, tk2}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watcher
	ws := memdb.NewWatchSet()
	if _, err := state.ACLTokenByAccessorID(ws, tk1.AccessorID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Delete the token
	if err := state.DeleteACLTokens(structs.MsgTypeTestSetup, 1001, []string{tk1.AccessorID, tk2.AccessorID}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure watching triggered
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Ensure we don't get the object back
	ws = memdb.NewWatchSet()
	out, err := state.ACLTokenByAccessorID(ws, tk1.AccessorID)
	assert.Equal(t, nil, err)
	if out != nil {
		t.Fatalf("bad: %#v", out)
	}

	iter, err := state.ACLTokens(ws)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we see both policies
	count := 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
	}
	if count != 0 {
		t.Fatalf("bad: %d", count)
	}

	index, err := state.Index("acl_token")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_ACLTokenByAccessorIDPrefix(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	prefixes := []string{
		"aaaa",
		"aabb",
		"bbbb",
		"bbcc",
		"ffff",
	}

	// Create the tokens
	var baseIndex uint64 = 1000
	for _, prefix := range prefixes {
		tk := mock.ACLToken()
		tk.AccessorID = prefix + tk.AccessorID[4:]
		if err := state.UpsertACLTokens(structs.MsgTypeTestSetup, baseIndex, []*structs.ACLToken{tk}); err != nil {
			t.Fatalf("err: %v", err)
		}
		baseIndex++
	}

	// Scan by prefix
	iter, err := state.ACLTokenByAccessorIDPrefix(nil, "aa")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we see both tokens
	count := 0
	out := []string{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
		out = append(out, raw.(*structs.ACLToken).AccessorID[:4])
	}
	if count != 2 {
		t.Fatalf("bad: %d %v", count, out)
	}
	sort.Strings(out)

	expect := []string{"aaaa", "aabb"}
	assert.Equal(t, expect, out)
}

func TestStateStore_ACLTokensByGlobal(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	tk1 := mock.ACLToken()
	tk2 := mock.ACLToken()
	tk3 := mock.ACLToken()
	tk4 := mock.ACLToken()
	tk3.Global = true

	if err := state.UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{tk1, tk2, tk3, tk4}); err != nil {
		t.Fatalf("err: %v", err)
	}

	iter, err := state.ACLTokensByGlobal(nil, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we see the one global policies
	count := 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
	}
	if count != 1 {
		t.Fatalf("bad: %d", count)
	}
}

func TestStateStore_OneTimeTokens(t *testing.T) {
	t.Parallel()
	index := uint64(100)
	state := testStateStore(t)

	// create some ACL tokens

	token1 := mock.ACLToken()
	token2 := mock.ACLToken()
	token3 := mock.ACLToken()
	index++
	require.Nil(t, state.UpsertACLTokens(
		structs.MsgTypeTestSetup, index,
		[]*structs.ACLToken{token1, token2, token3}))

	otts := []*structs.OneTimeToken{
		{
			// expired OTT for token1
			OneTimeSecretID: uuid.Generate(),
			AccessorID:      token1.AccessorID,
			ExpiresAt:       time.Now().Add(-1 * time.Minute),
		},
		{
			// valid OTT for token2
			OneTimeSecretID: uuid.Generate(),
			AccessorID:      token2.AccessorID,
			ExpiresAt:       time.Now().Add(10 * time.Minute),
		},
		{
			// new but expired OTT for token2; this will be accepted even
			// though it's expired and overwrite the other one
			OneTimeSecretID: uuid.Generate(),
			AccessorID:      token2.AccessorID,
			ExpiresAt:       time.Now().Add(-10 * time.Minute),
		},
		{
			// valid OTT for token3
			AccessorID:      token3.AccessorID,
			OneTimeSecretID: uuid.Generate(),
			ExpiresAt:       time.Now().Add(10 * time.Minute),
		},
		{
			// new valid OTT for token3
			OneTimeSecretID: uuid.Generate(),
			AccessorID:      token3.AccessorID,
			ExpiresAt:       time.Now().Add(5 * time.Minute),
		},
	}

	for _, ott := range otts {
		index++
		require.NoError(t, state.UpsertOneTimeToken(structs.MsgTypeTestSetup, index, ott))
	}

	// verify that we have exactly one OTT for each AccessorID

	txn := state.db.ReadTxn()
	iter, err := txn.Get("one_time_token", "id")
	require.NoError(t, err)
	results := []*structs.OneTimeToken{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		ott, ok := raw.(*structs.OneTimeToken)
		require.True(t, ok)
		results = append(results, ott)
	}

	// results aren't ordered but if we have 3 OTT and all 3 tokens, we know
	// we have no duplicate accessors
	require.Len(t, results, 3)
	accessors := []string{
		results[0].AccessorID, results[1].AccessorID, results[2].AccessorID}
	require.Contains(t, accessors, token1.AccessorID)
	require.Contains(t, accessors, token2.AccessorID)
	require.Contains(t, accessors, token3.AccessorID)

	// now verify expiration

	getExpiredTokens := func() []*structs.OneTimeToken {
		txn := state.db.ReadTxn()
		iter, err := state.oneTimeTokensExpiredTxn(txn, nil)
		require.NoError(t, err)

		results := []*structs.OneTimeToken{}
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			ott, ok := raw.(*structs.OneTimeToken)
			require.True(t, ok)
			results = append(results, ott)
		}
		return results
	}

	results = getExpiredTokens()
	require.Len(t, results, 2)

	// results aren't ordered
	expiredAccessors := []string{results[0].AccessorID, results[1].AccessorID}
	require.Contains(t, expiredAccessors, token1.AccessorID)
	require.Contains(t, expiredAccessors, token2.AccessorID)
	require.True(t, time.Now().After(results[0].ExpiresAt))
	require.True(t, time.Now().After(results[1].ExpiresAt))

	// clear the expired tokens and verify they're gone
	index++
	require.NoError(t,
		state.ExpireOneTimeTokens(structs.MsgTypeTestSetup, index))

	results = getExpiredTokens()
	require.Len(t, results, 0)

	// query the unexpired token
	ott, err := state.OneTimeTokenBySecret(nil, otts[len(otts)-1].OneTimeSecretID)
	require.NoError(t, err)
	require.Equal(t, token3.AccessorID, ott.AccessorID)
	require.True(t, time.Now().Before(ott.ExpiresAt))

	restore, err := state.Restore()
	require.NoError(t, err)
	err = restore.OneTimeTokenRestore(ott)
	require.NoError(t, err)
	require.NoError(t, restore.Commit())

	ott, err = state.OneTimeTokenBySecret(nil, otts[len(otts)-1].OneTimeSecretID)
	require.NoError(t, err)
	require.Equal(t, token3.AccessorID, ott.AccessorID)
}

func TestStateStore_ClusterMetadata(t *testing.T) {
	require := require.New(t)

	state := testStateStore(t)
	clusterID := "12345678-1234-1234-1234-1234567890"
	now := time.Now().UnixNano()
	meta := &structs.ClusterMetadata{ClusterID: clusterID, CreateTime: now}

	err := state.ClusterSetMetadata(100, meta)
	require.NoError(err)

	result, err := state.ClusterMetadata(nil)
	require.NoError(err)
	require.Equal(clusterID, result.ClusterID)
	require.Equal(now, result.CreateTime)
}

func TestStateStore_UpsertScalingPolicy(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)
	policy := mock.ScalingPolicy()
	policy2 := mock.ScalingPolicy()

	wsAll := memdb.NewWatchSet()
	all, err := state.ScalingPolicies(wsAll)
	require.NoError(err)
	require.Nil(all.Next())

	ws := memdb.NewWatchSet()
	out, err := state.ScalingPolicyByTargetAndType(ws, policy.Target, policy.Type)
	require.NoError(err)
	require.Nil(out)

	out, err = state.ScalingPolicyByTargetAndType(ws, policy2.Target, policy2.Type)
	require.NoError(err)
	require.Nil(out)

	err = state.UpsertScalingPolicies(1000, []*structs.ScalingPolicy{policy, policy2})
	require.NoError(err)
	require.True(watchFired(ws))
	require.True(watchFired(wsAll))

	ws = memdb.NewWatchSet()
	out, err = state.ScalingPolicyByTargetAndType(ws, policy.Target, policy.Type)
	require.NoError(err)
	require.Equal(policy, out)

	out, err = state.ScalingPolicyByTargetAndType(ws, policy2.Target, policy2.Type)
	require.NoError(err)
	require.Equal(policy2, out)

	// Ensure we see both policies
	countPolicies := func() (n int, err error) {
		iter, err := state.ScalingPolicies(ws)
		if err != nil {
			return
		}

		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			n++
		}
		return
	}

	count, err := countPolicies()
	require.NoError(err)
	require.Equal(2, count)

	index, err := state.Index("scaling_policy")
	require.NoError(err)
	require.True(1000 == index)
	require.False(watchFired(ws))

	// Check that we can add policy with same target but different type
	policy3 := mock.ScalingPolicy()
	for k, v := range policy2.Target {
		policy3.Target[k] = v
	}

	err = state.UpsertScalingPolicies(1000, []*structs.ScalingPolicy{policy3})
	require.NoError(err)

	// Ensure we see both policies, since target didn't change
	count, err = countPolicies()
	require.NoError(err)
	require.Equal(2, count)

	// Change type and check if we see 3
	policy3.Type = "other-type"

	err = state.UpsertScalingPolicies(1000, []*structs.ScalingPolicy{policy3})
	require.NoError(err)

	count, err = countPolicies()
	require.NoError(err)
	require.Equal(3, count)
}

func TestStateStore_UpsertScalingPolicy_Namespace(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	otherNamespace := "not-default-namespace"
	state := testStateStore(t)
	policy := mock.ScalingPolicy()
	policy2 := mock.ScalingPolicy()
	policy2.Target[structs.ScalingTargetNamespace] = otherNamespace

	ws1 := memdb.NewWatchSet()
	iter, err := state.ScalingPoliciesByNamespace(ws1, structs.DefaultNamespace, "")
	require.NoError(err)
	require.Nil(iter.Next())

	ws2 := memdb.NewWatchSet()
	iter, err = state.ScalingPoliciesByNamespace(ws2, otherNamespace, "")
	require.NoError(err)
	require.Nil(iter.Next())

	err = state.UpsertScalingPolicies(1000, []*structs.ScalingPolicy{policy, policy2})
	require.NoError(err)
	require.True(watchFired(ws1))
	require.True(watchFired(ws2))

	iter, err = state.ScalingPoliciesByNamespace(nil, structs.DefaultNamespace, "")
	require.NoError(err)
	policiesInDefaultNamespace := []string{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		policiesInDefaultNamespace = append(policiesInDefaultNamespace, raw.(*structs.ScalingPolicy).ID)
	}
	require.ElementsMatch([]string{policy.ID}, policiesInDefaultNamespace)

	iter, err = state.ScalingPoliciesByNamespace(nil, otherNamespace, "")
	require.NoError(err)
	policiesInOtherNamespace := []string{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		policiesInOtherNamespace = append(policiesInOtherNamespace, raw.(*structs.ScalingPolicy).ID)
	}
	require.ElementsMatch([]string{policy2.ID}, policiesInOtherNamespace)
}

func TestStateStore_UpsertScalingPolicy_Namespace_PrefixBug(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	ns1 := "name"
	ns2 := "name2" // matches prefix "name"
	state := testStateStore(t)
	policy1 := mock.ScalingPolicy()
	policy1.Target[structs.ScalingTargetNamespace] = ns1
	policy2 := mock.ScalingPolicy()
	policy2.Target[structs.ScalingTargetNamespace] = ns2

	ws1 := memdb.NewWatchSet()
	iter, err := state.ScalingPoliciesByNamespace(ws1, ns1, "")
	require.NoError(err)
	require.Nil(iter.Next())

	ws2 := memdb.NewWatchSet()
	iter, err = state.ScalingPoliciesByNamespace(ws2, ns2, "")
	require.NoError(err)
	require.Nil(iter.Next())

	err = state.UpsertScalingPolicies(1000, []*structs.ScalingPolicy{policy1, policy2})
	require.NoError(err)
	require.True(watchFired(ws1))
	require.True(watchFired(ws2))

	iter, err = state.ScalingPoliciesByNamespace(nil, ns1, "")
	require.NoError(err)
	policiesInNS1 := []string{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		policiesInNS1 = append(policiesInNS1, raw.(*structs.ScalingPolicy).ID)
	}
	require.ElementsMatch([]string{policy1.ID}, policiesInNS1)

	iter, err = state.ScalingPoliciesByNamespace(nil, ns2, "")
	require.NoError(err)
	policiesInNS2 := []string{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		policiesInNS2 = append(policiesInNS2, raw.(*structs.ScalingPolicy).ID)
	}
	require.ElementsMatch([]string{policy2.ID}, policiesInNS2)
}

// Scaling Policy IDs are generated randomly during Job.Register
// Subsequent updates of the job should preserve the ID for the scaling policy
// associated with a given target.
func TestStateStore_UpsertJob_PreserveScalingPolicyIDsAndIndex(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	state := testStateStore(t)
	job, policy := mock.JobWithScalingPolicy()

	var newIndex uint64 = 1000
	err := state.UpsertJob(structs.MsgTypeTestSetup, newIndex, job)
	require.NoError(err)

	ws := memdb.NewWatchSet()
	p1, err := state.ScalingPolicyByTargetAndType(ws, policy.Target, policy.Type)
	require.NoError(err)
	require.NotNil(p1)
	require.Equal(newIndex, p1.CreateIndex)
	require.Equal(newIndex, p1.ModifyIndex)

	index, err := state.Index("scaling_policy")
	require.NoError(err)
	require.Equal(newIndex, index)
	require.NotEmpty(p1.ID)

	// update the job
	job.Meta["new-meta"] = "new-value"
	newIndex += 100
	err = state.UpsertJob(structs.MsgTypeTestSetup, newIndex, job)
	require.NoError(err)
	require.False(watchFired(ws), "watch should not have fired")

	p2, err := state.ScalingPolicyByTargetAndType(nil, policy.Target, policy.Type)
	require.NoError(err)
	require.NotNil(p2)
	require.Equal(p1.ID, p2.ID, "ID should not have changed")
	require.Equal(p1.CreateIndex, p2.CreateIndex)
	require.Equal(p1.ModifyIndex, p2.ModifyIndex)

	index, err = state.Index("scaling_policy")
	require.NoError(err)
	require.Equal(index, p1.CreateIndex, "table index should not have changed")
}

// Updating the scaling policy for a job should update the index table and fire the watch.
// This test is the converse of TestStateStore_UpsertJob_PreserveScalingPolicyIDsAndIndex
func TestStateStore_UpsertJob_UpdateScalingPolicy(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	state := testStateStore(t)
	job, policy := mock.JobWithScalingPolicy()

	var oldIndex uint64 = 1000
	require.NoError(state.UpsertJob(structs.MsgTypeTestSetup, oldIndex, job))

	ws := memdb.NewWatchSet()
	p1, err := state.ScalingPolicyByTargetAndType(ws, policy.Target, policy.Type)
	require.NoError(err)
	require.NotNil(p1)
	require.Equal(oldIndex, p1.CreateIndex)
	require.Equal(oldIndex, p1.ModifyIndex)
	prevId := p1.ID

	index, err := state.Index("scaling_policy")
	require.NoError(err)
	require.Equal(oldIndex, index)
	require.NotEmpty(p1.ID)

	// update the job with the updated scaling policy; make sure to use a different object
	newPolicy := p1.Copy()
	newPolicy.Policy["new-field"] = "new-value"
	job.TaskGroups[0].Scaling = newPolicy
	require.NoError(state.UpsertJob(structs.MsgTypeTestSetup, oldIndex+100, job))
	require.True(watchFired(ws), "watch should have fired")

	p2, err := state.ScalingPolicyByTargetAndType(nil, policy.Target, policy.Type)
	require.NoError(err)
	require.NotNil(p2)
	require.Equal(p2.Policy["new-field"], "new-value")
	require.Equal(prevId, p2.ID, "ID should not have changed")
	require.Equal(oldIndex, p2.CreateIndex)
	require.Greater(p2.ModifyIndex, oldIndex, "ModifyIndex should have advanced")

	index, err = state.Index("scaling_policy")
	require.NoError(err)
	require.Greater(index, oldIndex, "table index should have advanced")
}

func TestStateStore_DeleteScalingPolicies(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	state := testStateStore(t)
	policy := mock.ScalingPolicy()
	policy2 := mock.ScalingPolicy()

	// Create the policy
	err := state.UpsertScalingPolicies(1000, []*structs.ScalingPolicy{policy, policy2})
	require.NoError(err)

	// Create a watcher
	ws := memdb.NewWatchSet()
	_, err = state.ScalingPolicyByTargetAndType(ws, policy.Target, policy.Type)
	require.NoError(err)

	// Delete the policy
	err = state.DeleteScalingPolicies(1001, []string{policy.ID, policy2.ID})
	require.NoError(err)

	// Ensure watching triggered
	require.True(watchFired(ws))

	// Ensure we don't get the objects back
	ws = memdb.NewWatchSet()
	out, err := state.ScalingPolicyByTargetAndType(ws, policy.Target, policy.Type)
	require.NoError(err)
	require.Nil(out)

	ws = memdb.NewWatchSet()
	out, err = state.ScalingPolicyByTargetAndType(ws, policy2.Target, policy2.Type)
	require.NoError(err)
	require.Nil(out)

	// Ensure we see both policies
	iter, err := state.ScalingPoliciesByNamespace(ws, policy.Target[structs.ScalingTargetNamespace], "")
	require.NoError(err)
	count := 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
	}
	require.Equal(0, count)

	index, err := state.Index("scaling_policy")
	require.NoError(err)
	require.True(1001 == index)
	require.False(watchFired(ws))
}

func TestStateStore_StopJob_DeleteScalingPolicies(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	state := testStateStore(t)

	job := mock.Job()

	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, job)
	require.NoError(err)

	policy := mock.ScalingPolicy()
	policy.Target[structs.ScalingTargetJob] = job.ID
	err = state.UpsertScalingPolicies(1100, []*structs.ScalingPolicy{policy})
	require.NoError(err)

	// Ensure the scaling policy is present and start some watches
	wsGet := memdb.NewWatchSet()
	out, err := state.ScalingPolicyByTargetAndType(wsGet, policy.Target, policy.Type)
	require.NoError(err)
	require.NotNil(out)
	wsList := memdb.NewWatchSet()
	_, err = state.ScalingPolicies(wsList)
	require.NoError(err)

	// Stop the job
	job, err = state.JobByID(nil, job.Namespace, job.ID)
	require.NoError(err)
	job.Stop = true
	err = state.UpsertJob(structs.MsgTypeTestSetup, 1200, job)
	require.NoError(err)

	// Ensure:
	// * the scaling policy was deleted
	// * the watches were fired
	// * the table index was advanced
	require.True(watchFired(wsGet))
	require.True(watchFired(wsList))
	out, err = state.ScalingPolicyByTargetAndType(nil, policy.Target, policy.Type)
	require.NoError(err)
	require.Nil(out)
	index, err := state.Index("scaling_policy")
	require.NoError(err)
	require.GreaterOrEqual(index, uint64(1200))
}

func TestStateStore_UnstopJob_UpsertScalingPolicies(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	state := testStateStore(t)

	job, policy := mock.JobWithScalingPolicy()
	job.Stop = true

	// establish watcher, verify there are no scaling policies yet
	ws := memdb.NewWatchSet()
	list, err := state.ScalingPolicies(ws)
	require.NoError(err)
	require.Nil(list.Next())

	// upsert a stopped job, verify that we don't fire the watcher or add any scaling policies
	err = state.UpsertJob(structs.MsgTypeTestSetup, 1000, job)
	require.NoError(err)
	require.True(watchFired(ws))
	list, err = state.ScalingPolicies(ws)
	require.NoError(err)
	require.NotNil(list.Next())

	// Establish a new watchset
	ws = memdb.NewWatchSet()
	_, err = state.ScalingPolicies(ws)
	require.NoError(err)
	// Unstop this job, say you'll run it again...
	job.Stop = false
	err = state.UpsertJob(structs.MsgTypeTestSetup, 1100, job)
	require.NoError(err)

	// Ensure the scaling policy still exists, watch was not fired, index was not advanced
	out, err := state.ScalingPolicyByTargetAndType(nil, policy.Target, policy.Type)
	require.NoError(err)
	require.NotNil(out)
	index, err := state.Index("scaling_policy")
	require.NoError(err)
	require.EqualValues(index, 1000)
	require.False(watchFired(ws))
}

func TestStateStore_DeleteJob_DeleteScalingPolicies(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	state := testStateStore(t)

	job := mock.Job()

	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, job)
	require.NoError(err)

	policy := mock.ScalingPolicy()
	policy.Target[structs.ScalingTargetJob] = job.ID
	err = state.UpsertScalingPolicies(1001, []*structs.ScalingPolicy{policy})
	require.NoError(err)

	// Delete the job
	err = state.DeleteJob(1002, job.Namespace, job.ID)
	require.NoError(err)

	// Ensure the scaling policy was deleted
	ws := memdb.NewWatchSet()
	out, err := state.ScalingPolicyByTargetAndType(ws, policy.Target, policy.Type)
	require.NoError(err)
	require.Nil(out)
	index, err := state.Index("scaling_policy")
	require.NoError(err)
	require.True(index > 1001)
}

func TestStateStore_DeleteJob_DeleteScalingPoliciesPrefixBug(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	state := testStateStore(t)

	job := mock.Job()
	require.NoError(state.UpsertJob(structs.MsgTypeTestSetup, 1000, job))
	job2 := job.Copy()
	job2.ID = job.ID + "-but-longer"
	require.NoError(state.UpsertJob(structs.MsgTypeTestSetup, 1001, job2))

	policy := mock.ScalingPolicy()
	policy.Target[structs.ScalingTargetJob] = job.ID
	policy2 := mock.ScalingPolicy()
	policy2.Target[structs.ScalingTargetJob] = job2.ID
	require.NoError(state.UpsertScalingPolicies(1002, []*structs.ScalingPolicy{policy, policy2}))

	// Delete job with the shorter prefix-ID
	require.NoError(state.DeleteJob(1003, job.Namespace, job.ID))

	// Ensure only the associated scaling policy was deleted, not the one matching the job with the longer ID
	out, err := state.ScalingPolicyByID(nil, policy.ID)
	require.NoError(err)
	require.Nil(out)
	out, err = state.ScalingPolicyByID(nil, policy2.ID)
	require.NoError(err)
	require.NotNil(out)
}

// This test ensures that deleting a job that doesn't have any scaling policies
// will not cause the scaling_policy table index to increase, on either job
// registration or deletion.
func TestStateStore_DeleteJob_ScalingPolicyIndexNoop(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	state := testStateStore(t)

	job := mock.Job()

	prevIndex, err := state.Index("scaling_policy")
	require.NoError(err)

	err = state.UpsertJob(structs.MsgTypeTestSetup, 1000, job)
	require.NoError(err)

	newIndex, err := state.Index("scaling_policy")
	require.NoError(err)
	require.Equal(prevIndex, newIndex)

	// Delete the job
	err = state.DeleteJob(1002, job.Namespace, job.ID)
	require.NoError(err)

	newIndex, err = state.Index("scaling_policy")
	require.NoError(err)
	require.Equal(prevIndex, newIndex)
}

func TestStateStore_ScalingPoliciesByType(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	state := testStateStore(t)

	// Create scaling policies of different types
	pHorzA := mock.ScalingPolicy()
	pHorzA.Type = structs.ScalingPolicyTypeHorizontal
	pHorzB := mock.ScalingPolicy()
	pHorzB.Type = structs.ScalingPolicyTypeHorizontal

	pOther1 := mock.ScalingPolicy()
	pOther1.Type = "other-type-1"

	pOther2 := mock.ScalingPolicy()
	pOther2.Type = "other-type-2"

	// Create search routine
	search := func(t string) (found []string) {
		found = []string{}
		iter, err := state.ScalingPoliciesByTypePrefix(nil, t)
		require.NoError(err)

		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			found = append(found, raw.(*structs.ScalingPolicy).Type)
		}
		return
	}

	// Create the policies
	var baseIndex uint64 = 1000
	err := state.UpsertScalingPolicies(baseIndex, []*structs.ScalingPolicy{pHorzA, pHorzB, pOther1, pOther2})
	require.NoError(err)

	// Check if we can read horizontal policies
	expect := []string{pHorzA.Type, pHorzB.Type}
	actual := search(structs.ScalingPolicyTypeHorizontal)
	require.ElementsMatch(expect, actual)

	// Check if we can read policies of other types
	expect = []string{pOther1.Type}
	actual = search("other-type-1")
	require.ElementsMatch(expect, actual)

	// Check that we can read policies by prefix
	expect = []string{"other-type-1", "other-type-2"}
	actual = search("other-type")
	require.Equal(expect, actual)

	// Check for empty result
	expect = []string{}
	actual = search("non-existing")
	require.ElementsMatch(expect, actual)
}

func TestStateStore_ScalingPoliciesByTypePrefix(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	state := testStateStore(t)

	// Create scaling policies of different types
	pHorzA := mock.ScalingPolicy()
	pHorzA.Type = structs.ScalingPolicyTypeHorizontal
	pHorzB := mock.ScalingPolicy()
	pHorzB.Type = structs.ScalingPolicyTypeHorizontal

	pOther1 := mock.ScalingPolicy()
	pOther1.Type = "other-type-1"

	pOther2 := mock.ScalingPolicy()
	pOther2.Type = "other-type-2"

	// Create search routine
	search := func(t string) (count int, found []string, err error) {
		found = []string{}
		iter, err := state.ScalingPoliciesByTypePrefix(nil, t)
		if err != nil {
			return
		}

		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			count++
			found = append(found, raw.(*structs.ScalingPolicy).Type)
		}
		return
	}

	// Create the policies
	var baseIndex uint64 = 1000
	err := state.UpsertScalingPolicies(baseIndex, []*structs.ScalingPolicy{pHorzA, pHorzB, pOther1, pOther2})
	require.NoError(err)

	// Check if we can read horizontal policies
	expect := []string{pHorzA.Type, pHorzB.Type}
	count, found, err := search("h")

	sort.Strings(found)
	sort.Strings(expect)

	require.NoError(err)
	require.Equal(expect, found)
	require.Equal(2, count)

	// Check if we can read other prefix policies
	expect = []string{pOther1.Type, pOther2.Type}
	count, found, err = search("other")

	sort.Strings(found)
	sort.Strings(expect)

	require.NoError(err)
	require.Equal(expect, found)
	require.Equal(2, count)

	// Check for empty result
	expect = []string{}
	count, found, err = search("non-existing")

	sort.Strings(found)
	sort.Strings(expect)

	require.NoError(err)
	require.Equal(expect, found)
	require.Equal(0, count)
}

func TestStateStore_ScalingPoliciesByJob(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	state := testStateStore(t)
	policyA := mock.ScalingPolicy()
	policyB1 := mock.ScalingPolicy()
	policyB2 := mock.ScalingPolicy()
	policyB1.Target[structs.ScalingTargetJob] = policyB2.Target[structs.ScalingTargetJob]

	// Create the policies
	var baseIndex uint64 = 1000
	err := state.UpsertScalingPolicies(baseIndex, []*structs.ScalingPolicy{policyA, policyB1, policyB2})
	require.NoError(err)

	iter, err := state.ScalingPoliciesByJob(nil,
		policyA.Target[structs.ScalingTargetNamespace],
		policyA.Target[structs.ScalingTargetJob], "")
	require.NoError(err)

	// Ensure we see expected policies
	count := 0
	found := []string{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
		found = append(found, raw.(*structs.ScalingPolicy).Target[structs.ScalingTargetGroup])
	}
	require.Equal(1, count)
	sort.Strings(found)
	expect := []string{policyA.Target[structs.ScalingTargetGroup]}
	sort.Strings(expect)
	require.Equal(expect, found)

	iter, err = state.ScalingPoliciesByJob(nil,
		policyB1.Target[structs.ScalingTargetNamespace],
		policyB1.Target[structs.ScalingTargetJob], "")
	require.NoError(err)

	// Ensure we see expected policies
	count = 0
	found = []string{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
		found = append(found, raw.(*structs.ScalingPolicy).Target[structs.ScalingTargetGroup])
	}
	require.Equal(2, count)
	sort.Strings(found)
	expect = []string{
		policyB1.Target[structs.ScalingTargetGroup],
		policyB2.Target[structs.ScalingTargetGroup],
	}
	sort.Strings(expect)
	require.Equal(expect, found)
}

func TestStateStore_ScalingPoliciesByJob_PrefixBug(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	jobPrefix := "job-name-" + uuid.Generate()

	state := testStateStore(t)
	policy1 := mock.ScalingPolicy()
	policy1.Target[structs.ScalingTargetJob] = jobPrefix
	policy2 := mock.ScalingPolicy()
	policy2.Target[structs.ScalingTargetJob] = jobPrefix + "-more"

	// Create the policies
	var baseIndex uint64 = 1000
	err := state.UpsertScalingPolicies(baseIndex, []*structs.ScalingPolicy{policy1, policy2})
	require.NoError(err)

	iter, err := state.ScalingPoliciesByJob(nil,
		policy1.Target[structs.ScalingTargetNamespace],
		jobPrefix, "")
	require.NoError(err)

	// Ensure we see expected policies
	count := 0
	found := []string{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
		found = append(found, raw.(*structs.ScalingPolicy).ID)
	}
	require.Equal(1, count)
	expect := []string{policy1.ID}
	require.Equal(expect, found)
}

func TestStateStore_ScalingPolicyByTargetAndType(t *testing.T) {
	t.Parallel()

	require := require.New(t)

	state := testStateStore(t)

	// Create scaling policies
	policyA := mock.ScalingPolicy()
	// Same target, different type
	policyB := mock.ScalingPolicy()
	policyC := mock.ScalingPolicy()
	for k, v := range policyB.Target {
		policyC.Target[k] = v
	}
	policyC.Type = "other-type"

	// Create the policies
	var baseIndex uint64 = 1000
	err := state.UpsertScalingPolicies(baseIndex, []*structs.ScalingPolicy{policyA, policyB, policyC})
	require.NoError(err)

	// Check if we can retrieve the right policies
	found, err := state.ScalingPolicyByTargetAndType(nil, policyA.Target, policyA.Type)
	require.NoError(err)
	require.Equal(policyA, found)

	// Check for wrong type
	found, err = state.ScalingPolicyByTargetAndType(nil, policyA.Target, "wrong_type")
	require.NoError(err)
	require.Nil(found)

	// Check for same target but different type
	found, err = state.ScalingPolicyByTargetAndType(nil, policyB.Target, policyB.Type)
	require.NoError(err)
	require.Equal(policyB, found)

	found, err = state.ScalingPolicyByTargetAndType(nil, policyB.Target, policyC.Type)
	require.NoError(err)
	require.Equal(policyC, found)
}

func TestStateStore_UpsertScalingEvent(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)
	job := mock.Job()
	groupName := job.TaskGroups[0].Name

	newEvent := structs.NewScalingEvent("message 1").SetMeta(map[string]interface{}{
		"a": 1,
	})

	wsAll := memdb.NewWatchSet()
	all, err := state.ScalingEvents(wsAll)
	require.NoError(err)
	require.Nil(all.Next())

	ws := memdb.NewWatchSet()
	out, _, err := state.ScalingEventsByJob(ws, job.Namespace, job.ID)
	require.NoError(err)
	require.Nil(out)

	err = state.UpsertScalingEvent(1000, &structs.ScalingEventRequest{
		Namespace:    job.Namespace,
		JobID:        job.ID,
		TaskGroup:    groupName,
		ScalingEvent: newEvent,
	})
	require.NoError(err)
	require.True(watchFired(ws))
	require.True(watchFired(wsAll))

	ws = memdb.NewWatchSet()
	out, eventsIndex, err := state.ScalingEventsByJob(ws, job.Namespace, job.ID)
	require.NoError(err)
	require.Equal(map[string][]*structs.ScalingEvent{
		groupName: {newEvent},
	}, out)
	require.EqualValues(eventsIndex, 1000)

	iter, err := state.ScalingEvents(ws)
	require.NoError(err)

	count := 0
	jobsReturned := []string{}
	var jobEvents *structs.JobScalingEvents
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		jobEvents = raw.(*structs.JobScalingEvents)
		jobsReturned = append(jobsReturned, jobEvents.JobID)
		count++
	}
	require.Equal(1, count)
	require.EqualValues(jobEvents.ModifyIndex, 1000)
	require.EqualValues(jobEvents.ScalingEvents[groupName][0].CreateIndex, 1000)

	index, err := state.Index("scaling_event")
	require.NoError(err)
	require.ElementsMatch([]string{job.ID}, jobsReturned)
	require.Equal(map[string][]*structs.ScalingEvent{
		groupName: {newEvent},
	}, jobEvents.ScalingEvents)
	require.EqualValues(1000, index)
	require.False(watchFired(ws))
}

func TestStateStore_UpsertScalingEvent_LimitAndOrder(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	state := testStateStore(t)
	namespace := uuid.Generate()
	jobID := uuid.Generate()
	group1 := uuid.Generate()
	group2 := uuid.Generate()

	index := uint64(1000)
	for i := 1; i <= structs.JobTrackedScalingEvents+10; i++ {
		newEvent := structs.NewScalingEvent("").SetMeta(map[string]interface{}{
			"i":     i,
			"group": group1,
		})
		err := state.UpsertScalingEvent(index, &structs.ScalingEventRequest{
			Namespace:    namespace,
			JobID:        jobID,
			TaskGroup:    group1,
			ScalingEvent: newEvent,
		})
		index++
		require.NoError(err)

		newEvent = structs.NewScalingEvent("").SetMeta(map[string]interface{}{
			"i":     i,
			"group": group2,
		})
		err = state.UpsertScalingEvent(index, &structs.ScalingEventRequest{
			Namespace:    namespace,
			JobID:        jobID,
			TaskGroup:    group2,
			ScalingEvent: newEvent,
		})
		index++
		require.NoError(err)
	}

	out, _, err := state.ScalingEventsByJob(nil, namespace, jobID)
	require.NoError(err)
	require.Len(out, 2)

	expectedEvents := []int{}
	for i := structs.JobTrackedScalingEvents; i > 0; i-- {
		expectedEvents = append(expectedEvents, i+10)
	}

	// checking order and content
	require.Len(out[group1], structs.JobTrackedScalingEvents)
	actualEvents := []int{}
	for _, event := range out[group1] {
		require.Equal(group1, event.Meta["group"])
		actualEvents = append(actualEvents, event.Meta["i"].(int))
	}
	require.Equal(expectedEvents, actualEvents)

	// checking order and content
	require.Len(out[group2], structs.JobTrackedScalingEvents)
	actualEvents = []int{}
	for _, event := range out[group2] {
		require.Equal(group2, event.Meta["group"])
		actualEvents = append(actualEvents, event.Meta["i"].(int))
	}
	require.Equal(expectedEvents, actualEvents)
}

func TestStateStore_Abandon(t *testing.T) {
	t.Parallel()

	s := testStateStore(t)
	abandonCh := s.AbandonCh()
	s.Abandon()
	select {
	case <-abandonCh:
	default:
		t.Fatalf("bad")
	}
}

// Verifies that an error is returned when an allocation doesn't exist in the state store.
func TestStateSnapshot_DenormalizeAllocationDiffSlice_AllocDoesNotExist(t *testing.T) {
	t.Parallel()

	state := testStateStore(t)
	alloc := mock.Alloc()
	require := require.New(t)

	// Insert job
	err := state.UpsertJob(structs.MsgTypeTestSetup, 999, alloc.Job)
	require.NoError(err)

	allocDiffs := []*structs.AllocationDiff{
		{
			ID: alloc.ID,
		},
	}

	snap, err := state.Snapshot()
	require.NoError(err)

	denormalizedAllocs, err := snap.DenormalizeAllocationDiffSlice(allocDiffs)

	require.EqualError(err, fmt.Sprintf("alloc %v doesn't exist", alloc.ID))
	require.Nil(denormalizedAllocs)
}

// TestStateStore_SnapshotMinIndex_OK asserts StateStore.SnapshotMinIndex blocks
// until the StateStore's latest index is >= the requested index.
func TestStateStore_SnapshotMinIndex_OK(t *testing.T) {
	t.Parallel()

	s := testStateStore(t)
	index, err := s.LatestIndex()
	require.NoError(t, err)

	node := mock.Node()
	require.NoError(t, s.UpsertNode(structs.MsgTypeTestSetup, index+1, node))

	// Assert SnapshotMinIndex returns immediately if index < latest index
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	snap, err := s.SnapshotMinIndex(ctx, index)
	cancel()
	require.NoError(t, err)

	snapIndex, err := snap.LatestIndex()
	require.NoError(t, err)
	if snapIndex <= index {
		require.Fail(t, "snapshot index should be greater than index")
	}

	// Assert SnapshotMinIndex returns immediately if index == latest index
	ctx, cancel = context.WithTimeout(context.Background(), 0)
	snap, err = s.SnapshotMinIndex(ctx, index+1)
	cancel()
	require.NoError(t, err)

	snapIndex, err = snap.LatestIndex()
	require.NoError(t, err)
	require.Equal(t, snapIndex, index+1)

	// Assert SnapshotMinIndex blocks if index > latest index
	errCh := make(chan error, 1)
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() {
		defer close(errCh)
		waitIndex := index + 2
		snap, err := s.SnapshotMinIndex(ctx, waitIndex)
		if err != nil {
			errCh <- err
			return
		}

		snapIndex, err := snap.LatestIndex()
		if err != nil {
			errCh <- err
			return
		}

		if snapIndex < waitIndex {
			errCh <- fmt.Errorf("snapshot index < wait index: %d < %d", snapIndex, waitIndex)
			return
		}
	}()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(500 * time.Millisecond):
		// Let it block for a bit before unblocking by upserting
	}

	node.Name = "hal"
	require.NoError(t, s.UpsertNode(structs.MsgTypeTestSetup, index+2, node))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for SnapshotMinIndex to unblock")
	}
}

// TestStateStore_SnapshotMinIndex_Timeout asserts StateStore.SnapshotMinIndex
// returns an error if the desired index is not reached within the deadline.
func TestStateStore_SnapshotMinIndex_Timeout(t *testing.T) {
	t.Parallel()

	s := testStateStore(t)
	index, err := s.LatestIndex()
	require.NoError(t, err)

	// Assert SnapshotMinIndex blocks if index > latest index
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	snap, err := s.SnapshotMinIndex(ctx, index+1)
	require.EqualError(t, err, context.DeadlineExceeded.Error())
	require.Nil(t, snap)
}

// watchFired is a helper for unit tests that returns if the given watch set
// fired (it doesn't care which watch actually fired). This uses a fixed
// timeout since we already expect the event happened before calling this and
// just need to distinguish a fire from a timeout. We do need a little time to
// allow the watch to set up any goroutines, though.
func watchFired(ws memdb.WatchSet) bool {
	timedOut := ws.Watch(time.After(50 * time.Millisecond))
	return !timedOut
}

// NodeIDSort is used to sort nodes by ID
type NodeIDSort []*structs.Node

func (n NodeIDSort) Len() int {
	return len(n)
}

func (n NodeIDSort) Less(i, j int) bool {
	return n[i].ID < n[j].ID
}

func (n NodeIDSort) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// JobIDis used to sort jobs by id
type JobIDSort []*structs.Job

func (n JobIDSort) Len() int {
	return len(n)
}

func (n JobIDSort) Less(i, j int) bool {
	return n[i].ID < n[j].ID
}

func (n JobIDSort) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// EvalIDis used to sort evals by id
type EvalIDSort []*structs.Evaluation

func (n EvalIDSort) Len() int {
	return len(n)
}

func (n EvalIDSort) Less(i, j int) bool {
	return n[i].ID < n[j].ID
}

func (n EvalIDSort) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// AllocIDsort used to sort allocations by id
type AllocIDSort []*structs.Allocation

func (n AllocIDSort) Len() int {
	return len(n)
}

func (n AllocIDSort) Less(i, j int) bool {
	return n[i].ID < n[j].ID
}

func (n AllocIDSort) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}

// nextIndex gets the LatestIndex for this store and assumes no error
// note: this helper is not safe for concurrent use
func nextIndex(s *StateStore) uint64 {
	index, _ := s.LatestIndex()
	index++
	return index
}
