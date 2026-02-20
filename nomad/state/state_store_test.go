// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/kr/pretty"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func testStateStore(t *testing.T) *StateStore {
	return TestStateStore(t)
}

func TestStateStore_InvalidConfig(t *testing.T) {
	config := &StateStoreConfig{
		// default zero value, but explicit because it causes validation failure
		JobTrackedVersions: 0,
	}
	store, err := NewStateStore(config)
	must.Nil(t, store)
	must.ErrorContains(t, err, "JobTrackedVersions must be positive")
}

func TestStateStore_Blocking_Error(t *testing.T) {
	ci.Parallel(t)

	expected := fmt.Errorf("test error")
	errFn := func(memdb.WatchSet, *StateStore) (interface{}, uint64, error) {
		return nil, 0, expected
	}

	state := testStateStore(t)
	_, idx, err := state.BlockingQuery(errFn, 10, context.Background())
	must.EqError(t, err, expected.Error())
	must.Zero(t, idx)
}

func TestStateStore_Blocking_Timeout(t *testing.T) {
	ci.Parallel(t)

	noopFn := func(memdb.WatchSet, *StateStore) (interface{}, uint64, error) {
		return nil, 5, nil
	}

	state := testStateStore(t)
	timeout := time.Now().Add(250 * time.Millisecond)
	deadlineCtx, cancel := context.WithDeadline(context.Background(), timeout)
	defer cancel()

	_, idx, err := state.BlockingQuery(noopFn, 10, deadlineCtx)
	must.EqError(t, err, context.DeadlineExceeded.Error())
	test.Eq(t, 5, idx)
	test.LessEq(t, 100*time.Millisecond, time.Since(timeout))
}

func TestStateStore_Blocking_MinQuery(t *testing.T) {
	ci.Parallel(t)

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
	must.NoError(t, err)
	test.Eq(t, 2, count)
	test.Eq(t, 11, idx)
	test.True(t, resp.(bool))
}

// This test checks that:
// 1) The job is denormalized
// 2) Allocations are created
func TestStateStore_UpsertPlanResults_AllocationsCreated_Denormalized(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	alloc := mock.Alloc()
	job := alloc.Job
	alloc.Job = nil

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, job))

	eval := mock.Eval()
	eval.JobID = job.ID

	// Create an eval
	must.NoError(t, state.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval}))

	// Create a plan result
	res := structs.ApplyPlanResultsRequest{
		AllocsUpdated: []*structs.Allocation{alloc},
		Job:           job,
		EvalID:        eval.ID,
	}

	err := state.UpsertPlanResults(structs.MsgTypeTestSetup, 1000, &res)
	must.NoError(t, err)

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	must.NoError(t, err)
	test.Eq(t, alloc, out)

	index, err := state.Index("allocs")
	must.NoError(t, err)
	test.Eq(t, 1000, index)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))

	evalOut, err := state.EvalByID(ws, eval.ID)
	must.NoError(t, err)
	must.NotNil(t, evalOut)
	test.Eq(t, 1000, evalOut.ModifyIndex)
}

// This test checks that:
// 1) The job is denormalized
// 2) Allocations are denormalized and updated with the diff
// That stopped allocs Job is unmodified
func TestStateStore_UpsertPlanResults_AllocationsDenormalized(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	alloc := mock.Alloc()
	job := alloc.Job
	alloc.Job = nil

	stoppedAlloc := mock.Alloc()
	stoppedAlloc.Job = job
	stoppedAlloc.JobID = job.ID
	stoppedAllocDiff := &structs.AllocationDiff{
		ID:                 stoppedAlloc.ID,
		DesiredDescription: "desired desc",
		ClientStatus:       structs.AllocClientStatusLost,
	}
	preemptedAlloc := mock.Alloc()
	preemptedAlloc.Job = job
	preemptedAlloc.JobID = job.ID
	preemptedAllocDiff := &structs.AllocationDiff{
		ID:                    preemptedAlloc.ID,
		PreemptedByAllocation: alloc.ID,
	}

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 900, nil, job))
	must.NoError(t, state.UpsertAllocs(
		structs.MsgTypeTestSetup, 999, []*structs.Allocation{stoppedAlloc, preemptedAlloc}))

	// modify job and ensure that stopped and preempted alloc point to original Job
	mJob := job.Copy()
	mJob.TaskGroups[0].Name = "other"

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, mJob))

	eval := mock.Eval()
	eval.JobID = job.ID

	// Create an eval
	must.NoError(t, state.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval}))

	// Create a plan result
	res := structs.ApplyPlanResultsRequest{
		AllocsUpdated:   []*structs.Allocation{alloc},
		AllocsStopped:   []*structs.AllocationDiff{stoppedAllocDiff},
		Job:             mJob,
		EvalID:          eval.ID,
		AllocsPreempted: []*structs.AllocationDiff{preemptedAllocDiff},
	}

	planModifyIndex := uint64(1000)
	err := state.UpsertPlanResults(structs.MsgTypeTestSetup, planModifyIndex, &res)
	must.NoError(t, err)

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	must.NoError(t, err)
	test.Eq(t, alloc, out)

	outJob, err := state.JobByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Eq(t, mJob.TaskGroups, outJob.TaskGroups)
	must.SliceNotEmpty(t, outJob.TaskGroups)

	updatedStoppedAlloc, err := state.AllocByID(ws, stoppedAlloc.ID)
	must.NoError(t, err)
	test.Eq(t, stoppedAllocDiff.DesiredDescription, updatedStoppedAlloc.DesiredDescription)
	test.Eq(t, structs.AllocDesiredStatusStop, updatedStoppedAlloc.DesiredStatus)
	test.Eq(t, stoppedAllocDiff.ClientStatus, updatedStoppedAlloc.ClientStatus)
	test.Eq(t, planModifyIndex, updatedStoppedAlloc.AllocModifyIndex)
	test.Eq(t, planModifyIndex, updatedStoppedAlloc.AllocModifyIndex)
	test.Eq(t, job.TaskGroups, updatedStoppedAlloc.Job.TaskGroups)

	updatedPreemptedAlloc, err := state.AllocByID(ws, preemptedAlloc.ID)
	must.NoError(t, err)
	test.Eq(t, structs.AllocDesiredStatusEvict, updatedPreemptedAlloc.DesiredStatus)
	test.Eq(t, preemptedAllocDiff.PreemptedByAllocation, updatedPreemptedAlloc.PreemptedByAllocation)
	test.Eq(t, planModifyIndex, updatedPreemptedAlloc.AllocModifyIndex)
	test.Eq(t, planModifyIndex, updatedPreemptedAlloc.AllocModifyIndex)
	test.Eq(t, job.TaskGroups, updatedPreemptedAlloc.Job.TaskGroups)

	index, err := state.Index("allocs")
	must.NoError(t, err)
	test.Eq(t, planModifyIndex, index)

	must.False(t, watchFired(ws))

	evalOut, err := state.EvalByID(ws, eval.ID)
	must.NoError(t, err)
	must.NotNil(t, evalOut)
	test.Eq(t, planModifyIndex, evalOut.ModifyIndex)
}

// This test checks that the deployment is created and allocations count towards
// the deployment
func TestStateStore_UpsertPlanResults_Deployment(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	alloc := mock.Alloc()
	alloc2 := mock.Alloc()
	job := alloc.Job
	alloc.Job = nil
	alloc.JobID = job.ID
	alloc2.Job = nil
	alloc2.JobID = job.ID

	d := mock.Deployment()
	alloc.DeploymentID = d.ID
	alloc2.DeploymentID = d.ID

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, job))

	eval := mock.Eval()
	eval.JobID = job.ID

	// Create an eval
	must.NoError(t, state.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval}))

	// Create a plan result
	res := structs.ApplyPlanResultsRequest{
		AllocsUpdated: []*structs.Allocation{alloc, alloc2},
		Job:           job,
		Deployment:    d,
		EvalID:        eval.ID,
	}

	err := state.UpsertPlanResults(structs.MsgTypeTestSetup, 1000, &res)
	must.NoError(t, err)

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	must.NoError(t, err)
	test.Eq(t, alloc, out)

	dout, err := state.DeploymentByID(ws, d.ID)
	must.NoError(t, err)
	must.NotNil(t, dout)

	tg, ok := dout.TaskGroups[alloc.TaskGroup]
	test.True(t, ok)
	must.NotNil(t, tg)
	test.Eq(t, 2, tg.PlacedAllocs)

	evalOut, err := state.EvalByID(ws, eval.ID)
	must.NoError(t, err)
	must.NotNil(t, evalOut)
	test.Eq(t, 1000, evalOut.ModifyIndex)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))

	// Update the allocs to be part of a new deployment
	d2 := d.Copy()
	d2.ID = uuid.Generate()

	allocNew := alloc.Copy()
	allocNew.DeploymentID = d2.ID
	allocNew2 := alloc2.Copy()
	allocNew2.DeploymentID = d2.ID

	// Create another plan
	res = structs.ApplyPlanResultsRequest{
		AllocsUpdated: []*structs.Allocation{allocNew, allocNew2},
		Job:           job,
		Deployment:    d2,
		EvalID:        eval.ID,
	}

	err = state.UpsertPlanResults(structs.MsgTypeTestSetup, 1001, &res)
	must.NoError(t, err)

	dout, err = state.DeploymentByID(ws, d2.ID)
	must.NoError(t, err)
	must.NotNil(t, dout)

	tg, ok = dout.TaskGroups[alloc.TaskGroup]
	test.True(t, ok)
	must.NotNil(t, tg)
	test.Eq(t, 2, tg.PlacedAllocs)

	evalOut, err = state.EvalByID(ws, eval.ID)
	must.NoError(t, err)
	must.NotNil(t, evalOut)
	test.Eq(t, 1001, evalOut.ModifyIndex)
}

// This test checks that:
// 1) Preempted allocations in plan results are updated
// 2) Evals are inserted for preempted jobs
func TestStateStore_UpsertPlanResults_PreemptedAllocs(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	alloc := mock.Alloc()
	job := alloc.Job
	alloc.Job = nil

	// Insert job
	err := state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, job)
	must.NoError(t, err)

	// Create an eval
	eval := mock.Eval()
	eval.JobID = job.ID
	err = state.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval})
	must.NoError(t, err)

	// Insert alloc that will be preempted in the plan
	preemptedAlloc := mock.Alloc()
	preemptedAlloc.JobID = job.ID
	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 2, []*structs.Allocation{preemptedAlloc})
	must.NoError(t, err)

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
		AllocsUpdated:   []*structs.Allocation{alloc},
		Job:             job,
		EvalID:          eval.ID,
		AllocsPreempted: []*structs.AllocationDiff{minimalPreemptedAlloc.AllocationDiff()},
		PreemptionEvals: []*structs.Evaluation{eval2},
	}

	err = state.UpsertPlanResults(structs.MsgTypeTestSetup, 1000, &res)
	must.NoError(t, err)

	ws := memdb.NewWatchSet()

	// Verify alloc and eval created by plan
	out, err := state.AllocByID(ws, alloc.ID)
	must.NoError(t, err)
	must.Eq(t, alloc, out)

	index, err := state.Index("allocs")
	must.NoError(t, err)
	must.Eq(t, 1000, index)

	evalOut, err := state.EvalByID(ws, eval.ID)
	must.NoError(t, err)
	must.NotNil(t, evalOut)
	must.Eq(t, 1000, evalOut.ModifyIndex)

	// Verify preempted alloc
	preempted, err := state.AllocByID(ws, preemptedAlloc.ID)
	must.NoError(t, err)
	must.Eq(t, preempted.DesiredStatus, structs.AllocDesiredStatusEvict)
	must.Eq(t, preempted.DesiredDescription, fmt.Sprintf("Preempted by alloc ID %v", alloc.ID))
	must.Eq(t, preempted.Job.ID, preemptedAlloc.Job.ID)
	must.Eq(t, preempted.Job, preemptedAlloc.Job)

	// Verify eval for preempted job
	preemptedJobEval, err := state.EvalByID(ws, eval2.ID)
	must.NoError(t, err)
	must.NotNil(t, preemptedJobEval)
	must.Eq(t, 1000, preemptedJobEval.ModifyIndex)

}

// This test checks that deployment updates are applied correctly
func TestStateStore_UpsertPlanResults_DeploymentUpdates(t *testing.T) {
	ci.Parallel(t)
	state := testStateStore(t)

	// Create a job that applies to all
	job := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 998, nil, job))

	// Create a deployment that we will update its status
	doutstanding := mock.Deployment()
	doutstanding.JobID = job.ID

	must.NoError(t, state.UpsertDeployment(1000, doutstanding))

	eval := mock.Eval()
	eval.JobID = job.ID

	// Create an eval
	must.NoError(t, state.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval}))
	alloc := mock.Alloc()
	alloc.Job = nil
	alloc.JobID = job.ID

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
		AllocsUpdated:     []*structs.Allocation{alloc},
		Job:               job,
		Deployment:        dnew,
		DeploymentUpdates: []*structs.DeploymentStatusUpdate{update},
		EvalID:            eval.ID,
	}

	err := state.UpsertPlanResults(structs.MsgTypeTestSetup, 1000, &res)
	must.NoError(t, err)
	ws := memdb.NewWatchSet()

	// Check the deployments are correctly updated.
	dout, err := state.DeploymentByID(ws, dnew.ID)
	must.NoError(t, err)
	must.NotNil(t, dout)

	tg, ok := dout.TaskGroups[alloc.TaskGroup]
	test.True(t, ok)
	must.NotNil(t, tg)
	test.Eq(t, 1, tg.PlacedAllocs)

	doutstandingout, err := state.DeploymentByID(ws, doutstanding.ID)
	must.NoError(t, err)
	must.NotNil(t, doutstandingout)
	test.Eq(t, update.Status, doutstandingout.Status)
	test.Eq(t, update.StatusDescription, doutstandingout.StatusDescription)
	test.Eq(t, 1000, doutstandingout.ModifyIndex)

	evalOut, err := state.EvalByID(ws, eval.ID)
	must.NoError(t, err)
	must.NotNil(t, evalOut)
	test.Eq(t, 1000, evalOut.ModifyIndex)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_UpsertPlanResults_AllocationResources(t *testing.T) {
	ci.Parallel(t)

	dev := &structs.RequestedDevice{Name: "nvidia/gpu/Tesla 60", Count: 1}
	structuredDev := &structs.AllocatedDeviceResource{
		Vendor:    "nvidia",
		Type:      "gpu",
		Name:      "Tesla 60",
		DeviceIDs: []string{"GPU-0668fc92-f8d5-07f6-e3cc-c07d76f466a1"},
	}

	state := testStateStore(t)
	alloc := mock.Alloc()
	job := alloc.Job
	alloc.Job = nil
	alloc.Resources = nil
	alloc.AllocatedResources.Tasks["web"].Devices = []*structs.AllocatedDeviceResource{structuredDev}

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, job))

	eval := mock.Eval()
	eval.JobID = job.ID

	// Create an eval
	must.NoError(t, state.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval}))

	// Create a plan result
	res := structs.ApplyPlanResultsRequest{
		AllocsUpdated: []*structs.Allocation{alloc},
		Job:           job,
		EvalID:        eval.ID,
	}

	must.NoError(t, state.UpsertPlanResults(structs.MsgTypeTestSetup, 1000, &res))

	out, err := state.AllocByID(nil, alloc.ID)
	must.NoError(t, err)
	must.Eq(t, alloc, out)

	must.Eq(t, alloc.Resources.Devices[0], dev)
}

func TestStateStore_UpsertDeployment(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	deployment := mock.Deployment()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.DeploymentsByJobID(ws, deployment.Namespace, deployment.ID, true)
	must.NoError(t, err)

	err = state.UpsertDeployment(1000, deployment)
	must.NoError(t, err)
	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	out, err := state.DeploymentByID(ws, deployment.ID)
	must.NoError(t, err)
	must.Eq(t, deployment, out)

	index, err := state.Index("deployment")
	must.NoError(t, err)
	must.Eq(t, 1000, index)

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

// Tests that deployments of older create index and same job id are not returned
func TestStateStore_OldDeployment(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job := mock.Job()
	job.ID = "job1"
	state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)

	deploy1 := mock.Deployment()
	deploy1.JobID = job.ID
	deploy1.JobCreateIndex = job.CreateIndex

	deploy2 := mock.Deployment()
	deploy2.JobID = job.ID
	deploy2.JobCreateIndex = 11

	// Insert both deployments
	err := state.UpsertDeployment(1001, deploy1)
	must.NoError(t, err)

	err = state.UpsertDeployment(1002, deploy2)
	must.NoError(t, err)

	ws := memdb.NewWatchSet()
	// Should return both deployments
	deploys, err := state.DeploymentsByJobID(ws, deploy1.Namespace, job.ID, true)
	must.NoError(t, err)
	must.Len(t, 2, deploys)

	// Should only return deploy1
	deploys, err = state.DeploymentsByJobID(ws, deploy1.Namespace, job.ID, false)
	must.NoError(t, err)
	must.Len(t, 1, deploys)
	must.Eq(t, deploy1.ID, deploys[0].ID)
}

func TestStateStore_DeleteDeployment(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	d1 := mock.Deployment()
	d2 := mock.Deployment()

	err := state.UpsertDeployment(1000, d1)
	must.NoError(t, err)
	must.NoError(t, state.UpsertDeployment(1001, d2))

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	_, err = state.DeploymentByID(ws, d1.ID)
	must.NoError(t, err)

	err = state.DeleteDeployment(1002, []string{d1.ID, d2.ID})
	must.NoError(t, err)

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	out, err := state.DeploymentByID(ws, d1.ID)
	must.NoError(t, err)
	must.Nil(t, out, must.Sprintf("expect no deployment %#v", d1))

	index, err := state.Index("deployment")
	must.NoError(t, err)
	must.Eq(t, 1002, index)

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_Deployments(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	var deployments []*structs.Deployment

	for i := range 10 {
		deployment := mock.Deployment()
		deployments = append(deployments, deployment)

		err := state.UpsertDeployment(1000+uint64(i), deployment)
		must.NoError(t, err)
	}

	ws := memdb.NewWatchSet()
	it, err := state.Deployments(ws, SortDefault)
	must.NoError(t, err)

	var out []*structs.Deployment
	for {
		raw := it.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Deployment))
	}

	must.Eq(t, deployments, out)
	must.False(t, watchFired(ws))
}

func TestStateStore_Deployments_Namespace(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	ns1 := mock.Namespace()
	ns1.Name = "namespaced"
	deploy1 := mock.Deployment()
	deploy2 := mock.Deployment()
	deploy1.Namespace = ns1.Name
	deploy2.Namespace = ns1.Name

	ns2 := mock.Namespace()
	ns2.Name = "new-namespace"
	deploy3 := mock.Deployment()
	deploy4 := mock.Deployment()
	deploy3.Namespace = ns2.Name
	deploy4.Namespace = ns2.Name

	must.NoError(t, state.UpsertNamespaces(998, []*structs.Namespace{ns1, ns2}))

	// Create watchsets so we can test that update fires the watch
	watches := []memdb.WatchSet{memdb.NewWatchSet(), memdb.NewWatchSet()}
	_, err := state.DeploymentsByNamespace(watches[0], ns1.Name)
	must.NoError(t, err)
	_, err = state.DeploymentsByNamespace(watches[1], ns2.Name)
	must.NoError(t, err)

	must.NoError(t, state.UpsertDeployment(1001, deploy1))
	must.NoError(t, state.UpsertDeployment(1002, deploy2))
	must.NoError(t, state.UpsertDeployment(1003, deploy3))
	must.NoError(t, state.UpsertDeployment(1004, deploy4))
	must.True(t, watchFired(watches[0]))
	must.True(t, watchFired(watches[1]))

	ws := memdb.NewWatchSet()
	iter1, err := state.DeploymentsByNamespace(ws, ns1.Name)
	must.NoError(t, err)
	iter2, err := state.DeploymentsByNamespace(ws, ns2.Name)
	must.NoError(t, err)

	var out1 []*structs.Deployment
	for {
		raw := iter1.Next()
		if raw == nil {
			break
		}
		out1 = append(out1, raw.(*structs.Deployment))
	}

	var out2 []*structs.Deployment
	for {
		raw := iter2.Next()
		if raw == nil {
			break
		}
		out2 = append(out2, raw.(*structs.Deployment))
	}

	must.Len(t, 2, out1)
	must.Len(t, 2, out2)

	for _, deploy := range out1 {
		must.Eq(t, ns1.Name, deploy.Namespace)
	}
	for _, deploy := range out2 {
		must.Eq(t, ns2.Name, deploy.Namespace)
	}

	index, err := state.Index("deployment")
	must.NoError(t, err)
	must.Eq(t, 1004, index)
	must.False(t, watchFired(ws))
}

func TestStateStore_DeploymentsByIDPrefix(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	deploy := mock.Deployment()

	deploy.ID = "11111111-662e-d0ab-d1c9-3e434af7bdb4"
	err := state.UpsertDeployment(1000, deploy)
	must.NoError(t, err)

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

	t.Run("first deployment", func(t *testing.T) {
		// Create a watchset so we can test that getters don't cause it to fire
		ws := memdb.NewWatchSet()
		iter, err := state.DeploymentsByIDPrefix(ws, deploy.Namespace, deploy.ID, SortDefault)
		must.NoError(t, err)

		deploys := gatherDeploys(iter)
		must.Len(t, 1, deploys)
		must.False(t, watchFired(ws))
	})

	t.Run("using prefix", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		iter, err := state.DeploymentsByIDPrefix(ws, deploy.Namespace, "11", SortDefault)
		must.NoError(t, err)

		deploys := gatherDeploys(iter)
		must.Len(t, 1, deploys)
		must.False(t, watchFired(ws))
	})

	deploy = mock.Deployment()
	deploy.ID = "11222222-662e-d0ab-d1c9-3e434af7bdb4"
	err = state.UpsertDeployment(1001, deploy)
	must.NoError(t, err)

	t.Run("more than one", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		iter, err := state.DeploymentsByIDPrefix(ws, deploy.Namespace, "11", SortDefault)
		must.NoError(t, err)

		deploys := gatherDeploys(iter)
		must.Len(t, 2, deploys)
	})

	t.Run("filter to one", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		iter, err := state.DeploymentsByIDPrefix(ws, deploy.Namespace, "1111", SortDefault)
		must.NoError(t, err)

		deploys := gatherDeploys(iter)
		must.Len(t, 1, deploys)
		must.False(t, watchFired(ws))
	})

	t.Run("reverse order", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		iter, err := state.DeploymentsByIDPrefix(ws, deploy.Namespace, "11", SortReverse)
		must.NoError(t, err)

		got := []string{}
		for _, d := range gatherDeploys(iter) {
			got = append(got, d.ID)
		}
		expected := []string{
			"11222222-662e-d0ab-d1c9-3e434af7bdb4",
			"11111111-662e-d0ab-d1c9-3e434af7bdb4",
		}
		must.Eq(t, expected, got)
		must.False(t, watchFired(ws))
	})
}

func TestStateStore_DeploymentsByIDPrefix_Namespaces(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	deploy1 := mock.Deployment()
	deploy1.ID = "aabbbbbb-7bfb-395d-eb95-0685af2176b2"
	deploy2 := mock.Deployment()
	deploy2.ID = "aabbcbbb-7bfb-395d-eb95-0685af2176b2"
	sharedPrefix := "aabb"

	ns1 := mock.Namespace()
	ns1.Name = "namespace1"
	ns2 := mock.Namespace()
	ns2.Name = "namespace2"
	deploy1.Namespace = ns1.Name
	deploy2.Namespace = ns2.Name

	must.NoError(t, state.UpsertNamespaces(998, []*structs.Namespace{ns1, ns2}))
	must.NoError(t, state.UpsertDeployment(1000, deploy1))
	must.NoError(t, state.UpsertDeployment(1001, deploy2))

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

	ws := memdb.NewWatchSet()
	iter1, err := state.DeploymentsByIDPrefix(ws, ns1.Name, sharedPrefix, SortDefault)
	must.NoError(t, err)
	iter2, err := state.DeploymentsByIDPrefix(ws, ns2.Name, sharedPrefix, SortDefault)
	must.NoError(t, err)

	deploysNs1 := gatherDeploys(iter1)
	deploysNs2 := gatherDeploys(iter2)
	must.Len(t, 1, deploysNs1)
	must.Len(t, 1, deploysNs2)

	iter1, err = state.DeploymentsByIDPrefix(ws, ns1.Name, deploy1.ID[:8], SortDefault)
	must.NoError(t, err)

	deploysNs1 = gatherDeploys(iter1)
	must.Len(t, 1, deploysNs1)
	must.False(t, watchFired(ws))
}

func TestStateStore_UpsertNamespaces(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NamespaceByName(ws, ns1.Name)
	must.NoError(t, err)

	must.NoError(t, state.UpsertNamespaces(1000, []*structs.Namespace{ns1, ns2}))
	must.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NamespaceByName(ws, ns1.Name)
	must.NoError(t, err)
	must.Eq(t, ns1, out)

	out, err = state.NamespaceByName(ws, ns2.Name)
	must.NoError(t, err)
	must.Eq(t, ns2, out)

	index, err := state.Index(TableNamespaces)
	must.NoError(t, err)
	must.Eq(t, 1000, index)
	must.False(t, watchFired(ws))
}

func TestStateStore_DeleteNamespaces(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()

	must.NoError(t, state.UpsertNamespaces(1000, []*structs.Namespace{ns1, ns2}))

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NamespaceByName(ws, ns1.Name)
	must.NoError(t, err)

	must.NoError(t, state.DeleteNamespaces(1001, []string{ns1.Name, ns2.Name}))
	must.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NamespaceByName(ws, ns1.Name)
	must.NoError(t, err)
	must.Nil(t, out)

	out, err = state.NamespaceByName(ws, ns2.Name)
	must.NoError(t, err)
	must.Nil(t, out)

	index, err := state.Index(TableNamespaces)
	must.NoError(t, err)
	must.Eq(t, 1001, index)
	must.False(t, watchFired(ws))
}

func TestStateStore_DeleteNamespaces_Default(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	ns := mock.Namespace()
	ns.Name = structs.DefaultNamespace
	must.NoError(t, state.UpsertNamespaces(1000, []*structs.Namespace{ns}))

	err := state.DeleteNamespaces(1002, []string{ns.Name})
	must.Error(t, err)
	must.ErrorContains(t, err, "can not be deleted")
}

func TestStateStore_DeleteNamespaces_NonTerminalJobs(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	ns := mock.Namespace()
	must.NoError(t, state.UpsertNamespaces(1000, []*structs.Namespace{ns}))

	job := mock.Job()
	job.Namespace = ns.Name
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, job))

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NamespaceByName(ws, ns.Name)
	must.NoError(t, err)

	err = state.DeleteNamespaces(1002, []string{ns.Name})
	must.Error(t, err)
	must.ErrorContains(t, err, "one non-terminal")
	must.False(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NamespaceByName(ws, ns.Name)
	must.NoError(t, err)
	must.NotNil(t, out)

	index, err := state.Index(TableNamespaces)
	must.NoError(t, err)
	must.Eq(t, 1000, index)
	must.False(t, watchFired(ws))
}

func TestStateStore_DeleteNamespaces_CSIVolumes(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	ns := mock.Namespace()
	must.NoError(t, state.UpsertNamespaces(1000, []*structs.Namespace{ns}))

	plugin := mock.CSIPlugin()
	vol := mock.CSIVolume(plugin)
	vol.Namespace = ns.Name

	must.NoError(t, state.UpsertCSIVolume(1001, []*structs.CSIVolume{vol}))

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NamespaceByName(ws, ns.Name)
	must.NoError(t, err)

	err = state.DeleteNamespaces(1002, []string{ns.Name})
	must.Error(t, err)
	must.ErrorContains(t, err, "one CSI volume")
	must.False(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NamespaceByName(ws, ns.Name)
	must.NoError(t, err)
	must.NotNil(t, out)

	index, err := state.Index(TableNamespaces)
	must.NoError(t, err)
	must.Eq(t, 1000, index)
	must.False(t, watchFired(ws))
}

func TestStateStore_DeleteNamespaces_Variables(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	ns := mock.Namespace()
	must.NoError(t, state.UpsertNamespaces(1000, []*structs.Namespace{ns}))

	sv := mock.VariableEncrypted()
	sv.Namespace = ns.Name

	resp := state.VarSet(1001, &structs.VarApplyStateRequest{
		Op:  structs.VarOpSet,
		Var: sv,
	})
	must.NoError(t, resp.Error)

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NamespaceByName(ws, ns.Name)
	must.NoError(t, err)

	err = state.DeleteNamespaces(1002, []string{ns.Name})
	must.Error(t, err)
	must.ErrorContains(t, err, "one variable")
	must.False(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NamespaceByName(ws, ns.Name)
	must.NoError(t, err)
	must.NotNil(t, out)

	index, err := state.Index(TableNamespaces)
	must.NoError(t, err)
	must.Eq(t, 1000, index)
	must.False(t, watchFired(ws))
}

func TestStateStore_Namespaces(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	var namespaces []*structs.Namespace

	for range 10 {
		ns := mock.Namespace()
		namespaces = append(namespaces, ns)
	}

	must.NoError(t, state.UpsertNamespaces(1000, namespaces))

	// Create a watchset so we can test that getters don't cause it to fire
	ws := memdb.NewWatchSet()
	iter, err := state.Namespaces(ws)
	must.NoError(t, err)

	var out []*structs.Namespace
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		ns := raw.(*structs.Namespace)
		if ns.Name == structs.DefaultNamespace {
			continue
		}
		out = append(out, ns)
	}

	namespaceSort(namespaces)
	namespaceSort(out)
	must.Eq(t, namespaces, out)
	must.False(t, watchFired(ws))
}

func TestStateStore_NamespaceNames(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	var namespaces []*structs.Namespace
	expectedNames := []string{structs.DefaultNamespace}

	for range 10 {
		ns := mock.Namespace()
		namespaces = append(namespaces, ns)
		expectedNames = append(expectedNames, ns.Name)
	}

	err := state.UpsertNamespaces(1000, namespaces)
	must.NoError(t, err)

	found, err := state.NamespaceNames()
	must.NoError(t, err)

	sort.Strings(expectedNames)
	sort.Strings(found)

	must.Eq(t, expectedNames, found)
}

func TestStateStore_NamespaceByNamePrefix(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	ns := mock.Namespace()

	ns.Name = "foobar"
	must.NoError(t, state.UpsertNamespaces(1000, []*structs.Namespace{ns}))

	// Create a watchset so we can test that getters don't cause it to fire
	ws := memdb.NewWatchSet()
	iter, err := state.NamespacesByNamePrefix(ws, ns.Name)
	must.NoError(t, err)

	gatherNamespaces := func(iter memdb.ResultIterator) []*structs.Namespace {
		var namespaces []*structs.Namespace
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			ns := raw.(*structs.Namespace)
			namespaces = append(namespaces, ns)
		}
		return namespaces
	}

	namespaces := gatherNamespaces(iter)
	must.Len(t, 1, namespaces)
	must.False(t, watchFired(ws))

	iter, err = state.NamespacesByNamePrefix(ws, "foo")
	must.NoError(t, err)

	namespaces = gatherNamespaces(iter)
	must.Len(t, 1, namespaces)

	ns = mock.Namespace()
	ns.Name = "foozip"
	err = state.UpsertNamespaces(1001, []*structs.Namespace{ns})
	must.NoError(t, err)
	must.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	iter, err = state.NamespacesByNamePrefix(ws, "foo")
	must.NoError(t, err)

	namespaces = gatherNamespaces(iter)
	must.Len(t, 2, namespaces)

	iter, err = state.NamespacesByNamePrefix(ws, "foob")
	must.NoError(t, err)

	namespaces = gatherNamespaces(iter)
	must.Len(t, 1, namespaces)
	must.False(t, watchFired(ws))
}

func TestStateStore_RestoreNamespace(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	ns := mock.Namespace()

	restore, err := state.Restore()
	must.NoError(t, err)

	must.NoError(t, restore.NamespaceRestore(ns))
	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.NamespaceByName(ws, ns.Name)
	must.NoError(t, err)
	must.Eq(t, out, ns)
}

// namespaceSort is used to sort namespaces by name
func namespaceSort(namespaces []*structs.Namespace) {
	sort.Slice(namespaces, func(i, j int) bool {
		return namespaces[i].Name < namespaces[j].Name
	})
}

func TestStateStore_UpsertNode_Node(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	node := mock.Node()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NodeByID(ws, node.ID)
	must.NoError(t, err)

	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1000, node))
	must.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	must.NoError(t, err)

	out2, err := state.NodeBySecretID(ws, node.SecretID)
	must.NoError(t, err)
	must.Eq(t, node, out)
	must.Eq(t, node, out2)
	must.Len(t, 1, out.Events)
	must.Eq(t, NodeRegisterEventRegistered, out.Events[0].Message)

	index, err := state.Index("nodes")
	must.NoError(t, err)
	must.Eq(t, 1000, index)
	must.False(t, watchFired(ws))

	// Transition the node to down and then up and ensure we get a re-register
	// event
	down := out.Copy()
	down.Status = structs.NodeStatusDown
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1001, down))
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1002, out))

	out, err = state.NodeByID(ws, node.ID)
	must.NoError(t, err)
	must.Len(t, 2, out.Events)
	must.Eq(t, NodeRegisterEventReregistered, out.Events[1].Message)
}

func TestStateStore_UpsertNode_NodePool(t *testing.T) {
	ci.Parallel(t)

	devPoolName := "dev"
	nodeWithPoolID := uuid.Generate()
	nodeWithoutPoolID := uuid.Generate()

	testCases := []struct {
		name                        string
		nodeID                      string
		pool                        string
		createPool                  bool
		expectedPool                string
		expectedPoolNodeIdentityTTL time.Duration
		expectedPoolExists          bool
		validateFn                  func(*testing.T, *structs.Node, *structs.NodePool)
	}{
		{
			name:               "register new node in new node pool",
			nodeID:             "",
			pool:               "new",
			createPool:         true,
			expectedPool:       "new",
			expectedPoolExists: true,
			validateFn: func(t *testing.T, node *structs.Node, pool *structs.NodePool) {
				// Verify node pool was created in the same transaction as the
				// node registration.
				must.Eq(t, pool.CreateIndex, node.ModifyIndex)
			},
		},
		{
			name:                        "register new node in existing node pool",
			nodeID:                      "",
			pool:                        devPoolName,
			expectedPool:                devPoolName,
			expectedPoolNodeIdentityTTL: 720 * time.Hour,
			expectedPoolExists:          true,
			validateFn: func(t *testing.T, node *structs.Node, pool *structs.NodePool) {
				// Verify node pool was not modified.
				must.NotEq(t, pool.CreateIndex, node.ModifyIndex)
			},
		},
		{
			name:               "register new node in built-in node pool",
			nodeID:             "",
			pool:               structs.NodePoolDefault,
			expectedPool:       structs.NodePoolDefault,
			expectedPoolExists: true,
			validateFn: func(t *testing.T, node *structs.Node, pool *structs.NodePool) {
				// Verify node pool was not modified.
				must.Eq(t, 1, pool.ModifyIndex)
			},
		},
		{
			name:               "move existing node to new node pool",
			nodeID:             nodeWithPoolID,
			pool:               "new",
			createPool:         true,
			expectedPool:       "new",
			expectedPoolExists: true,
			validateFn: func(t *testing.T, node *structs.Node, pool *structs.NodePool) {
				// Verify node pool was created in the same transaction as the
				// node was updated.
				must.Eq(t, pool.CreateIndex, node.ModifyIndex)
			},
		},
		{
			name:                        "move existing node to existing node pool",
			nodeID:                      nodeWithPoolID,
			pool:                        devPoolName,
			expectedPool:                devPoolName,
			expectedPoolNodeIdentityTTL: 720 * time.Hour,
			expectedPoolExists:          true,
		},
		{
			name:               "move existing node to built-in node pool",
			nodeID:             nodeWithPoolID,
			pool:               structs.NodePoolDefault,
			expectedPool:       structs.NodePoolDefault,
			expectedPoolExists: true,
		},
		{
			name:               "update node without pool to new node pool",
			nodeID:             nodeWithoutPoolID,
			pool:               "new",
			createPool:         true,
			expectedPool:       "new",
			expectedPoolExists: true,
		},
		{
			name:                        "update node without pool to existing node pool",
			nodeID:                      nodeWithoutPoolID,
			pool:                        devPoolName,
			expectedPool:                devPoolName,
			expectedPoolNodeIdentityTTL: 720 * time.Hour,
			expectedPoolExists:          true,
		},
		{
			name:               "update node without pool with empty string to default",
			nodeID:             nodeWithoutPoolID,
			pool:               "",
			expectedPool:       structs.NodePoolDefault,
			expectedPoolExists: true,
		},
		{
			name:               "register new node in new node pool without creating it",
			nodeID:             "",
			pool:               "new",
			createPool:         false,
			expectedPool:       "new",
			expectedPoolExists: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := testStateStore(t)

			// Populate state with pre-existing node pool.
			devPool := mock.NodePool()
			devPool.Name = devPoolName
			err := state.UpsertNodePools(structs.MsgTypeTestSetup, 1000, []*structs.NodePool{devPool})
			must.NoError(t, err)

			// Populate state with pre-existing node assigned to the
			// pre-existing node pool.
			nodeWithPool := mock.Node()
			nodeWithPool.ID = nodeWithPoolID
			nodeWithPool.NodePool = devPool.Name
			err = state.UpsertNode(structs.MsgTypeTestSetup, 1001, nodeWithPool)
			must.NoError(t, err)

			// Populate state with pre-existing node with nil node pool to
			// simulate an upgrade path.
			nodeWithoutPool := mock.Node()
			nodeWithoutPool.ID = nodeWithoutPoolID
			err = state.UpsertNode(structs.MsgTypeTestSetup, 1002, nodeWithoutPool)
			must.NoError(t, err)

			// Upsert test node.
			var node *structs.Node
			switch tc.nodeID {
			case nodeWithPoolID:
				node = nodeWithPool.Copy()
			case nodeWithoutPoolID:
				node = nodeWithoutPool.Copy()
			default:
				node = mock.Node()
			}

			node.NodePool = tc.pool
			opts := []NodeUpsertOption{}
			if tc.createPool {
				opts = append(opts, NodeUpsertWithNodePool)
			}
			err = state.UpsertNode(structs.MsgTypeTestSetup, 1003, node, opts...)
			must.NoError(t, err)

			// Verify that node is part of the expected pool.
			got, err := state.NodeByID(nil, node.ID)
			must.NoError(t, err)
			must.NotNil(t, got)

			// Verify node pool exists if requests.
			pool, err := state.NodePoolByName(nil, tc.expectedPool)
			must.NoError(t, err)
			if tc.expectedPoolExists {
				must.NotNil(t, pool)

				// Ensure the pool identitiy TTL is correctly set depending on
				// whether a custom value was expected, or whether the default
				// should be applied.
				if tc.expectedPoolNodeIdentityTTL == 0 {
					must.Eq(t, structs.DefaultNodePoolNodeIdentityTTL, pool.NodeIdentityTTL)
				} else {
					must.Eq(t, tc.expectedPoolNodeIdentityTTL, pool.NodeIdentityTTL)
				}
			} else {
				must.Nil(t, pool)
			}

			// Verify node was assigned to node pool.
			must.Eq(t, tc.expectedPool, got.NodePool)

			if tc.validateFn != nil {
				tc.validateFn(t, got, pool)
			}
		})
	}
}

func TestStateStore_DeleteNode_Node(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	// Create and insert two nodes, which we'll delete
	node0 := mock.Node()
	node1 := mock.Node()
	err := state.UpsertNode(structs.MsgTypeTestSetup, 1000, node0)
	must.NoError(t, err)
	err = state.UpsertNode(structs.MsgTypeTestSetup, 1001, node1)
	must.NoError(t, err)

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()

	// Check that both nodes are not nil
	out, err := state.NodeByID(ws, node0.ID)
	must.NoError(t, err)
	must.NotNil(t, out)
	out, err = state.NodeByID(ws, node1.ID)
	must.NoError(t, err)
	must.NotNil(t, out)

	// Delete both nodes in a batch, fires the watch
	err = state.DeleteNode(structs.MsgTypeTestSetup, 1002, []string{node0.ID, node1.ID})
	must.NoError(t, err)
	must.True(t, watchFired(ws))

	// Check that both nodes are nil
	ws = memdb.NewWatchSet()
	out, err = state.NodeByID(ws, node0.ID)
	must.NoError(t, err)
	must.Nil(t, out)
	out, err = state.NodeByID(ws, node1.ID)
	must.NoError(t, err)
	must.Nil(t, out)

	// Ensure that the index is still at 1002, from DeleteNode
	index, err := state.Index("nodes")
	must.NoError(t, err)
	must.Eq(t, uint64(1002), index)
	must.False(t, watchFired(ws))
}

func TestStateStore_UpdateNodeStatus_Node(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	node := mock.Node()

	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 800, node))

	// Create a watchset so we can test that update node status fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NodeByID(ws, node.ID)
	must.NoError(t, err)

	event := &structs.NodeEvent{
		Message:   "Node ready foo",
		Subsystem: structs.NodeEventSubsystemCluster,
		Timestamp: time.Now(),
	}

	signingKeyID := uuid.Generate()

	stateReq := structs.NodeUpdateStatusRequest{
		NodeID:               node.ID,
		Status:               structs.NodeStatusReady,
		IdentitySigningKeyID: signingKeyID,
		NodeEvent:            event,
		UpdatedAt:            70,
	}

	must.NoError(t, state.UpdateNodeStatus(structs.MsgTypeTestSetup, 801, &stateReq))
	must.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	must.NoError(t, err)
	must.Eq(t, structs.NodeStatusReady, out.Status)
	must.Eq(t, 801, out.ModifyIndex)
	must.Eq(t, 70, out.StatusUpdatedAt)
	must.Len(t, 2, out.Events)
	must.Eq(t, event.Message, out.Events[1].Message)
	must.Eq(t, signingKeyID, out.IdentitySigningKeyID)

	index, err := state.Index(TableNodes)
	must.NoError(t, err)
	must.Eq(t, 801, index)
	must.False(t, watchFired(ws))

	// Send another update, but the signing key ID is empty, this should not
	// overwrite the existing signing key ID.
	stateReq = structs.NodeUpdateStatusRequest{
		NodeID:               node.ID,
		Status:               structs.NodeStatusReady,
		IdentitySigningKeyID: "",
		NodeEvent: &structs.NodeEvent{
			Message:   "Node even more ready foo",
			Subsystem: structs.NodeEventSubsystemCluster,
			Timestamp: time.Now(),
		},
		UpdatedAt: 80,
	}

	must.NoError(t, state.UpdateNodeStatus(structs.MsgTypeTestSetup, 802, &stateReq))
	out, err = state.NodeByID(ws, node.ID)
	must.NoError(t, err)
	must.Eq(t, signingKeyID, out.IdentitySigningKeyID)
}

func TestStatStore_UpdateNodeStatus_LastMissedHeartbeatIndex(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name            string
		transitions     []string
		expectedIndexes []uint64
	}{
		{
			name: "disconnect",
			transitions: []string{
				structs.NodeStatusReady,
				structs.NodeStatusDisconnected,
			},
			expectedIndexes: []uint64{0, 1001},
		},
		{
			name: "reconnect",
			transitions: []string{
				structs.NodeStatusReady,
				structs.NodeStatusDisconnected,
				structs.NodeStatusInit,
				structs.NodeStatusReady,
			},
			expectedIndexes: []uint64{0, 1001, 1001, 0},
		},
		{
			name: "down",
			transitions: []string{
				structs.NodeStatusReady,
				structs.NodeStatusDown,
			},
			expectedIndexes: []uint64{0, 1001},
		},
		{
			name: "multiple reconnects",
			transitions: []string{
				structs.NodeStatusReady,
				structs.NodeStatusDisconnected,
				structs.NodeStatusInit,
				structs.NodeStatusReady,
				structs.NodeStatusDown,
				structs.NodeStatusReady,
				structs.NodeStatusDisconnected,
				structs.NodeStatusInit,
				structs.NodeStatusReady,
			},
			expectedIndexes: []uint64{0, 1001, 1001, 0, 1004, 0, 1006, 1006, 0},
		},
		{
			name: "multiple heartbeats",
			transitions: []string{
				structs.NodeStatusReady,
				structs.NodeStatusDisconnected,
				structs.NodeStatusInit,
				structs.NodeStatusReady,
				structs.NodeStatusReady,
				structs.NodeStatusReady,
			},
			expectedIndexes: []uint64{0, 1001, 1001, 0, 0, 0},
		},
		{
			name: "delayed alloc update",
			transitions: []string{
				structs.NodeStatusReady,
				structs.NodeStatusDisconnected,
				structs.NodeStatusInit,
				structs.NodeStatusInit,
				structs.NodeStatusInit,
				structs.NodeStatusReady,
			},
			expectedIndexes: []uint64{0, 1001, 1001, 1001, 1001, 0},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := testStateStore(t)
			node := mock.Node()
			must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 999, node))

			for i, status := range tc.transitions {
				now := time.Now().UnixNano()
				req := structs.NodeUpdateStatusRequest{
					NodeID:    node.ID,
					Status:    status,
					UpdatedAt: now,
				}
				err := state.UpdateNodeStatus(structs.MsgTypeTestSetup, uint64(1000+i), &req)
				must.NoError(t, err)

				ws := memdb.NewWatchSet()
				out, err := state.NodeByID(ws, node.ID)
				must.NoError(t, err)
				must.Eq(t, tc.expectedIndexes[i], out.LastMissedHeartbeatIndex)
				must.Eq(t, status, out.Status)
			}
		})
	}
}

func TestStateStore_BatchUpdateNodeDrain(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	n1, n2 := mock.Node(), mock.Node()
	must.Nil(t, state.UpsertNode(structs.MsgTypeTestSetup, 1000, n1))
	must.Nil(t, state.UpsertNode(structs.MsgTypeTestSetup, 1001, n2))

	// Create a watchset so we can test that update node drain fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NodeByID(ws, n1.ID)
	must.NoError(t, err)

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

	must.Nil(t, state.BatchUpdateNodeDrain(structs.MsgTypeTestSetup, 1002, 7, update, events))
	must.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	for _, id := range []string{n1.ID, n2.ID} {
		out, err := state.NodeByID(ws, id)
		must.NoError(t, err)
		must.NotNil(t, out.DrainStrategy)
		must.Eq(t, out.DrainStrategy, expectedDrain)
		must.NotNil(t, out.LastDrain)
		must.Eq(t, structs.DrainStatusDraining, out.LastDrain.Status)
		must.Len(t, 2, out.Events)
		must.Eq(t, 1002, out.ModifyIndex)
		must.Eq(t, 7, out.StatusUpdatedAt)
	}

	index, err := state.Index("nodes")
	must.NoError(t, err)
	must.Eq(t, 1002, index)
	must.False(t, watchFired(ws))
}

func TestStateStore_UpdateNodeDrain_Node(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	node := mock.Node()

	must.Nil(t, state.UpsertNode(structs.MsgTypeTestSetup, 1000, node))

	// Create a watchset so we can test that update node drain fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NodeByID(ws, node.ID)
	must.NoError(t, err)

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
	must.Nil(t, state.UpdateNodeDrain(structs.MsgTypeTestSetup, 1001, node.ID, expectedDrain, false, 7, event, nil, ""))
	must.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	must.NoError(t, err)
	must.NotNil(t, out.DrainStrategy)
	must.NotNil(t, out.LastDrain)
	must.Eq(t, structs.DrainStatusDraining, out.LastDrain.Status)
	must.Eq(t, out.DrainStrategy, expectedDrain)
	must.Len(t, 2, out.Events)
	must.Eq(t, 1001, out.ModifyIndex)
	must.Eq(t, 7, out.StatusUpdatedAt)

	index, err := state.Index("nodes")
	must.NoError(t, err)
	must.Eq(t, 1001, index)
	must.False(t, watchFired(ws))
}

func TestStateStore_AddSingleNodeEvent(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	node := mock.Node()

	// We create a new node event every time we register a node
	err := state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	must.NoError(t, err)

	must.Eq(t, 1, len(node.Events))
	must.Eq(t, structs.NodeEventSubsystemCluster, node.Events[0].Subsystem)
	must.Eq(t, NodeRegisterEventRegistered, node.Events[0].Message)

	// Create a watchset so we can test that AddNodeEvent fires the watch
	ws := memdb.NewWatchSet()
	_, err = state.NodeByID(ws, node.ID)
	must.NoError(t, err)

	nodeEvent := &structs.NodeEvent{
		Message:   "failed",
		Subsystem: "Driver",
		Timestamp: time.Now(),
	}
	nodeEvents := map[string][]*structs.NodeEvent{
		node.ID: {nodeEvent},
	}
	err = state.UpsertNodeEvents(structs.MsgTypeTestSetup, uint64(1001), nodeEvents)
	must.NoError(t, err)

	must.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	must.NoError(t, err)

	must.Eq(t, 2, len(out.Events))
	must.Eq(t, nodeEvent, out.Events[1])
}

// To prevent stale node events from accumulating, we limit the number of
// stored node events to 10.
func TestStateStore_NodeEvents_RetentionWindow(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	node := mock.Node()

	err := state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	must.NoError(t, err)
	must.Eq(t, 1, len(node.Events))
	must.Eq(t, structs.NodeEventSubsystemCluster, node.Events[0].Subsystem)
	must.Eq(t, NodeRegisterEventRegistered, node.Events[0].Message)

	var out *structs.Node
	for i := 1; i <= 20; i++ {
		ws := memdb.NewWatchSet()
		out, err = state.NodeByID(ws, node.ID)
		must.NoError(t, err)

		nodeEvent := &structs.NodeEvent{
			Message:   fmt.Sprintf("%dith failed", i),
			Subsystem: "Driver",
			Timestamp: time.Now(),
		}

		nodeEvents := map[string][]*structs.NodeEvent{
			out.ID: {nodeEvent},
		}
		err := state.UpsertNodeEvents(structs.MsgTypeTestSetup, uint64(i), nodeEvents)
		must.NoError(t, err)

		must.True(t, watchFired(ws))
		ws = memdb.NewWatchSet()
		out, err = state.NodeByID(ws, node.ID)
		must.NoError(t, err)
	}

	ws := memdb.NewWatchSet()
	out, err = state.NodeByID(ws, node.ID)
	must.NoError(t, err)

	must.Eq(t, 10, len(out.Events))
	must.Eq(t, uint64(11), out.Events[0].CreateIndex)
	must.Eq(t, uint64(20), out.Events[len(out.Events)-1].CreateIndex)
}

func TestStateStore_UpdateNodeDrain_ResetEligiblity(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	node := mock.Node()
	must.Nil(t, state.UpsertNode(structs.MsgTypeTestSetup, 1000, node))

	// Create a watchset so we can test that update node drain fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NodeByID(ws, node.ID)
	must.NoError(t, err)

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
	must.Nil(t, state.UpdateNodeDrain(structs.MsgTypeTestSetup, 1001, node.ID, drain, false, 7, event1, nil, ""))
	must.True(t, watchFired(ws))

	// Remove the drain
	event2 := &structs.NodeEvent{
		Message:   "Drain strategy disabled",
		Subsystem: structs.NodeEventSubsystemDrain,
		Timestamp: time.Now(),
	}
	must.Nil(t, state.UpdateNodeDrain(structs.MsgTypeTestSetup, 1002, node.ID, nil, true, 9, event2, nil, ""))

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	must.NoError(t, err)
	must.Nil(t, out.DrainStrategy)
	must.Eq(t, out.SchedulingEligibility, structs.NodeSchedulingEligible)
	must.NotNil(t, out.LastDrain)
	must.Eq(t, structs.DrainStatusCanceled, out.LastDrain.Status)
	must.Eq(t, time.Unix(7, 0), out.LastDrain.StartedAt)
	must.Eq(t, time.Unix(9, 0), out.LastDrain.UpdatedAt)
	must.Len(t, 3, out.Events)
	must.Eq(t, 1002, out.ModifyIndex)
	must.Eq(t, 9, out.StatusUpdatedAt)

	index, err := state.Index("nodes")
	must.NoError(t, err)
	must.Eq(t, 1002, index)
	must.False(t, watchFired(ws))
}

func TestStateStore_UpdateNodeEligibility(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	node := mock.Node()

	err := state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	must.NoError(t, err)

	expectedEligibility := structs.NodeSchedulingIneligible

	// Create a watchset so we can test that update node drain fires the watch
	ws := memdb.NewWatchSet()
	_, err = state.NodeByID(ws, node.ID)
	must.NoError(t, err)

	event := &structs.NodeEvent{
		Message:   "Node marked as ineligible",
		Subsystem: structs.NodeEventSubsystemCluster,
		Timestamp: time.Now(),
	}
	must.Nil(t, state.UpdateNodeEligibility(structs.MsgTypeTestSetup, 1001, node.ID, expectedEligibility, 7, event))
	must.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	must.NoError(t, err)
	must.Eq(t, out.SchedulingEligibility, expectedEligibility)
	must.Len(t, 2, out.Events)
	must.Eq(t, out.Events[1], event)
	must.Eq(t, 1001, out.ModifyIndex)
	must.Eq(t, 7, out.StatusUpdatedAt)

	index, err := state.Index("nodes")
	must.NoError(t, err)
	must.Eq(t, 1001, index)
	must.False(t, watchFired(ws))

	// Set a drain strategy
	expectedDrain := &structs.DrainStrategy{
		DrainSpec: structs.DrainSpec{
			Deadline: -1 * time.Second,
		},
	}
	must.Nil(t, state.UpdateNodeDrain(structs.MsgTypeTestSetup, 1002, node.ID, expectedDrain, false, 7, nil, nil, ""))

	// Try to set the node to eligible
	err = state.UpdateNodeEligibility(structs.MsgTypeTestSetup, 1003, node.ID, structs.NodeSchedulingEligible, 9, nil)
	must.Error(t, err)
	must.ErrorContains(t, err, "while it is draining")
}

func TestStateStore_Nodes(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	var nodes []*structs.Node

	for i := range 10 {
		node := mock.Node()
		nodes = append(nodes, node)

		err := state.UpsertNode(structs.MsgTypeTestSetup, 1000+uint64(i), node)
		must.NoError(t, err)
	}

	// Create a watchset so we can test that getters don't cause it to fire
	ws := memdb.NewWatchSet()
	iter, err := state.Nodes(ws)
	must.NoError(t, err)

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

	must.Eq(t, nodes, out)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_NodesByIDPrefix(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	node := mock.Node()

	node.ID = "11111111-662e-d0ab-d1c9-3e434af7bdb4"
	err := state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	must.NoError(t, err)

	// Create a watchset so we can test that getters don't cause it to fire
	ws := memdb.NewWatchSet()
	iter, err := state.NodesByIDPrefix(ws, node.ID)
	must.NoError(t, err)

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
	must.Len(t, 1, nodes)

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))

	iter, err = state.NodesByIDPrefix(ws, "11")
	must.NoError(t, err)

	nodes = gatherNodes(iter)
	must.Len(t, 1, nodes)

	node = mock.Node()
	node.ID = "11222222-662e-d0ab-d1c9-3e434af7bdb4"
	err = state.UpsertNode(structs.MsgTypeTestSetup, 1001, node)
	must.NoError(t, err)

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	iter, err = state.NodesByIDPrefix(ws, "11")
	must.NoError(t, err)

	nodes = gatherNodes(iter)
	must.Len(t, 2, nodes)

	iter, err = state.NodesByIDPrefix(ws, "1111")
	must.NoError(t, err)

	nodes = gatherNodes(iter)
	must.Len(t, 1, nodes)

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_NodesByNodePool(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	pool := mock.NodePool()
	err := state.UpsertNodePools(structs.MsgTypeTestSetup, 1000, []*structs.NodePool{pool})
	must.NoError(t, err)

	node1 := mock.Node()
	node1.NodePool = structs.NodePoolDefault
	err = state.UpsertNode(structs.MsgTypeTestSetup, 1001, node1)
	must.NoError(t, err)

	node2 := mock.Node()
	node2.NodePool = pool.Name
	err = state.UpsertNode(structs.MsgTypeTestSetup, 1002, node2)
	must.NoError(t, err)

	testCases := []struct {
		name     string
		pool     string
		expected []string
	}{
		{
			name: "default",
			pool: structs.NodePoolDefault,
			expected: []string{
				node1.ID,
			},
		},
		{
			name: "pool",
			pool: pool.Name,
			expected: []string{
				node2.ID,
			},
		},
		{
			name:     "empty pool",
			pool:     "",
			expected: []string{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create watcher to test that getters don't cause it to fire.
			ws := memdb.NewWatchSet()

			iter, err := state.NodesByNodePool(ws, tc.pool)
			must.NoError(t, err)

			got := []string{}
			for raw := iter.Next(); raw != nil; raw = iter.Next() {
				got = append(got, raw.(*structs.Node).ID)
			}

			must.SliceContainsAll(t, tc.expected, got)
			must.False(t, watchFired(ws))
		})
	}
}

func TestStateStore_UpsertJob_Job(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job := mock.Job()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))
	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Eq(t, job, out)

	index, err := state.Index("jobs")
	must.NoError(t, err)
	must.Eq(t, 1000, index)

	summary, err := state.JobSummaryByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.NotNil(t, summary)
	must.Eq(t, job.ID, summary.JobID, must.Sprint("bad summary id"))
	_, ok := summary.Summary["web"]
	must.True(t, ok, must.Sprint("nil summary for task group"))
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))

	// Check the job versions
	allVersions, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Len(t, 1, allVersions)

	if a := allVersions[0]; a.ID != job.ID || a.Version != 0 {
		t.Fatalf("bad: %v", a)
	}

	// Test the looking up the job by version returns the same results
	vout, err := state.JobByIDAndVersion(ws, job.Namespace, job.ID, 0)
	must.NoError(t, err)
	must.Eq(t, out, vout)
}

func TestStateStore_UpdateUpsertJob_Job(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job := mock.Job()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))

	job2 := mock.Job()
	job2.ID = job.ID
	job2.AllAtOnce = true
	err = state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, job2)
	must.NoError(t, err)

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Eq(t, job2, out)
	must.Eq(t, 1000, out.CreateIndex)
	must.Eq(t, 1001, out.ModifyIndex)
	must.Eq(t, 1, out.Version)

	index, err := state.Index("jobs")
	must.NoError(t, err)
	must.Eq(t, 1001, index)

	// Test the looking up the job by version returns the same results
	vout, err := state.JobByIDAndVersion(ws, job.Namespace, job.ID, 1)
	must.NoError(t, err)
	must.Eq(t, out, vout)

	// Test that the job summary remains the same if the job is updated but
	// count remains same
	summary, err := state.JobSummaryByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.NotNil(t, summary)
	if summary.JobID != job.ID {
		t.Fatalf("bad summary id: %v", summary.JobID)
	}
	_, ok := summary.Summary["web"]
	if !ok {
		t.Fatalf("nil summary for task group")
	}

	// Check the job versions
	allVersions, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	if len(allVersions) != 2 {
		t.Fatalf("got %d; want 1", len(allVersions))
	}

	if a := allVersions[0]; a.ID != job.ID || a.Version != 1 || !a.AllAtOnce {
		t.Fatalf("bad: %+v", a)
	}
	if a := allVersions[1]; a.ID != job.ID || a.Version != 0 || a.AllAtOnce {
		t.Fatalf("bad: %+v", a)
	}

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_UpdateUpsertJob_PeriodicJob(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job := mock.PeriodicJob()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))

	// Create a child and an evaluation
	job2 := job.Copy()
	job2.Periodic = nil
	job2.ID = fmt.Sprintf("%v/%s-1490635020", job.ID, structs.PeriodicLaunchSuffix)
	err = state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, job2)
	must.NoError(t, err)

	eval := mock.Eval()
	eval.JobID = job2.ID
	err = state.UpsertEvals(structs.MsgTypeTestSetup, 1002, []*structs.Evaluation{eval})
	must.NoError(t, err)

	job3 := job.Copy()
	job3.TaskGroups[0].Tasks[0].Name = "new name"
	err = state.UpsertJob(structs.MsgTypeTestSetup, 1003, nil, job3)
	must.NoError(t, err)

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)

	if s, e := out.Status, structs.JobStatusRunning; s != e {
		t.Fatalf("got status %v; want %v", s, e)
	}

}

func TestStateStore_UpsertJob_BadNamespace(t *testing.T) {
	ci.Parallel(t)
	state := testStateStore(t)
	job := mock.Job()
	job.Namespace = "foo"

	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	must.ErrorContains(t, err, "nonexistent namespace")

	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	test.Nil(t, out)
}

func TestStateStore_UpsertJob_NodePool(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	testCases := []struct {
		name         string
		pool         string
		expectedPool string
		expectedErr  string
	}{
		{
			name:         "empty node pool uses default",
			pool:         "",
			expectedPool: structs.NodePoolDefault,
		},
		{
			name:         "job uses pool defined",
			pool:         structs.NodePoolDefault,
			expectedPool: structs.NodePoolDefault,
		},
		{
			name:        "error when pool doesn't exist",
			pool:        "nonexisting",
			expectedErr: "nonexistent node pool",
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			job := mock.Job()
			job.NodePool = tc.pool

			err := state.UpsertJob(structs.MsgTypeTestSetup, uint64(1000+i), nil, job)
			if tc.expectedErr != "" {
				must.ErrorContains(t, err, tc.expectedErr)
			} else {
				must.NoError(t, err)

				ws := memdb.NewWatchSet()
				got, err := state.JobByID(ws, job.Namespace, job.ID)
				must.NoError(t, err)
				must.Eq(t, tc.expectedPool, got.NodePool)
			}
		})
	}
}

// Upsert a job that is the child of a parent job and ensures its summary gets
// updated.
func TestStateStore_UpsertJob_ChildJob(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	// Create a watchset so we can test that upsert fires the watch
	parent := mock.Job()
	ws := memdb.NewWatchSet()
	_, err := state.JobByID(ws, parent.Namespace, parent.ID)
	must.NoError(t, err)

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, parent))

	child := mock.Job()
	child.Status = ""
	child.ParentID = parent.ID
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, child))

	summary, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	must.NoError(t, err)
	must.NotNil(t, summary)
	must.Eq(t, parent.ID, summary.JobID)
	must.NotNil(t, summary.Children)
	must.Eq(t, &structs.JobChildrenSummary{Pending: 1}, summary.Children)
	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))
}

func TestStateStore_UpsertJob_submission(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	job := mock.Job()
	job.Meta = map[string]string{"version": "1"}
	submission := &structs.JobSubmission{
		Source:  "source",
		Version: 0,
	}

	index := uint64(1000)

	// initially non-existent
	sub, err := state.JobSubmission(nil, job.Namespace, job.ID, 0)
	must.NoError(t, err)
	must.Nil(t, sub)

	// insert first one, version 0, index 1001
	index++
	err = state.UpsertJob(structs.JobRegisterRequestType, index, submission, job)
	must.NoError(t, err)

	// query first one, version 0
	sub, err = state.JobSubmission(nil, job.Namespace, job.ID, 0)
	must.NoError(t, err)
	must.NotNil(t, sub)
	must.Eq(t, 0, sub.Version)
	must.Eq(t, index, sub.JobModifyIndex)

	// insert 6 more, going over the limit
	for i := 1; i <= structs.JobDefaultTrackedVersions; i++ {
		index++
		job2 := job.Copy()
		job2.Meta["version"] = strconv.Itoa(i)
		sub2 := &structs.JobSubmission{
			Source:  "source",
			Version: uint64(i),
		}
		err = state.UpsertJob(structs.JobRegisterRequestType, index, sub2, job2)
		must.NoError(t, err)
	}

	// the version 0 submission is now dropped
	sub, err = state.JobSubmission(nil, job.Namespace, job.ID, 0)
	must.NoError(t, err)
	must.Nil(t, sub)

	// but we do have version 1
	sub, err = state.JobSubmission(nil, job.Namespace, job.ID, 1)
	must.NoError(t, err)
	must.NotNil(t, sub)
	must.Eq(t, 1, sub.Version)
	must.Eq(t, 1002, sub.JobModifyIndex)

	// and up to version 6
	sub, err = state.JobSubmission(nil, job.Namespace, job.ID, 6)
	must.NoError(t, err)
	must.NotNil(t, sub)
	must.Eq(t, 6, sub.Version)
	must.Eq(t, 1007, sub.JobModifyIndex)
}

func TestStateStore_GetJobSubmissions(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	// Generate some job submissions and upsert these into state.
	mockJobSubmissions := []*structs.JobSubmission{
		{
			Source:         "job{}",
			Namespace:      "default",
			JobID:          "example",
			Version:        10,
			JobModifyIndex: 20,
		},
		{
			Source:         "job{}",
			Namespace:      "platform",
			JobID:          "example",
			Version:        20,
			JobModifyIndex: 20,
		},
	}

	txn := state.db.WriteTxn(20)

	for _, mockSubmission := range mockJobSubmissions {
		must.NoError(t, state.updateJobSubmission(
			20, mockSubmission, mockSubmission.Namespace, mockSubmission.JobID, mockSubmission.Version, txn))
	}

	must.NoError(t, txn.Commit())

	// List out all the job submissions in state and ensure they match the
	// items we previously wrote.
	ws := memdb.NewWatchSet()
	iter, err := state.GetJobSubmissions(ws)
	must.NoError(t, err)

	var submissions []*structs.JobSubmission

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		submissions = append(submissions, raw.(*structs.JobSubmission))
	}

	must.SliceLen(t, 2, submissions)
	must.Eq(t, mockJobSubmissions, submissions)
}

func TestStateStore_UpdateUpsertJob_JobVersion(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	// Create a job and mark it as stable
	job := mock.Job()
	job.Stable = true
	job.Name = "0"

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	var finalJob *structs.Job
	for i := 1; i < 300; i++ {
		finalJob = mock.Job()
		finalJob.ID = job.ID
		finalJob.Name = fmt.Sprintf("%d", i)
		err = state.UpsertJob(structs.MsgTypeTestSetup, uint64(1000+i), nil, finalJob)
		must.NoError(t, err)
	}

	ws = memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Eq(t, finalJob, out)

	must.Eq(t, 1000, out.CreateIndex)
	if out.ModifyIndex != 1299 {
		t.Fatalf("bad: %#v", out)
	}
	if out.Version != 299 {
		t.Fatalf("bad: %#v", out)
	}

	index, err := state.Index("job_version")
	must.NoError(t, err)
	if index != 1299 {
		t.Fatalf("bad: %d", index)
	}

	// Check the job versions
	allVersions, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	if len(allVersions) != structs.JobDefaultTrackedVersions {
		t.Fatalf("got %d; want %d", len(allVersions), structs.JobDefaultTrackedVersions)
	}

	if a := allVersions[0]; a.ID != job.ID || a.Version != 299 || a.Name != "299" {
		t.Fatalf("bad: %+v", a)
	}
	if a := allVersions[1]; a.ID != job.ID || a.Version != 298 || a.Name != "298" {
		t.Fatalf("bad: %+v", a)
	}

	// Ensure we didn't delete the stable job
	if a := allVersions[structs.JobDefaultTrackedVersions-1]; a.ID != job.ID ||
		a.Version != 0 || a.Name != "0" || !a.Stable {
		t.Fatalf("bad: %+v", a)
	}

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_UpsertJobWithRequest(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job := mock.Job()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)

	must.NoError(t, state.UpsertJobWithRequest(structs.MsgTypeTestSetup, 1000, &structs.JobRegisterRequest{Job: job}))
	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Eq(t, job, out)

	index, err := state.Index("jobs")
	must.NoError(t, err)
	must.Eq(t, 1000, index)

	summary, err := state.JobSummaryByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.NotNil(t, summary)
	must.Eq(t, job.ID, summary.JobID, must.Sprint("bad summary id"))
	_, ok := summary.Summary["web"]
	must.True(t, ok, must.Sprint("nil summary for task group"))
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))

	// Check the job versions
	allVersions, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Len(t, 1, allVersions)

	a := allVersions[0]
	must.Eq(t, a.ID, job.ID)
	must.Eq(t, a.Version, 0)

	// Test the looking up the job by version returns the same results
	vout, err := state.JobByIDAndVersion(ws, job.Namespace, job.ID, 0)
	must.NoError(t, err)
	must.Eq(t, out, vout)
}

func TestStateStore_UpsertJobWithRequest_PreserveCount(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	// Create a job
	job := mock.Job()
	job.TaskGroups[0].Count = 10
	job.TaskGroups[0].Tasks[0].Resources = &structs.Resources{
		CPU:      500,
		MemoryMB: 256,
	}

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))

	job2 := job.Copy()
	job2.TaskGroups[0].Count = 5
	job2.TaskGroups[0].Tasks[0].Resources = &structs.Resources{
		CPU:      750,
		MemoryMB: 500,
	}

	must.NoError(t, state.UpsertJobWithRequest(structs.MsgTypeTestSetup, 1001, &structs.JobRegisterRequest{PreserveCounts: true, PreserveResources: true, Job: job2}))

	out, err := state.JobByID(nil, job.Namespace, job.ID)
	must.NoError(t, err)

	must.Eq(t, 10, out.TaskGroups[0].Count)
	must.Eq(t, out.TaskGroups[0].Tasks[0].Resources.CPU, 500)
	must.Eq(t, out.TaskGroups[0].Tasks[0].Resources.MemoryMB, 256)
}

func TestStateStore_DeleteJob_Job(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job := mock.Job()

	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	must.NoError(t, err)

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	_, err = state.JobByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)

	err = state.DeleteJob(1001, job.Namespace, job.ID)
	must.NoError(t, err)

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Nil(t, out)

	index, err := state.Index("jobs")
	must.NoError(t, err)
	must.Eq(t, 1001, index)

	summary, err := state.JobSummaryByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	if summary != nil {
		t.Fatalf("expected summary to be nil, but got: %v", summary)
	}

	index, err = state.Index("job_summary")
	must.NoError(t, err)
	must.Eq(t, 1001, index)

	versions, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Len(t, 0, versions, must.Sprint("expected no job versions"))

	index, err = state.Index("job_summary")
	must.NoError(t, err)
	must.Eq(t, 1001, index)

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_DeleteJobTxn_BatchDeletes(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	const testJobCount = 10
	const jobVersionCount = 4

	stateIndex := uint64(1000)

	jobs := make([]*structs.Job, testJobCount)
	for i := range testJobCount {
		stateIndex++
		job := mock.BatchJob()

		err := state.UpsertJob(structs.MsgTypeTestSetup, stateIndex, nil, job)
		must.NoError(t, err)

		jobs[i] = job

		// Create some versions
		for vi := 1; vi < jobVersionCount; vi++ {
			stateIndex++

			job := job.Copy()
			job.TaskGroups[0].Tasks[0].Env = map[string]string{
				"Version": fmt.Sprintf("%d", vi),
			}

			must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, stateIndex, nil, job))
		}
	}

	ws := memdb.NewWatchSet()

	// Check that jobs are present in DB
	job, err := state.JobByID(ws, jobs[0].Namespace, jobs[0].ID)
	must.NoError(t, err)
	must.Eq(t, jobs[0].ID, job.ID)

	jobVersions, err := state.JobVersionsByID(ws, jobs[0].Namespace, jobs[0].ID)
	must.NoError(t, err)
	must.Eq(t, jobVersionCount, len(jobVersions))

	// Actually delete
	const deletionIndex = uint64(10001)
	err = state.WithWriteTransaction(structs.MsgTypeTestSetup, deletionIndex, func(txn Txn) error {
		for i, job := range jobs {
			err := state.DeleteJobTxn(deletionIndex, job.Namespace, job.ID, txn)
			must.NoError(t, err, must.Sprintf("failed at %d %e", i, err))
		}
		return nil
	})
	must.NoError(t, err)

	test.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.JobByID(ws, jobs[0].Namespace, jobs[0].ID)
	must.NoError(t, err)
	must.Nil(t, out)

	jobVersions, err = state.JobVersionsByID(ws, jobs[0].Namespace, jobs[0].ID)
	must.NoError(t, err)
	must.Len(t, 0, jobVersions)

	index, err := state.Index("jobs")
	must.NoError(t, err)
	must.Eq(t, deletionIndex, index)
}

// TestStatestore_JobVersionTag tests that job versions which are tagged
// do not count against the configured server.job_tracked_versions count,
// do not get deleted when new versions are created,
// and *do* get deleted immediately when its tag is removed.
func TestStatestore_JobVersionTag(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	// tagged versions should be excluded from this limit
	state.config.JobTrackedVersions = 5

	job := mock.MinJob()
	job.Stable = true

	// helpers for readability
	upsertJob := func(t *testing.T) {
		t.Helper()
		must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, nextIndex(state), nil, job.Copy()))
	}

	applyTag := func(t *testing.T, version uint64) {
		t.Helper()
		name := fmt.Sprintf("v%d", version)
		desc := fmt.Sprintf("version %d", version)
		req := &structs.JobApplyTagRequest{
			JobID: job.ID,
			Name:  name,
			Tag: &structs.JobVersionTag{
				Name:        name,
				Description: desc,
			},
			Version: version,
		}
		must.NoError(t, state.UpdateJobVersionTag(nextIndex(state), job.Namespace, req))

		// confirm
		got, err := state.JobVersionByTagName(nil, job.Namespace, job.ID, name)
		must.NoError(t, err)
		must.Eq(t, version, got.Version)
		must.Eq(t, name, got.VersionTag.Name)
		must.Eq(t, desc, got.VersionTag.Description)
	}
	unsetTag := func(t *testing.T, name string) {
		t.Helper()
		req := &structs.JobApplyTagRequest{
			JobID: job.ID,
			Name:  name,
			Tag:   nil, // this triggers unset
		}
		must.NoError(t, state.UpdateJobVersionTag(nextIndex(state), job.Namespace, req))
	}

	assertVersions := func(t *testing.T, expect []uint64) {
		t.Helper()
		jobs, err := state.JobVersionsByID(nil, job.Namespace, job.ID)
		must.NoError(t, err)
		vs := make([]uint64, len(jobs))
		for i, j := range jobs {
			vs[i] = j.Version
		}
		must.Eq(t, expect, vs)
	}

	// we want to end up with JobTrackedVersions (5) versions,
	// 0-2 tagged and 3-4 untagged, but also interleave the tagging
	// to be somewhat true to normal behavior in reality.
	{
		// upsert 3 jobs
		for range 3 {
			upsertJob(t)
		}
		assertVersions(t, []uint64{2, 1, 0})

		// tag 2 of them
		applyTag(t, 1)
		applyTag(t, 2)
		// nothing should change
		assertVersions(t, []uint64{2, 1, 0})

		// add 3 more, up to JobTrackedVersions (5) + 1 (6)
		for range 3 {
			upsertJob(t)
		}
		assertVersions(t, []uint64{5, 4, 3, 2, 1, 0})

		// tag one more
		applyTag(t, 3)
		// again nothing should change
		assertVersions(t, []uint64{5, 4, 3, 2, 1, 0})
	}

	// removing a tag at this point should leave the version in place,
	// because we still have room within JobTrackedVersions
	{
		unsetTag(t, "v3")
		assertVersions(t, []uint64{5, 4, 3, 2, 1, 0})
	}

	// adding more versions should replace 0,3-5
	// and leave 1-2 in place because they are tagged
	{
		for range 10 {
			upsertJob(t)
		}
		assertVersions(t, []uint64{15, 14, 13, 12, 11, 2, 1})
	}

	// untagging version 1 now should delete it immediately,
	// since we now have more than JobTrackedVersions
	{
		unsetTag(t, "v1")
		assertVersions(t, []uint64{15, 14, 13, 12, 11, 2})
	}

	// test some error conditions
	{
		// job does not exist
		err := state.UpdateJobVersionTag(nextIndex(state), job.Namespace, &structs.JobApplyTagRequest{
			JobID:   "non-existent-job",
			Tag:     &structs.JobVersionTag{Name: "tag name"},
			Version: 0,
		})
		must.ErrorContains(t, err, `job "non-existent-job" version 0 not found`)

		// version does not exist
		err = state.UpdateJobVersionTag(nextIndex(state), job.Namespace, &structs.JobApplyTagRequest{
			JobID:   job.ID,
			Tag:     &structs.JobVersionTag{Name: "tag name"},
			Version: 999,
		})
		must.ErrorContains(t, err, fmt.Sprintf("job %q version 999 not found", job.ID))

		// tag name already exists
		err = state.UpdateJobVersionTag(nextIndex(state), job.Namespace, &structs.JobApplyTagRequest{
			JobID:   job.ID,
			Tag:     &structs.JobVersionTag{Name: "v2"},
			Version: 10,
		})
		must.ErrorContains(t, err, fmt.Sprintf(`"v2" already exists on a different version of job %q`, job.ID))
	}

	// deleting all versions should also delete tagged versions
	txn := state.db.WriteTxn(nextIndex(state))
	must.NoError(t, state.deleteJobVersions(nextIndex(state), job, txn))
	must.NoError(t, txn.Commit())
	assertVersions(t, []uint64{})
}

func TestStateStore_DeleteJob_MultipleVersions(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	// Create a job and mark it as stable
	job := mock.Job()
	job.Stable = true
	job.Priority = 0

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))
	test.True(t, watchFired(ws))

	var finalJob *structs.Job
	for i := 1; i < 20; i++ {
		finalJob = mock.Job()
		finalJob.ID = job.ID
		finalJob.Priority = i
		must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, uint64(1000+i), nil, finalJob))
	}

	must.NoError(t, state.DeleteJob(1020, job.Namespace, job.ID))
	test.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	test.Nil(t, out)

	index, err := state.Index("jobs")
	must.NoError(t, err)
	test.Eq(t, 1020, index)

	summary, err := state.JobSummaryByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	test.Nil(t, summary)

	index, err = state.Index("job_version")
	must.NoError(t, err)
	test.Eq(t, 1020, index)

	versions, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	test.Len(t, 0, versions)

	index, err = state.Index("job_summary")
	must.NoError(t, err)
	test.Eq(t, 1020, index)

	must.False(t, watchFired(ws), must.Sprint("expected watch not to fire"))
}

func TestStateStore_DeleteJob_ChildJob(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	parent := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 998, nil, parent))

	child := mock.Job()
	child.Status = ""
	child.ParentID = parent.ID

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, child))

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	must.NoError(t, err)

	err = state.DeleteJob(1001, child.Namespace, child.ID)
	must.NoError(t, err)
	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	summary, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	must.NoError(t, err)
	must.NotNil(t, summary)
	must.Eq(t, parent.ID, summary.JobID)
	must.NotNil(t, summary.Children)
	must.Eq(t, &structs.JobChildrenSummary{Dead: 1}, summary.Children)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_Jobs(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	var jobs []*structs.Job

	for i := range 10 {
		job := mock.Job()
		jobs = append(jobs, job)

		err := state.UpsertJob(structs.MsgTypeTestSetup, 1000+uint64(i), nil, job)
		must.NoError(t, err)
	}

	ws := memdb.NewWatchSet()
	iter, err := state.Jobs(ws, SortDefault)
	must.NoError(t, err)

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

	must.Eq(t, jobs, out)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_JobVersions(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	var jobs []*structs.Job

	for i := range 10 {
		job := mock.Job()
		jobs = append(jobs, job)

		err := state.UpsertJob(structs.MsgTypeTestSetup, 1000+uint64(i), nil, job)
		must.NoError(t, err)
	}

	ws := memdb.NewWatchSet()
	iter, err := state.JobVersions(ws)
	must.NoError(t, err)

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

	must.Eq(t, jobs, out)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_JobsByIDPrefix(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job := mock.Job()

	job.ID = "redis"
	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	must.NoError(t, err)

	ws := memdb.NewWatchSet()
	iter, err := state.JobsByIDPrefix(ws, job.Namespace, job.ID, SortDefault)
	must.NoError(t, err)

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

	iter, err = state.JobsByIDPrefix(ws, job.Namespace, "re", SortDefault)
	must.NoError(t, err)

	jobs = gatherJobs(iter)
	if len(jobs) != 1 {
		t.Fatalf("err: %v", err)
	}
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))

	job = mock.Job()
	job.ID = "riak"
	err = state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, job)
	must.NoError(t, err)

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	iter, err = state.JobsByIDPrefix(ws, job.Namespace, "r", SortDefault)
	must.NoError(t, err)

	jobs = gatherJobs(iter)
	if len(jobs) != 2 {
		t.Fatalf("err: %v", err)
	}

	iter, err = state.JobsByIDPrefix(ws, job.Namespace, "ri", SortDefault)
	must.NoError(t, err)

	jobs = gatherJobs(iter)
	if len(jobs) != 1 {
		t.Fatalf("err: %v", err)
	}
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_JobsByIDPrefix_Namespaces(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job1 := mock.Job()
	job2 := mock.Job()

	ns1 := mock.Namespace()
	ns1.Name = "namespace1"
	ns2 := mock.Namespace()
	ns2.Name = "namespace2"

	jobID := "redis"
	job1.ID = jobID
	job2.ID = jobID
	job1.Namespace = ns1.Name
	job2.Namespace = ns2.Name

	must.NoError(t, state.UpsertNamespaces(998, []*structs.Namespace{ns1, ns2}))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job1))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, job2))

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

	// Try full match
	ws := memdb.NewWatchSet()
	iter1, err := state.JobsByIDPrefix(ws, ns1.Name, jobID, SortDefault)
	must.NoError(t, err)
	iter2, err := state.JobsByIDPrefix(ws, ns2.Name, jobID, SortDefault)
	must.NoError(t, err)

	jobsNs1 := gatherJobs(iter1)
	must.Len(t, 1, jobsNs1)

	jobsNs2 := gatherJobs(iter2)
	must.Len(t, 1, jobsNs2)

	// Try prefix
	iter1, err = state.JobsByIDPrefix(ws, ns1.Name, "re", SortDefault)
	must.NoError(t, err)
	iter2, err = state.JobsByIDPrefix(ws, ns2.Name, "re", SortDefault)
	must.NoError(t, err)

	jobsNs1 = gatherJobs(iter1)
	jobsNs2 = gatherJobs(iter2)
	must.Len(t, 1, jobsNs1)
	must.Len(t, 1, jobsNs2)

	job3 := mock.Job()
	job3.ID = "riak"
	job3.Namespace = ns1.Name
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1003, nil, job3))
	must.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	iter1, err = state.JobsByIDPrefix(ws, ns1.Name, "r", SortDefault)
	must.NoError(t, err)
	iter2, err = state.JobsByIDPrefix(ws, ns2.Name, "r", SortDefault)
	must.NoError(t, err)

	jobsNs1 = gatherJobs(iter1)
	jobsNs2 = gatherJobs(iter2)
	must.Len(t, 2, jobsNs1)
	must.Len(t, 1, jobsNs2)

	iter1, err = state.JobsByIDPrefix(ws, ns1.Name, "ri", SortDefault)
	must.NoError(t, err)

	jobsNs1 = gatherJobs(iter1)
	must.Len(t, 1, jobsNs1)
	must.False(t, watchFired(ws))
}

func TestStateStore_JobsByNamespace(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	ns1 := mock.Namespace()
	ns1.Name = "new"
	job1 := mock.Job()
	job2 := mock.Job()
	job1.Namespace = ns1.Name
	job2.Namespace = ns1.Name

	ns2 := mock.Namespace()
	ns2.Name = "new-namespace"
	job3 := mock.Job()
	job4 := mock.Job()
	job3.Namespace = ns2.Name
	job4.Namespace = ns2.Name

	must.NoError(t, state.UpsertNamespaces(998, []*structs.Namespace{ns1, ns2}))

	// Create watchsets so we can test that update fires the watch
	watches := []memdb.WatchSet{memdb.NewWatchSet(), memdb.NewWatchSet()}
	_, err := state.JobsByNamespace(watches[0], ns1.Name, SortDefault)
	must.NoError(t, err)
	_, err = state.JobsByNamespace(watches[1], ns2.Name, SortDefault)
	must.NoError(t, err)

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, job1))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1002, nil, job2))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1003, nil, job3))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1004, nil, job4))
	must.True(t, watchFired(watches[0]))
	must.True(t, watchFired(watches[1]))

	ws := memdb.NewWatchSet()
	iter1, err := state.JobsByNamespace(ws, ns1.Name, SortDefault)
	must.NoError(t, err)
	iter2, err := state.JobsByNamespace(ws, ns2.Name, SortDefault)
	must.NoError(t, err)

	var out1 []*structs.Job
	for {
		raw := iter1.Next()
		if raw == nil {
			break
		}
		out1 = append(out1, raw.(*structs.Job))
	}

	var out2 []*structs.Job
	for {
		raw := iter2.Next()
		if raw == nil {
			break
		}
		out2 = append(out2, raw.(*structs.Job))
	}

	must.Len(t, 2, out1)
	must.Len(t, 2, out2)

	for _, job := range out1 {
		must.Eq(t, ns1.Name, job.Namespace)
	}
	for _, job := range out2 {
		must.Eq(t, ns2.Name, job.Namespace)
	}

	index, err := state.Index("jobs")
	must.NoError(t, err)
	must.Eq(t, 1004, index)
	must.False(t, watchFired(ws))
}

func TestStateStore_JobsByPeriodic(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	var periodic, nonPeriodic []*structs.Job

	for i := range 10 {
		job := mock.Job()
		nonPeriodic = append(nonPeriodic, job)

		err := state.UpsertJob(structs.MsgTypeTestSetup, 1000+uint64(i), nil, job)
		must.NoError(t, err)
	}

	for i := range 10 {
		job := mock.PeriodicJob()
		periodic = append(periodic, job)

		err := state.UpsertJob(structs.MsgTypeTestSetup, 2000+uint64(i), nil, job)
		must.NoError(t, err)
	}

	ws := memdb.NewWatchSet()
	iter, err := state.JobsByPeriodic(ws, true)
	must.NoError(t, err)

	var outPeriodic []*structs.Job
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		outPeriodic = append(outPeriodic, raw.(*structs.Job))
	}

	iter, err = state.JobsByPeriodic(ws, false)
	must.NoError(t, err)

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

	must.Eq(t, periodic, outPeriodic)
	must.Eq(t, nonPeriodic, outNonPeriodic)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_JobsByScheduler(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	var serviceJobs []*structs.Job
	var sysJobs []*structs.Job

	for i := range 10 {
		job := mock.Job()
		serviceJobs = append(serviceJobs, job)

		err := state.UpsertJob(structs.MsgTypeTestSetup, 1000+uint64(i), nil, job)
		must.NoError(t, err)
	}

	for i := range 10 {
		job := mock.SystemJob()
		job.Status = structs.JobStatusRunning
		sysJobs = append(sysJobs, job)

		err := state.UpsertJob(structs.MsgTypeTestSetup, 2000+uint64(i), nil, job)
		must.NoError(t, err)
	}

	ws := memdb.NewWatchSet()
	iter, err := state.JobsByScheduler(ws, "service")
	must.NoError(t, err)

	var outService []*structs.Job
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		outService = append(outService, raw.(*structs.Job))
	}

	iter, err = state.JobsByScheduler(ws, "system")
	must.NoError(t, err)

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

	must.Eq(t, serviceJobs, outService)
	must.Eq(t, sysJobs, outSystem)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_JobsByGC(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	gc, nonGc := make(map[string]struct{}), make(map[string]struct{})

	for i := range 20 {
		var job *structs.Job
		if i%2 == 0 {
			job = mock.Job()
		} else {
			job = mock.PeriodicJob()
		}
		nonGc[job.ID] = struct{}{}

		must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000+uint64(i), nil, job))
	}

	for i := 0; i < 20; i += 2 {
		idx := 2000 + uint64(i+1)
		job := mock.Job()
		job.Type = structs.JobTypeBatch
		job.ModifyIndex = idx
		gc[job.ID] = struct{}{}

		must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, idx, nil, job))

		// Create an eval for it
		eval := mock.Eval()
		eval.JobID = job.ID
		eval.JobModifyIndex = job.ModifyIndex
		eval.Status = structs.EvalStatusComplete
		must.NoError(t, state.UpsertEvals(structs.MsgTypeTestSetup, idx, []*structs.Evaluation{eval}))

	}

	ws := memdb.NewWatchSet()
	iter, err := state.JobsByGC(ws, true)
	must.NoError(t, err)

	outGc := make(map[string]struct{})
	for i := iter.Next(); i != nil; i = iter.Next() {
		j := i.(*structs.Job)
		outGc[j.ID] = struct{}{}
	}

	iter, err = state.JobsByGC(ws, false)
	must.NoError(t, err)

	outNonGc := make(map[string]struct{})
	for i := iter.Next(); i != nil; i = iter.Next() {
		j := i.(*structs.Job)
		outNonGc[j.ID] = struct{}{}
	}

	must.Eq(t, gc, outGc)
	must.Eq(t, nonGc, outNonGc)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_UpsertPeriodicLaunch(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job := mock.Job()
	launch := &structs.PeriodicLaunch{
		ID:        job.ID,
		Namespace: job.Namespace,
		Launch:    time.Now(),
	}

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.PeriodicLaunchByID(ws, job.Namespace, launch.ID)
	must.NoError(t, err)

	err = state.UpsertPeriodicLaunch(1000, launch)
	must.NoError(t, err)

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	out, err := state.PeriodicLaunchByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Eq(t, 1000, out.CreateIndex)
	if out.ModifyIndex != 1000 {
		t.Fatalf("bad: %#v", out)
	}

	must.Eq(t, launch, out)
	index, err := state.Index("periodic_launch")
	must.NoError(t, err)
	must.Eq(t, 1000, index)

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_UpdateUpsertPeriodicLaunch(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job := mock.Job()
	launch := &structs.PeriodicLaunch{
		ID:        job.ID,
		Namespace: job.Namespace,
		Launch:    time.Now(),
	}

	err := state.UpsertPeriodicLaunch(1000, launch)
	must.NoError(t, err)

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err = state.PeriodicLaunchByID(ws, job.Namespace, launch.ID)
	must.NoError(t, err)

	launch2 := &structs.PeriodicLaunch{
		ID:        job.ID,
		Namespace: job.Namespace,
		Launch:    launch.Launch.Add(1 * time.Second),
	}
	err = state.UpsertPeriodicLaunch(1001, launch2)
	must.NoError(t, err)

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	out, err := state.PeriodicLaunchByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Eq(t, 1000, out.CreateIndex)
	must.Eq(t, 1001, out.ModifyIndex)
	must.Eq(t, launch2, out)

	index, err := state.Index("periodic_launch")
	must.NoError(t, err)
	must.Eq(t, 1001, index)

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_DeletePeriodicLaunch(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job := mock.Job()
	launch := &structs.PeriodicLaunch{
		ID:        job.ID,
		Namespace: job.Namespace,
		Launch:    time.Now(),
	}

	err := state.UpsertPeriodicLaunch(1000, launch)
	must.NoError(t, err)

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	_, err = state.PeriodicLaunchByID(ws, job.Namespace, launch.ID)
	must.NoError(t, err)

	err = state.DeletePeriodicLaunch(1001, launch.Namespace, launch.ID)
	must.NoError(t, err)

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	out, err := state.PeriodicLaunchByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Nil(t, out)

	index, err := state.Index("periodic_launch")
	must.NoError(t, err)
	must.Eq(t, 1001, index)

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_PeriodicLaunches(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	var launches []*structs.PeriodicLaunch

	for i := range 10 {
		job := mock.Job()
		launch := &structs.PeriodicLaunch{
			ID:        job.ID,
			Namespace: job.Namespace,
			Launch:    time.Now(),
		}
		launches = append(launches, launch)

		err := state.UpsertPeriodicLaunch(1000+uint64(i), launch)
		must.NoError(t, err)
	}

	ws := memdb.NewWatchSet()
	iter, err := state.PeriodicLaunches(ws)
	must.NoError(t, err)

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
		must.True(t, ok, must.Sprintf("bad %v", launch.ID))
		must.Eq(t, launch, l)
		delete(out, launch.ID)
	}

	must.MapLen(t, 0, out, must.Sprintf("leftover: %#v", out))
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

// TestStateStore_CSIVolume checks register, list and deregister for csi_volumes
func TestStateStore_CSIVolume(t *testing.T) {
	state := testStateStore(t)
	index := uint64(1000)

	// Volume IDs
	vol0, vol1 := uuid.Generate(), uuid.Generate()

	mockJob := mock.Job()
	mockJob.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{
		"foo": {
			Name:   "foo",
			Source: vol0,
			Type:   "csi",
		},
	}
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, index, nil, mockJob))

	// Create a node running a healthy instance of the plugin
	node := mock.Node()
	pluginID := "minnie"
	alloc := mock.Alloc()
	alloc.JobID = mockJob.ID
	alloc.Job = mockJob
	alloc.DesiredStatus = "run"
	alloc.ClientStatus = "running"
	alloc.NodeID = node.ID

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
	must.NoError(t, err)
	defer state.DeleteNode(structs.MsgTypeTestSetup, 9999, []string{pluginID})

	now := time.Now().UnixNano()

	index++
	err = state.UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{alloc})
	must.NoError(t, err)

	ns := structs.DefaultNamespace

	v0 := structs.NewCSIVolume("foo", index)
	v0.ID = vol0
	v0.Namespace = ns
	v0.PluginID = "minnie"
	v0.Schedulable = true
	v0.AccessMode = structs.CSIVolumeAccessModeMultiNodeSingleWriter
	v0.AttachmentMode = structs.CSIVolumeAttachmentModeFilesystem
	v0.RequestedCapabilities = []*structs.CSIVolumeCapability{{
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
	}}

	index++
	v1 := structs.NewCSIVolume("foo", index)
	v1.ID = vol1
	v1.Namespace = ns
	v1.PluginID = "adam"
	v1.Schedulable = true
	v1.AccessMode = structs.CSIVolumeAccessModeMultiNodeSingleWriter
	v1.AttachmentMode = structs.CSIVolumeAttachmentModeFilesystem
	v1.RequestedCapabilities = []*structs.CSIVolumeCapability{{
		AccessMode:     structs.CSIVolumeAccessModeMultiNodeSingleWriter,
		AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
	}}

	index++
	err = state.UpsertCSIVolume(index, []*structs.CSIVolume{v0, v1})
	must.NoError(t, err)

	// volume registration is idempotent, unless identies are changed
	index++
	err = state.UpsertCSIVolume(index, []*structs.CSIVolume{v0, v1})
	must.NoError(t, err)

	index++
	v2 := v0.Copy()
	v2.PluginID = "new-id"
	err = state.UpsertCSIVolume(index, []*structs.CSIVolume{v2})
	must.EqError(t, err, fmt.Sprintf("volume identity cannot be updated: %s", v0.ID))

	ws := memdb.NewWatchSet()
	iter, err := state.CSIVolumesByNamespace(ws, ns, "")
	must.NoError(t, err)

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
	must.Len(t, 2, vs)

	ws = memdb.NewWatchSet()
	iter, err = state.CSIVolumesByPluginID(ws, ns, "", "minnie")
	must.NoError(t, err)
	vs = slurp(iter)
	must.Eq(t, 1, len(vs))

	ws = memdb.NewWatchSet()
	iter, err = state.CSIVolumesByNodeID(ws, "", node.ID)
	must.NoError(t, err)
	vs = slurp(iter)
	must.Eq(t, 1, len(vs))

	// Allocs
	a0 := mock.Alloc()
	a0.JobID = mockJob.ID
	a0.Job = mockJob
	a1 := mock.Alloc()
	a1.JobID = mockJob.ID
	a1.Job = mockJob
	index++
	err = state.UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{a0, a1})
	must.NoError(t, err)

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
	err = state.CSIVolumeClaim(index, now, ns, vol0, claim0)
	must.NoError(t, err)
	index++
	err = state.CSIVolumeClaim(index, now, ns, vol0, claim1)
	must.NoError(t, err)

	ws = memdb.NewWatchSet()
	iter, err = state.CSIVolumesByPluginID(ws, ns, "", "minnie")
	must.NoError(t, err)
	vs = slurp(iter)
	must.False(t, vs[0].HasFreeWriteClaims())
	must.MapLen(t, 1, vs[0].ReadClaims)
	must.MapLen(t, 0, vs[0].PastClaims)

	claim2 := new(structs.CSIVolumeClaim)
	*claim2 = *claim0
	claim2.Mode = u
	err = state.CSIVolumeClaim(2, now, ns, vol0, claim2)
	must.NoError(t, err)
	ws = memdb.NewWatchSet()
	iter, err = state.CSIVolumesByPluginID(ws, ns, "", "minnie")
	must.NoError(t, err)
	vs = slurp(iter)
	must.True(t, vs[0].ReadSchedulable())

	// alloc finishes, so we should see a past claim
	a0 = a0.Copy()
	a0.ClientStatus = structs.AllocClientStatusComplete
	index++
	err = state.UpsertAllocs(structs.MsgTypeTestSetup, index, []*structs.Allocation{a0})
	must.NoError(t, err)

	v0, err = state.CSIVolumeByID(nil, ns, vol0)
	must.NoError(t, err)
	must.MapLen(t, 1, v0.ReadClaims)
	must.MapLen(t, 1, v0.PastClaims)

	// but until this claim is freed the volume is in use, so deregistration is
	// still an error
	index++
	err = state.CSIVolumeDeregister(index, ns, []string{vol0}, false)
	must.Error(t, err, must.Sprint("volume deregistered while in use"))

	// even if forced, because we have a non-terminal claim
	index++
	err = state.CSIVolumeDeregister(index, ns, []string{vol0}, true)
	must.Error(t, err, must.Sprint("volume force deregistered while in use"))

	// we use the ID, not a prefix
	index++
	err = state.CSIVolumeDeregister(index, ns, []string{"fo"}, true)
	must.Error(t, err, must.Sprint("volume deregistered by prefix"))

	// release claims to unblock deregister
	index++
	claim3 := new(structs.CSIVolumeClaim)
	*claim3 = *claim2
	claim3.State = structs.CSIVolumeClaimStateReadyToFree
	err = state.CSIVolumeClaim(index, now, ns, vol0, claim3)
	must.NoError(t, err)
	index++
	claim1.Mode = u
	claim1.State = structs.CSIVolumeClaimStateReadyToFree
	err = state.CSIVolumeClaim(index, now, ns, vol0, claim1)
	must.NoError(t, err)

	index++
	err = state.CSIVolumeDeregister(index, ns, []string{vol0}, false)
	must.NoError(t, err)

	// List, now omitting the deregistered volume
	ws = memdb.NewWatchSet()
	iter, err = state.CSIVolumesByPluginID(ws, ns, "", "minnie")
	must.NoError(t, err)
	vs = slurp(iter)
	must.Eq(t, 0, len(vs))

	ws = memdb.NewWatchSet()
	iter, err = state.CSIVolumesByNamespace(ws, ns, "")
	must.NoError(t, err)
	vs = slurp(iter)
	must.Eq(t, 1, len(vs))
}

func TestStateStore_CSIPlugin_Lifecycle(t *testing.T) {
	ci.Parallel(t)

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
	checkPlugin := func(t *testing.T, plugID string, counts pluginCounts) *structs.CSIPlugin {
		t.Helper()
		plug, err := store.CSIPluginByID(memdb.NewWatchSet(), plugID)
		must.NotNil(t, plug, must.Sprint("plugin was nil"))
		must.NoError(t, err)
		must.MapLen(t, counts.controllerFingerprints, plug.Controllers, must.Sprint("controllers fingerprinted"))
		must.MapLen(t, counts.nodeFingerprints, plug.Nodes, must.Sprint("nodes fingerprinted"))
		must.Eq(t, counts.controllersHealthy, plug.ControllersHealthy, must.Sprint("controllers healthy"))
		must.Eq(t, counts.nodesHealthy, plug.NodesHealthy, must.Sprint("nodes healthy"))
		must.Eq(t, counts.controllersExpected, plug.ControllersExpected, must.Sprint("controllers expected"))
		must.Eq(t, counts.nodesExpected, plug.NodesExpected, must.Sprint("nodes expected"))
		return plug.Copy()
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
			must.NoError(t, err)
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
		must.NoError(t, err)
		return allocs
	}

	// helper function calling UpsertNode for fingerprinting
	updateNodeFn := func(nodeID string, transform func(node *structs.Node)) {
		ws := memdb.NewWatchSet()
		node, _ := store.NodeByID(ws, nodeID)
		node = node.Copy()
		transform(node)
		err = store.UpsertNode(structs.MsgTypeTestSetup, nextIndex(store), node)
		must.NoError(t, err)
	}

	nodes := []*structs.Node{mock.Node(), mock.Node(), mock.Node()}
	for _, node := range nodes {
		err = store.UpsertNode(structs.MsgTypeTestSetup, nextIndex(store), node)
		must.NoError(t, err)
	}

	// Note: these are all subtests for clarity but are expected to be
	// ordered, because they walk through all the phases of plugin
	// instance registration and deregistration

	t.Run("register plugin jobs", func(t *testing.T) {

		controllerJob := mock.CSIPluginJob(structs.CSIPluginTypeController, plugID)
		controllerJobID = controllerJob.ID
		err = store.UpsertJob(structs.MsgTypeTestSetup, nextIndex(store), nil, controllerJob)

		nodeJob := mock.CSIPluginJob(structs.CSIPluginTypeNode, plugID)
		nodeJobID = nodeJob.ID
		err = store.UpsertJob(structs.MsgTypeTestSetup, nextIndex(store), nil, nodeJob)

		// plugins created, but no fingerprints or allocs yet
		// note: there's no job summary yet, but we know the task
		// group count for the non-system job
		//
		// TODO: that's the current code but we really should be able
		// to figure out the system jobs too
		plug := checkPlugin(t, plugID, pluginCounts{
			controllerFingerprints: 0,
			nodeFingerprints:       0,
			controllersHealthy:     0,
			nodesHealthy:           0,
			controllersExpected:    2,
			nodesExpected:          0,
		})
		must.False(t, plug.ControllerRequired)
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
		must.NoError(t, err)

		// node plugin now has expected counts too
		plug := checkPlugin(t, plugID, pluginCounts{
			controllerFingerprints: 0,
			nodeFingerprints:       0,
			controllersHealthy:     0,
			nodesHealthy:           0,
			controllersExpected:    2,
			nodesExpected:          3,
		})
		must.False(t, plug.ControllerRequired)
	})

	t.Run("client upserts alloc status", func(t *testing.T) {

		updateAllocsFn(allocIDs, CLIENT, func(alloc *structs.Allocation) {
			alloc.ClientStatus = structs.AllocClientStatusRunning
		})

		// plugin still has allocs but no fingerprints
		plug := checkPlugin(t, plugID, pluginCounts{
			controllerFingerprints: 0,
			nodeFingerprints:       0,
			controllersHealthy:     0,
			nodesHealthy:           0,
			controllersExpected:    2,
			nodesExpected:          3,
		})
		must.False(t, plug.ControllerRequired)
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
		plug := checkPlugin(t, plugID, pluginCounts{
			controllerFingerprints: 2,
			nodeFingerprints:       3,
			controllersHealthy:     2,
			nodesHealthy:           3,
			controllersExpected:    2,
			nodesExpected:          3,
		})
		must.True(t, plug.ControllerRequired)
	})

	t.Run("node marked for drain", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		nodeAllocs, err := store.AllocsByNode(ws, nodes[0].ID)
		must.NoError(t, err)
		must.Len(t, 2, nodeAllocs)

		updateAllocsFn([]string{nodeAllocs[0].ID, nodeAllocs[1].ID},
			SERVER, func(alloc *structs.Allocation) {
				alloc.DesiredStatus = structs.AllocDesiredStatusStop
			})

		plug := checkPlugin(t, plugID, pluginCounts{
			controllerFingerprints: 2,
			nodeFingerprints:       3,
			controllersHealthy:     2,
			nodesHealthy:           3,
			controllersExpected:    2, // job summary hasn't changed
			nodesExpected:          3, // job summary hasn't changed
		})
		must.True(t, plug.ControllerRequired)
	})

	t.Run("client removes fingerprints after node drain", func(t *testing.T) {
		updateNodeFn(nodes[0].ID, func(node *structs.Node) {
			node.CSIControllerPlugins = nil
			node.CSINodePlugins = nil
		})

		plug := checkPlugin(t, plugID, pluginCounts{
			controllerFingerprints: 1,
			nodeFingerprints:       2,
			controllersHealthy:     1,
			nodesHealthy:           2,
			controllersExpected:    2,
			nodesExpected:          3,
		})
		must.True(t, plug.ControllerRequired)
	})

	t.Run("client updates alloc status to stopped after node drain", func(t *testing.T) {
		nodeAllocs, err := store.AllocsByNode(memdb.NewWatchSet(), nodes[0].ID)
		must.NoError(t, err)
		must.Len(t, 2, nodeAllocs)

		updateAllocsFn([]string{nodeAllocs[0].ID, nodeAllocs[1].ID}, CLIENT,
			func(alloc *structs.Allocation) {
				alloc.ClientStatus = structs.AllocClientStatusComplete
			})

		plug := checkPlugin(t, plugID, pluginCounts{
			controllerFingerprints: 1,
			nodeFingerprints:       2,
			controllersHealthy:     1,
			nodesHealthy:           2,
			controllersExpected:    2, // still 2 because count=2
			nodesExpected:          2, // has to use nodes we're actually placed on
		})
		must.True(t, plug.ControllerRequired)
	})

	t.Run("job stop with purge", func(t *testing.T) {

		vol := &structs.CSIVolume{
			ID:        uuid.Generate(),
			Namespace: structs.DefaultNamespace,
			PluginID:  plugID,
		}
		err = store.UpsertCSIVolume(nextIndex(store), []*structs.CSIVolume{vol})
		must.NoError(t, err)

		err = store.DeleteJob(nextIndex(store), structs.DefaultNamespace, controllerJobID)
		must.NoError(t, err)

		err = store.DeleteJob(nextIndex(store), structs.DefaultNamespace, nodeJobID)
		must.NoError(t, err)

		plug := checkPlugin(t, plugID, pluginCounts{
			controllerFingerprints: 1, // no changes till we get fingerprint
			nodeFingerprints:       2,
			controllersHealthy:     1,
			nodesHealthy:           2,
			controllersExpected:    0,
			nodesExpected:          0,
		})
		must.True(t, plug.ControllerRequired)
		must.False(t, plug.IsEmpty())

		for _, node := range nodes {
			updateNodeFn(node.ID, func(node *structs.Node) {
				node.CSIControllerPlugins = nil
			})
		}

		plug = checkPlugin(t, plugID, pluginCounts{
			controllerFingerprints: 0,
			nodeFingerprints:       2, // haven't removed fingerprints yet
			controllersHealthy:     0,
			nodesHealthy:           2,
			controllersExpected:    0,
			nodesExpected:          0,
		})
		must.True(t, plug.ControllerRequired)
		must.False(t, plug.IsEmpty())

		for _, node := range nodes {
			updateNodeFn(node.ID, func(node *structs.Node) {
				node.CSINodePlugins = nil
			})
		}

		ws := memdb.NewWatchSet()
		plug, err := store.CSIPluginByID(ws, plugID)
		must.NoError(t, err)
		must.Nil(t, plug, must.Sprint("plugin was not deleted"))

		vol, err = store.CSIVolumeByID(ws, vol.Namespace, vol.ID)
		must.NoError(t, err)
		must.NotNil(t, vol, must.Sprint("volume should be queryable even if plugin is deleted"))
		must.False(t, vol.Schedulable)
	})
}

func TestStateStore_Indexes(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	node := mock.Node()

	err := state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	must.NoError(t, err)

	iter, err := state.Indexes()
	must.NoError(t, err)

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
	ci.Parallel(t)

	state := testStateStore(t)

	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1000, mock.Node()))

	exp := uint64(2000)
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, exp, nil, mock.Job()))

	latest, err := state.LatestIndex()
	must.NoError(t, err)

	if latest != exp {
		t.Fatalf("LatestIndex() returned %d; want %d", latest, exp)
	}
}

func TestStateStore_UpsertEvals_Eval(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	eval := mock.Eval()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.EvalByID(ws, eval.ID)
	must.NoError(t, err)

	err = state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval})
	must.NoError(t, err)

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval.ID)
	must.NoError(t, err)

	must.Eq(t, eval, out)

	index, err := state.Index("evals")
	must.NoError(t, err)
	must.Eq(t, 1000, index)

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_UpsertEvals_CancelBlocked(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	// Create two blocked evals for the same job
	j := "test-job"
	b1, b2 := mock.Eval(), mock.Eval()
	b1.JobID = j
	b1.Status = structs.EvalStatusBlocked
	b2.JobID = j
	b2.Status = structs.EvalStatusBlocked

	err := state.UpsertEvals(structs.MsgTypeTestSetup, 999, []*structs.Evaluation{b1, b2})
	must.NoError(t, err)

	// Create one complete and successful eval for the job
	eval := mock.Eval()
	eval.JobID = j
	eval.Status = structs.EvalStatusComplete

	// Create a watchset so we can test that the upsert of the complete eval
	// fires the watch
	ws := memdb.NewWatchSet()
	_, err = state.EvalByID(ws, b1.ID)
	must.NoError(t, err)
	_, err = state.EvalByID(ws, b2.ID)
	must.NoError(t, err)

	must.NoError(t, state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval}))

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval.ID)
	must.NoError(t, err)

	must.Eq(t, eval, out)

	index, err := state.Index("evals")
	must.NoError(t, err)
	must.Eq(t, 1000, index)

	// Get b1/b2 and check they are cancelled
	out1, err := state.EvalByID(ws, b1.ID)
	must.NoError(t, err)

	out2, err := state.EvalByID(ws, b2.ID)
	must.NoError(t, err)

	if out1.Status != structs.EvalStatusCancelled || out2.Status != structs.EvalStatusCancelled {
		t.Fatalf("bad: %#v %#v", out1, out2)
	}

	if !strings.Contains(out1.StatusDescription, eval.ID) || !strings.Contains(out2.StatusDescription, eval.ID) {
		t.Fatalf("bad status description %#v %#v", out1, out2)
	}

	if out1.ModifyTime != eval.ModifyTime || out2.ModifyTime != eval.ModifyTime {
		t.Fatalf("bad modify time %#v %#v", out1, out2)
	}

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_UpsertEvals_Namespace(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	ns1 := mock.Namespace()
	ns1.Name = "new"
	eval1 := mock.Eval()
	eval2 := mock.Eval()
	eval1.Namespace = ns1.Name
	eval2.Namespace = ns1.Name

	ns2 := mock.Namespace()
	ns2.Name = "new-namespace"
	eval3 := mock.Eval()
	eval4 := mock.Eval()
	eval3.Namespace = ns2.Name
	eval4.Namespace = ns2.Name

	must.NoError(t, state.UpsertNamespaces(998, []*structs.Namespace{ns1, ns2}))

	// Create watchsets so we can test that update fires the watch
	watches := []memdb.WatchSet{memdb.NewWatchSet(), memdb.NewWatchSet()}
	_, err := state.EvalsByNamespace(watches[0], ns1.Name)
	must.NoError(t, err)
	_, err = state.EvalsByNamespace(watches[1], ns2.Name)
	must.NoError(t, err)

	must.NoError(t, state.UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{eval1, eval2, eval3, eval4}))
	must.True(t, watchFired(watches[0]))
	must.True(t, watchFired(watches[1]))

	ws := memdb.NewWatchSet()
	iter1, err := state.EvalsByNamespace(ws, ns1.Name)
	must.NoError(t, err)
	iter2, err := state.EvalsByNamespace(ws, ns2.Name)
	must.NoError(t, err)

	var out1 []*structs.Evaluation
	for {
		raw := iter1.Next()
		if raw == nil {
			break
		}
		out1 = append(out1, raw.(*structs.Evaluation))
	}

	var out2 []*structs.Evaluation
	for {
		raw := iter2.Next()
		if raw == nil {
			break
		}
		out2 = append(out2, raw.(*structs.Evaluation))
	}

	must.Len(t, 2, out1)
	must.Len(t, 2, out2)

	for _, eval := range out1 {
		must.Eq(t, ns1.Name, eval.Namespace)
	}
	for _, eval := range out2 {
		must.Eq(t, ns2.Name, eval.Namespace)
	}

	index, err := state.Index("evals")
	must.NoError(t, err)
	must.Eq(t, 1001, index)
	must.False(t, watchFired(ws))
}

func TestStateStore_Update_UpsertEvals_Eval(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	eval := mock.Eval()

	err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval})
	must.NoError(t, err)

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	ws2 := memdb.NewWatchSet()
	_, err = state.EvalByID(ws, eval.ID)
	must.NoError(t, err)

	_, err = state.EvalsByJob(ws2, eval.Namespace, eval.JobID)
	must.NoError(t, err)

	eval2 := mock.Eval()
	eval2.ID = eval.ID
	eval2.JobID = eval.JobID
	err = state.UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{eval2})
	must.NoError(t, err)

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))
	must.True(t, watchFired(ws2))

	ws = memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval.ID)
	must.NoError(t, err)

	must.Eq(t, eval2, out)

	must.Eq(t, 1000, out.CreateIndex)
	must.Eq(t, 1001, out.ModifyIndex)

	index, err := state.Index("evals")
	must.NoError(t, err)
	must.Eq(t, 1001, index)

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_UpsertEvals_Eval_ChildJob(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	parent := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 998, nil, parent))

	child := mock.Job()
	child.Status = ""
	child.ParentID = parent.ID

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, child))

	eval := mock.Eval()
	eval.Status = structs.EvalStatusComplete
	eval.JobID = child.ID
	eval.JobModifyIndex = child.ModifyIndex

	// Create watchsets so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	ws2 := memdb.NewWatchSet()
	ws3 := memdb.NewWatchSet()
	_, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	must.NoError(t, err)
	_, err = state.EvalByID(ws2, eval.ID)
	must.NoError(t, err)
	_, err = state.EvalsByJob(ws3, eval.Namespace, eval.JobID)
	must.NoError(t, err)

	err = state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval})
	must.NoError(t, err)

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))
	must.True(t, watchFired(ws2), must.Sprint("expected watch to fire"))
	must.True(t, watchFired(ws3), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval.ID)
	must.NoError(t, err)
	must.Eq(t, eval, out)

	index, err := state.Index("evals")
	must.NoError(t, err)
	must.Eq(t, 1000, index)

	summary, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	must.NoError(t, err)
	must.NotNil(t, summary)
	must.Eq(t, parent.ID, summary.JobID, must.Sprint("bad summary id"))
	must.NotNil(t, summary.Children)
	must.Eq(t, &structs.JobChildrenSummary{Dead: 1}, summary.Children)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_DeleteEval_Eval(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	mockJob1 := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 898, nil, mockJob1))

	mockJob2 := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 899, nil, mockJob2))

	eval1 := mock.Eval()
	eval1.JobID = mockJob1.ID
	eval2 := mock.Eval()
	eval2.JobID = mockJob2.ID
	alloc1 := mock.Alloc()
	alloc1.JobID = mockJob1.ID
	alloc2 := mock.Alloc()
	alloc2.JobID = mockJob2.ID

	// Create watchsets so we can test that upsert fires the watch
	watches := make([]memdb.WatchSet, 12)
	for i := range 12 {
		watches[i] = memdb.NewWatchSet()
	}
	_, err := state.EvalByID(watches[0], eval1.ID)
	must.NoError(t, err)
	_, err = state.EvalByID(watches[1], eval2.ID)
	must.NoError(t, err)
	_, err = state.EvalsByJob(watches[2], eval1.Namespace, eval1.JobID)
	must.NoError(t, err)
	_, err = state.EvalsByJob(watches[3], eval2.Namespace, eval2.JobID)
	must.NoError(t, err)
	_, err = state.AllocByID(watches[4], alloc1.ID)
	must.NoError(t, err)
	_, err = state.AllocByID(watches[5], alloc2.ID)
	must.NoError(t, err)
	_, err = state.AllocsByEval(watches[6], alloc1.EvalID)
	must.NoError(t, err)
	_, err = state.AllocsByEval(watches[7], alloc2.EvalID)
	must.NoError(t, err)
	_, err = state.AllocsByJob(watches[8], alloc1.Namespace, alloc1.JobID, false)
	must.NoError(t, err)
	_, err = state.AllocsByJob(watches[9], alloc2.Namespace, alloc2.JobID, false)
	must.NoError(t, err)
	_, err = state.AllocsByNode(watches[10], alloc1.NodeID)
	must.NoError(t, err)
	_, err = state.AllocsByNode(watches[11], alloc2.NodeID)
	must.NoError(t, err)

	state.UpsertJobSummary(900, mock.JobSummary(eval1.JobID))
	state.UpsertJobSummary(901, mock.JobSummary(eval2.JobID))
	state.UpsertJobSummary(902, mock.JobSummary(alloc1.JobID))
	state.UpsertJobSummary(903, mock.JobSummary(alloc2.JobID))
	err = state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1, eval2})
	must.NoError(t, err)

	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc1, alloc2})
	must.NoError(t, err)

	err = state.DeleteEval(1002, []string{eval1.ID, eval2.ID}, []string{alloc1.ID, alloc2.ID}, false)
	must.NoError(t, err)

	for _, ws := range watches {
		must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))
	}

	ws := memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval1.ID)
	must.NoError(t, err)
	must.Nil(t, out)

	out, err = state.EvalByID(ws, eval2.ID)
	must.NoError(t, err)
	must.Nil(t, out)

	outA, err := state.AllocByID(ws, alloc1.ID)
	must.NoError(t, err)
	must.Nil(t, outA)

	outA, err = state.AllocByID(ws, alloc2.ID)
	must.NoError(t, err)
	must.Nil(t, outA)

	index, err := state.Index("evals")
	must.NoError(t, err)
	must.Eq(t, 1002, index)

	index, err = state.Index("allocs")
	must.NoError(t, err)
	must.Eq(t, 1002, index)

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))

	// Call the eval delete function with zero length eval and alloc ID arrays.
	// This should result in the table indexes both staying the same, rather
	// than updating without cause.
	must.NoError(t, state.DeleteEval(1010, []string{}, []string{}, false))

	allocsIndex, err := state.Index("allocs")
	must.NoError(t, err)
	must.Eq(t, uint64(1002), allocsIndex)

	evalsIndex, err := state.Index("evals")
	must.NoError(t, err)
	must.Eq(t, uint64(1002), evalsIndex)
}

// This tests the evalDelete boolean by deleting a Pending eval and Pending Alloc.
func TestStateStore_DeleteEval_ChildJob(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	parent := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 998, nil, parent))

	child := mock.Job()
	child.Status = ""
	child.ParentID = parent.ID

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, child))

	eval1 := mock.Eval()
	eval1.JobID = child.ID
	alloc1 := mock.Alloc()
	alloc1.JobID = child.ID

	err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1})
	must.NoError(t, err)

	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc1})
	must.NoError(t, err)

	// Create watchsets so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	_, err = state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	must.NoError(t, err)

	err = state.DeleteEval(1002, []string{eval1.ID}, []string{alloc1.ID}, false)
	must.NoError(t, err)

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	summary, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	must.NoError(t, err)
	must.NotNil(t, summary)
	must.Eq(t, parent.ID, summary.JobID)
	must.NotNil(t, summary.Children)
	must.Eq(t, &structs.JobChildrenSummary{Dead: 1}, summary.Children)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_DeleteEval_UserInitiated(t *testing.T) {
	ci.Parallel(t)

	testState := testStateStore(t)

	// Upsert a scheduler config object, so we have something to check and
	// modify.
	schedulerConfig := structs.SchedulerConfiguration{PauseEvalBroker: false}
	must.NoError(t, testState.SchedulerSetConfig(10, &schedulerConfig))

	// Generate some mock evals and upsert these into state.
	mockEval1 := mock.Eval()
	mockEval2 := mock.Eval()
	must.NoError(t, testState.UpsertEvals(
		structs.MsgTypeTestSetup, 20, []*structs.Evaluation{mockEval1, mockEval2}))

	mockEvalIDs := []string{mockEval1.ID, mockEval2.ID}

	// Try and delete the evals without pausing the eval broker.
	err := testState.DeleteEval(30, mockEvalIDs, []string{}, true)
	must.ErrorContains(t, err, "eval broker is enabled")

	// Pause the eval broker on the scheduler config, and try deleting the
	// evals again.
	schedulerConfig.PauseEvalBroker = true
	must.NoError(t, testState.SchedulerSetConfig(30, &schedulerConfig))

	must.NoError(t, testState.DeleteEval(40, mockEvalIDs, []string{}, true))

	ws := memdb.NewWatchSet()
	mockEval1Lookup, err := testState.EvalByID(ws, mockEval1.ID)
	must.NoError(t, err)
	must.Nil(t, mockEval1Lookup)

	mockEval2Lookup, err := testState.EvalByID(ws, mockEval1.ID)
	must.NoError(t, err)
	must.Nil(t, mockEval2Lookup)
}

// TestStateStore_DeleteEvalsByFilter_Pagination tests the pagination logic for
// deleting evals by filter; the business logic is tested more fully in the eval
// endpoint tests.
func TestStateStore_DeleteEvalsByFilter_Pagination(t *testing.T) {

	evalCount := 100
	index := uint64(100)

	store := testStateStore(t)

	// Create a set of pending evaluations

	schedulerConfig := &structs.SchedulerConfiguration{
		PauseEvalBroker: true,
		CreateIndex:     index,
		ModifyIndex:     index,
	}
	must.NoError(t, store.SchedulerSetConfig(index, schedulerConfig))

	evals := []*structs.Evaluation{}
	for range evalCount {
		mockEval := mock.Eval()
		evals = append(evals, mockEval)
	}
	index++
	must.NoError(t, store.UpsertEvals(
		structs.MsgTypeTestSetup, index, evals))

	// Delete one page
	index++
	must.NoError(t, store.DeleteEvalsByFilter(index, "JobID != \"\"", "", 10))

	countRemaining := func() (string, int) {
		lastSeen := ""
		remaining := 0

		iter, err := store.Evals(nil, SortDefault)
		must.NoError(t, err)
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			eval := raw.(*structs.Evaluation)
			lastSeen = eval.ID
			remaining++
		}
		return lastSeen, remaining
	}

	lastSeen, remaining := countRemaining()
	must.Eq(t, 90, remaining)

	// Delete starting from lastSeen, which should only delete 1
	index++
	must.NoError(t, store.DeleteEvalsByFilter(index, "JobID != \"\"", lastSeen, 10))

	_, remaining = countRemaining()
	must.Eq(t, 89, remaining)
}

func TestStateStore_EvalIsUserDeleteSafe(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		inputAllocs    []*structs.Allocation
		inputJob       *structs.Job
		expectedResult bool
		name           string
	}{
		{
			inputAllocs:    nil,
			inputJob:       nil,
			expectedResult: true,
			name:           "job not in state",
		},
		{
			inputAllocs:    nil,
			inputJob:       &structs.Job{Status: structs.JobStatusDead},
			expectedResult: true,
			name:           "job stopped",
		},
		{
			inputAllocs:    nil,
			inputJob:       &structs.Job{Stop: true},
			expectedResult: true,
			name:           "job dead",
		},
		{
			inputAllocs:    []*structs.Allocation{},
			inputJob:       &structs.Job{Status: structs.JobStatusRunning},
			expectedResult: true,
			name:           "no allocs for eval",
		},
		{
			inputAllocs: []*structs.Allocation{
				{ClientStatus: structs.AllocClientStatusComplete},
				{ClientStatus: structs.AllocClientStatusRunning},
			},
			inputJob:       &structs.Job{Status: structs.JobStatusRunning},
			expectedResult: false,
			name:           "running alloc for eval",
		},
		{
			inputAllocs: []*structs.Allocation{
				{ClientStatus: structs.AllocClientStatusComplete},
				{ClientStatus: structs.AllocClientStatusUnknown},
			},
			inputJob:       &structs.Job{Status: structs.JobStatusRunning},
			expectedResult: false,
			name:           "unknown alloc for eval",
		},
		{
			inputAllocs: []*structs.Allocation{
				{ClientStatus: structs.AllocClientStatusComplete},
				{ClientStatus: structs.AllocClientStatusLost},
			},
			inputJob:       &structs.Job{Status: structs.JobStatusRunning},
			expectedResult: true,
			name:           "complete and lost allocs for eval",
		},
		{
			inputAllocs: []*structs.Allocation{
				{
					ClientStatus: structs.AllocClientStatusFailed,
					TaskGroup:    "test",
				},
			},
			inputJob: &structs.Job{
				Status: structs.JobStatusPending,
				TaskGroups: []*structs.TaskGroup{
					{
						Name:             "test",
						ReschedulePolicy: nil,
					},
				},
			},
			expectedResult: true,
			name:           "failed alloc job without reschedule",
		},
		{
			inputAllocs: []*structs.Allocation{
				{
					ClientStatus: structs.AllocClientStatusFailed,
					TaskGroup:    "test",
				},
			},
			inputJob: &structs.Job{
				Status: structs.JobStatusPending,
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "test",
						ReschedulePolicy: &structs.ReschedulePolicy{
							Unlimited: false,
							Attempts:  0,
						},
					},
				},
			},
			expectedResult: true,
			name:           "failed alloc job reschedule disabled",
		},
		{
			inputAllocs: []*structs.Allocation{
				{
					ClientStatus: structs.AllocClientStatusFailed,
					TaskGroup:    "test",
				},
			},
			inputJob: &structs.Job{
				Status: structs.JobStatusPending,
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "test",
						ReschedulePolicy: &structs.ReschedulePolicy{
							Unlimited: false,
							Attempts:  3,
						},
					},
				},
			},
			expectedResult: false,
			name:           "failed alloc next alloc not set",
		},
		{
			inputAllocs: []*structs.Allocation{
				{
					ClientStatus:   structs.AllocClientStatusFailed,
					TaskGroup:      "test",
					NextAllocation: "4aa4930a-8749-c95b-9c67-5ef29b0fc653",
				},
			},
			inputJob: &structs.Job{
				Status: structs.JobStatusPending,
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "test",
						ReschedulePolicy: &structs.ReschedulePolicy{
							Unlimited: false,
							Attempts:  3,
						},
					},
				},
			},
			expectedResult: false,
			name:           "failed alloc next alloc set",
		},
		{
			inputAllocs: []*structs.Allocation{
				{
					ClientStatus: structs.AllocClientStatusFailed,
					TaskGroup:    "test",
				},
			},
			inputJob: &structs.Job{
				Status: structs.JobStatusPending,
				TaskGroups: []*structs.TaskGroup{
					{
						Name: "test",
						ReschedulePolicy: &structs.ReschedulePolicy{
							Unlimited: true,
						},
					},
				},
			},
			expectedResult: false,
			name:           "failed alloc job reschedule unlimited",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := isEvalDeleteSafe(tc.inputAllocs, tc.inputJob)
			must.Eq(t, tc.expectedResult, actualResult)
		})
	}
}

func TestStateStore_EvalsByJob(t *testing.T) {
	ci.Parallel(t)

	t.Run("return all evals for job", func(t *testing.T) {
		state := testStateStore(t)

		eval1 := mock.Eval()
		eval2 := mock.Eval()
		eval2.JobID = eval1.JobID
		eval3 := mock.Eval()
		evals := []*structs.Evaluation{eval1, eval2}

		err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, evals)
		must.NoError(t, err)
		err = state.UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{eval3})
		must.NoError(t, err)

		ws := memdb.NewWatchSet()
		out, err := state.EvalsByJob(ws, eval1.Namespace, eval1.JobID)
		must.NoError(t, err)

		sort.Sort(EvalIDSort(evals))
		sort.Sort(EvalIDSort(out))

		must.Eq(t, evals, out)
		must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
	})

	t.Run("excludes job with matching prefix", func(t *testing.T) {
		state := testStateStore(t)
		eval1 := mock.Eval()
		eval1.JobID = "hello"
		must.NoError(t, state.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval1}))

		eval2 := mock.Eval()
		eval2.JobID = "hellohello"
		must.NoError(t, state.UpsertEvals(structs.MsgTypeTestSetup, 2, []*structs.Evaluation{eval2}))

		ws := memdb.NewWatchSet()
		evals, err := state.EvalsByJob(ws, structs.DefaultNamespace, "hello")
		must.NoError(t, err)
		must.Len(t, 1, evals)
		must.Eq(t, evals[0].JobID, eval1.JobID)
	})
}

func TestStateStore_Evals(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	var evals []*structs.Evaluation

	for i := range 10 {
		eval := mock.Eval()
		evals = append(evals, eval)

		err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000+uint64(i), []*structs.Evaluation{eval})
		must.NoError(t, err)
	}

	ws := memdb.NewWatchSet()
	iter, err := state.Evals(ws, false)
	must.NoError(t, err)

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

	must.Eq(t, evals, out)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_EvalsByIDPrefix(t *testing.T) {
	ci.Parallel(t)

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
	for i := range 9 {
		eval := mock.Eval()
		eval.ID = ids[i]
		evals = append(evals, eval)
	}

	err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, evals)
	must.NoError(t, err)

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

	t.Run("list by prefix", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		iter, err := state.EvalsByIDPrefix(ws, structs.DefaultNamespace, "aaaa", SortDefault)
		must.NoError(t, err)

		got := []string{}
		for _, e := range gatherEvals(iter) {
			got = append(got, e.ID)
		}

		expected := []string{
			"aaaaaaaa-7bfb-395d-eb95-0685af2176b2",
			"aaaaaaab-7bfb-395d-eb95-0685af2176b2",
			"aaaaaabb-7bfb-395d-eb95-0685af2176b2",
			"aaaaabbb-7bfb-395d-eb95-0685af2176b2",
			"aaaabbbb-7bfb-395d-eb95-0685af2176b2",
		}
		must.Len(t, 5, got, must.Sprint("expected five evaluations"))
		must.Eq(t, expected, got) // Must be in this order.
	})

	t.Run("invalid prefix", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		iter, err := state.EvalsByIDPrefix(ws, structs.DefaultNamespace, "b-a7bfb", SortDefault)
		must.NoError(t, err)

		out := gatherEvals(iter)
		must.Len(t, 0, out, must.Sprint("expected zero evaluations"))
		must.False(t, watchFired(ws))
	})

	t.Run("reverse order", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		iter, err := state.EvalsByIDPrefix(ws, structs.DefaultNamespace, "aaaa", SortReverse)
		must.NoError(t, err)

		got := []string{}
		for _, e := range gatherEvals(iter) {
			got = append(got, e.ID)
		}

		expected := []string{
			"aaaabbbb-7bfb-395d-eb95-0685af2176b2",
			"aaaaabbb-7bfb-395d-eb95-0685af2176b2",
			"aaaaaabb-7bfb-395d-eb95-0685af2176b2",
			"aaaaaaab-7bfb-395d-eb95-0685af2176b2",
			"aaaaaaaa-7bfb-395d-eb95-0685af2176b2",
		}
		must.Len(t, 5, got, must.Sprint("expected five evaluations"))
		must.Eq(t, expected, got) // Must be in this order.
	})
}

func TestStateStore_EvalsByIDPrefix_Namespaces(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	eval1 := mock.Eval()
	eval1.ID = "aabbbbbb-7bfb-395d-eb95-0685af2176b2"
	eval2 := mock.Eval()
	eval2.ID = "aabbcbbb-7bfb-395d-eb95-0685af2176b2"
	sharedPrefix := "aabb"

	ns1 := mock.Namespace()
	ns1.Name = "namespace1"
	ns2 := mock.Namespace()
	ns2.Name = "namespace2"
	eval1.Namespace = ns1.Name
	eval2.Namespace = ns2.Name

	must.NoError(t, state.UpsertNamespaces(998, []*structs.Namespace{ns1, ns2}))
	must.NoError(t, state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{eval1, eval2}))

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

	ws := memdb.NewWatchSet()
	iter1, err := state.EvalsByIDPrefix(ws, ns1.Name, sharedPrefix, SortDefault)
	must.NoError(t, err)
	iter2, err := state.EvalsByIDPrefix(ws, ns2.Name, sharedPrefix, SortDefault)
	must.NoError(t, err)
	iter3, err := state.EvalsByIDPrefix(ws, structs.AllNamespacesSentinel, sharedPrefix, SortDefault)
	must.NoError(t, err)

	evalsNs1 := gatherEvals(iter1)
	evalsNs2 := gatherEvals(iter2)
	evalsNs3 := gatherEvals(iter3)
	must.Len(t, 1, evalsNs1)
	must.Len(t, 1, evalsNs2)
	must.Len(t, 2, evalsNs3)

	iter1, err = state.EvalsByIDPrefix(ws, ns1.Name, eval1.ID[:8], SortDefault)
	must.NoError(t, err)

	evalsNs1 = gatherEvals(iter1)
	must.Len(t, 1, evalsNs1)
	must.False(t, watchFired(ws))
}

func TestStateStore_EvalsRelatedToID(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	// Create sample evals.
	e1 := mock.Eval()
	e2 := mock.Eval()
	e3 := mock.Eval()
	e4 := mock.Eval()
	e5 := mock.Eval()
	e6 := mock.Eval()

	// Link evals.
	// This is not accurate for a real scenario, but it's helpful for testing
	// the general approach.
	//
	//   e1 -> e2 -> e3 -> e5
	//               └─-> e4 (blocked) -> e6
	e1.NextEval = e2.ID
	e2.PreviousEval = e1.ID

	e2.NextEval = e3.ID
	e3.PreviousEval = e2.ID

	e3.BlockedEval = e4.ID
	e4.PreviousEval = e3.ID

	e3.NextEval = e5.ID
	e5.PreviousEval = e3.ID

	e4.NextEval = e6.ID
	e6.PreviousEval = e4.ID

	// Create eval not in chain.
	e7 := mock.Eval()

	// Create eval with GC'ed related eval.
	e8 := mock.Eval()
	e8.NextEval = uuid.Generate()

	err := state.UpsertEvals(structs.MsgTypeTestSetup, 1000, []*structs.Evaluation{e1, e2, e3, e4, e5, e6, e7, e8})
	must.NoError(t, err)

	testCases := []struct {
		name     string
		id       string
		expected []string
	}{
		{
			name: "linear history",
			id:   e1.ID,
			expected: []string{
				e2.ID,
				e3.ID,
				e4.ID,
				e5.ID,
				e6.ID,
			},
		},
		{
			name: "linear history from middle",
			id:   e4.ID,
			expected: []string{
				e1.ID,
				e2.ID,
				e3.ID,
				e5.ID,
				e6.ID,
			},
		},
		{
			name:     "eval not in chain",
			id:       e7.ID,
			expected: []string{},
		},
		{
			name:     "eval with gc",
			id:       e8.ID,
			expected: []string{},
		},
		{
			name:     "non-existing eval",
			id:       uuid.Generate(),
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ws := memdb.NewWatchSet()
			related, err := state.EvalsRelatedToID(ws, tc.id)
			must.NoError(t, err)

			got := []string{}
			for _, e := range related {
				got = append(got, e.ID)
			}
			must.SliceContainsAll(t, tc.expected, got)
		})
	}

	t.Run("blocking query", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		_, err = state.EvalsRelatedToID(ws, e2.ID)
		must.NoError(t, err)

		// Update an eval off the chain and make sure watchset doesn't fire.
		e7.Status = structs.EvalStatusComplete
		state.UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{e7})
		must.False(t, watchFired(ws))

		// Update an eval in the chain and make sure watchset does fire.
		e3.Status = structs.EvalStatusComplete
		state.UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{e3})
		must.True(t, watchFired(ws))
	})
}

func TestStateStore_UpdateAllocsFromClient(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	node := mock.Node()
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 997, node))

	parent := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 998, nil, parent))

	child := mock.Job()
	child.Type = structs.JobTypeBatch
	child.Status = ""
	child.ParentID = parent.ID
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, child))

	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	alloc.JobID = child.ID
	alloc.Job = child
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc}))

	eval := mock.Eval()
	eval.Status = structs.EvalStatusComplete
	eval.JobID = child.ID
	must.NoError(t, state.UpsertEvals(structs.MsgTypeTestSetup, 1001, []*structs.Evaluation{eval}))

	ws := memdb.NewWatchSet()
	summary, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	must.NoError(t, err)
	must.NotNil(t, summary)
	must.Eq(t, parent.ID, summary.JobID)
	must.NotNil(t, summary.Children)
	must.Eq(t, 0, summary.Children.Pending)
	must.Eq(t, 1, summary.Children.Running)
	must.Eq(t, 0, summary.Children.Dead)

	// Create watchsets so we can test that update fires the watch
	ws = memdb.NewWatchSet()
	_, err = state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	must.NoError(t, err)

	// Create the delta updates
	ts := map[string]*structs.TaskState{"web": {State: structs.TaskStateRunning}}
	update := &structs.Allocation{
		ID:           alloc.ID,
		NodeID:       alloc.NodeID,
		ClientStatus: structs.AllocClientStatusComplete,
		TaskStates:   ts,
		JobID:        alloc.JobID,
		TaskGroup:    alloc.TaskGroup,
	}
	err = state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{update})
	must.NoError(t, err)

	must.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	summary, err = state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	must.NoError(t, err)
	must.NotNil(t, summary)
	must.Eq(t, parent.ID, summary.JobID)
	must.NotNil(t, summary.Children)
	must.Eq(t, 0, summary.Children.Pending)
	must.Eq(t, 0, summary.Children.Running)
	must.Eq(t, 1, summary.Children.Dead)

	must.False(t, watchFired(ws))
}

func TestStateStore_UpdateAllocsFromClient_ChildJob(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	node := mock.Node()

	alloc1 := mock.Alloc()
	alloc1.NodeID = node.ID

	alloc2 := mock.Alloc()
	alloc2.NodeID = node.ID

	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 998, node))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, alloc1.Job))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, alloc2.Job))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc1, alloc2}))

	// Create watchsets so we can test that update fires the watch
	watches := make([]memdb.WatchSet, 8)
	for i := range 8 {
		watches[i] = memdb.NewWatchSet()
	}
	_, err := state.AllocByID(watches[0], alloc1.ID)
	must.NoError(t, err)
	_, err = state.AllocByID(watches[1], alloc2.ID)
	must.NoError(t, err)

	_, err = state.AllocsByEval(watches[2], alloc1.EvalID)
	must.NoError(t, err)
	_, err = state.AllocsByEval(watches[3], alloc2.EvalID)
	must.NoError(t, err)

	_, err = state.AllocsByJob(watches[4], alloc1.Namespace, alloc1.JobID, false)
	must.NoError(t, err)
	_, err = state.AllocsByJob(watches[5], alloc2.Namespace, alloc2.JobID, false)
	must.NoError(t, err)

	_, err = state.AllocsByNode(watches[6], alloc1.NodeID)
	must.NoError(t, err)
	_, err = state.AllocsByNode(watches[7], alloc2.NodeID)
	must.NoError(t, err)

	// Create the delta updates
	ts := map[string]*structs.TaskState{"web": {State: structs.TaskStatePending}}
	update := &structs.Allocation{
		ID:           alloc1.ID,
		NodeID:       alloc1.NodeID,
		ClientStatus: structs.AllocClientStatusFailed,
		TaskStates:   ts,
		JobID:        alloc1.JobID,
		TaskGroup:    alloc1.TaskGroup,
	}
	update2 := &structs.Allocation{
		ID:           alloc2.ID,
		NodeID:       alloc2.NodeID,
		ClientStatus: structs.AllocClientStatusRunning,
		TaskStates:   ts,
		JobID:        alloc2.JobID,
		TaskGroup:    alloc2.TaskGroup,
	}

	err = state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{update, update2})
	must.NoError(t, err)

	for _, ws := range watches {
		must.True(t, watchFired(ws))
	}

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc1.ID)
	must.NoError(t, err)

	alloc1.CreateIndex = 1000
	alloc1.ModifyIndex = 1001
	alloc1.TaskStates = ts
	alloc1.ClientStatus = structs.AllocClientStatusFailed
	must.Eq(t, alloc1, out)

	out, err = state.AllocByID(ws, alloc2.ID)
	must.NoError(t, err)

	alloc2.ModifyIndex = 1000
	alloc2.ModifyIndex = 1001
	alloc2.ClientStatus = structs.AllocClientStatusRunning
	alloc2.TaskStates = ts
	must.Eq(t, alloc2, out)

	index, err := state.Index("allocs")
	must.NoError(t, err)
	must.Eq(t, 1001, index)

	// Ensure summaries have been updated
	summary, err := state.JobSummaryByID(ws, alloc1.Namespace, alloc1.JobID)
	must.NoError(t, err)

	tgSummary := summary.Summary["web"]
	must.Eq(t, 1, tgSummary.Failed)

	summary2, err := state.JobSummaryByID(ws, alloc2.Namespace, alloc2.JobID)
	must.NoError(t, err)

	tgSummary2 := summary2.Summary["web"]
	must.Eq(t, 1, tgSummary2.Running)

	must.False(t, watchFired(ws))
}

func TestStateStore_UpdateMultipleAllocsFromClient(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	node := mock.Node()

	alloc := mock.Alloc()
	alloc.NodeID = node.ID

	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 998, node))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, alloc.Job))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc}))

	// Create the delta updates
	ts := map[string]*structs.TaskState{"web": {State: structs.TaskStatePending}}
	update := &structs.Allocation{
		ID:           alloc.ID,
		NodeID:       alloc.NodeID,
		ClientStatus: structs.AllocClientStatusRunning,
		TaskStates:   ts,
		JobID:        alloc.JobID,
		TaskGroup:    alloc.TaskGroup,
	}
	update2 := &structs.Allocation{
		ID:           alloc.ID,
		NodeID:       alloc.NodeID,
		ClientStatus: structs.AllocClientStatusPending,
		TaskStates:   ts,
		JobID:        alloc.JobID,
		TaskGroup:    alloc.TaskGroup,
	}

	err := state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{update, update2})
	must.NoError(t, err)

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	must.NoError(t, err)

	alloc.CreateIndex = 1000
	alloc.ModifyIndex = 1001
	alloc.TaskStates = ts
	alloc.ClientStatus = structs.AllocClientStatusPending
	must.Eq(t, alloc, out)

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
	must.NoError(t, err)
	must.Eq(t, summary, expectedSummary)
}

func TestStateStore_UpdateAllocsFromClient_Deployment(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	node := mock.Node()

	alloc := mock.Alloc()
	now := time.Now()
	alloc.NodeID = node.ID
	alloc.CreateTime = now.UnixNano()

	pdeadline := 5 * time.Minute
	deployment := mock.Deployment()
	deployment.TaskGroups[alloc.TaskGroup].ProgressDeadline = pdeadline
	alloc.DeploymentID = deployment.ID

	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 998, node))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, alloc.Job))
	must.NoError(t, state.UpsertDeployment(1000, deployment))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}))

	healthy := now.Add(time.Second)
	update := &structs.Allocation{
		ID:           alloc.ID,
		NodeID:       alloc.NodeID,
		ClientStatus: structs.AllocClientStatusRunning,
		JobID:        alloc.JobID,
		TaskGroup:    alloc.TaskGroup,
		DeploymentStatus: &structs.AllocDeploymentStatus{
			Healthy:   pointer.Of(true),
			Timestamp: healthy,
		},
	}
	must.NoError(t, state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{update}))

	// Check that the deployment state was updated because the healthy
	// deployment
	dout, err := state.DeploymentByID(nil, deployment.ID)
	must.NoError(t, err)
	must.NotNil(t, dout)
	must.MapLen(t, 1, dout.TaskGroups)
	dstate := dout.TaskGroups[alloc.TaskGroup]
	must.NotNil(t, dstate)
	must.Eq(t, 1, dstate.PlacedAllocs)
	must.True(t, healthy.Add(pdeadline).Equal(dstate.RequireProgressBy))
}

// This tests that the deployment state is merged correctly
func TestStateStore_UpdateAllocsFromClient_DeploymentStateMerges(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	node := mock.Node()

	alloc := mock.Alloc()
	now := time.Now()
	alloc.NodeID = node.ID
	alloc.CreateTime = now.UnixNano()

	pdeadline := 5 * time.Minute
	deployment := mock.Deployment()
	deployment.TaskGroups[alloc.TaskGroup].ProgressDeadline = pdeadline
	alloc.DeploymentID = deployment.ID
	alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
		Canary: true,
	}

	must.False(t, alloc.DeploymentStatus.IsHealthy())

	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 998, node))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, alloc.Job))
	must.NoError(t, state.UpsertDeployment(1000, deployment))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}))

	// note this is the equivalent of the "stripped" alloc update that the
	// client sends
	update := &structs.Allocation{
		ID:           alloc.ID,
		NodeID:       alloc.NodeID,
		ClientStatus: structs.AllocClientStatusRunning,
		JobID:        alloc.JobID,
		TaskGroup:    alloc.TaskGroup,
		DeploymentStatus: &structs.AllocDeploymentStatus{
			Canary: false, // should not update
		},
	}
	must.NoError(t, state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{update}))

	// Check that the merging of the deployment status was correct
	out, err := state.AllocByID(nil, alloc.ID)
	must.NoError(t, err)
	must.NotNil(t, out)
	must.True(t, out.DeploymentStatus.Canary)

	update = update.Copy()
	update.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(true), // should update
		Canary:  false,            // should not update
	}
	must.NoError(t, state.UpdateAllocsFromClient(
		structs.MsgTypeTestSetup, 1010, []*structs.Allocation{update}))

	out, err = state.AllocByID(nil, alloc.ID)
	must.NoError(t, err)
	must.NotNil(t, out)
	must.True(t, out.DeploymentStatus.Canary)
	must.NotNil(t, out.DeploymentStatus.Healthy)
	must.True(t, *out.DeploymentStatus.Healthy)

	d, err := state.DeploymentByID(nil, deployment.ID)
	must.NoError(t, err)
	must.Eq(t, 1, d.TaskGroups[alloc.TaskGroup].HealthyAllocs)
}

// TestStateStore_UpdateAllocsFromClient_UpdateNodes verifies that the relevant
// node data is updated when clients update their allocs.
func TestStateStore_UpdateAllocsFromClient_UpdateNodes(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	node1 := mock.Node()
	alloc1 := mock.Alloc()
	alloc1.NodeID = node1.ID

	node2 := mock.Node()
	alloc2 := mock.Alloc()
	alloc2.NodeID = node2.ID

	node3 := mock.Node()
	alloc3 := mock.Alloc()
	alloc3.NodeID = node3.ID

	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1000, node1))
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1001, node2))
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1002, node3))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1003, nil, alloc1.Job))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1004, nil, alloc2.Job))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1005, []*structs.Allocation{alloc1, alloc2, alloc3}))

	// Create watches to make sure they fire when nodes are updated.
	ws1 := memdb.NewWatchSet()
	_, err := state.NodeByID(ws1, node1.ID)
	must.NoError(t, err)

	ws2 := memdb.NewWatchSet()
	_, err = state.NodeByID(ws2, node2.ID)
	must.NoError(t, err)

	ws3 := memdb.NewWatchSet()
	_, err = state.NodeByID(ws3, node3.ID)
	must.NoError(t, err)

	// Create and apply alloc updates.
	// Don't update alloc 3.
	updateAlloc1 := &structs.Allocation{
		ID:           alloc1.ID,
		NodeID:       alloc1.NodeID,
		ClientStatus: structs.AllocClientStatusRunning,
		JobID:        alloc1.JobID,
		TaskGroup:    alloc1.TaskGroup,
	}
	updateAlloc2 := &structs.Allocation{
		ID:           alloc2.ID,
		NodeID:       alloc2.NodeID,
		ClientStatus: structs.AllocClientStatusRunning,
		JobID:        alloc2.JobID,
		TaskGroup:    alloc2.TaskGroup,
	}
	updateAllocNonExisting := &structs.Allocation{
		ID:           uuid.Generate(),
		NodeID:       uuid.Generate(),
		ClientStatus: structs.AllocClientStatusRunning,
		JobID:        uuid.Generate(),
		TaskGroup:    "group",
	}

	err = state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1005, []*structs.Allocation{
		updateAlloc1, updateAlloc2, updateAllocNonExisting,
	})
	must.NoError(t, err)

	// Check that node update watches fired.
	must.True(t, watchFired(ws1))
	must.True(t, watchFired(ws2))

	// Check that node LastAllocUpdateIndex were updated.
	ws := memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node1.ID)
	must.NoError(t, err)
	must.NotNil(t, out)
	must.Eq(t, 1005, out.LastAllocUpdateIndex)
	must.False(t, watchFired(ws))

	out, err = state.NodeByID(ws, node2.ID)
	must.NoError(t, err)
	must.NotNil(t, out)
	must.Eq(t, 1005, out.LastAllocUpdateIndex)
	must.False(t, watchFired(ws))

	// Node 3 should not be updated.
	out, err = state.NodeByID(ws, node3.ID)
	must.NoError(t, err)
	must.NotNil(t, out)
	must.Eq(t, 0, out.LastAllocUpdateIndex)
	must.False(t, watchFired(ws))
}

func TestStateStore_UpsertAlloc_Alloc(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	alloc := mock.Alloc()

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, alloc.Job))

	// Create watchsets so we can test that update fires the watch
	watches := make([]memdb.WatchSet, 4)
	for i := range 4 {
		watches[i] = memdb.NewWatchSet()
	}
	_, err := state.AllocByID(watches[0], alloc.ID)
	must.NoError(t, err)
	_, err = state.AllocsByEval(watches[1], alloc.EvalID)
	must.NoError(t, err)
	_, err = state.AllocsByJob(watches[2], alloc.Namespace, alloc.JobID, false)
	must.NoError(t, err)
	_, err = state.AllocsByNode(watches[3], alloc.NodeID)
	must.NoError(t, err)

	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc})
	must.NoError(t, err)

	for _, ws := range watches {
		must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))
	}

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	must.NoError(t, err)
	must.Eq(t, alloc, out)

	index, err := state.Index("allocs")
	must.NoError(t, err)
	must.Eq(t, 1000, index)

	summary, err := state.JobSummaryByID(ws, alloc.Namespace, alloc.JobID)
	must.NoError(t, err)

	tgSummary, ok := summary.Summary["web"]
	if !ok {
		t.Fatalf("no summary for task group web")
	}
	if tgSummary.Starting != 1 {
		t.Fatalf("expected queued: %v, actual: %v", 1, tgSummary.Starting)
	}

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_UpsertAlloc_Deployment(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	alloc := mock.Alloc()
	now := time.Now()
	alloc.CreateTime = now.UnixNano()
	alloc.ModifyTime = now.UnixNano()
	pdeadline := 5 * time.Minute
	deployment := mock.Deployment()
	deployment.TaskGroups[alloc.TaskGroup].ProgressDeadline = pdeadline
	alloc.DeploymentID = deployment.ID

	must.Nil(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, alloc.Job))
	must.Nil(t, state.UpsertDeployment(1000, deployment))

	// Create a watch set so we can test that update fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.AllocsByDeployment(ws, alloc.DeploymentID)
	must.NoError(t, err)

	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc})
	must.NoError(t, err)
	must.True(t, watchFired(ws), must.Sprint("watch not fired"))

	ws = memdb.NewWatchSet()
	allocs, err := state.AllocsByDeployment(ws, alloc.DeploymentID)
	must.NoError(t, err)
	must.Len(t, 1, allocs)
	must.Eq(t, alloc, allocs[0])

	index, err := state.Index("allocs")
	must.NoError(t, err)
	must.Eq(t, 1001, index)
	must.False(t, watchFired(ws), must.Sprint("watch was fired"))

	// Check that the deployment state was updated
	dout, err := state.DeploymentByID(nil, deployment.ID)
	must.NoError(t, err)
	must.NotNil(t, dout)
	must.MapLen(t, 1, dout.TaskGroups)
	dstate := dout.TaskGroups[alloc.TaskGroup]
	must.NotNil(t, dstate)
	must.Eq(t, 1, dstate.PlacedAllocs)
	must.True(t, now.Add(pdeadline).Equal(dstate.RequireProgressBy))
}

func TestStateStore_UpsertAlloc_AllocsByNamespace(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	ns1 := mock.Namespace()
	ns1.Name = "namespaced"
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc1.Namespace = ns1.Name
	alloc1.Job.Namespace = ns1.Name
	alloc2.Namespace = ns1.Name
	alloc2.Job.Namespace = ns1.Name

	ns2 := mock.Namespace()
	ns2.Name = "new-namespace"
	alloc3 := mock.Alloc()
	alloc4 := mock.Alloc()
	alloc3.Namespace = ns2.Name
	alloc3.Job.Namespace = ns2.Name
	alloc4.Namespace = ns2.Name
	alloc4.Job.Namespace = ns2.Name

	must.NoError(t, state.UpsertNamespaces(998, []*structs.Namespace{ns1, ns2}))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, alloc1.Job))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, alloc2.Job))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, alloc3.Job))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1002, nil, alloc4.Job))

	// Create watchsets so we can test that update fires the watch
	watches := []memdb.WatchSet{memdb.NewWatchSet(), memdb.NewWatchSet()}
	_, err := state.AllocsByNamespace(watches[0], ns1.Name)
	must.NoError(t, err)
	_, err = state.AllocsByNamespace(watches[1], ns2.Name)
	must.NoError(t, err)

	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc1, alloc2, alloc3, alloc4}))
	must.True(t, watchFired(watches[0]))
	must.True(t, watchFired(watches[1]))

	ws := memdb.NewWatchSet()
	iter1, err := state.AllocsByNamespace(ws, ns1.Name)
	must.NoError(t, err)
	iter2, err := state.AllocsByNamespace(ws, ns2.Name)
	must.NoError(t, err)

	var out1 []*structs.Allocation
	for {
		raw := iter1.Next()
		if raw == nil {
			break
		}
		out1 = append(out1, raw.(*structs.Allocation))
	}

	var out2 []*structs.Allocation
	for {
		raw := iter2.Next()
		if raw == nil {
			break
		}
		out2 = append(out2, raw.(*structs.Allocation))
	}

	must.Len(t, 2, out1)
	must.Len(t, 2, out2)

	for _, alloc := range out1 {
		must.Eq(t, ns1.Name, alloc.Namespace)
	}
	for _, alloc := range out2 {
		must.Eq(t, ns2.Name, alloc.Namespace)
	}

	index, err := state.Index("allocs")
	must.NoError(t, err)
	must.Eq(t, 1001, index)
	must.False(t, watchFired(ws))
}

// Testing to ensure we keep issue
// https://github.com/hashicorp/nomad/issues/2583 fixed
func TestStateStore_UpsertAlloc_No_Job(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	alloc := mock.Alloc()
	alloc.Job = nil

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 999, []*structs.Allocation{alloc})
	if err == nil || !strings.Contains(err.Error(), "without a job") {
		t.Fatalf("expect err: %v", err)
	}
}

func TestStateStore_UpsertAlloc_StickyVolumes(t *testing.T) {
	ci.Parallel(t)

	store := testStateStore(t)

	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
	}

	hostVolCapsReadWrite := []*structs.HostVolumeCapability{
		{
			AttachmentMode: structs.HostVolumeAttachmentModeFilesystem,
			AccessMode:     structs.HostVolumeAccessModeSingleNodeReader,
		},
		{
			AttachmentMode: structs.HostVolumeAttachmentModeFilesystem,
			AccessMode:     structs.HostVolumeAccessModeSingleNodeWriter,
		},
	}
	dhv := &structs.HostVolume{
		Namespace:             structs.DefaultNamespace,
		ID:                    uuid.Generate(),
		Name:                  "foo",
		NodeID:                nodes[1].ID,
		RequestedCapabilities: hostVolCapsReadWrite,
		State:                 structs.HostVolumeStateReady,
	}

	nodes[0].HostVolumes = map[string]*structs.ClientHostVolumeConfig{}
	nodes[1].HostVolumes = map[string]*structs.ClientHostVolumeConfig{"foo": {ID: dhv.ID, Name: dhv.Name}}

	stickyRequest := map[string]*structs.VolumeRequest{
		"foo": {
			Type:           "host",
			Source:         "foo",
			Sticky:         true,
			AccessMode:     structs.CSIVolumeAccessModeSingleNodeWriter,
			AttachmentMode: structs.CSIVolumeAttachmentModeFilesystem,
		},
	}

	for _, node := range nodes {
		must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 1000, node))
	}

	stickyJob := mock.Job()
	stickyJob.TaskGroups[0].Volumes = stickyRequest

	existingClaim := &structs.TaskGroupHostVolumeClaim{
		ID:            uuid.Generate(),
		Namespace:     structs.DefaultNamespace,
		JobID:         stickyJob.ID,
		TaskGroupName: stickyJob.TaskGroups[0].Name,
		VolumeID:      dhv.ID,
		VolumeName:    dhv.Name,
	}
	must.NoError(t, store.UpsertTaskGroupHostVolumeClaim(structs.MsgTypeTestSetup, 1000, existingClaim))

	allocWithClaimedVol := mock.AllocForNode(nodes[1])
	allocWithClaimedVol.Namespace = structs.DefaultNamespace
	allocWithClaimedVol.JobID = stickyJob.ID
	allocWithClaimedVol.Job = stickyJob
	allocWithClaimedVol.NodeID = nodes[1].ID

	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, allocWithClaimedVol.Job))
	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{allocWithClaimedVol}))

	// there must be exactly one claim in the state
	claims := []*structs.TaskGroupHostVolumeClaim{}
	iter, err := store.TaskGroupHostVolumeClaimsByFields(nil, TgvcSearchableFields{
		Namespace:     structs.DefaultNamespace,
		JobID:         stickyJob.ID,
		TaskGroupName: stickyJob.TaskGroups[0].Name,
	})
	must.NoError(t, err)
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		claim := raw.(*structs.TaskGroupHostVolumeClaim)
		claims = append(claims, claim)
	}
	must.Len(t, 1, claims)

	// clean up the state
	txn := store.db.WriteTxn(1000)
	_, err = txn.DeletePrefix(TableTaskGroupHostVolumeClaim, "id_prefix", stickyJob.ID)
	must.NoError(t, err)
	must.NoError(t, store.deleteAllocsForJobTxn(txn, 1000, structs.DefaultNamespace, stickyJob.ID))
	must.NoError(t, txn.Commit())

	// try to upsert an alloc for which there is no existing claim
	stickyJob2 := mock.Job()
	stickyJob2.TaskGroups[0].Volumes = stickyRequest
	allocWithNoClaimedVol := mock.AllocForNode(nodes[1])
	allocWithNoClaimedVol.Namespace = structs.DefaultNamespace
	allocWithNoClaimedVol.JobID = stickyJob2.ID
	allocWithNoClaimedVol.Job = stickyJob2
	allocWithNoClaimedVol.NodeID = nodes[1].ID

	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, allocWithNoClaimedVol.Job))
	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{allocWithNoClaimedVol}))

	// make sure we recorded a claim
	claim, err := store.GetTaskGroupHostVolumeClaim(nil, structs.DefaultNamespace, stickyJob2.ID, stickyJob2.TaskGroups[0].Name, dhv.ID)
	must.NoError(t, err)
	must.Eq(t, claim.Namespace, structs.DefaultNamespace)
	must.Eq(t, claim.JobID, stickyJob2.ID)
	must.Eq(t, claim.TaskGroupName, stickyJob2.TaskGroups[0].Name)
}

func TestStateStore_UpsertAlloc_ChildJob(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	parent := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 998, nil, parent))

	child := mock.Job()
	child.Status = ""
	child.ParentID = parent.ID

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, child))

	alloc := mock.Alloc()
	alloc.JobID = child.ID
	alloc.Job = child

	// Create watchsets so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	must.NoError(t, err)

	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc})
	must.NoError(t, err)

	must.True(t, watchFired(ws))

	ws = memdb.NewWatchSet()
	summary, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID)
	must.NoError(t, err)
	must.NotNil(t, summary)

	must.Eq(t, parent.ID, summary.JobID)
	must.NotNil(t, summary.Children)

	must.Eq(t, int64(0), summary.Children.Pending)
	must.Eq(t, int64(1), summary.Children.Running)
	must.Eq(t, int64(0), summary.Children.Dead)

	must.False(t, watchFired(ws))
}

func TestStateStore_UpsertAlloc_NextAllocation(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc2.PreviousAllocation = alloc1.ID

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 900, nil, alloc1.Job))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 901, nil, alloc2.Job))

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc1, alloc2})
	must.NoError(t, err)

	// alloc1 should have the correct NextAllocation
	actual, err := state.AllocByID(nil, alloc1.ID)
	must.Eq(t, actual.NextAllocation, alloc2.ID)

	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc2, alloc1})
	must.NoError(t, err)

	// upsert in a different order, alloc1 should still have the correct NextAllocation
	actual, err = state.AllocByID(nil, alloc1.ID)
	must.NoError(t, err)
	must.Eq(t, actual.NextAllocation, alloc2.ID)
}

func TestStateStore_UpdateAlloc_Alloc(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	alloc := mock.Alloc()

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, alloc.Job))

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc})
	must.NoError(t, err)

	ws := memdb.NewWatchSet()
	summary, err := state.JobSummaryByID(ws, alloc.Namespace, alloc.JobID)
	must.NoError(t, err)
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
	for i := range 4 {
		watches[i] = memdb.NewWatchSet()
	}
	_, err = state.AllocByID(watches[0], alloc2.ID)
	must.NoError(t, err)
	_, err = state.AllocsByEval(watches[1], alloc2.EvalID)
	must.NoError(t, err)
	_, err = state.AllocsByJob(watches[2], alloc2.Namespace, alloc2.JobID, false)
	must.NoError(t, err)
	_, err = state.AllocsByNode(watches[3], alloc2.NodeID)
	must.NoError(t, err)

	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{alloc2})
	must.NoError(t, err)

	for _, ws := range watches {
		must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))
	}

	ws = memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	must.NoError(t, err)

	must.Eq(t, alloc2, out)

	must.Eq(t, 1000, out.CreateIndex)
	must.Eq(t, 1002, out.ModifyIndex)

	index, err := state.Index("allocs")
	must.NoError(t, err)
	must.Eq(t, 1002, index)

	// Ensure that summary hasb't changed
	summary, err = state.JobSummaryByID(ws, alloc.Namespace, alloc.JobID)
	must.NoError(t, err)
	tgSummary = summary.Summary["web"]
	if tgSummary.Starting != 1 {
		t.Fatalf("expected starting: %v, actual: %v", 1, tgSummary.Starting)
	}

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

// This test ensures that the state store will mark the clients status as lost
// when set rather than preferring the existing status.
func TestStateStore_UpdateAlloc_Lost(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	alloc := mock.Alloc()
	alloc.ClientStatus = "foo"

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, alloc.Job))

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc})
	must.NoError(t, err)

	alloc2 := new(structs.Allocation)
	*alloc2 = *alloc
	alloc2.ClientStatus = structs.AllocClientStatusLost
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc2}))

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc2.ID)
	must.NoError(t, err)

	if out.ClientStatus != structs.AllocClientStatusLost {
		t.Fatalf("bad: %#v", out)
	}
}

// TestStateStore_UpdateAlloc_JobPurge tests that updating an allocation after
// its job has been purged does not recreate the allocation in state.
func TestStateStore_UpdateAlloc_JobPurge(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	alloc := mock.Alloc()

	// Upsert a job.
	must.NoError(t, state.UpsertJobSummary(998, mock.JobSummary(alloc.JobID)))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, alloc.Job))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc}))

	// The DeleteJob function calls the DeleteJobTxn method, which is called
	// when a job is purged and is the only caller. Here we simulate the job
	// being purged from the state which will remove the job from the state and
	// all associated allocations.
	must.NoError(t, state.DeleteJob(1001, alloc.Namespace, alloc.JobID))

	// Update the client state of the allocation to complete and use the state
	// function which is triggered by the client updater process RPC call. This
	// should no return an error, but also not re-create the allocation in
	// state as it has already been removed by the purge.
	allocCopy1 := alloc.Copy()
	allocCopy1.ClientStatus = structs.AllocClientStatusComplete
	must.NoError(t, state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{allocCopy1}))

	// Ensure the client allocation update was a noop due to the job being
	// purged.
	out, err := state.AllocByID(nil, alloc.ID)
	must.NoError(t, err)
	must.Nil(t, out)
}

func TestStateStore_UpdateAllocDesiredTransition(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	alloc := mock.Alloc()

	must.Nil(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, alloc.Job))
	must.Nil(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc}))

	t1 := &structs.DesiredTransition{
		Migrate: pointer.Of(true),
	}
	t2 := &structs.DesiredTransition{
		Migrate: pointer.Of(false),
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
	must.Nil(t, state.UpdateAllocsDesiredTransitions(structs.MsgTypeTestSetup, 1001, m, evals))

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	must.NoError(t, err)
	must.NotNil(t, out.DesiredTransition.Migrate)
	must.True(t, *out.DesiredTransition.Migrate)
	must.Eq(t, 1000, out.CreateIndex)
	must.Eq(t, 1001, out.ModifyIndex)

	index, err := state.Index("allocs")
	must.NoError(t, err)
	must.Eq(t, 1001, index)

	// Check the eval is created
	eout, err := state.EvalByID(nil, eval.ID)
	must.NoError(t, err)
	must.NotNil(t, eout)

	m = map[string]*structs.DesiredTransition{alloc.ID: t2}
	must.Nil(t, state.UpdateAllocsDesiredTransitions(structs.MsgTypeTestSetup, 1002, m, evals))

	ws = memdb.NewWatchSet()
	out, err = state.AllocByID(ws, alloc.ID)
	must.NoError(t, err)
	must.NotNil(t, out.DesiredTransition.Migrate)
	must.False(t, *out.DesiredTransition.Migrate)
	must.Eq(t, 1000, out.CreateIndex)
	must.Eq(t, 1002, out.ModifyIndex)

	index, err = state.Index("allocs")
	must.NoError(t, err)
	must.Eq(t, 1002, index)

	// Try with a bogus alloc id
	m = map[string]*structs.DesiredTransition{uuid.Generate(): t2}
	must.Nil(t, state.UpdateAllocsDesiredTransitions(structs.MsgTypeTestSetup, 1003, m, evals))
}

func TestStateStore_JobSummary(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	// Add a job
	job := mock.Job()
	state.UpsertJob(structs.MsgTypeTestSetup, 900, nil, job)

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

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

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
	must.Eq(t, &expectedSummary, summary)

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
	state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job1)
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
	must.Eq(t, &expectedSummary, summary)
}

func TestStateStore_ReconcileJobSummary(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	// Create an alloc
	alloc := mock.Alloc()

	// Add another task group to the job
	tg2 := alloc.Job.TaskGroups[0].Copy()
	tg2.Name = "db"
	alloc.Job.TaskGroups = append(alloc.Job.TaskGroups, tg2)
	state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, alloc.Job)

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

	alloc12 := mock.Alloc()
	alloc12.JobID = alloc.JobID
	alloc12.Job = alloc.Job
	alloc12.TaskGroup = "db"
	alloc12.ClientStatus = structs.AllocClientStatusUnknown

	state.UpsertAllocs(structs.MsgTypeTestSetup, 130, []*structs.Allocation{alloc4, alloc6, alloc8, alloc10, alloc12})

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
				Unknown:  1,
			},
		},
		CreateIndex: 100,
		ModifyIndex: 120,
	}
	must.Eq(t, &expectedSummary, summary)
}

func TestStateStore_ReconcileParentJobSummary(t *testing.T) {
	ci.Parallel(t)

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
	state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job1)

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

	must.Nil(t, state.UpsertJob(structs.MsgTypeTestSetup, 110, nil, childJob))
	must.Nil(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 111, []*structs.Allocation{alloc, alloc2}))

	// Make the summary incorrect in the state store
	summary, err := state.JobSummaryByID(nil, job1.Namespace, job1.ID)
	must.NoError(t, err)

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
	must.Eq(t, &expectedSummary, summary)

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
	must.Eq(t, &expectedChildSummary, childSummary)
}

func TestStateStore_UpdateAlloc_JobNotPresent(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	alloc := mock.Alloc()
	state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, alloc.Job)
	state.UpsertAllocs(structs.MsgTypeTestSetup, 200, []*structs.Allocation{alloc})

	// Delete the job
	state.DeleteJob(300, alloc.Namespace, alloc.Job.ID)

	// Update the alloc
	alloc1 := alloc.Copy()
	alloc1.ClientStatus = structs.AllocClientStatusRunning

	// Updating allocation should not throw any error
	must.NoError(t, state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 400, []*structs.Allocation{alloc1}))

	// Re-Register the job
	state.UpsertJob(structs.MsgTypeTestSetup, 500, nil, alloc.Job)

	// Update the alloc again
	alloc2 := alloc.Copy()
	alloc2.ClientStatus = structs.AllocClientStatusComplete
	must.NoError(t, state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 400, []*structs.Allocation{alloc1}))

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
	must.Eq(t, &expectedSummary, summary)
}

func TestStateStore_EvictAlloc_Alloc(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	alloc := mock.Alloc()

	state.UpsertJobSummary(998, mock.JobSummary(alloc.JobID))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, alloc.Job))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc}))

	alloc2 := new(structs.Allocation)
	*alloc2 = *alloc
	alloc2.DesiredStatus = structs.AllocDesiredStatusEvict
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc2}))

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	must.NoError(t, err)

	must.Eq(t, structs.AllocDesiredStatusEvict, out.DesiredStatus,
		must.Sprintf("bad: %#v %#v", alloc, out))

	index, err := state.Index("allocs")
	must.NoError(t, err)
	must.Eq(t, 1001, index)
}

func TestStateStore_AllocsByNode(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	var allocs []*structs.Allocation

	for range 10 {
		alloc := mock.Alloc()
		alloc.NodeID = "foo"
		allocs = append(allocs, alloc)
	}

	for idx, alloc := range allocs {
		must.NoError(t, state.UpsertJobSummary(uint64(900+idx), mock.JobSummary(alloc.JobID)))
		must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, uint64(900+idx), nil, alloc.Job))
	}

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs)
	must.NoError(t, err)

	ws := memdb.NewWatchSet()
	out, err := state.AllocsByNode(ws, "foo")
	must.NoError(t, err)

	sort.Sort(AllocIDSort(allocs))
	sort.Sort(AllocIDSort(out))

	must.Eq(t, allocs, out)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_AllocsByNodeTerminal(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	var allocs, term, nonterm []*structs.Allocation

	for i := range 10 {
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
		must.NoError(t, state.UpsertJobSummary(uint64(900+idx), mock.JobSummary(alloc.JobID)))
		must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, uint64(900+idx), nil, alloc.Job))
	}

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs)
	must.NoError(t, err)

	// Verify the terminal allocs
	ws := memdb.NewWatchSet()
	out, err := state.AllocsByNodeTerminal(ws, "foo", true)
	must.NoError(t, err)

	sort.Sort(AllocIDSort(term))
	sort.Sort(AllocIDSort(out))

	must.Eq(t, term, out)

	// Verify the non-terminal allocs
	out, err = state.AllocsByNodeTerminal(ws, "foo", false)
	must.NoError(t, err)

	sort.Sort(AllocIDSort(nonterm))
	sort.Sort(AllocIDSort(out))

	must.Eq(t, nonterm, out)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_AllocsByJob(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	var allocs []*structs.Allocation

	for range 10 {
		alloc := mock.Alloc()
		alloc.JobID = "foo"
		alloc.Job.ID = "foo"
		allocs = append(allocs, alloc)
	}

	for i, alloc := range allocs {
		must.NoError(t, state.UpsertJobSummary(uint64(900+i), mock.JobSummary(alloc.JobID)))
		must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, uint64(900+i), nil, alloc.Job))
	}

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs)
	must.NoError(t, err)

	ws := memdb.NewWatchSet()
	out, err := state.AllocsByJob(ws, mock.Alloc().Namespace, "foo", false)
	must.NoError(t, err)

	sort.Sort(AllocIDSort(allocs))
	sort.Sort(AllocIDSort(out))

	must.Eq(t, allocs, out)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_AllocsForRegisteredJob(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	var allocs []*structs.Allocation
	var allocs1 []*structs.Allocation

	job := mock.Job()
	job.ID = "foo"
	state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job)
	for range 3 {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		allocs = append(allocs, alloc)
	}
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 200, allocs))

	must.NoError(t, state.DeleteJob(250, job.Namespace, job.ID))

	job1 := mock.Job()
	job1.ID = "foo"
	job1.CreateIndex = 50
	state.UpsertJob(structs.MsgTypeTestSetup, 300, nil, job1)
	for range 4 {
		alloc := mock.Alloc()
		alloc.Job = job1
		alloc.JobID = job1.ID
		allocs1 = append(allocs1, alloc)
	}

	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs1))

	ws := memdb.NewWatchSet()
	out, err := state.AllocsByJob(ws, job1.Namespace, job1.ID, true)
	must.NoError(t, err)

	expected := len(allocs1) // state.DeleteJob corresponds to stop -purge, so all allocs from the original job should be gone
	if len(out) != expected {
		t.Fatalf("expected: %v, actual: %v", expected, len(out))
	}

	out1, err := state.AllocsByJob(ws, job1.Namespace, job1.ID, false)
	must.NoError(t, err)

	expected = len(allocs1)
	if len(out1) != expected {
		t.Fatalf("expected: %v, actual: %v", expected, len(out1))
	}

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_AllocsByIDPrefix(t *testing.T) {
	ci.Parallel(t)

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
	for i := range 9 {
		alloc := mock.Alloc()
		alloc.ID = ids[i]
		allocs = append(allocs, alloc)
	}

	for i, alloc := range allocs {
		must.NoError(t, state.UpsertJobSummary(uint64(900+i), mock.JobSummary(alloc.JobID)))
		must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, uint64(900+i), nil, alloc.Job))
	}

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs)
	must.NoError(t, err)

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

	t.Run("allocs by prefix", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		iter, err := state.AllocsByIDPrefix(ws, structs.DefaultNamespace, "aaaa", SortDefault)
		must.NoError(t, err)

		out := gatherAllocs(iter)
		must.Len(t, 5, out, must.Sprint("expected five allocations"))

		got := []string{}
		for _, a := range out {
			got = append(got, a.ID)
		}
		expected := []string{
			"aaaaaaaa-7bfb-395d-eb95-0685af2176b2",
			"aaaaaaab-7bfb-395d-eb95-0685af2176b2",
			"aaaaaabb-7bfb-395d-eb95-0685af2176b2",
			"aaaaabbb-7bfb-395d-eb95-0685af2176b2",
			"aaaabbbb-7bfb-395d-eb95-0685af2176b2",
		}
		must.Eq(t, expected, got)
		must.False(t, watchFired(ws))
	})

	t.Run("invalid prefix", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		iter, err := state.AllocsByIDPrefix(ws, structs.DefaultNamespace, "b-a7bfb", SortDefault)
		must.NoError(t, err)

		out := gatherAllocs(iter)
		must.Len(t, 0, out)
		must.False(t, watchFired(ws))
	})

	t.Run("reverse", func(t *testing.T) {
		ws := memdb.NewWatchSet()
		iter, err := state.AllocsByIDPrefix(ws, structs.DefaultNamespace, "aaaa", SortReverse)
		must.NoError(t, err)

		out := gatherAllocs(iter)
		must.Len(t, 5, out, must.Sprint("expected five allocations"))

		got := []string{}
		for _, a := range out {
			got = append(got, a.ID)
		}
		expected := []string{
			"aaaabbbb-7bfb-395d-eb95-0685af2176b2",
			"aaaaabbb-7bfb-395d-eb95-0685af2176b2",
			"aaaaaabb-7bfb-395d-eb95-0685af2176b2",
			"aaaaaaab-7bfb-395d-eb95-0685af2176b2",
			"aaaaaaaa-7bfb-395d-eb95-0685af2176b2",
		}
		must.Eq(t, expected, got)
		must.False(t, watchFired(ws))
	})
}

func TestStateStore_AllocsByIDPrefix_Namespaces(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	alloc1 := mock.Alloc()
	alloc1.ID = "aabbbbbb-7bfb-395d-eb95-0685af2176b2"
	alloc2 := mock.Alloc()
	alloc2.ID = "aabbcbbb-7bfb-395d-eb95-0685af2176b2"
	sharedPrefix := "aabb"

	ns1 := mock.Namespace()
	ns1.Name = "namespace1"
	ns2 := mock.Namespace()
	ns2.Name = "namespace2"

	alloc1.Namespace = ns1.Name
	alloc1.Job.Namespace = ns1.Name
	alloc2.Namespace = ns2.Name
	alloc2.Job.Namespace = ns2.Name

	must.NoError(t, state.UpsertNamespaces(998, []*structs.Namespace{ns1, ns2}))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, alloc1.Job))
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, alloc2.Job))
	must.NoError(t, state.UpsertAllocs(
		structs.MsgTypeTestSetup, 1000, []*structs.Allocation{alloc1, alloc2}))

	gatherAllocs := func(iter memdb.ResultIterator) []*structs.Allocation {
		var allocs []*structs.Allocation
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			alloc := raw.(*structs.Allocation)
			allocs = append(allocs, alloc)
		}
		return allocs
	}

	ws := memdb.NewWatchSet()
	iter1, err := state.AllocsByIDPrefix(ws, ns1.Name, sharedPrefix, SortDefault)
	must.NoError(t, err)
	iter2, err := state.AllocsByIDPrefix(ws, ns2.Name, sharedPrefix, SortDefault)
	must.NoError(t, err)

	allocsNs1 := gatherAllocs(iter1)
	allocsNs2 := gatherAllocs(iter2)
	must.Len(t, 1, allocsNs1)
	must.Len(t, 1, allocsNs2)

	iter1, err = state.AllocsByIDPrefix(ws, ns1.Name, alloc1.ID[:8], SortDefault)
	must.NoError(t, err)

	allocsNs1 = gatherAllocs(iter1)
	must.Len(t, 1, allocsNs1)
	must.False(t, watchFired(ws))
}

func TestStateStore_Allocs(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	var allocs []*structs.Allocation

	for range 10 {
		alloc := mock.Alloc()
		allocs = append(allocs, alloc)
	}
	for i, alloc := range allocs {
		must.NoError(t, state.UpsertJobSummary(uint64(900+i), mock.JobSummary(alloc.JobID)))
		must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, uint64(900+i), nil, alloc.Job))
	}

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs)
	must.NoError(t, err)

	ws := memdb.NewWatchSet()
	iter, err := state.Allocs(ws, SortDefault)
	must.NoError(t, err)

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

	must.Eq(t, allocs, out)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_Allocs_PrevAlloc(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	var allocs []*structs.Allocation

	for range 5 {
		alloc := mock.Alloc()
		allocs = append(allocs, alloc)
	}
	for i, alloc := range allocs {
		must.NoError(t, state.UpsertJobSummary(uint64(900+i), mock.JobSummary(alloc.JobID)))
		must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, uint64(900+i), nil, alloc.Job))
	}
	// Set some previous alloc ids
	allocs[1].PreviousAllocation = allocs[0].ID
	allocs[2].PreviousAllocation = allocs[1].ID

	err := state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, allocs)
	must.NoError(t, err)

	ws := memdb.NewWatchSet()
	iter, err := state.Allocs(ws, SortDefault)
	must.NoError(t, err)

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

	must.Eq(t, allocs, out)
	must.False(t, watchFired(ws))

	// Insert another alloc, verify index of previous alloc also got updated
	alloc := mock.Alloc()
	alloc.Job = allocs[0].Job
	alloc.JobID = allocs[0].JobID
	alloc.PreviousAllocation = allocs[0].ID
	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc})
	must.NoError(t, err)
	alloc0, err := state.AllocByID(nil, allocs[0].ID)
	must.NoError(t, err)
	must.Eq(t, alloc0.ModifyIndex, uint64(1001))
}

func TestStateStore_SetJobStatus_ForceStatus(t *testing.T) {
	ci.Parallel(t)

	index := uint64(0)
	state := testStateStore(t)
	txn := state.db.WriteTxn(index)

	// Create and insert a mock job.
	job := mock.Job()
	job.Status = ""
	job.ModifyIndex = index
	must.NoError(t, txn.Insert("jobs", job))

	exp := "foobar"
	index = uint64(1000)
	must.NoError(t, state.setJobStatus(index, txn, job, false, exp))

	i, err := txn.First("jobs", "id", job.Namespace, job.ID)
	must.NoError(t, err)
	updated := i.(*structs.Job)

	must.Eq(t, exp, updated.Status)
	must.Eq(t, index, updated.ModifyIndex)
}

func TestStateStore_SetJobStatus_NoOp(t *testing.T) {
	ci.Parallel(t)

	index := uint64(0)
	state := testStateStore(t)
	txn := state.db.WriteTxn(index)

	// Create and insert a mock job that should be pending.
	job := mock.Job()
	job.Status = structs.JobStatusPending
	job.ModifyIndex = 10
	must.NoError(t, txn.Insert("jobs", job))

	index = uint64(1000)
	must.NoError(t, state.setJobStatus(index, txn, job, false, ""))

	i, err := txn.First("jobs", "id", job.Namespace, job.ID)
	must.NoError(t, err)
	updated := i.(*structs.Job)
	must.Eq(t, 10, updated.ModifyIndex, must.Sprint("should be no-op"))
}

func TestStateStore_SetJobStatus(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	txn := state.db.WriteTxn(uint64(0))

	// Create and insert a mock job that should be pending but has an incorrect
	// status.
	job := mock.Job()
	job.Status = "foobar"
	job.ModifyIndex = 10
	must.NoError(t, txn.Insert("jobs", job))

	index := uint64(1000)
	must.NoError(t, state.setJobStatus(index, txn, job, false, ""))

	i, err := txn.First("jobs", "id", job.Namespace, job.ID)
	must.NoError(t, err)
	updated := i.(*structs.Job)
	must.Eq(t, structs.JobStatusPending, updated.Status)
	must.Eq(t, index, updated.ModifyIndex)
}

func TestStateStore_GetJobStatus(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name  string
		setup func(*testing.T, *txn) *structs.Job
		exp   string
	}{
		{
			name: "stopped job with running allocations is still running",
			setup: func(t *testing.T, txn *txn) *structs.Job {
				j := mock.Job()

				a := mock.Alloc()
				a.JobID = j.ID
				a.Job = j
				a.ClientStatus = structs.AllocClientStatusRunning

				err := txn.Insert("allocs", a)
				must.NoError(t, err)

				stoppedJob := j.Copy()
				stoppedJob.Stop = true
				stoppedJob.Version += 1
				return stoppedJob
			},
			exp: structs.JobStatusRunning,
		},
		{
			name: "stopped job with terminal allocs is dead",
			setup: func(t *testing.T, txn *txn) *structs.Job {
				j := mock.Job()
				j.Stop = true

				a := mock.Alloc()
				a.JobID = j.ID
				a.Job = j
				a.ClientStatus = structs.AllocClientStatusComplete
				err := txn.Insert("allocs", a)
				must.NoError(t, err)
				return j
			},
			exp: structs.JobStatusDead,
		},
		{
			name: "parameterized job",
			setup: func(t *testing.T, txn *txn) *structs.Job {
				j := mock.Job()
				j.ParameterizedJob = &structs.ParameterizedJobConfig{}
				j.Dispatched = false
				return j
			},
			exp: structs.JobStatusRunning,
		},
		{
			name: "periodic job",
			setup: func(t *testing.T, txn *txn) *structs.Job {
				j := mock.Job()
				j.Periodic = &structs.PeriodicConfig{}
				return j
			},
			exp: structs.JobStatusRunning,
		},
		{
			name: "no allocs",
			setup: func(t *testing.T, txn *txn) *structs.Job {
				return mock.Job()
			},
			exp: structs.JobStatusPending,
		},
		{
			name: "current job has pending alloc",
			setup: func(t *testing.T, txn *txn) *structs.Job {
				j := mock.Job()
				a := mock.Alloc()

				a.JobID = j.ID

				err := txn.Insert("allocs", a)
				must.NoError(t, err)
				return j
			},
			exp: structs.JobStatusRunning,
		},
		{
			name: "previous job version had allocs",
			setup: func(t *testing.T, txn *txn) *structs.Job {
				j := mock.Job()
				a := mock.Alloc()
				e := mock.Eval()

				e.JobID = j.ID
				e.JobModifyIndex = j.ModifyIndex
				e.Status = structs.EvalStatusPending

				a.JobID = j.ID
				a.Job = j
				a.ClientStatus = structs.AllocClientStatusFailed

				j.Version += 1
				err := txn.Insert("allocs", a)
				must.NoError(t, err)

				err = txn.Insert("evals", e)
				must.NoError(t, err)
				return j
			},
			exp: structs.JobStatusPending,
		},
		{
			name: "batch job has all terminal allocs and terminal evals",
			setup: func(t *testing.T, txn *txn) *structs.Job {
				j := mock.Job()
				j.Type = structs.JobTypeBatch

				a := mock.Alloc()
				a.ClientStatus = structs.AllocClientStatusFailed
				a.JobID = j.ID
				a.Job = j

				err := txn.Insert("allocs", a)
				must.NoError(t, err)

				e := mock.Eval()
				e.JobID = j.ID
				e.Status = structs.EvalStatusComplete
				err = txn.Insert("evals", e)
				must.NoError(t, err)
				return j
			},
			exp: structs.JobStatusDead,
		},
		{
			name: "job has all terminal allocs, but pending eval",
			setup: func(t *testing.T, txn *txn) *structs.Job {
				j := mock.Job()
				a := mock.Alloc()

				a.ClientStatus = structs.AllocClientStatusFailed
				a.JobID = j.ID

				e := mock.Eval()
				e.JobID = j.ID
				e.JobModifyIndex = j.ModifyIndex
				e.Status = structs.EvalStatusPending

				err := txn.Insert("allocs", a)
				must.NoError(t, err)

				err = txn.Insert("evals", e)
				must.NoError(t, err)
				return j

			},
			exp: structs.JobStatusPending,
		},
		{
			name: "reschedulable alloc is pending waiting for replacement",
			setup: func(t *testing.T, txn *txn) *structs.Job {
				j := mock.Job()
				must.NotNil(t, j.TaskGroups[0].ReschedulePolicy)

				a := mock.Alloc()
				a.Job = j
				a.JobID = j.ID
				a.ClientStatus = structs.AllocClientStatusFailed
				err := txn.Insert("allocs", a)
				must.NoError(t, err)
				return j
			},
			exp: structs.JobStatusPending,
		},
		{
			name: "reschedulable alloc is dead after replacement fails",
			setup: func(t *testing.T, txn *txn) *structs.Job {
				j := mock.Job()
				// give job one reschedule attempt
				j.TaskGroups[0].ReschedulePolicy.Attempts = 1
				j.TaskGroups[0].ReschedulePolicy.Interval = time.Hour

				// Replacement alloc
				a := mock.Alloc()
				a.Job = j
				a.JobID = j.ID
				a.ClientStatus = structs.AllocClientStatusFailed
				a.RescheduleTracker = &structs.RescheduleTracker{
					Events: []*structs.RescheduleEvent{
						structs.NewRescheduleEvent(time.Now().UTC().UnixNano(), "", "", time.Minute),
					},
				}

				err := txn.Insert("allocs", a)
				must.NoError(t, err)

				// Original alloc
				a2 := mock.Alloc()
				a2.Job = j
				a2.JobID = j.ID
				a2.ClientStatus = structs.AllocClientStatusFailed
				a2.NextAllocation = a.ID

				err = txn.Insert("allocs", a2)
				must.NoError(t, err)

				e := mock.Eval()
				e.JobID = j.ID
				e.Status = structs.EvalStatusComplete
				err = txn.Insert("evals", e)
				must.NoError(t, err)
				return j
			},
			exp: structs.JobStatusDead,
		},
		{
			name: "filters evals with matching job ID prefix",
			setup: func(t *testing.T, txn *txn) *structs.Job {

				j := mock.Job()
				must.NotNil(t, j.TaskGroups[0].ReschedulePolicy)

				e1 := mock.Eval()
				e1.JobID = j.ID
				e1.Status = structs.EvalStatusComplete
				err := txn.Insert("evals", e1)
				must.NoError(t, err)

				e2 := mock.Eval()
				e2.JobID = fmt.Sprintf("%s%s", j.ID, j.ID)
				e2.Status = structs.EvalStatusPending
				err = txn.Insert("evals", e2)
				must.NoError(t, err)
				return j
			},
			exp: structs.JobStatusDead,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)

			state := testStateStore(t)

			txn := state.db.WriteTxn(0)

			job := tc.setup(t, txn)

			status, err := state.getJobStatus(txn, job, false)
			must.NoError(t, err)
			must.Eq(t, tc.exp, status)
		})
	}
}

func TestStateJobSummary_UpdateJobCount(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	alloc := mock.Alloc()
	job := alloc.Job
	job.TaskGroups[0].Count = 3

	// Create watchsets so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobSummaryByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))

	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}))

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

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
	must.Eq(t, summary, &expectedSummary)

	// Create watchsets so we can test that upsert fires the watch
	ws2 := memdb.NewWatchSet()
	_, err = state.JobSummaryByID(ws2, job.Namespace, job.ID)
	must.NoError(t, err)

	alloc2 := mock.Alloc()
	alloc2.Job = job
	alloc2.JobID = job.ID

	alloc3 := mock.Alloc()
	alloc3.Job = job
	alloc3.JobID = job.ID

	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{alloc2, alloc3}))

	must.True(t, watchFired(ws2))

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
	must.Eq(t, summary, &expectedSummary)

	// Create watchsets so we can test that upsert fires the watch
	ws3 := memdb.NewWatchSet()
	_, err = state.JobSummaryByID(ws3, job.Namespace, job.ID)
	must.NoError(t, err)

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

	must.NoError(t, state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1004, []*structs.Allocation{alloc4, alloc5}))

	must.True(t, watchFired(ws2))

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
	must.Eq(t, summary, &expectedSummary)
}

func TestJobSummary_UpdateClientStatus(t *testing.T) {
	ci.Parallel(t)

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

	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	must.NoError(t, err)

	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc, alloc2, alloc3}))

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

	must.NoError(t, state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, 1002, []*structs.Allocation{alloc4, alloc5, alloc6}))

	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	summary, _ = state.JobSummaryByID(ws, job.Namespace, job.ID)
	if summary.Summary["web"].Running != 1 || summary.Summary["web"].Failed != 1 || summary.Summary["web"].Complete != 1 {
		t.Fatalf("bad job summary: %v", summary)
	}

	alloc7 := mock.Alloc()
	alloc7.Job = alloc.Job
	alloc7.JobID = alloc.JobID

	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1003, []*structs.Allocation{alloc7}))
	summary, _ = state.JobSummaryByID(ws, job.Namespace, job.ID)
	if summary.Summary["web"].Starting != 1 || summary.Summary["web"].Running != 1 || summary.Summary["web"].Failed != 1 || summary.Summary["web"].Complete != 1 {
		t.Fatalf("bad job summary: %v", summary)
	}
}

// Test that nonexistent deployment can't be updated
func TestStateStore_UpsertDeploymentStatusUpdate_Nonexistent(t *testing.T) {
	ci.Parallel(t)

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
	ci.Parallel(t)

	state := testStateStore(t)

	// Insert a terminal deployment
	d := mock.Deployment()
	d.Status = structs.DeploymentStatusFailed

	must.NoError(t, state.UpsertDeployment(1, d))

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
	ci.Parallel(t)

	state := testStateStore(t)

	// Insert a deployment
	d := mock.Deployment()
	must.NoError(t, state.UpsertDeployment(1, d))

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
	must.NoError(t, err)

	// Check that the status was updated properly
	ws := memdb.NewWatchSet()
	dout, err := state.DeploymentByID(ws, d.ID)
	must.NoError(t, err)
	if dout.Status != status || dout.StatusDescription != desc {
		t.Fatalf("bad: %#v", dout)
	}

	// Check that the evaluation was created
	eout, _ := state.EvalByID(ws, e.ID)
	must.NoError(t, err)
	if eout == nil {
		t.Fatalf("bad: %#v", eout)
	}

	// Check that the job was created
	jout, _ := state.JobByID(ws, j.Namespace, j.ID)
	must.NoError(t, err)
	if jout == nil {
		t.Fatalf("bad: %#v", jout)
	}
}

// Test that when a deployment is updated to successful the job is updated to
// stable
func TestStateStore_UpsertDeploymentStatusUpdate_Successful(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	now := time.Now().UnixNano()

	// Insert a job
	job := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1, nil, job))

	// Insert a deployment
	d := structs.NewDeployment(job, 50, now)
	must.NoError(t, state.UpsertDeployment(2, d))

	// Update the deployment
	req := &structs.DeploymentStatusUpdateRequest{
		DeploymentUpdate: &structs.DeploymentStatusUpdate{
			DeploymentID:      d.ID,
			Status:            structs.DeploymentStatusSuccessful,
			StatusDescription: structs.DeploymentStatusDescriptionSuccessful,
		},
	}
	err := state.UpdateDeploymentStatus(structs.MsgTypeTestSetup, 3, req)
	must.NoError(t, err)

	// Check that the status was updated properly
	ws := memdb.NewWatchSet()
	dout, err := state.DeploymentByID(ws, d.ID)
	must.NoError(t, err)
	if dout.Status != structs.DeploymentStatusSuccessful ||
		dout.StatusDescription != structs.DeploymentStatusDescriptionSuccessful {
		t.Fatalf("bad: %#v", dout)
	}

	// Check that the job was created
	jout, _ := state.JobByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
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
	ci.Parallel(t)

	state := testStateStore(t)

	// Insert a job twice to get two versions
	job := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1, nil, job))

	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 2, nil, job.Copy()))

	// Update the stability to true
	err := state.UpdateJobStability(3, job.Namespace, job.ID, 0, true)
	must.NoError(t, err)

	// Check that the job was updated properly
	ws := memdb.NewWatchSet()
	jout, err := state.JobByIDAndVersion(ws, job.Namespace, job.ID, 0)
	must.NoError(t, err)
	must.NotNil(t, jout)
	must.True(t, jout.Stable, must.Sprint("job not marked as stable"))

	// Update the stability to false
	err = state.UpdateJobStability(3, job.Namespace, job.ID, 0, false)
	must.NoError(t, err)

	// Check that the job was updated properly
	jout, err = state.JobByIDAndVersion(ws, job.Namespace, job.ID, 0)
	must.NoError(t, err)
	must.NotNil(t, jout)
	must.False(t, jout.Stable)
}

// Test that nonexistent deployment can't be promoted
func TestStateStore_UpsertDeploymentPromotion_Nonexistent(t *testing.T) {
	ci.Parallel(t)

	store := testStateStore(t)

	// Promote the nonexistent deployment
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: uuid.Generate(),
			All:          true,
		},
	}
	err := store.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, 2, req)
	must.ErrorContains(t, err, "does not exist")
}

// Test that terminal deployment can't be updated
func TestStateStore_UpsertDeploymentPromotion_Terminal(t *testing.T) {
	ci.Parallel(t)

	store := testStateStore(t)

	// Insert a terminal deployment
	d := mock.Deployment()
	d.Status = structs.DeploymentStatusFailed

	must.NoError(t, store.UpsertDeployment(1, d))

	// Promote the deployment
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
	}
	err := store.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, 2, req)
	must.ErrorContains(t, err, "has terminal status", must.Sprint(
		"expected error updating the status because the deployment is terminal"))
}

// Test promoting unhealthy canaries in a deployment.
func TestStateStore_UpsertDeploymentPromotion_Unhealthy(t *testing.T) {
	ci.Parallel(t)

	store := testStateStore(t)

	// Create a job
	j := mock.Job()
	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 1, nil, j))

	// Create a deployment
	d := mock.Deployment()
	d.JobID = j.ID
	d.TaskGroups["web"].DesiredCanaries = 2
	must.NoError(t, store.UpsertDeployment(2, d))

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
	c3.DeploymentStatus = &structs.AllocDeploymentStatus{Healthy: pointer.Of(true)}
	d.TaskGroups[c3.TaskGroup].PlacedCanaries = append(d.TaskGroups[c3.TaskGroup].PlacedCanaries, c3.ID)

	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, 3, []*structs.Allocation{c1, c2, c3}))

	// Promote the canaries
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
	}
	err := store.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, 4, req)
	must.ErrorContains(t, err, `Task group "web" has 0/2 healthy allocations`)
}

// Test promoting a deployment with no canaries
func TestStateStore_UpsertDeploymentPromotion_NoCanaries(t *testing.T) {
	ci.Parallel(t)

	store := testStateStore(t)

	// Create a job
	j := mock.Job()
	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 1, nil, j))

	// Create a deployment
	d := mock.Deployment()
	d.TaskGroups["web"].DesiredCanaries = 2
	d.JobID = j.ID
	must.NoError(t, store.UpsertDeployment(2, d))

	// Promote the canaries
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
	}
	err := store.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, 4, req)
	must.ErrorContains(t, err, `Task group "web" has 0/2 healthy allocations`)
}

// Test promoting all canaries in a deployment.
func TestStateStore_UpsertDeploymentPromotion_All(t *testing.T) {
	ci.Parallel(t)

	store := testStateStore(t)

	// Create a job with two task groups
	j := mock.Job()
	tg1 := j.TaskGroups[0]
	tg2 := tg1.Copy()
	tg2.Name = "foo"
	j.TaskGroups = append(j.TaskGroups, tg2)
	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 1, nil, j))

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
	must.NoError(t, store.UpsertDeployment(2, d))

	// Create a set of allocations
	c1 := mock.Alloc()
	c1.JobID = j.ID
	c1.DeploymentID = d.ID
	d.TaskGroups[c1.TaskGroup].PlacedCanaries = append(d.TaskGroups[c1.TaskGroup].PlacedCanaries, c1.ID)
	c1.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(true),
	}
	c2 := mock.Alloc()
	c2.JobID = j.ID
	c2.DeploymentID = d.ID
	d.TaskGroups[c2.TaskGroup].PlacedCanaries = append(d.TaskGroups[c2.TaskGroup].PlacedCanaries, c2.ID)
	c2.TaskGroup = tg2.Name
	c2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(true),
	}

	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, 3, []*structs.Allocation{c1, c2}))
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
	must.NoError(t, store.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, 4, req))

	// Check that the status per task group was updated properly
	ws := memdb.NewWatchSet()
	dout, err := store.DeploymentByID(ws, d.ID)
	must.NoError(t, err)

	must.Eq(t, structs.DeploymentStatusDescriptionRunning, dout.StatusDescription,
		must.Sprint("expected status description to be updated"))
	must.MapLen(t, 2, dout.TaskGroups)
	for tg, state := range dout.TaskGroups {
		must.True(t, state.Promoted,
			must.Sprintf("expected group %q to be promoted %#v", tg, state))
	}

	// Check that the evaluation was created
	eout, err := store.EvalByID(ws, e.ID)
	must.NoError(t, err)
	must.NotNil(t, eout)
}

// Test promoting a subset of canaries in a deployment.
func TestStateStore_UpsertDeploymentPromotion_Subset(t *testing.T) {
	ci.Parallel(t)
	store := testStateStore(t)

	// Create a job with two task groups
	j := mock.Job()
	tg1 := j.TaskGroups[0]
	tg2 := tg1.Copy()
	tg2.Name = "foo"
	j.TaskGroups = append(j.TaskGroups, tg2)
	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 1, nil, j))

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
	must.NoError(t, store.UpsertDeployment(2, d))

	// Create a set of allocations for both groups, including an unhealthy one
	c1 := mock.Alloc()
	c1.JobID = j.ID
	c1.DeploymentID = d.ID
	d.TaskGroups[c1.TaskGroup].PlacedCanaries = append(d.TaskGroups[c1.TaskGroup].PlacedCanaries, c1.ID)
	c1.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(true),
		Canary:  true,
	}

	// Should still be a canary
	c2 := mock.Alloc()
	c2.JobID = j.ID
	c2.DeploymentID = d.ID
	d.TaskGroups[c2.TaskGroup].PlacedCanaries = append(d.TaskGroups[c2.TaskGroup].PlacedCanaries, c2.ID)
	c2.TaskGroup = tg2.Name
	c2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(true),
		Canary:  true,
	}

	c3 := mock.Alloc()
	c3.JobID = j.ID
	c3.DeploymentID = d.ID
	d.TaskGroups[c3.TaskGroup].PlacedCanaries = append(d.TaskGroups[c3.TaskGroup].PlacedCanaries, c3.ID)
	c3.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(false),
		Canary:  true,
	}

	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, 3, []*structs.Allocation{c1, c2, c3}))

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
	must.NoError(t, store.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, 4, req))

	// Check that the status per task group was updated properly
	ws := memdb.NewWatchSet()
	dout, err := store.DeploymentByID(ws, d.ID)
	must.NoError(t, err)
	must.MapLen(t, 2, dout.TaskGroups)
	must.MapContainsKey(t, dout.TaskGroups, "web")
	must.True(t, dout.TaskGroups["web"].Promoted)

	// Check that the evaluation was created
	eout, err := store.EvalByID(ws, e.ID)
	must.NoError(t, err)
	must.NotNil(t, eout)

	// Check the canary field was set properly
	aout1, err1 := store.AllocByID(ws, c1.ID)
	aout2, err2 := store.AllocByID(ws, c2.ID)
	aout3, err3 := store.AllocByID(ws, c3.ID)

	must.NoError(t, err1)
	must.NoError(t, err2)
	must.NoError(t, err3)
	must.NotNil(t, aout1)
	must.NotNil(t, aout2)
	must.NotNil(t, aout2)
	must.False(t, aout1.DeploymentStatus.Canary)
	must.True(t, aout2.DeploymentStatus.Canary)
	must.True(t, aout3.DeploymentStatus.Canary)
}

// Test that allocation health can't be set against a nonexistent deployment
func TestStateStore_UpsertDeploymentAllocHealth_Nonexistent(t *testing.T) {
	ci.Parallel(t)

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
	ci.Parallel(t)

	state := testStateStore(t)

	// Insert a terminal deployment
	d := mock.Deployment()
	d.Status = structs.DeploymentStatusFailed

	must.NoError(t, state.UpsertDeployment(1, d))

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
	ci.Parallel(t)

	state := testStateStore(t)

	// Insert a deployment
	d := mock.Deployment()
	must.NoError(t, state.UpsertDeployment(1, d))

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
	ci.Parallel(t)

	state := testStateStore(t)

	// Create a deployment
	d1 := mock.Deployment()
	must.NoError(t, state.UpsertDeployment(2, d1))

	// Create a Job
	job := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 3, nil, job))

	// Create alloc with canary status
	a := mock.Alloc()
	a.JobID = job.ID
	a.DeploymentID = d1.ID
	a.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(false),
		Canary:  true,
	}
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 4, []*structs.Allocation{a}))

	// Pull the deployment from state
	ws := memdb.NewWatchSet()
	deploy, err := state.DeploymentByID(ws, d1.ID)
	must.NoError(t, err)

	// Ensure that PlacedCanaries is accurate
	must.Eq(t, 1, len(deploy.TaskGroups[job.TaskGroups[0].Name].PlacedCanaries))

	// Create alloc without canary status
	b := mock.Alloc()
	b.JobID = job.ID
	b.DeploymentID = d1.ID
	b.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(false),
		Canary:  false,
	}
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 4, []*structs.Allocation{b}))

	// Pull the deployment from state
	ws = memdb.NewWatchSet()
	deploy, err = state.DeploymentByID(ws, d1.ID)
	must.NoError(t, err)

	// Ensure that PlacedCanaries is accurate
	must.Eq(t, 1, len(deploy.TaskGroups[job.TaskGroups[0].Name].PlacedCanaries))

	// Create a second deployment
	d2 := mock.Deployment()
	must.NoError(t, state.UpsertDeployment(5, d2))

	c := mock.Alloc()
	c.JobID = job.ID
	c.DeploymentID = d2.ID
	c.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(false),
		Canary:  true,
	}
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 6, []*structs.Allocation{c}))

	ws = memdb.NewWatchSet()
	deploy2, err := state.DeploymentByID(ws, d2.ID)
	must.NoError(t, err)

	// Ensure that PlacedCanaries is accurate
	must.Eq(t, 1, len(deploy2.TaskGroups[job.TaskGroups[0].Name].PlacedCanaries))
}

func TestStateStore_UpsertDeploymentAlloc_NoCanaries(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	// Create a deployment
	d1 := mock.Deployment()
	must.NoError(t, state.UpsertDeployment(2, d1))

	// Create a Job
	job := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 3, nil, job))

	// Create alloc with canary status
	a := mock.Alloc()
	a.JobID = job.ID
	a.DeploymentID = d1.ID
	a.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: pointer.Of(true),
		Canary:  false,
	}
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 4, []*structs.Allocation{a}))

	// Pull the deployment from state
	ws := memdb.NewWatchSet()
	deploy, err := state.DeploymentByID(ws, d1.ID)
	must.NoError(t, err)

	// Ensure that PlacedCanaries is accurate
	must.Eq(t, 0, len(deploy.TaskGroups[job.TaskGroups[0].Name].PlacedCanaries))
}

// Test that allocation health can't be set for an alloc with mismatched
// deployment ids
func TestStateStore_UpsertDeploymentAllocHealth_BadAlloc_MismatchDeployment(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	// Insert two  deployment
	d1 := mock.Deployment()
	d2 := mock.Deployment()
	must.NoError(t, state.UpsertDeployment(1, d1))
	must.NoError(t, state.UpsertDeployment(2, d2))

	// Insert an alloc for a random deployment
	a := mock.Alloc()
	a.DeploymentID = d1.ID
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 3, nil, a.Job))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 4, []*structs.Allocation{a}))

	// Set health against the terminal deployment
	req := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:         d2.ID,
			HealthyAllocationIDs: []string{a.ID},
		},
	}
	err := state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, 5, req)
	if err == nil || !strings.Contains(err.Error(), "not part of deployment") {
		t.Fatalf("expected error because the alloc isn't part of the deployment: %v", err)
	}
}

// Test that allocation health is properly set
func TestStateStore_UpsertDeploymentAllocHealth(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	// Create a job to roll back to
	mockJob := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1, nil, mockJob))

	// Insert a deployment
	d := mock.Deployment()
	d.TaskGroups["web"].ProgressDeadline = 5 * time.Minute
	must.NoError(t, state.UpsertDeployment(2, d))

	// Insert two allocations
	a1 := mock.Alloc()
	a1.DeploymentID = d.ID
	a1.JobID = mockJob.ID
	a2 := mock.Alloc()
	a2.DeploymentID = d.ID
	a2.JobID = mockJob.ID
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 3, []*structs.Allocation{a1, a2}))

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
		Job:              mockJob,
		Eval:             e,
		DeploymentUpdate: u,
		Timestamp:        ts,
	}
	err := state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, 4, req)
	must.NoError(t, err)

	// Check that the status was updated properly
	ws := memdb.NewWatchSet()
	dout, err := state.DeploymentByID(ws, d.ID)
	must.NoError(t, err)
	if dout.Status != status || dout.StatusDescription != desc {
		t.Fatalf("bad: %#v", dout)
	}

	// Check that the evaluation was created
	eout, _ := state.EvalByID(ws, e.ID)
	must.NoError(t, err)
	if eout == nil {
		t.Fatalf("bad: %#v", eout)
	}

	// Check that the job was created
	jout, _ := state.JobByID(ws, mockJob.Namespace, mockJob.ID)
	must.NoError(t, err)
	if jout == nil {
		t.Fatalf("bad: %#v", jout)
	}

	// Check the status of the allocs
	out1, err := state.AllocByID(ws, a1.ID)
	must.NoError(t, err)
	out2, err := state.AllocByID(ws, a2.ID)
	must.NoError(t, err)

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

func TestStateStore_UpsertACLPolicy(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	policy := mock.ACLPolicy()
	policy2 := mock.ACLPolicy()

	ws := memdb.NewWatchSet()
	_, err := state.ACLPolicyByName(ws, policy.Name)
	must.NoError(t, err)
	_, err = state.ACLPolicyByName(ws, policy2.Name)
	must.NoError(t, err)

	must.NoError(t, state.UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{policy, policy2}))
	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	out, err := state.ACLPolicyByName(ws, policy.Name)
	must.NoError(t, err)
	must.Eq(t, policy, out)

	out, err = state.ACLPolicyByName(ws, policy2.Name)
	must.Eq(t, nil, err)
	must.Eq(t, policy2, out)

	iter, err := state.ACLPolicies(ws)
	must.NoError(t, err)

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
	must.NoError(t, err)
	must.Eq(t, 1000, index)

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_ACLPolicyByNamespace(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	policy := mock.ACLPolicy()
	policy.JobACL = &structs.JobACL{
		Namespace: "default",
	}
	policy1 := mock.ACLPolicy()
	policy1.JobACL = &structs.JobACL{
		Namespace: "default",
		JobID:     "test-ack-job-name",
	}
	policy2 := mock.ACLPolicy()
	policy2.JobACL = &structs.JobACL{
		Namespace: "default",
		JobID:     "testing-job",
	}
	policy3 := mock.ACLPolicy()
	policy3.JobACL = &structs.JobACL{
		Namespace: "testing",
		JobID:     "test-job",
	}

	err := state.UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{policy, policy1, policy2})
	must.NoError(t, err)

	iter, err := state.ACLPolicyByNamespace(nil, "default")
	must.NoError(t, err)

	out := iter.Next()
	must.NotNil(t, out)
	must.Eq(t, policy, out.(*structs.ACLPolicy))

	count := 0
	for {
		if iter.Next() == nil {
			break
		}

		count++
	}
	must.Eq(t, 0, count)

	iter, err = state.ACLPolicyByNamespace(nil, "testing")
	must.NoError(t, err)
	must.Nil(t, iter.Next())
}

func TestStateStore_ACLPolicyByJob(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	policy := mock.ACLPolicy()
	policy.JobACL = &structs.JobACL{
		Namespace: "default",
		JobID:     "test-acl-job-name",
	}
	policy1 := mock.ACLPolicy()
	policy1.JobACL = &structs.JobACL{
		Namespace: "testing",
		JobID:     "test-acl-job-name",
	}
	policy2 := mock.ACLPolicy()
	policy2.JobACL = &structs.JobACL{
		Namespace: "default",
		JobID:     "test-acl-job-name-secondary",
	}
	policy3 := mock.ACLPolicy()
	policy3.JobACL = &structs.JobACL{
		Namespace: "default",
		JobID:     "test-acl-job-name",
		Group:     "collection",
	}

	err := state.UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{policy, policy1, policy2, policy3})
	must.NoError(t, err)

	// Expect two matching policies with exact matching JobIDs
	iter, err := state.ACLPolicyByJob(nil, "default", "test-acl-job-name")
	must.NoError(t, err)
	for i := 0; i < 2; i++ {
		out := iter.Next()
		must.NotNil(t, out)
		p := out.(*structs.ACLPolicy)
		must.Eq(t, "default", p.JobACL.Namespace)
		must.Eq(t, "test-acl-job-name", p.JobACL.JobID)
	}
	// Ensure no remaining results
	must.Nil(t, iter.Next())

	// Expect single matching policy
	iter, err = state.ACLPolicyByJob(nil, "testing", "test-acl-job-name")
	out := iter.Next()
	must.Eq(t, policy1, out.(*structs.ACLPolicy))
	// Ensure no remaining results
	must.Nil(t, iter.Next())

	// Expect no matching policies
	iter, err = state.ACLPolicyByJob(nil, "unknown", "test-acl-job-name")
	must.NoError(t, err)
	// Check for no results
	must.Nil(t, iter.Next())
}

func TestStateStore_DeleteACLPolicy(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	policy := mock.ACLPolicy()
	policy2 := mock.ACLPolicy()

	// Create the policy
	must.NoError(t, state.UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{policy, policy2}))

	// Create a watcher
	ws := memdb.NewWatchSet()
	_, err := state.ACLPolicyByName(ws, policy.Name)
	must.NoError(t, err)

	// Delete the policy
	must.NoError(t, state.DeleteACLPolicies(structs.MsgTypeTestSetup, 1001, []string{policy.Name, policy2.Name}))

	// Ensure watching triggered
	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	// Ensure we don't get the object back
	ws = memdb.NewWatchSet()
	out, err := state.ACLPolicyByName(ws, policy.Name)
	must.NoError(t, err)
	must.Nil(t, out)

	iter, err := state.ACLPolicies(ws)
	must.NoError(t, err)
	must.Nil(t, iter.Next())

	index, err := state.Index("acl_policy")
	must.NoError(t, err)
	must.Eq(t, 1001, index)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_ACLPolicyByNamePrefix(t *testing.T) {
	ci.Parallel(t)

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
		must.NoError(t, state.UpsertACLPolicies(structs.MsgTypeTestSetup, baseIndex, []*structs.ACLPolicy{p}))
		baseIndex++
	}

	// Scan by prefix
	iter, err := state.ACLPolicyByNamePrefix(nil, "foo")
	must.NoError(t, err)

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
	must.Eq(t, expect, out)
}

func TestStateStore_BootstrapACLTokens(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	tk1 := mock.ACLToken()
	tk2 := mock.ACLToken()

	ok, resetIdx, err := state.CanBootstrapACLToken()
	must.NoError(t, err)
	must.Eq(t, true, ok)
	test.Eq(t, 0, resetIdx)

	must.NoError(t, state.BootstrapACLTokens(structs.MsgTypeTestSetup, 1000, 0, tk1))

	out, err := state.ACLTokenByAccessorID(nil, tk1.AccessorID)
	must.Eq(t, nil, err)
	must.Eq(t, tk1, out)

	ok, resetIdx, err = state.CanBootstrapACLToken()
	must.NoError(t, err)
	must.Eq(t, false, ok)
	test.Eq(t, 1000, resetIdx)

	must.Error(t, state.BootstrapACLTokens(structs.MsgTypeTestSetup, 1001, 0, tk2))

	iter, err := state.ACLTokens(nil, SortDefault)
	must.NoError(t, err)

	// Ensure we see both policies
	count := 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
	}
	must.Eq(t, 1, count)

	index, err := state.Index("acl_token")
	must.NoError(t, err)
	must.Eq(t, 1000, index)
	index, err = state.Index("acl_token_bootstrap")
	must.NoError(t, err)
	must.Eq(t, 1000, index)

	// Should allow bootstrap with reset index
	must.NoError(t, state.BootstrapACLTokens(structs.MsgTypeTestSetup, 1001, 1000, tk2))

	// Check we've modified the index
	index, err = state.Index("acl_token")
	must.NoError(t, err)
	must.Eq(t, 1001, index)
	index, err = state.Index("acl_token_bootstrap")
	must.NoError(t, err)
	must.Eq(t, 1001, index)
}

func TestStateStore_UpsertACLTokens(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	tk1 := mock.ACLToken()
	tk2 := mock.ACLToken()

	ws := memdb.NewWatchSet()
	_, err := state.ACLTokenByAccessorID(ws, tk1.AccessorID)
	must.NoError(t, err)
	_, err = state.ACLTokenByAccessorID(ws, tk2.AccessorID)
	must.NoError(t, err)

	must.NoError(t, state.UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{tk1, tk2}))
	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	ws = memdb.NewWatchSet()
	out, err := state.ACLTokenByAccessorID(ws, tk1.AccessorID)
	must.Eq(t, nil, err)
	must.Eq(t, tk1, out)

	out, err = state.ACLTokenByAccessorID(ws, tk2.AccessorID)
	must.Eq(t, nil, err)
	must.Eq(t, tk2, out)

	out, err = state.ACLTokenBySecretID(ws, tk1.SecretID)
	must.Eq(t, nil, err)
	must.Eq(t, tk1, out)

	out, err = state.ACLTokenBySecretID(ws, tk2.SecretID)
	must.Eq(t, nil, err)
	must.Eq(t, tk2, out)

	iter, err := state.ACLTokens(ws, SortDefault)
	must.NoError(t, err)

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
	must.NoError(t, err)
	must.Eq(t, 1000, index)

	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_DeleteACLTokens(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	tk1 := mock.ACLToken()
	tk2 := mock.ACLToken()

	// Create the tokens
	err := state.UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{tk1, tk2})
	must.NoError(t, err)

	// Create a watcher
	ws := memdb.NewWatchSet()
	_, err = state.ACLTokenByAccessorID(ws, tk1.AccessorID)
	must.NoError(t, err)

	// Delete the token
	err = state.DeleteACLTokens(structs.MsgTypeTestSetup, 1001, []string{tk1.AccessorID, tk2.AccessorID})
	must.NoError(t, err)

	// Ensure watching triggered
	must.True(t, watchFired(ws), must.Sprint("expected watch to fire"))

	// Ensure we don't get the object back
	ws = memdb.NewWatchSet()
	out, err := state.ACLTokenByAccessorID(ws, tk1.AccessorID)
	must.NoError(t, err)
	must.Nil(t, out)

	iter, err := state.ACLTokens(ws, SortDefault)
	must.NoError(t, err)
	must.Nil(t, iter.Next())

	index, err := state.Index("acl_token")
	must.NoError(t, err)
	must.Eq(t, 1001, index)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))
}

func TestStateStore_ACLTokenByAccessorIDPrefix(t *testing.T) {
	ci.Parallel(t)

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
		err := state.UpsertACLTokens(structs.MsgTypeTestSetup, baseIndex, []*structs.ACLToken{tk})
		must.NoError(t, err)
		baseIndex++
	}

	gatherTokens := func(iter memdb.ResultIterator) []*structs.ACLToken {
		var tokens []*structs.ACLToken
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			tokens = append(tokens, raw.(*structs.ACLToken))
		}
		return tokens
	}

	t.Run("scan by prefix", func(t *testing.T) {
		iter, err := state.ACLTokenByAccessorIDPrefix(nil, "aa", SortDefault)
		must.NoError(t, err)

		// Ensure we see both tokens
		out := gatherTokens(iter)
		must.Len(t, 2, out)

		got := []string{}
		for _, t := range out {
			got = append(got, t.AccessorID[:4])
		}
		expect := []string{"aaaa", "aabb"}
		must.Eq(t, expect, got)
	})

	t.Run("reverse order", func(t *testing.T) {
		iter, err := state.ACLTokenByAccessorIDPrefix(nil, "aa", SortReverse)
		must.NoError(t, err)

		// Ensure we see both tokens
		out := gatherTokens(iter)
		must.Len(t, 2, out)

		got := []string{}
		for _, t := range out {
			got = append(got, t.AccessorID[:4])
		}
		expect := []string{"aabb", "aaaa"}
		must.Eq(t, expect, got)
	})
}

func TestStateStore_ACLTokensByGlobal(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	tk1 := mock.ACLToken()
	tk1.AccessorID = "aaaa" + tk1.AccessorID[4:]

	tk2 := mock.ACLToken()
	tk2.AccessorID = "aabb" + tk2.AccessorID[4:]

	tk3 := mock.ACLToken()
	tk3.AccessorID = "bbbb" + tk3.AccessorID[4:]
	tk3.Global = true

	tk4 := mock.ACLToken()
	tk4.AccessorID = "ffff" + tk4.AccessorID[4:]

	err := state.UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{tk1, tk2, tk3, tk4})
	must.NoError(t, err)

	gatherTokens := func(iter memdb.ResultIterator) []*structs.ACLToken {
		var tokens []*structs.ACLToken
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			tokens = append(tokens, raw.(*structs.ACLToken))
		}
		return tokens
	}

	t.Run("only global tokens", func(t *testing.T) {
		iter, err := state.ACLTokensByGlobal(nil, true, SortDefault)
		must.NoError(t, err)

		got := gatherTokens(iter)
		must.Len(t, 1, got)
		must.Eq(t, tk3.AccessorID, got[0].AccessorID)
	})

	t.Run("reverse order", func(t *testing.T) {
		iter, err := state.ACLTokensByGlobal(nil, false, SortReverse)
		must.NoError(t, err)

		expected := []*structs.ACLToken{tk4, tk2, tk1}
		got := gatherTokens(iter)
		must.Len(t, 3, got)
		must.Eq(t, expected, got)
	})
}

func TestStateStore_OneTimeTokens(t *testing.T) {
	ci.Parallel(t)
	index := uint64(100)
	state := testStateStore(t)

	// create some ACL tokens

	token1 := mock.ACLToken()
	token2 := mock.ACLToken()
	token3 := mock.ACLToken()
	index++
	must.Nil(t, state.UpsertACLTokens(
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
		must.NoError(t, state.UpsertOneTimeToken(structs.MsgTypeTestSetup, index, ott))
	}

	// verify that we have exactly one OTT for each AccessorID

	txn := state.db.ReadTxn()
	iter, err := txn.Get("one_time_token", "id")
	must.NoError(t, err)
	results := []*structs.OneTimeToken{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		ott, ok := raw.(*structs.OneTimeToken)
		must.True(t, ok)
		results = append(results, ott)
	}

	// results aren't ordered but if we have 3 OTT and all 3 tokens, we know
	// we have no duplicate accessors
	must.Len(t, 3, results)
	accessors := []string{
		results[0].AccessorID, results[1].AccessorID, results[2].AccessorID}
	must.SliceContains(t, accessors, token1.AccessorID)
	must.SliceContains(t, accessors, token2.AccessorID)
	must.SliceContains(t, accessors, token3.AccessorID)

	// now verify expiration

	getExpiredTokens := func(now time.Time) []*structs.OneTimeToken {
		txn := state.db.ReadTxn()
		iter, err := state.oneTimeTokensExpiredTxn(txn, nil, now)
		must.NoError(t, err)

		results := []*structs.OneTimeToken{}
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			ott, ok := raw.(*structs.OneTimeToken)
			must.True(t, ok)
			results = append(results, ott)
		}
		return results
	}

	results = getExpiredTokens(time.Now())
	must.Len(t, 2, results)

	// results aren't ordered
	expiredAccessors := []string{results[0].AccessorID, results[1].AccessorID}
	must.SliceContains(t, expiredAccessors, token1.AccessorID)
	must.SliceContains(t, expiredAccessors, token2.AccessorID)
	must.True(t, time.Now().After(results[0].ExpiresAt))
	must.True(t, time.Now().After(results[1].ExpiresAt))

	// clear the expired tokens and verify they're gone
	index++
	must.NoError(t, state.ExpireOneTimeTokens(
		structs.MsgTypeTestSetup, index, time.Now()))

	results = getExpiredTokens(time.Now())
	must.Len(t, 0, results)

	// query the unexpired token
	ott, err := state.OneTimeTokenBySecret(nil, otts[len(otts)-1].OneTimeSecretID)
	must.NoError(t, err)
	must.Eq(t, token3.AccessorID, ott.AccessorID)
	must.True(t, time.Now().Before(ott.ExpiresAt))

	restore, err := state.Restore()
	must.NoError(t, err)
	err = restore.OneTimeTokenRestore(ott)
	must.NoError(t, err)
	must.NoError(t, restore.Commit())

	ott, err = state.OneTimeTokenBySecret(nil, otts[len(otts)-1].OneTimeSecretID)
	must.NoError(t, err)
	must.Eq(t, token3.AccessorID, ott.AccessorID)
}

func TestStateStore_ClusterMetadata(t *testing.T) {

	state := testStateStore(t)
	clusterID := "12345678-1234-1234-1234-1234567890"
	now := time.Now().UnixNano()
	meta := &structs.ClusterMetadata{ClusterID: clusterID, CreateTime: now}

	err := state.ClusterSetMetadata(100, meta)
	must.NoError(t, err)

	result, err := state.ClusterMetadata(nil)
	must.NoError(t, err)
	must.Eq(t, clusterID, result.ClusterID)
	must.Eq(t, now, result.CreateTime)
}

func TestStateStore_UpsertScalingPolicy(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	policy := mock.ScalingPolicy()
	policy2 := mock.ScalingPolicy()

	wsAll := memdb.NewWatchSet()
	all, err := state.ScalingPolicies(wsAll)
	must.NoError(t, err)
	must.Nil(t, all.Next())

	ws := memdb.NewWatchSet()
	out, err := state.ScalingPolicyByTargetAndType(ws, policy.Target, policy.Type)
	must.NoError(t, err)
	must.Nil(t, out)

	out, err = state.ScalingPolicyByTargetAndType(ws, policy2.Target, policy2.Type)
	must.NoError(t, err)
	must.Nil(t, out)

	err = state.UpsertScalingPolicies(1000, []*structs.ScalingPolicy{policy, policy2})
	must.NoError(t, err)
	must.True(t, watchFired(ws))
	must.True(t, watchFired(wsAll))

	ws = memdb.NewWatchSet()
	out, err = state.ScalingPolicyByTargetAndType(ws, policy.Target, policy.Type)
	must.NoError(t, err)
	must.Eq(t, policy, out)

	out, err = state.ScalingPolicyByTargetAndType(ws, policy2.Target, policy2.Type)
	must.NoError(t, err)
	must.Eq(t, policy2, out)

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
	must.NoError(t, err)
	must.Eq(t, 2, count)

	index, err := state.Index("scaling_policy")
	must.NoError(t, err)
	must.True(t, 1000 == index)
	must.False(t, watchFired(ws))

	// Check that we can add policy with same target but different type
	policy3 := mock.ScalingPolicy()
	for k, v := range policy2.Target {
		policy3.Target[k] = v
	}

	err = state.UpsertScalingPolicies(1000, []*structs.ScalingPolicy{policy3})
	must.NoError(t, err)

	// Ensure we see both policies, since target didn't change
	count, err = countPolicies()
	must.NoError(t, err)
	must.Eq(t, 2, count)

	// Change type and check if we see 3
	policy3.Type = "other-type"

	err = state.UpsertScalingPolicies(1000, []*structs.ScalingPolicy{policy3})
	must.NoError(t, err)

	count, err = countPolicies()
	must.NoError(t, err)
	must.Eq(t, 3, count)
}

func TestStateStore_UpsertScalingPolicy_Namespace(t *testing.T) {
	ci.Parallel(t)

	otherNamespace := "not-default-namespace"
	state := testStateStore(t)
	policy := mock.ScalingPolicy()
	policy2 := mock.ScalingPolicy()
	policy2.Target[structs.ScalingTargetNamespace] = otherNamespace

	ws1 := memdb.NewWatchSet()
	iter, err := state.ScalingPoliciesByNamespace(ws1, structs.DefaultNamespace, "")
	must.NoError(t, err)
	must.Nil(t, iter.Next())

	ws2 := memdb.NewWatchSet()
	iter, err = state.ScalingPoliciesByNamespace(ws2, otherNamespace, "")
	must.NoError(t, err)
	must.Nil(t, iter.Next())

	err = state.UpsertScalingPolicies(1000, []*structs.ScalingPolicy{policy, policy2})
	must.NoError(t, err)
	must.True(t, watchFired(ws1))
	must.True(t, watchFired(ws2))

	iter, err = state.ScalingPoliciesByNamespace(nil, structs.DefaultNamespace, "")
	must.NoError(t, err)
	policiesInDefaultNamespace := []string{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		policiesInDefaultNamespace = append(policiesInDefaultNamespace, raw.(*structs.ScalingPolicy).ID)
	}
	must.SliceContainsAll(t, []string{policy.ID}, policiesInDefaultNamespace)

	iter, err = state.ScalingPoliciesByNamespace(nil, otherNamespace, "")
	must.NoError(t, err)
	policiesInOtherNamespace := []string{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		policiesInOtherNamespace = append(policiesInOtherNamespace, raw.(*structs.ScalingPolicy).ID)
	}
	must.SliceContainsAll(t, []string{policy2.ID}, policiesInOtherNamespace)
}

func TestStateStore_UpsertScalingPolicy_Namespace_PrefixBug(t *testing.T) {
	ci.Parallel(t)

	ns1 := "name"
	ns2 := "name2" // matches prefix "name"
	state := testStateStore(t)
	policy1 := mock.ScalingPolicy()
	policy1.Target[structs.ScalingTargetNamespace] = ns1
	policy2 := mock.ScalingPolicy()
	policy2.Target[structs.ScalingTargetNamespace] = ns2

	ws1 := memdb.NewWatchSet()
	iter, err := state.ScalingPoliciesByNamespace(ws1, ns1, "")
	must.NoError(t, err)
	must.Nil(t, iter.Next())

	ws2 := memdb.NewWatchSet()
	iter, err = state.ScalingPoliciesByNamespace(ws2, ns2, "")
	must.NoError(t, err)
	must.Nil(t, iter.Next())

	err = state.UpsertScalingPolicies(1000, []*structs.ScalingPolicy{policy1, policy2})
	must.NoError(t, err)
	must.True(t, watchFired(ws1))
	must.True(t, watchFired(ws2))

	iter, err = state.ScalingPoliciesByNamespace(nil, ns1, "")
	must.NoError(t, err)
	policiesInNS1 := []string{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		policiesInNS1 = append(policiesInNS1, raw.(*structs.ScalingPolicy).ID)
	}
	must.SliceContainsAll(t, []string{policy1.ID}, policiesInNS1)

	iter, err = state.ScalingPoliciesByNamespace(nil, ns2, "")
	must.NoError(t, err)
	policiesInNS2 := []string{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		policiesInNS2 = append(policiesInNS2, raw.(*structs.ScalingPolicy).ID)
	}
	must.SliceContainsAll(t, []string{policy2.ID}, policiesInNS2)
}

// Scaling Policy IDs are generated randomly during Job.Register
// Subsequent updates of the job should preserve the ID for the scaling policy
// associated with a given target.
func TestStateStore_UpsertJob_PreserveScalingPolicyIDsAndIndex(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job, policy := mock.JobWithScalingPolicy()

	var newIndex uint64 = 1000
	err := state.UpsertJob(structs.MsgTypeTestSetup, newIndex, nil, job)
	must.NoError(t, err)

	ws := memdb.NewWatchSet()
	p1, err := state.ScalingPolicyByTargetAndType(ws, policy.Target, policy.Type)
	must.NoError(t, err)
	must.NotNil(t, p1)
	must.Eq(t, newIndex, p1.CreateIndex)
	must.Eq(t, newIndex, p1.ModifyIndex)

	index, err := state.Index("scaling_policy")
	must.NoError(t, err)
	must.Eq(t, newIndex, index)
	must.NotEq(t, "", p1.ID)

	// update the job
	job.Meta["new-meta"] = "new-value"
	newIndex += 100
	err = state.UpsertJob(structs.MsgTypeTestSetup, newIndex, nil, job)
	must.NoError(t, err)
	must.False(t, watchFired(ws), must.Sprint("watch should not have fired"))

	p2, err := state.ScalingPolicyByTargetAndType(nil, policy.Target, policy.Type)
	must.NoError(t, err)
	must.NotNil(t, p2)
	must.Eq(t, p1.ID, p2.ID, must.Sprint("ID should not have changed"))
	must.Eq(t, p1.CreateIndex, p2.CreateIndex)
	must.Eq(t, p1.ModifyIndex, p2.ModifyIndex)

	index, err = state.Index("scaling_policy")
	must.NoError(t, err)
	must.Eq(t, index, p1.CreateIndex, must.Sprint("table index should not have changed"))
}

// Updating the scaling policy for a job should update the index table and fire the watch.
// This test is the converse of TestStateStore_UpsertJob_PreserveScalingPolicyIDsAndIndex
func TestStateStore_UpsertJob_UpdateScalingPolicy(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job, policy := mock.JobWithScalingPolicy()

	var oldIndex uint64 = 1000
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, oldIndex, nil, job))

	ws := memdb.NewWatchSet()
	p1, err := state.ScalingPolicyByTargetAndType(ws, policy.Target, policy.Type)
	must.NoError(t, err)
	must.NotNil(t, p1)
	must.Eq(t, oldIndex, p1.CreateIndex)
	must.Eq(t, oldIndex, p1.ModifyIndex)
	prevId := p1.ID

	index, err := state.Index("scaling_policy")
	must.NoError(t, err)
	must.Eq(t, oldIndex, index)
	must.NotEq(t, "", p1.ID)

	// update the job with the updated scaling policy; make sure to use a different object
	newPolicy := p1.Copy()
	newPolicy.Policy["new-field"] = "new-value"
	job.TaskGroups[0].Scaling = newPolicy
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, oldIndex+100, nil, job))
	must.True(t, watchFired(ws), must.Sprint("watch should have fired"))

	p2, err := state.ScalingPolicyByTargetAndType(nil, policy.Target, policy.Type)
	must.NoError(t, err)
	must.NotNil(t, p2)
	must.Eq(t, p2.Policy["new-field"], "new-value")
	must.Eq(t, prevId, p2.ID, must.Sprint("ID should not have changed"))
	must.Eq(t, oldIndex, p2.CreateIndex)
	must.Greater(t, oldIndex, p2.ModifyIndex, must.Sprint("ModifyIndex should have advanced"))

	index, err = state.Index("scaling_policy")
	must.NoError(t, err)
	must.Greater(t, oldIndex, index, must.Sprint("table index should have advanced"))
}

func TestStateStore_DeleteScalingPolicies(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	policy := mock.ScalingPolicy()
	policy2 := mock.ScalingPolicy()

	// Create the policy
	err := state.UpsertScalingPolicies(1000, []*structs.ScalingPolicy{policy, policy2})
	must.NoError(t, err)

	// Create a watcher
	ws := memdb.NewWatchSet()
	_, err = state.ScalingPolicyByTargetAndType(ws, policy.Target, policy.Type)
	must.NoError(t, err)

	// Delete the policy
	err = state.DeleteScalingPolicies(1001, []string{policy.ID, policy2.ID})
	must.NoError(t, err)

	// Ensure watching triggered
	must.True(t, watchFired(ws))

	// Ensure we don't get the objects back
	ws = memdb.NewWatchSet()
	out, err := state.ScalingPolicyByTargetAndType(ws, policy.Target, policy.Type)
	must.NoError(t, err)
	must.Nil(t, out)

	ws = memdb.NewWatchSet()
	out, err = state.ScalingPolicyByTargetAndType(ws, policy2.Target, policy2.Type)
	must.NoError(t, err)
	must.Nil(t, out)

	// Ensure we see both policies
	iter, err := state.ScalingPoliciesByNamespace(ws, policy.Target[structs.ScalingTargetNamespace], "")
	must.NoError(t, err)
	count := 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
	}
	must.Eq(t, 0, count)

	index, err := state.Index("scaling_policy")
	must.NoError(t, err)
	must.True(t, 1001 == index)
	must.False(t, watchFired(ws))
}

func TestStateStore_StopJob_DeleteScalingPolicies(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	job := mock.Job()

	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	must.NoError(t, err)

	policy := mock.ScalingPolicy()
	policy.Target[structs.ScalingTargetJob] = job.ID
	err = state.UpsertScalingPolicies(1100, []*structs.ScalingPolicy{policy})
	must.NoError(t, err)

	// Ensure the scaling policy is present and start some watches
	wsGet := memdb.NewWatchSet()
	out, err := state.ScalingPolicyByTargetAndType(wsGet, policy.Target, policy.Type)
	must.NoError(t, err)
	must.NotNil(t, out)
	wsList := memdb.NewWatchSet()
	_, err = state.ScalingPolicies(wsList)
	must.NoError(t, err)

	// Stop the job
	job, err = state.JobByID(nil, job.Namespace, job.ID)
	must.NoError(t, err)
	job.Stop = true
	err = state.UpsertJob(structs.MsgTypeTestSetup, 1200, nil, job)
	must.NoError(t, err)

	// Ensure:
	// * the scaling policy was deleted
	// * the watches were fired
	// * the table index was advanced
	must.True(t, watchFired(wsGet))
	must.True(t, watchFired(wsList))
	out, err = state.ScalingPolicyByTargetAndType(nil, policy.Target, policy.Type)
	must.NoError(t, err)
	must.Nil(t, out)
	index, err := state.Index("scaling_policy")
	must.NoError(t, err)
	must.GreaterEq(t, uint64(1200), index)
}

func TestStateStore_UnstopJob_UpsertScalingPolicies(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	job, policy := mock.JobWithScalingPolicy()
	job.Stop = true

	// establish watcher, verify there are no scaling policies yet
	ws := memdb.NewWatchSet()
	list, err := state.ScalingPolicies(ws)
	must.NoError(t, err)
	must.Nil(t, list.Next())

	// upsert a stopped job, verify that we don't fire the watcher or add any scaling policies
	err = state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	must.NoError(t, err)
	must.True(t, watchFired(ws))
	list, err = state.ScalingPolicies(ws)
	must.NoError(t, err)
	must.NotNil(t, list.Next())

	// Establish a new watchset
	ws = memdb.NewWatchSet()
	_, err = state.ScalingPolicies(ws)
	must.NoError(t, err)
	// Unstop this job, say you'll run it again...
	job.Stop = false
	err = state.UpsertJob(structs.MsgTypeTestSetup, 1100, nil, job)
	must.NoError(t, err)

	// Ensure the scaling policy still exists, watch was not fired, index was not advanced
	out, err := state.ScalingPolicyByTargetAndType(nil, policy.Target, policy.Type)
	must.NoError(t, err)
	must.NotNil(t, out)
	index, err := state.Index("scaling_policy")
	must.NoError(t, err)
	must.Eq(t, index, 1000)
	must.False(t, watchFired(ws))
}

func TestStateStore_DeleteJob_DeleteScalingPolicies(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	job := mock.Job()

	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	must.NoError(t, err)

	policy := mock.ScalingPolicy()
	policy.Target[structs.ScalingTargetJob] = job.ID
	err = state.UpsertScalingPolicies(1001, []*structs.ScalingPolicy{policy})
	must.NoError(t, err)

	// Delete the job
	err = state.DeleteJob(1002, job.Namespace, job.ID)
	must.NoError(t, err)

	// Ensure the scaling policy was deleted
	ws := memdb.NewWatchSet()
	out, err := state.ScalingPolicyByTargetAndType(ws, policy.Target, policy.Type)
	must.NoError(t, err)
	must.Nil(t, out)
	index, err := state.Index("scaling_policy")
	must.NoError(t, err)
	must.True(t, index > 1001)
}

func TestStateStore_DeleteJob_DeleteScalingPoliciesPrefixBug(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	job := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job))
	job2 := job.Copy()
	job2.ID = job.ID + "-but-longer"
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, job2))

	policy := mock.ScalingPolicy()
	policy.Target[structs.ScalingTargetJob] = job.ID
	policy2 := mock.ScalingPolicy()
	policy2.Target[structs.ScalingTargetJob] = job2.ID
	must.NoError(t, state.UpsertScalingPolicies(1002, []*structs.ScalingPolicy{policy, policy2}))

	// Delete job with the shorter prefix-ID
	must.NoError(t, state.DeleteJob(1003, job.Namespace, job.ID))

	// Ensure only the associated scaling policy was deleted, not the one matching the job with the longer ID
	out, err := state.ScalingPolicyByID(nil, policy.ID)
	must.NoError(t, err)
	must.Nil(t, out)
	out, err = state.ScalingPolicyByID(nil, policy2.ID)
	must.NoError(t, err)
	must.NotNil(t, out)
}

// This test ensures that deleting a job that doesn't have any scaling policies
// will not cause the scaling_policy table index to increase, on either job
// registration or deletion.
func TestStateStore_DeleteJob_ScalingPolicyIndexNoop(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	job := mock.Job()

	prevIndex, err := state.Index("scaling_policy")
	must.NoError(t, err)

	err = state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, job)
	must.NoError(t, err)

	newIndex, err := state.Index("scaling_policy")
	must.NoError(t, err)
	must.Eq(t, prevIndex, newIndex)

	// Delete the job
	err = state.DeleteJob(1002, job.Namespace, job.ID)
	must.NoError(t, err)

	newIndex, err = state.Index("scaling_policy")
	must.NoError(t, err)
	must.Eq(t, prevIndex, newIndex)
}

func TestStateStore_ScalingPoliciesByType(t *testing.T) {
	ci.Parallel(t)

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
	search := func(ty string) (found []string) {
		found = []string{}
		iter, err := state.ScalingPoliciesByTypePrefix(nil, ty)
		must.NoError(t, err)

		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			found = append(found, raw.(*structs.ScalingPolicy).Type)
		}
		return
	}

	// Create the policies
	var baseIndex uint64 = 1000
	err := state.UpsertScalingPolicies(baseIndex, []*structs.ScalingPolicy{pHorzA, pHorzB, pOther1, pOther2})
	must.NoError(t, err)

	// Check if we can read horizontal policies
	expect := []string{pHorzA.Type, pHorzB.Type}
	actual := search(structs.ScalingPolicyTypeHorizontal)
	must.SliceContainsAll(t, expect, actual)

	// Check if we can read policies of other types
	expect = []string{pOther1.Type}
	actual = search("other-type-1")
	must.SliceContainsAll(t, expect, actual)

	// Check that we can read policies by prefix
	expect = []string{"other-type-1", "other-type-2"}
	actual = search("other-type")
	must.Eq(t, expect, actual)

	// Check for empty result
	expect = []string{}
	actual = search("non-existing")
	must.SliceContainsAll(t, expect, actual)
}

func TestStateStore_ScalingPoliciesByTypePrefix(t *testing.T) {
	ci.Parallel(t)

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
	must.NoError(t, err)

	// Check if we can read horizontal policies
	expect := []string{pHorzA.Type, pHorzB.Type}
	count, found, err := search("h")

	sort.Strings(found)
	sort.Strings(expect)

	must.NoError(t, err)
	must.Eq(t, expect, found)
	must.Eq(t, 2, count)

	// Check if we can read other prefix policies
	expect = []string{pOther1.Type, pOther2.Type}
	count, found, err = search("other")

	sort.Strings(found)
	sort.Strings(expect)

	must.NoError(t, err)
	must.Eq(t, expect, found)
	must.Eq(t, 2, count)

	// Check for empty result
	expect = []string{}
	count, found, err = search("non-existing")

	sort.Strings(found)
	sort.Strings(expect)

	must.NoError(t, err)
	must.Eq(t, expect, found)
	must.Eq(t, 0, count)
}

func TestStateStore_ScalingPoliciesByJob(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	policyA := mock.ScalingPolicy()
	policyB1 := mock.ScalingPolicy()
	policyB2 := mock.ScalingPolicy()
	policyB1.Target[structs.ScalingTargetJob] = policyB2.Target[structs.ScalingTargetJob]

	// Create the policies
	var baseIndex uint64 = 1000
	err := state.UpsertScalingPolicies(baseIndex, []*structs.ScalingPolicy{policyA, policyB1, policyB2})
	must.NoError(t, err)

	iter, err := state.ScalingPoliciesByJob(nil,
		policyA.Target[structs.ScalingTargetNamespace],
		policyA.Target[structs.ScalingTargetJob], "")
	must.NoError(t, err)

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
	must.Eq(t, 1, count)
	sort.Strings(found)
	expect := []string{policyA.Target[structs.ScalingTargetGroup]}
	sort.Strings(expect)
	must.Eq(t, expect, found)

	iter, err = state.ScalingPoliciesByJob(nil,
		policyB1.Target[structs.ScalingTargetNamespace],
		policyB1.Target[structs.ScalingTargetJob], "")
	must.NoError(t, err)

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
	must.Eq(t, 2, count)
	sort.Strings(found)
	expect = []string{
		policyB1.Target[structs.ScalingTargetGroup],
		policyB2.Target[structs.ScalingTargetGroup],
	}
	sort.Strings(expect)
	must.Eq(t, expect, found)
}

func TestStateStore_ScalingPoliciesByJob_PrefixBug(t *testing.T) {
	ci.Parallel(t)

	jobPrefix := "job-name-" + uuid.Generate()

	state := testStateStore(t)
	policy1 := mock.ScalingPolicy()
	policy1.Target[structs.ScalingTargetJob] = jobPrefix
	policy2 := mock.ScalingPolicy()
	policy2.Target[structs.ScalingTargetJob] = jobPrefix + "-more"

	// Create the policies
	var baseIndex uint64 = 1000
	err := state.UpsertScalingPolicies(baseIndex, []*structs.ScalingPolicy{policy1, policy2})
	must.NoError(t, err)

	iter, err := state.ScalingPoliciesByJob(nil,
		policy1.Target[structs.ScalingTargetNamespace],
		jobPrefix, "")
	must.NoError(t, err)

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
	must.Eq(t, 1, count)
	expect := []string{policy1.ID}
	must.Eq(t, expect, found)
}

func TestStateStore_ScalingPolicyByTargetAndType(t *testing.T) {
	ci.Parallel(t)

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
	must.NoError(t, err)

	// Check if we can retrieve the right policies
	found, err := state.ScalingPolicyByTargetAndType(nil, policyA.Target, policyA.Type)
	must.NoError(t, err)
	must.Eq(t, policyA, found)

	// Check for wrong type
	found, err = state.ScalingPolicyByTargetAndType(nil, policyA.Target, "wrong_type")
	must.NoError(t, err)
	must.Nil(t, found)

	// Check for same target but different type
	found, err = state.ScalingPolicyByTargetAndType(nil, policyB.Target, policyB.Type)
	must.NoError(t, err)
	must.Eq(t, policyB, found)

	found, err = state.ScalingPolicyByTargetAndType(nil, policyB.Target, policyC.Type)
	must.NoError(t, err)
	must.Eq(t, policyC, found)
}

func TestStateStore_UpsertScalingEvent(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job := mock.Job()
	groupName := job.TaskGroups[0].Name

	newEvent := structs.NewScalingEvent("message 1")
	newEvent.Meta = map[string]interface{}{
		"a": 1,
	}

	wsAll := memdb.NewWatchSet()
	all, err := state.ScalingEvents(wsAll)
	must.NoError(t, err)
	must.Nil(t, all.Next())

	ws := memdb.NewWatchSet()
	out, _, err := state.ScalingEventsByJob(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Nil(t, out)

	err = state.UpsertScalingEvent(1000, &structs.ScalingEventRequest{
		Namespace:    job.Namespace,
		JobID:        job.ID,
		TaskGroup:    groupName,
		ScalingEvent: newEvent,
	})
	must.NoError(t, err)
	must.True(t, watchFired(ws))
	must.True(t, watchFired(wsAll))

	ws = memdb.NewWatchSet()
	out, eventsIndex, err := state.ScalingEventsByJob(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Eq(t, map[string][]*structs.ScalingEvent{
		groupName: {newEvent},
	}, out)
	must.Eq(t, eventsIndex, 1000)

	iter, err := state.ScalingEvents(ws)
	must.NoError(t, err)

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
	must.Eq(t, 1, count)
	must.Eq(t, jobEvents.ModifyIndex, 1000)
	must.Eq(t, jobEvents.ScalingEvents[groupName][0].CreateIndex, 1000)

	index, err := state.Index("scaling_event")
	must.NoError(t, err)
	must.SliceContainsAll(t, []string{job.ID}, jobsReturned)
	must.Eq(t, map[string][]*structs.ScalingEvent{
		groupName: {newEvent},
	}, jobEvents.ScalingEvents)
	must.Eq(t, 1000, index)
	must.False(t, watchFired(ws))
}

func TestStateStore_UpsertScalingEvent_LimitAndOrder(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	namespace := uuid.Generate()
	jobID := uuid.Generate()
	group1 := uuid.Generate()
	group2 := uuid.Generate()

	index := uint64(1000)
	for i := 1; i <= structs.JobTrackedScalingEvents+10; i++ {
		newEvent := structs.NewScalingEvent("")
		newEvent.Meta = map[string]interface{}{
			"i":     i,
			"group": group1,
		}
		err := state.UpsertScalingEvent(index, &structs.ScalingEventRequest{
			Namespace:    namespace,
			JobID:        jobID,
			TaskGroup:    group1,
			ScalingEvent: newEvent,
		})
		index++
		must.NoError(t, err)

		newEvent = structs.NewScalingEvent("")
		newEvent.Meta = map[string]interface{}{
			"i":     i,
			"group": group2,
		}
		err = state.UpsertScalingEvent(index, &structs.ScalingEventRequest{
			Namespace:    namespace,
			JobID:        jobID,
			TaskGroup:    group2,
			ScalingEvent: newEvent,
		})
		index++
		must.NoError(t, err)
	}

	out, _, err := state.ScalingEventsByJob(nil, namespace, jobID)
	must.NoError(t, err)
	must.MapLen(t, 2, out)

	expectedEvents := []int{}
	for i := structs.JobTrackedScalingEvents; i > 0; i-- {
		expectedEvents = append(expectedEvents, i+10)
	}

	// checking order and content
	must.Len(t, structs.JobTrackedScalingEvents, out[group1])
	actualEvents := []int{}
	for _, event := range out[group1] {
		must.Eq(t, group1, event.Meta["group"].(string))
		actualEvents = append(actualEvents, event.Meta["i"].(int))
	}
	must.Eq(t, expectedEvents, actualEvents)

	// checking order and content
	must.Len(t, structs.JobTrackedScalingEvents, out[group2])
	actualEvents = []int{}
	for _, event := range out[group2] {
		must.Eq(t, group2, event.Meta["group"].(string))
		actualEvents = append(actualEvents, event.Meta["i"].(int))
	}
	must.Eq(t, expectedEvents, actualEvents)
}

func TestStateStore_Abandon(t *testing.T) {
	ci.Parallel(t)

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
	ci.Parallel(t)

	state := testStateStore(t)
	alloc := mock.Alloc()

	// Insert job
	err := state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, alloc.Job)
	must.NoError(t, err)

	allocDiffs := []*structs.AllocationDiff{
		{
			ID: alloc.ID,
		},
	}

	snap, err := state.Snapshot()
	must.NoError(t, err)

	denormalizedAllocs, err := snap.DenormalizeAllocationDiffSlice(allocDiffs)

	must.EqError(t, err, fmt.Sprintf("alloc %v doesn't exist", alloc.ID))
	must.Nil(t, denormalizedAllocs)
}

// TestStateStore_SnapshotMinIndex_OK asserts StateStore.SnapshotMinIndex blocks
// until the StateStore's latest index is >= the requested index.
func TestStateStore_SnapshotMinIndex_OK(t *testing.T) {
	ci.Parallel(t)

	s := testStateStore(t)
	index, err := s.LatestIndex()
	must.NoError(t, err)

	node := mock.Node()
	must.NoError(t, s.UpsertNode(structs.MsgTypeTestSetup, index+1, node))

	// Assert SnapshotMinIndex returns immediately if index < latest index
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	snap, err := s.SnapshotMinIndex(ctx, index)
	cancel()
	must.NoError(t, err)

	snapIndex, err := snap.LatestIndex()
	must.NoError(t, err)
	if snapIndex <= index {
		t.Fatal("snapshot index should be greater than index")
	}

	// Assert SnapshotMinIndex returns immediately if index == latest index
	ctx, cancel = context.WithTimeout(context.Background(), 0)
	snap, err = s.SnapshotMinIndex(ctx, index+1)
	cancel()
	must.NoError(t, err)

	snapIndex, err = snap.LatestIndex()
	must.NoError(t, err)
	must.Eq(t, snapIndex, index+1)

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
		must.NoError(t, err)
	case <-time.After(500 * time.Millisecond):
		// Let it block for a bit before unblocking by upserting
	}

	node.Name = "hal"
	must.NoError(t, s.UpsertNode(structs.MsgTypeTestSetup, index+2, node))

	select {
	case err := <-errCh:
		must.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for SnapshotMinIndex to unblock")
	}
}

// TestStateStore_SnapshotMinIndex_Timeout asserts StateStore.SnapshotMinIndex
// returns an error if the desired index is not reached within the deadline.
func TestStateStore_SnapshotMinIndex_Timeout(t *testing.T) {
	ci.Parallel(t)

	s := testStateStore(t)
	index, err := s.LatestIndex()
	must.NoError(t, err)

	// Assert SnapshotMinIndex blocks if index > latest index
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	snap, err := s.SnapshotMinIndex(ctx, index+1)
	must.EqError(t, err, context.DeadlineExceeded.Error())
	must.Nil(t, snap)
}

// TestStateStore_CancelFollowupEvalsOnReconnect exercises the behavior when an
// alloc on a disconnected node reconnects and may need to cancel any follow-up
// evals.
func TestStateStore_CancelFollowupEvalsOnReconnect(t *testing.T) {
	ci.Parallel(t)

	evalID := uuid.Generate()

	testCases := []struct {
		name                 string
		alloc0NewStatus      string // alloc0 will be the alloc we update
		alloc1Status         string // alloc1 will be another alloc on the same job
		alloc1FollowupEvalID string
		expectEvalStatus     string
	}{
		{
			name:             "reconnecting alloc cancels followup eval",
			alloc0NewStatus:  structs.AllocClientStatusRunning,
			alloc1Status:     structs.AllocClientStatusRunning,
			expectEvalStatus: structs.EvalStatusCancelled,
		},
		{
			name:                 "allocs waiting on same followup block cancel",
			alloc0NewStatus:      structs.AllocClientStatusRunning,
			alloc1Status:         structs.AllocClientStatusUnknown,
			alloc1FollowupEvalID: evalID,
			expectEvalStatus:     structs.EvalStatusPending,
		},
		{
			name:                 "allocs waiting on different followup allow cancel",
			alloc0NewStatus:      structs.AllocClientStatusRunning,
			alloc1Status:         structs.AllocClientStatusUnknown,
			alloc1FollowupEvalID: uuid.Generate(),
			expectEvalStatus:     structs.EvalStatusCancelled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(*testing.T) {
			store := testStateStore(t)
			index, err := store.LatestIndex()
			must.NoError(t, err)

			job := mock.MinJob()
			must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, index, nil, job))

			index++
			followupEval := mock.Eval()
			followupEval.ID = evalID
			followupEval.JobID = job.ID
			followupEval.Status = structs.EvalStatusPending
			followupEval.TriggeredBy = structs.EvalTriggerMaxDisconnectTimeout
			must.NoError(t, store.UpsertEvals(structs.MsgTypeTestSetup, index,
				[]*structs.Evaluation{followupEval}))

			index++
			alloc0, alloc1 := mock.MinAlloc(), mock.MinAlloc()
			alloc0.JobID = job.ID
			alloc0.ClientStatus = structs.AllocClientStatusUnknown
			alloc0.FollowupEvalID = evalID
			alloc0.AllocStates = []*structs.AllocState{{
				Field: structs.AllocStateFieldClientStatus,
				Value: structs.AllocClientStatusUnknown,
			}}
			alloc1.JobID = job.ID
			alloc1.ClientStatus = tc.alloc1Status
			alloc1.FollowupEvalID = tc.alloc1FollowupEvalID
			must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, index,
				[]*structs.Allocation{alloc0, alloc1}))

			index++
			alloc0 = alloc0.Copy()
			alloc0.ClientStatus = tc.alloc0NewStatus
			must.NoError(t, store.UpdateAllocsFromClient(structs.MsgTypeTestSetup, index,
				[]*structs.Allocation{alloc0}))

			eval, err := store.EvalByID(nil, evalID)
			must.NoError(t, err)
			must.NotNil(t, eval)
			must.Eq(t, tc.expectEvalStatus, eval.Status)

			alloc, err := store.AllocByID(nil, alloc0.ID)
			must.NoError(t, err)
			must.NotNil(t, alloc)
			must.Eq(t, "", alloc.FollowupEvalID)
		})
	}
}

func TestStateStore_CheckIdempotencyToken(t *testing.T) {
	ci.Parallel(t)

	store := testStateStore(t)
	parent := mock.BatchJob()
	parent.ID = "parent"
	index, _ := store.LatestIndex()

	index++
	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, index, nil, parent))

	token := uuid.Generate()
	dispatch1 := parent.Copy()
	dispatch1.ParentID = parent.ID
	dispatch1.ID = structs.DispatchedID(parent.ID, "", time.Now())
	dispatch1.DispatchIdempotencyToken = token
	index++
	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, index, nil, dispatch1))

	found, err := store.CheckIdempotencyToken(parent.Namespace, parent.ID, token)
	must.NoError(t, err)
	must.NotNil(t, found)
	must.Eq(t, index, found.CreateIndex)

	other := uuid.Generate()
	found, err = store.CheckIdempotencyToken(parent.Namespace, parent.ID, other)
	must.NoError(t, err)
	must.Nil(t, found)
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
