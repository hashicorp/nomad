package state

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		state.UpsertNode(11, node)
	})

	resp, idx, err := state.BlockingQuery(queryFn, 10, deadlineCtx)
	if assert.Nil(t, err) {
		assert.Equal(t, 2, count)
		assert.EqualValues(t, 11, idx)
		assert.True(t, resp.(bool))
	}
}

// This test checks that:
// 1) The job is denormalized
// 2) Allocations are created
func TestStateStore_UpsertPlanResults_AllocationsCreated_Denormalized(t *testing.T) {
	state := testStateStore(t)
	alloc := mock.Alloc()
	job := alloc.Job
	alloc.Job = nil

	if err := state.UpsertJob(999, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	eval := mock.Eval()
	eval.JobID = job.ID

	// Create an eval
	if err := state.UpsertEvals(1, []*structs.Evaluation{eval}); err != nil {
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
	err := state.UpsertPlanResults(1000, &res)
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

// This test checks that the deployment is created and allocations count towards
// the deployment
func TestStateStore_UpsertPlanResults_Deployment(t *testing.T) {
	state := testStateStore(t)
	alloc := mock.Alloc()
	alloc2 := mock.Alloc()
	job := alloc.Job
	alloc.Job = nil
	alloc2.Job = nil

	d := mock.Deployment()
	alloc.DeploymentID = d.ID
	alloc2.DeploymentID = d.ID

	if err := state.UpsertJob(999, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	eval := mock.Eval()
	eval.JobID = job.ID

	// Create an eval
	if err := state.UpsertEvals(1, []*structs.Evaluation{eval}); err != nil {
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

	err := state.UpsertPlanResults(1000, &res)
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

	err = state.UpsertPlanResults(1001, &res)
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
	require := require.New(t)

	state := testStateStore(t)
	alloc := mock.Alloc()
	job := alloc.Job
	alloc.Job = nil

	// Insert job
	err := state.UpsertJob(999, job)
	require.NoError(err)

	// Create an eval
	eval := mock.Eval()
	eval.JobID = job.ID
	err = state.UpsertEvals(1, []*structs.Evaluation{eval})
	require.NoError(err)

	// Insert alloc that will be preempted in the plan
	preemptedAlloc := mock.Alloc()
	err = state.UpsertAllocs(2, []*structs.Allocation{preemptedAlloc})
	require.NoError(err)

	minimalPreemptedAlloc := &structs.Allocation{
		ID:                 preemptedAlloc.ID,
		Namespace:          preemptedAlloc.Namespace,
		DesiredStatus:      structs.AllocDesiredStatusEvict,
		ModifyTime:         time.Now().Unix(),
		DesiredDescription: fmt.Sprintf("Preempted by allocation %v", alloc.ID),
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

	err = state.UpsertPlanResults(1000, &res)
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
	require.Equal(preempted.DesiredDescription, fmt.Sprintf("Preempted by allocation %v", alloc.ID))

	// Verify eval for preempted job
	preemptedJobEval, err := state.EvalByID(ws, eval2.ID)
	require.NoError(err)
	require.NotNil(preemptedJobEval)
	require.EqualValues(1000, preemptedJobEval.ModifyIndex)

}

// This test checks that deployment updates are applied correctly
func TestStateStore_UpsertPlanResults_DeploymentUpdates(t *testing.T) {
	state := testStateStore(t)

	// Create a job that applies to all
	job := mock.Job()
	if err := state.UpsertJob(998, job); err != nil {
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
	if err := state.UpsertEvals(1, []*structs.Evaluation{eval}); err != nil {
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

	err := state.UpsertPlanResults(1000, &res)
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
	state := testStateStore(t)
	deployment := mock.Deployment()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.DeploymentsByJobID(ws, deployment.Namespace, deployment.ID)
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

func TestStateStore_DeleteDeployment(t *testing.T) {
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
	state := testStateStore(t)
	var deployments []*structs.Deployment

	for i := 0; i < 10; i++ {
		deployment := mock.Deployment()
		deployments = append(deployments, deployment)

		err := state.UpsertDeployment(1000+uint64(i), deployment)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	ws := memdb.NewWatchSet()
	iter, err := state.Deployments(ws)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out []*structs.Deployment
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Deployment))
	}

	lessThan := func(i, j int) bool {
		return deployments[i].ID < deployments[j].ID
	}
	sort.Slice(deployments, lessThan)
	sort.Slice(out, lessThan)

	if !reflect.DeepEqual(deployments, out) {
		t.Fatalf("bad: %#v %#v", deployments, out)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_DeploymentsByIDPrefix(t *testing.T) {
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
	require := require.New(t)
	state := testStateStore(t)
	node := mock.Node()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NodeByID(ws, node.ID)
	require.NoError(err)

	require.NoError(state.UpsertNode(1000, node))
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
	require.NoError(state.UpsertNode(1001, down))
	require.NoError(state.UpsertNode(1002, out))

	out, err = state.NodeByID(ws, node.ID)
	require.NoError(err)
	require.Len(out.Events, 2)
	require.Equal(NodeRegisterEventReregistered, out.Events[1].Message)
}

func TestStateStore_DeleteNode_Node(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()

	err := state.UpsertNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watchset so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.NodeByID(ws, node.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	err = state.DeleteNode(1001, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", node, out)
	}

	index, err := state.Index("nodes")
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

func TestStateStore_UpdateNodeStatus_Node(t *testing.T) {
	require := require.New(t)
	state := testStateStore(t)
	node := mock.Node()

	require.NoError(state.UpsertNode(800, node))

	// Create a watchset so we can test that update node status fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NodeByID(ws, node.ID)
	require.NoError(err)

	event := &structs.NodeEvent{
		Message:   "Node ready foo",
		Subsystem: structs.NodeEventSubsystemCluster,
		Timestamp: time.Now(),
	}

	require.NoError(state.UpdateNodeStatus(801, node.ID, structs.NodeStatusReady, event))
	require.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	require.NoError(err)
	require.Equal(structs.NodeStatusReady, out.Status)
	require.EqualValues(801, out.ModifyIndex)
	require.Len(out.Events, 2)
	require.Equal(event.Message, out.Events[1].Message)

	index, err := state.Index("nodes")
	require.NoError(err)
	require.EqualValues(801, index)
	require.False(watchFired(ws))
}

func TestStateStore_BatchUpdateNodeDrain(t *testing.T) {
	require := require.New(t)
	state := testStateStore(t)

	n1, n2 := mock.Node(), mock.Node()
	require.Nil(state.UpsertNode(1000, n1))
	require.Nil(state.UpsertNode(1001, n2))

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

	require.Nil(state.BatchUpdateNodeDrain(1002, update, events))
	require.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	for _, id := range []string{n1.ID, n2.ID} {
		out, err := state.NodeByID(ws, id)
		require.Nil(err)
		require.True(out.Drain)
		require.NotNil(out.DrainStrategy)
		require.Equal(out.DrainStrategy, expectedDrain)
		require.Len(out.Events, 2)
		require.EqualValues(1002, out.ModifyIndex)
	}

	index, err := state.Index("nodes")
	require.Nil(err)
	require.EqualValues(1002, index)
	require.False(watchFired(ws))
}

func TestStateStore_UpdateNodeDrain_Node(t *testing.T) {
	require := require.New(t)
	state := testStateStore(t)
	node := mock.Node()

	require.Nil(state.UpsertNode(1000, node))

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
	require.Nil(state.UpdateNodeDrain(1001, node.ID, expectedDrain, false, event))
	require.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	require.Nil(err)
	require.True(out.Drain)
	require.NotNil(out.DrainStrategy)
	require.Equal(out.DrainStrategy, expectedDrain)
	require.Len(out.Events, 2)
	require.EqualValues(1001, out.ModifyIndex)

	index, err := state.Index("nodes")
	require.Nil(err)
	require.EqualValues(1001, index)
	require.False(watchFired(ws))
}

func TestStateStore_AddSingleNodeEvent(t *testing.T) {
	require := require.New(t)
	state := testStateStore(t)

	node := mock.Node()

	// We create a new node event every time we register a node
	err := state.UpsertNode(1000, node)
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
	err = state.UpsertNodeEvents(uint64(1001), nodeEvents)
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
	require := require.New(t)
	state := testStateStore(t)

	node := mock.Node()

	err := state.UpsertNode(1000, node)
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
		err := state.UpsertNodeEvents(uint64(i), nodeEvents)
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
	require := require.New(t)
	state := testStateStore(t)
	node := mock.Node()
	require.Nil(state.UpsertNode(1000, node))

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
	require.Nil(state.UpdateNodeDrain(1001, node.ID, drain, false, event1))
	require.True(watchFired(ws))

	// Remove the drain
	event2 := &structs.NodeEvent{
		Message:   "Drain strategy disabled",
		Subsystem: structs.NodeEventSubsystemDrain,
		Timestamp: time.Now(),
	}
	require.Nil(state.UpdateNodeDrain(1002, node.ID, nil, true, event2))

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	require.Nil(err)
	require.False(out.Drain)
	require.Nil(out.DrainStrategy)
	require.Equal(out.SchedulingEligibility, structs.NodeSchedulingEligible)
	require.Len(out.Events, 3)
	require.EqualValues(1002, out.ModifyIndex)

	index, err := state.Index("nodes")
	require.Nil(err)
	require.EqualValues(1002, index)
	require.False(watchFired(ws))
}

func TestStateStore_UpdateNodeEligibility(t *testing.T) {
	require := require.New(t)
	state := testStateStore(t)
	node := mock.Node()

	err := state.UpsertNode(1000, node)
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
	require.Nil(state.UpdateNodeEligibility(1001, node.ID, expectedEligibility, event))
	require.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	require.Nil(err)
	require.Equal(out.SchedulingEligibility, expectedEligibility)
	require.Len(out.Events, 2)
	require.Equal(out.Events[1], event)
	require.EqualValues(1001, out.ModifyIndex)

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
	require.Nil(state.UpdateNodeDrain(1002, node.ID, expectedDrain, false, nil))

	// Try to set the node to eligible
	err = state.UpdateNodeEligibility(1003, node.ID, structs.NodeSchedulingEligible, nil)
	require.NotNil(err)
	require.Contains(err.Error(), "while it is draining")
}

func TestStateStore_Nodes(t *testing.T) {
	state := testStateStore(t)
	var nodes []*structs.Node

	for i := 0; i < 10; i++ {
		node := mock.Node()
		nodes = append(nodes, node)

		err := state.UpsertNode(1000+uint64(i), node)
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
	state := testStateStore(t)
	node := mock.Node()

	node.ID = "11111111-662e-d0ab-d1c9-3e434af7bdb4"
	err := state.UpsertNode(1000, node)
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
	err = state.UpsertNode(1001, node)
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

func TestStateStore_RestoreNode(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.NodeRestore(node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, node) {
		t.Fatalf("Bad: %#v %#v", out, node)
	}
}

func TestStateStore_UpsertJob_Job(t *testing.T) {
	state := testStateStore(t)
	job := mock.Job()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	if err := state.UpsertJob(1000, job); err != nil {
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
	state := testStateStore(t)
	job := mock.Job()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	if err := state.UpsertJob(1000, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	job2 := mock.Job()
	job2.ID = job.ID
	job2.AllAtOnce = true
	err = state.UpsertJob(1001, job2)
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
	state := testStateStore(t)
	job := mock.PeriodicJob()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	if err := state.UpsertJob(1000, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a child and an evaluation
	job2 := job.Copy()
	job2.Periodic = nil
	job2.ID = fmt.Sprintf("%v/%s-1490635020", job.ID, structs.PeriodicLaunchSuffix)
	err = state.UpsertJob(1001, job2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	eval := mock.Eval()
	eval.JobID = job2.ID
	err = state.UpsertEvals(1002, []*structs.Evaluation{eval})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	job3 := job.Copy()
	job3.TaskGroups[0].Tasks[0].Name = "new name"
	err = state.UpsertJob(1003, job3)
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
	assert := assert.New(t)
	state := testStateStore(t)
	job := mock.Job()
	job.Namespace = "foo"

	err := state.UpsertJob(1000, job)
	assert.Contains(err.Error(), "nonexistent namespace")

	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	assert.Nil(err)
	assert.Nil(out)
}

// Upsert a job that is the child of a parent job and ensures its summary gets
// updated.
func TestStateStore_UpsertJob_ChildJob(t *testing.T) {
	state := testStateStore(t)

	// Create a watchset so we can test that upsert fires the watch
	parent := mock.Job()
	ws := memdb.NewWatchSet()
	_, err := state.JobByID(ws, parent.Namespace, parent.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	if err := state.UpsertJob(1000, parent); err != nil {
		t.Fatalf("err: %v", err)
	}

	child := mock.Job()
	child.ParentID = parent.ID
	if err := state.UpsertJob(1001, child); err != nil {
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

	if err := state.UpsertJob(1000, job); err != nil {
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
		err = state.UpsertJob(uint64(1000+i), finalJob)
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
	state := testStateStore(t)
	job := mock.Job()

	err := state.UpsertJob(1000, job)
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
	state := testStateStore(t)

	const testJobCount = 10
	const jobVersionCount = 4

	stateIndex := uint64(1000)

	jobs := make([]*structs.Job, testJobCount)
	for i := 0; i < testJobCount; i++ {
		stateIndex++
		job := mock.BatchJob()

		err := state.UpsertJob(stateIndex, job)
		require.NoError(t, err)

		jobs[i] = job

		// Create some versions
		for vi := 1; vi < jobVersionCount; vi++ {
			stateIndex++

			job := job.Copy()
			job.TaskGroups[0].Tasks[0].Env = map[string]string{
				"Version": fmt.Sprintf("%d", vi),
			}

			require.NoError(t, state.UpsertJob(stateIndex, job))
		}
	}

	ws := memdb.NewWatchSet()

	// Sanity check that jobs are present in DB
	job, err := state.JobByID(ws, jobs[0].Namespace, jobs[0].ID)
	require.NoError(t, err)
	require.Equal(t, jobs[0].ID, job.ID)

	jobVersions, err := state.JobVersionsByID(ws, jobs[0].Namespace, jobs[0].ID)
	require.NoError(t, err)
	require.Equal(t, jobVersionCount, len(jobVersions))

	// Actually delete
	const deletionIndex = uint64(10001)
	err = state.WithWriteTransaction(func(txn Txn) error {
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
	assert.Nil(state.UpsertJob(1000, job))
	assert.True(watchFired(ws))

	var finalJob *structs.Job
	for i := 1; i < 20; i++ {
		finalJob = mock.Job()
		finalJob.ID = job.ID
		finalJob.Priority = i
		assert.Nil(state.UpsertJob(uint64(1000+i), finalJob))
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
	state := testStateStore(t)

	parent := mock.Job()
	if err := state.UpsertJob(998, parent); err != nil {
		t.Fatalf("err: %v", err)
	}

	child := mock.Job()
	child.ParentID = parent.ID

	if err := state.UpsertJob(999, child); err != nil {
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
	state := testStateStore(t)
	var jobs []*structs.Job

	for i := 0; i < 10; i++ {
		job := mock.Job()
		jobs = append(jobs, job)

		err := state.UpsertJob(1000+uint64(i), job)
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
	state := testStateStore(t)
	var jobs []*structs.Job

	for i := 0; i < 10; i++ {
		job := mock.Job()
		jobs = append(jobs, job)

		err := state.UpsertJob(1000+uint64(i), job)
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
	state := testStateStore(t)
	job := mock.Job()

	job.ID = "redis"
	err := state.UpsertJob(1000, job)
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
	err = state.UpsertJob(1001, job)
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
	state := testStateStore(t)
	var periodic, nonPeriodic []*structs.Job

	for i := 0; i < 10; i++ {
		job := mock.Job()
		nonPeriodic = append(nonPeriodic, job)

		err := state.UpsertJob(1000+uint64(i), job)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	for i := 0; i < 10; i++ {
		job := mock.PeriodicJob()
		periodic = append(periodic, job)

		err := state.UpsertJob(2000+uint64(i), job)
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
	state := testStateStore(t)
	var serviceJobs []*structs.Job
	var sysJobs []*structs.Job

	for i := 0; i < 10; i++ {
		job := mock.Job()
		serviceJobs = append(serviceJobs, job)

		err := state.UpsertJob(1000+uint64(i), job)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	for i := 0; i < 10; i++ {
		job := mock.SystemJob()
		job.Status = structs.JobStatusRunning
		sysJobs = append(sysJobs, job)

		err := state.UpsertJob(2000+uint64(i), job)
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

		if err := state.UpsertJob(1000+uint64(i), job); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	for i := 0; i < 20; i += 2 {
		job := mock.Job()
		job.Type = structs.JobTypeBatch
		gc[job.ID] = struct{}{}

		if err := state.UpsertJob(2000+uint64(i), job); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Create an eval for it
		eval := mock.Eval()
		eval.JobID = job.ID
		eval.Status = structs.EvalStatusComplete
		if err := state.UpsertEvals(2000+uint64(i+1), []*structs.Evaluation{eval}); err != nil {
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

func TestStateStore_RestoreJob(t *testing.T) {
	state := testStateStore(t)
	job := mock.Job()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.JobRestore(job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, job) {
		t.Fatalf("Bad: %#v %#v", out, job)
	}
}

func TestStateStore_UpsertPeriodicLaunch(t *testing.T) {
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

func TestStateStore_RestorePeriodicLaunch(t *testing.T) {
	state := testStateStore(t)
	job := mock.Job()
	launch := &structs.PeriodicLaunch{
		ID:        job.ID,
		Namespace: job.Namespace,
		Launch:    time.Now(),
	}

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.PeriodicLaunchRestore(launch)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.PeriodicLaunchByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, launch) {
		t.Fatalf("Bad: %#v %#v", out, job)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_RestoreJobVersion(t *testing.T) {
	state := testStateStore(t)
	job := mock.Job()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.JobVersionRestore(job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.JobByIDAndVersion(ws, job.Namespace, job.ID, job.Version)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, job) {
		t.Fatalf("Bad: %#v %#v", out, job)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_RestoreDeployment(t *testing.T) {
	state := testStateStore(t)
	d := mock.Deployment()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.DeploymentRestore(d)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.DeploymentByID(ws, d.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, d) {
		t.Fatalf("Bad: %#v %#v", out, d)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_RestoreJobSummary(t *testing.T) {
	state := testStateStore(t)
	job := mock.Job()
	jobSummary := &structs.JobSummary{
		JobID:     job.ID,
		Namespace: job.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {
				Starting: 10,
			},
		},
	}
	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.JobSummaryRestore(jobSummary)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.JobSummaryByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, jobSummary) {
		t.Fatalf("Bad: %#v %#v", out, jobSummary)
	}
}

func TestStateStore_Indexes(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()

	err := state.UpsertNode(1000, node)
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
	if l := len(out); l != 1 && l != 2 {
		t.Fatalf("unexpected number of index entries: %v", out)
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
	state := testStateStore(t)

	if err := state.UpsertNode(1000, mock.Node()); err != nil {
		t.Fatalf("err: %v", err)
	}

	exp := uint64(2000)
	if err := state.UpsertJob(exp, mock.Job()); err != nil {
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

func TestStateStore_RestoreIndex(t *testing.T) {
	state := testStateStore(t)

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	index := &IndexEntry{"jobs", 1000}
	err = restore.IndexRestore(index)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	restore.Commit()

	out, err := state.Index("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != 1000 {
		t.Fatalf("Bad: %#v %#v", out, 1000)
	}
}

func TestStateStore_UpsertEvals_Eval(t *testing.T) {
	state := testStateStore(t)
	eval := mock.Eval()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.EvalByID(ws, eval.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
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
	state := testStateStore(t)

	// Create two blocked evals for the same job
	j := "test-job"
	b1, b2 := mock.Eval(), mock.Eval()
	b1.JobID = j
	b1.Status = structs.EvalStatusBlocked
	b2.JobID = j
	b2.Status = structs.EvalStatusBlocked

	err := state.UpsertEvals(999, []*structs.Evaluation{b1, b2})
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

	if err := state.UpsertEvals(1000, []*structs.Evaluation{eval}); err != nil {
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
	state := testStateStore(t)
	eval := mock.Eval()

	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
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
	err = state.UpsertEvals(1001, []*structs.Evaluation{eval2})
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
	state := testStateStore(t)

	parent := mock.Job()
	if err := state.UpsertJob(998, parent); err != nil {
		t.Fatalf("err: %v", err)
	}

	child := mock.Job()
	child.ParentID = parent.ID

	if err := state.UpsertJob(999, child); err != nil {
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

	err := state.UpsertEvals(1000, []*structs.Evaluation{eval})
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
	err := state.UpsertEvals(1000, []*structs.Evaluation{eval1, eval2})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.UpsertAllocs(1001, []*structs.Allocation{alloc1, alloc2})
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
	state := testStateStore(t)

	parent := mock.Job()
	if err := state.UpsertJob(998, parent); err != nil {
		t.Fatalf("err: %v", err)
	}

	child := mock.Job()
	child.ParentID = parent.ID

	if err := state.UpsertJob(999, child); err != nil {
		t.Fatalf("err: %v", err)
	}

	eval1 := mock.Eval()
	eval1.JobID = child.ID
	alloc1 := mock.Alloc()
	alloc1.JobID = child.ID

	err := state.UpsertEvals(1000, []*structs.Evaluation{eval1})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.UpsertAllocs(1001, []*structs.Allocation{alloc1})
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
	state := testStateStore(t)

	eval1 := mock.Eval()
	eval2 := mock.Eval()
	eval2.JobID = eval1.JobID
	eval3 := mock.Eval()
	evals := []*structs.Evaluation{eval1, eval2}

	err := state.UpsertEvals(1000, evals)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	err = state.UpsertEvals(1001, []*structs.Evaluation{eval3})
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
	state := testStateStore(t)
	var evals []*structs.Evaluation

	for i := 0; i < 10; i++ {
		eval := mock.Eval()
		evals = append(evals, eval)

		err := state.UpsertEvals(1000+uint64(i), []*structs.Evaluation{eval})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	ws := memdb.NewWatchSet()
	iter, err := state.Evals(ws)
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

	err := state.UpsertEvals(1000, evals)
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

func TestStateStore_RestoreEval(t *testing.T) {
	state := testStateStore(t)
	eval := mock.Eval()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.EvalRestore(eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, eval) {
		t.Fatalf("Bad: %#v %#v", out, eval)
	}
}

func TestStateStore_UpdateAllocsFromClient(t *testing.T) {
	state := testStateStore(t)
	parent := mock.Job()
	if err := state.UpsertJob(998, parent); err != nil {
		t.Fatalf("err: %v", err)
	}

	child := mock.Job()
	child.ParentID = parent.ID
	if err := state.UpsertJob(999, child); err != nil {
		t.Fatalf("err: %v", err)
	}

	alloc := mock.Alloc()
	alloc.JobID = child.ID
	alloc.Job = child

	err := state.UpsertAllocs(1000, []*structs.Allocation{alloc})
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
	err = state.UpdateAllocsFromClient(1001, []*structs.Allocation{update})
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
	state := testStateStore(t)
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()

	if err := state.UpsertJob(999, alloc1.Job); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := state.UpsertJob(999, alloc2.Job); err != nil {
		t.Fatalf("err: %v", err)
	}

	err := state.UpsertAllocs(1000, []*structs.Allocation{alloc1, alloc2})
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

	err = state.UpdateAllocsFromClient(1001, []*structs.Allocation{update, update2})
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
	state := testStateStore(t)
	alloc := mock.Alloc()

	if err := state.UpsertJob(999, alloc.Job); err != nil {
		t.Fatalf("err: %v", err)
	}
	err := state.UpsertAllocs(1000, []*structs.Allocation{alloc})
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

	err = state.UpdateAllocsFromClient(1001, []*structs.Allocation{update, update2})
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
	require := require.New(t)
	state := testStateStore(t)

	alloc := mock.Alloc()
	now := time.Now()
	alloc.CreateTime = now.UnixNano()
	pdeadline := 5 * time.Minute
	deployment := mock.Deployment()
	deployment.TaskGroups[alloc.TaskGroup].ProgressDeadline = pdeadline
	alloc.DeploymentID = deployment.ID

	require.Nil(state.UpsertJob(999, alloc.Job))
	require.Nil(state.UpsertDeployment(1000, deployment))
	require.Nil(state.UpsertAllocs(1001, []*structs.Allocation{alloc}))

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
	require.Nil(state.UpdateAllocsFromClient(1001, []*structs.Allocation{update}))

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

	require.Nil(state.UpsertJob(999, alloc.Job))
	require.Nil(state.UpsertDeployment(1000, deployment))
	require.Nil(state.UpsertAllocs(1001, []*structs.Allocation{alloc}))

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
	require.Nil(state.UpdateAllocsFromClient(1001, []*structs.Allocation{update}))

	// Check that the merging of the deployment status was correct
	out, err := state.AllocByID(nil, alloc.ID)
	require.Nil(err)
	require.NotNil(out)
	require.True(out.DeploymentStatus.Canary)
	require.NotNil(out.DeploymentStatus.Healthy)
	require.True(*out.DeploymentStatus.Healthy)
}

func TestStateStore_UpsertAlloc_Alloc(t *testing.T) {
	state := testStateStore(t)
	alloc := mock.Alloc()

	if err := state.UpsertJob(999, alloc.Job); err != nil {
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

	err := state.UpsertAllocs(1000, []*structs.Allocation{alloc})
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

	require.Nil(state.UpsertJob(999, alloc.Job))
	require.Nil(state.UpsertDeployment(1000, deployment))

	// Create a watch set so we can test that update fires the watch
	ws := memdb.NewWatchSet()
	require.Nil(state.AllocsByDeployment(ws, alloc.DeploymentID))

	err := state.UpsertAllocs(1001, []*structs.Allocation{alloc})
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
	state := testStateStore(t)
	alloc := mock.Alloc()
	alloc.Job = nil

	err := state.UpsertAllocs(999, []*structs.Allocation{alloc})
	if err == nil || !strings.Contains(err.Error(), "without a job") {
		t.Fatalf("expect err: %v", err)
	}
}

func TestStateStore_UpsertAlloc_ChildJob(t *testing.T) {
	state := testStateStore(t)

	parent := mock.Job()
	if err := state.UpsertJob(998, parent); err != nil {
		t.Fatalf("err: %v", err)
	}

	child := mock.Job()
	child.ParentID = parent.ID

	if err := state.UpsertJob(999, child); err != nil {
		t.Fatalf("err: %v", err)
	}

	alloc := mock.Alloc()
	alloc.JobID = child.ID
	alloc.Job = child

	// Create watchsets so we can test that delete fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.JobSummaryByID(ws, parent.Namespace, parent.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	err := state.UpsertAllocs(1000, []*structs.Allocation{alloc})
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
	if summary.Children.Pending != 0 || summary.Children.Running != 1 || summary.Children.Dead != 0 {
		t.Fatalf("bad children summary: %v", summary.Children)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpdateAlloc_Alloc(t *testing.T) {
	state := testStateStore(t)
	alloc := mock.Alloc()

	if err := state.UpsertJob(999, alloc.Job); err != nil {
		t.Fatalf("err: %v", err)
	}

	err := state.UpsertAllocs(1000, []*structs.Allocation{alloc})
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

	err = state.UpsertAllocs(1002, []*structs.Allocation{alloc2})
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
	state := testStateStore(t)
	alloc := mock.Alloc()
	alloc.ClientStatus = "foo"

	if err := state.UpsertJob(999, alloc.Job); err != nil {
		t.Fatalf("err: %v", err)
	}

	err := state.UpsertAllocs(1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	alloc2 := new(structs.Allocation)
	*alloc2 = *alloc
	alloc2.ClientStatus = structs.AllocClientStatusLost
	if err := state.UpsertAllocs(1001, []*structs.Allocation{alloc2}); err != nil {
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
	state := testStateStore(t)
	alloc := mock.Alloc()

	// Upsert a job
	state.UpsertJobSummary(998, mock.JobSummary(alloc.JobID))
	if err := state.UpsertJob(999, alloc.Job); err != nil {
		t.Fatalf("err: %v", err)
	}

	err := state.UpsertAllocs(1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := state.DeleteJob(1001, alloc.Namespace, alloc.JobID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the desired state of the allocation to stop
	allocCopy := alloc.Copy()
	allocCopy.DesiredStatus = structs.AllocDesiredStatusStop
	if err := state.UpsertAllocs(1002, []*structs.Allocation{allocCopy}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the client state of the allocation to complete
	allocCopy1 := allocCopy.Copy()
	allocCopy1.ClientStatus = structs.AllocClientStatusComplete
	if err := state.UpdateAllocsFromClient(1003, []*structs.Allocation{allocCopy1}); err != nil {
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

	require.Nil(state.UpsertJob(999, alloc.Job))
	require.Nil(state.UpsertAllocs(1000, []*structs.Allocation{alloc}))

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
	require.Nil(state.UpdateAllocsDesiredTransitions(1001, m, evals))

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
	require.Nil(state.UpdateAllocsDesiredTransitions(1002, m, evals))

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
	require.Nil(state.UpdateAllocsDesiredTransitions(1003, m, evals))
}

func TestStateStore_JobSummary(t *testing.T) {
	state := testStateStore(t)

	// Add a job
	job := mock.Job()
	state.UpsertJob(900, job)

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
	state.UpsertAllocs(910, []*structs.Allocation{alloc})

	// Update the alloc from client
	alloc1 := alloc.Copy()
	alloc1.ClientStatus = structs.AllocClientStatusPending
	alloc1.DesiredStatus = ""
	state.UpdateAllocsFromClient(920, []*structs.Allocation{alloc})

	alloc3 := alloc.Copy()
	alloc3.ClientStatus = structs.AllocClientStatusRunning
	alloc3.DesiredStatus = ""
	state.UpdateAllocsFromClient(930, []*structs.Allocation{alloc3})

	// Upsert the alloc
	alloc4 := alloc.Copy()
	alloc4.ClientStatus = structs.AllocClientStatusPending
	alloc4.DesiredStatus = structs.AllocDesiredStatusRun
	state.UpsertAllocs(950, []*structs.Allocation{alloc4})

	// Again upsert the alloc
	alloc5 := alloc.Copy()
	alloc5.ClientStatus = structs.AllocClientStatusPending
	alloc5.DesiredStatus = structs.AllocDesiredStatusRun
	state.UpsertAllocs(970, []*structs.Allocation{alloc5})

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
	state.UpdateAllocsFromClient(990, []*structs.Allocation{alloc6})

	// We shouldn't have any summary at this point
	summary, _ = state.JobSummaryByID(ws, job.Namespace, job.ID)
	if summary != nil {
		t.Fatalf("expected nil, actual: %#v", summary)
	}

	// Re-register the same job
	job1 := mock.Job()
	job1.ID = job.ID
	state.UpsertJob(1000, job1)
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
	state.UpdateAllocsFromClient(1020, []*structs.Allocation{alloc7})

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
	state := testStateStore(t)

	// Create an alloc
	alloc := mock.Alloc()

	// Add another task group to the job
	tg2 := alloc.Job.TaskGroups[0].Copy()
	tg2.Name = "db"
	alloc.Job.TaskGroups = append(alloc.Job.TaskGroups, tg2)
	state.UpsertJob(100, alloc.Job)

	// Create one more alloc for the db task group
	alloc2 := mock.Alloc()
	alloc2.TaskGroup = "db"
	alloc2.JobID = alloc.JobID
	alloc2.Job = alloc.Job

	// Upserts the alloc
	state.UpsertAllocs(110, []*structs.Allocation{alloc, alloc2})

	// Change the state of the first alloc to running
	alloc3 := alloc.Copy()
	alloc3.ClientStatus = structs.AllocClientStatusRunning
	state.UpdateAllocsFromClient(120, []*structs.Allocation{alloc3})

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

	state.UpsertAllocs(130, []*structs.Allocation{alloc4, alloc6, alloc8, alloc10})

	state.UpdateAllocsFromClient(150, []*structs.Allocation{alloc5, alloc7, alloc9, alloc11})

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
	state.UpsertNode(80, node)

	// Make a parameterized job
	job1 := mock.BatchJob()
	job1.ID = "test"
	job1.ParameterizedJob = &structs.ParameterizedJobConfig{
		Payload: "random",
	}
	job1.TaskGroups[0].Count = 1
	state.UpsertJob(100, job1)

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

	require.Nil(state.UpsertJob(110, childJob))
	require.Nil(state.UpsertAllocs(111, []*structs.Allocation{alloc, alloc2}))

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
	state := testStateStore(t)

	alloc := mock.Alloc()
	state.UpsertJob(100, alloc.Job)
	state.UpsertAllocs(200, []*structs.Allocation{alloc})

	// Delete the job
	state.DeleteJob(300, alloc.Namespace, alloc.Job.ID)

	// Update the alloc
	alloc1 := alloc.Copy()
	alloc1.ClientStatus = structs.AllocClientStatusRunning

	// Updating allocation should not throw any error
	if err := state.UpdateAllocsFromClient(400, []*structs.Allocation{alloc1}); err != nil {
		t.Fatalf("expect err: %v", err)
	}

	// Re-Register the job
	state.UpsertJob(500, alloc.Job)

	// Update the alloc again
	alloc2 := alloc.Copy()
	alloc2.ClientStatus = structs.AllocClientStatusComplete
	if err := state.UpdateAllocsFromClient(400, []*structs.Allocation{alloc1}); err != nil {
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
	state := testStateStore(t)
	alloc := mock.Alloc()

	state.UpsertJobSummary(999, mock.JobSummary(alloc.JobID))
	err := state.UpsertAllocs(1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	alloc2 := new(structs.Allocation)
	*alloc2 = *alloc
	alloc2.DesiredStatus = structs.AllocDesiredStatusEvict
	err = state.UpsertAllocs(1001, []*structs.Allocation{alloc2})
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

	err := state.UpsertAllocs(1000, allocs)
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

	err := state.UpsertAllocs(1000, allocs)
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

	err := state.UpsertAllocs(1000, allocs)
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
	state := testStateStore(t)
	var allocs []*structs.Allocation
	var allocs1 []*structs.Allocation

	job := mock.Job()
	job.ID = "foo"
	state.UpsertJob(100, job)
	for i := 0; i < 3; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		allocs = append(allocs, alloc)
	}
	if err := state.UpsertAllocs(200, allocs); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := state.DeleteJob(250, job.Namespace, job.ID); err != nil {
		t.Fatalf("err: %v", err)
	}

	job1 := mock.Job()
	job1.ID = "foo"
	job1.CreateIndex = 50
	state.UpsertJob(300, job1)
	for i := 0; i < 4; i++ {
		alloc := mock.Alloc()
		alloc.Job = job1
		alloc.JobID = job1.ID
		allocs1 = append(allocs1, alloc)
	}

	if err := state.UpsertAllocs(1000, allocs1); err != nil {
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

	err := state.UpsertAllocs(1000, allocs)
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
	state := testStateStore(t)
	var allocs []*structs.Allocation

	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		allocs = append(allocs, alloc)
	}
	for i, alloc := range allocs {
		state.UpsertJobSummary(uint64(900+i), mock.JobSummary(alloc.JobID))
	}

	err := state.UpsertAllocs(1000, allocs)
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

	err := state.UpsertAllocs(1000, allocs)
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
	err = state.UpsertAllocs(1001, []*structs.Allocation{alloc})
	require.Nil(err)
	alloc0, err := state.AllocByID(nil, allocs[0].ID)
	require.Nil(err)
	require.Equal(alloc0.ModifyIndex, uint64(1001))
}

func TestStateStore_RestoreAlloc(t *testing.T) {
	state := testStateStore(t)
	alloc := mock.Alloc()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.AllocRestore(alloc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, alloc) {
		t.Fatalf("Bad: %#v %#v", out, alloc)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_SetJobStatus_ForceStatus(t *testing.T) {
	state := testStateStore(t)
	txn := state.db.Txn(true)

	// Create and insert a mock job.
	job := mock.Job()
	job.Status = ""
	job.ModifyIndex = 0
	if err := txn.Insert("jobs", job); err != nil {
		t.Fatalf("job insert failed: %v", err)
	}

	exp := "foobar"
	index := uint64(1000)
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
	state := testStateStore(t)
	txn := state.db.Txn(true)

	// Create and insert a mock job that should be pending.
	job := mock.Job()
	job.Status = structs.JobStatusPending
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

	if updated.ModifyIndex == index {
		t.Fatalf("setJobStatus() should have been a no-op")
	}
}

func TestStateStore_SetJobStatus(t *testing.T) {
	state := testStateStore(t)
	txn := state.db.Txn(true)

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
	job := mock.Job()
	state := testStateStore(t)
	txn := state.db.Txn(false)
	status, err := state.getJobStatus(txn, job, false)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if status != structs.JobStatusPending {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, structs.JobStatusPending)
	}
}

func TestStateStore_GetJobStatus_NoEvalsOrAllocs_Periodic(t *testing.T) {
	job := mock.PeriodicJob()
	state := testStateStore(t)
	txn := state.db.Txn(false)
	status, err := state.getJobStatus(txn, job, false)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if status != structs.JobStatusRunning {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, structs.JobStatusRunning)
	}
}

func TestStateStore_GetJobStatus_NoEvalsOrAllocs_EvalDelete(t *testing.T) {
	job := mock.Job()
	state := testStateStore(t)
	txn := state.db.Txn(false)
	status, err := state.getJobStatus(txn, job, true)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if status != structs.JobStatusDead {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, structs.JobStatusDead)
	}
}

func TestStateStore_GetJobStatus_DeadEvalsAndAllocs(t *testing.T) {
	state := testStateStore(t)
	job := mock.Job()

	// Create a mock alloc that is dead.
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusStop
	state.UpsertJobSummary(999, mock.JobSummary(alloc.JobID))
	if err := state.UpsertAllocs(1000, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a mock eval that is complete
	eval := mock.Eval()
	eval.JobID = job.ID
	eval.Status = structs.EvalStatusComplete
	if err := state.UpsertEvals(1001, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("err: %v", err)
	}

	txn := state.db.Txn(false)
	status, err := state.getJobStatus(txn, job, false)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if status != structs.JobStatusDead {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, structs.JobStatusDead)
	}
}

func TestStateStore_GetJobStatus_RunningAlloc(t *testing.T) {
	state := testStateStore(t)
	job := mock.Job()

	// Create a mock alloc that is running.
	alloc := mock.Alloc()
	alloc.JobID = job.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusRun
	state.UpsertJobSummary(999, mock.JobSummary(alloc.JobID))
	if err := state.UpsertAllocs(1000, []*structs.Allocation{alloc}); err != nil {
		t.Fatalf("err: %v", err)
	}

	txn := state.db.Txn(false)
	status, err := state.getJobStatus(txn, job, true)
	if err != nil {
		t.Fatalf("getJobStatus() failed: %v", err)
	}

	if status != structs.JobStatusRunning {
		t.Fatalf("getJobStatus() returned %v; expected %v", status, structs.JobStatusRunning)
	}
}

func TestStateStore_GetJobStatus_PeriodicJob(t *testing.T) {
	state := testStateStore(t)
	job := mock.PeriodicJob()

	txn := state.db.Txn(false)
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
	state := testStateStore(t)
	job := mock.Job()
	job.ParameterizedJob = &structs.ParameterizedJobConfig{}

	txn := state.db.Txn(false)
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
	state := testStateStore(t)
	job := mock.Job()

	// Create a mock eval that is pending.
	eval := mock.Eval()
	eval.JobID = job.ID
	eval.Status = structs.EvalStatusPending
	if err := state.UpsertEvals(1000, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("err: %v", err)
	}

	txn := state.db.Txn(false)
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
	state := testStateStore(t)
	job := mock.SystemJob()

	// Create a mock eval that is pending.
	eval := mock.Eval()
	eval.JobID = job.ID
	eval.Type = job.Type
	eval.Status = structs.EvalStatusComplete
	if err := state.UpsertEvals(1000, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("err: %v", err)
	}

	txn := state.db.Txn(false)
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
	state := testStateStore(t)
	alloc := mock.Alloc()
	job := alloc.Job
	job.TaskGroups[0].Count = 3

	// Create watchsets so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.JobSummaryByID(ws, job.Namespace, job.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	if err := state.UpsertJob(1000, job); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := state.UpsertAllocs(1001, []*structs.Allocation{alloc}); err != nil {
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

	if err := state.UpsertAllocs(1002, []*structs.Allocation{alloc2, alloc3}); err != nil {
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

	if err := state.UpdateAllocsFromClient(1004, []*structs.Allocation{alloc4, alloc5}); err != nil {
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

	err := state.UpsertJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := state.UpsertAllocs(1001, []*structs.Allocation{alloc, alloc2, alloc3}); err != nil {
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

	if err := state.UpdateAllocsFromClient(1002, []*structs.Allocation{alloc4, alloc5, alloc6}); err != nil {
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

	if err := state.UpsertAllocs(1003, []*structs.Allocation{alloc7}); err != nil {
		t.Fatalf("err: %v", err)
	}
	summary, _ = state.JobSummaryByID(ws, job.Namespace, job.ID)
	if summary.Summary["web"].Starting != 1 || summary.Summary["web"].Running != 1 || summary.Summary["web"].Failed != 1 || summary.Summary["web"].Complete != 1 {
		t.Fatalf("bad job summary: %v", summary)
	}
}

// Test that nonexistent deployment can't be updated
func TestStateStore_UpsertDeploymentStatusUpdate_Nonexistent(t *testing.T) {
	state := testStateStore(t)

	// Update the nonexistent deployment
	req := &structs.DeploymentStatusUpdateRequest{
		DeploymentUpdate: &structs.DeploymentStatusUpdate{
			DeploymentID: uuid.Generate(),
			Status:       structs.DeploymentStatusRunning,
		},
	}
	err := state.UpdateDeploymentStatus(2, req)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected error updating the status because the deployment doesn't exist")
	}
}

// Test that terminal deployment can't be updated
func TestStateStore_UpsertDeploymentStatusUpdate_Terminal(t *testing.T) {
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
	err := state.UpdateDeploymentStatus(2, req)
	if err == nil || !strings.Contains(err.Error(), "has terminal status") {
		t.Fatalf("expected error updating the status because the deployment is terminal")
	}
}

// Test that a non terminal deployment is updated and that a job and eval are
// created.
func TestStateStore_UpsertDeploymentStatusUpdate_NonTerminal(t *testing.T) {
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
	err := state.UpdateDeploymentStatus(2, req)
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
	state := testStateStore(t)

	// Insert a job
	job := mock.Job()
	if err := state.UpsertJob(1, job); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Insert a deployment
	d := structs.NewDeployment(job)
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
	err := state.UpdateDeploymentStatus(3, req)
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
	state := testStateStore(t)

	// Insert a job twice to get two versions
	job := mock.Job()
	if err := state.UpsertJob(1, job); err != nil {
		t.Fatalf("bad: %v", err)
	}

	if err := state.UpsertJob(2, job); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Update the stability to true
	err := state.UpdateJobStability(3, job.Namespace, job.ID, 0, true)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check that the job was updated properly
	ws := memdb.NewWatchSet()
	jout, _ := state.JobByIDAndVersion(ws, job.Namespace, job.ID, 0)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if jout == nil {
		t.Fatalf("bad: %#v", jout)
	}
	if !jout.Stable {
		t.Fatalf("job not marked stable %#v", jout)
	}

	// Update the stability to false
	err = state.UpdateJobStability(3, job.Namespace, job.ID, 0, false)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Check that the job was updated properly
	jout, _ = state.JobByIDAndVersion(ws, job.Namespace, job.ID, 0)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}
	if jout == nil {
		t.Fatalf("bad: %#v", jout)
	}
	if jout.Stable {
		t.Fatalf("job marked stable %#v", jout)
	}
}

// Test that nonexistent deployment can't be promoted
func TestStateStore_UpsertDeploymentPromotion_Nonexistent(t *testing.T) {
	state := testStateStore(t)

	// Promote the nonexistent deployment
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: uuid.Generate(),
			All:          true,
		},
	}
	err := state.UpdateDeploymentPromotion(2, req)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected error promoting because the deployment doesn't exist")
	}
}

// Test that terminal deployment can't be updated
func TestStateStore_UpsertDeploymentPromotion_Terminal(t *testing.T) {
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
	err := state.UpdateDeploymentPromotion(2, req)
	if err == nil || !strings.Contains(err.Error(), "has terminal status") {
		t.Fatalf("expected error updating the status because the deployment is terminal: %v", err)
	}
}

// Test promoting unhealthy canaries in a deployment.
func TestStateStore_UpsertDeploymentPromotion_Unhealthy(t *testing.T) {
	state := testStateStore(t)
	require := require.New(t)

	// Create a job
	j := mock.Job()
	require.Nil(state.UpsertJob(1, j))

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

	require.Nil(state.UpsertAllocs(3, []*structs.Allocation{c1, c2, c3}))

	// Promote the canaries
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
	}
	err := state.UpdateDeploymentPromotion(4, req)
	require.NotNil(err)
	require.Contains(err.Error(), `Task group "web" has 0/2 healthy allocations`)
}

// Test promoting a deployment with no canaries
func TestStateStore_UpsertDeploymentPromotion_NoCanaries(t *testing.T) {
	state := testStateStore(t)
	require := require.New(t)

	// Create a job
	j := mock.Job()
	require.Nil(state.UpsertJob(1, j))

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
	err := state.UpdateDeploymentPromotion(4, req)
	require.NotNil(err)
	require.Contains(err.Error(), `Task group "web" has 0/2 healthy allocations`)
}

// Test promoting all canaries in a deployment.
func TestStateStore_UpsertDeploymentPromotion_All(t *testing.T) {
	state := testStateStore(t)

	// Create a job with two task groups
	j := mock.Job()
	tg1 := j.TaskGroups[0]
	tg2 := tg1.Copy()
	tg2.Name = "foo"
	j.TaskGroups = append(j.TaskGroups, tg2)
	if err := state.UpsertJob(1, j); err != nil {
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

	if err := state.UpsertAllocs(3, []*structs.Allocation{c1, c2}); err != nil {
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
	err := state.UpdateDeploymentPromotion(4, req)
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
	state := testStateStore(t)
	require := require.New(t)

	// Create a job with two task groups
	j := mock.Job()
	tg1 := j.TaskGroups[0]
	tg2 := tg1.Copy()
	tg2.Name = "foo"
	j.TaskGroups = append(j.TaskGroups, tg2)
	require.Nil(state.UpsertJob(1, j))

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

	require.Nil(state.UpsertAllocs(3, []*structs.Allocation{c1, c2, c3}))

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
	require.Nil(state.UpdateDeploymentPromotion(4, req))

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
	state := testStateStore(t)

	// Set health against the nonexistent deployment
	req := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:         uuid.Generate(),
			HealthyAllocationIDs: []string{uuid.Generate()},
		},
	}
	err := state.UpdateDeploymentAllocHealth(2, req)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected error because the deployment doesn't exist: %v", err)
	}
}

// Test that allocation health can't be set against a terminal deployment
func TestStateStore_UpsertDeploymentAllocHealth_Terminal(t *testing.T) {
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
	err := state.UpdateDeploymentAllocHealth(2, req)
	if err == nil || !strings.Contains(err.Error(), "has terminal status") {
		t.Fatalf("expected error because the deployment is terminal: %v", err)
	}
}

// Test that allocation health can't be set against a nonexistent alloc
func TestStateStore_UpsertDeploymentAllocHealth_BadAlloc_Nonexistent(t *testing.T) {
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
	err := state.UpdateDeploymentAllocHealth(2, req)
	if err == nil || !strings.Contains(err.Error(), "unknown alloc") {
		t.Fatalf("expected error because the alloc doesn't exist: %v", err)
	}
}

// Test that allocation health can't be set for an alloc with mismatched
// deployment ids
func TestStateStore_UpsertDeploymentAllocHealth_BadAlloc_MismatchDeployment(t *testing.T) {
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
	if err := state.UpsertAllocs(3, []*structs.Allocation{a}); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Set health against the terminal deployment
	req := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:         d2.ID,
			HealthyAllocationIDs: []string{a.ID},
		},
	}
	err := state.UpdateDeploymentAllocHealth(4, req)
	if err == nil || !strings.Contains(err.Error(), "not part of deployment") {
		t.Fatalf("expected error because the alloc isn't part of the deployment: %v", err)
	}
}

// Test that allocation health is properly set
func TestStateStore_UpsertDeploymentAllocHealth(t *testing.T) {
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
	if err := state.UpsertAllocs(2, []*structs.Allocation{a1, a2}); err != nil {
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
	err := state.UpdateDeploymentAllocHealth(3, req)
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

func TestStateStore_RestoreVaultAccessor(t *testing.T) {
	state := testStateStore(t)
	a := mock.VaultAccessor()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.VaultAccessorRestore(a)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.VaultAccessor(ws, a.Accessor)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, a) {
		t.Fatalf("Bad: %#v %#v", out, a)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpsertACLPolicy(t *testing.T) {
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

	if err := state.UpsertACLPolicies(1000,
		[]*structs.ACLPolicy{policy, policy2}); err != nil {
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
	state := testStateStore(t)
	policy := mock.ACLPolicy()
	policy2 := mock.ACLPolicy()

	// Create the policy
	if err := state.UpsertACLPolicies(1000,
		[]*structs.ACLPolicy{policy, policy2}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watcher
	ws := memdb.NewWatchSet()
	if _, err := state.ACLPolicyByName(ws, policy.Name); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Delete the policy
	if err := state.DeleteACLPolicies(1001,
		[]string{policy.Name, policy2.Name}); err != nil {
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
		if err := state.UpsertACLPolicies(baseIndex, []*structs.ACLPolicy{p}); err != nil {
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
	state := testStateStore(t)
	tk1 := mock.ACLToken()
	tk2 := mock.ACLToken()

	ok, resetIdx, err := state.CanBootstrapACLToken()
	assert.Nil(t, err)
	assert.Equal(t, true, ok)
	assert.EqualValues(t, 0, resetIdx)

	if err := state.BootstrapACLTokens(1000, 0, tk1); err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.ACLTokenByAccessorID(nil, tk1.AccessorID)
	assert.Equal(t, nil, err)
	assert.Equal(t, tk1, out)

	ok, resetIdx, err = state.CanBootstrapACLToken()
	assert.Nil(t, err)
	assert.Equal(t, false, ok)
	assert.EqualValues(t, 1000, resetIdx)

	if err := state.BootstrapACLTokens(1001, 0, tk2); err == nil {
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
	if err := state.BootstrapACLTokens(1001, 1000, tk2); err != nil {
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

	if err := state.UpsertACLTokens(1000,
		[]*structs.ACLToken{tk1, tk2}); err != nil {
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
	state := testStateStore(t)
	tk1 := mock.ACLToken()
	tk2 := mock.ACLToken()

	// Create the tokens
	if err := state.UpsertACLTokens(1000,
		[]*structs.ACLToken{tk1, tk2}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watcher
	ws := memdb.NewWatchSet()
	if _, err := state.ACLTokenByAccessorID(ws, tk1.AccessorID); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Delete the token
	if err := state.DeleteACLTokens(1001,
		[]string{tk1.AccessorID, tk2.AccessorID}); err != nil {
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
		if err := state.UpsertACLTokens(baseIndex, []*structs.ACLToken{tk}); err != nil {
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

func TestStateStore_RestoreACLPolicy(t *testing.T) {
	state := testStateStore(t)
	policy := mock.ACLPolicy()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.ACLPolicyRestore(policy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.ACLPolicyByName(ws, policy.Name)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, policy, out)
}

func TestStateStore_ACLTokensByGlobal(t *testing.T) {
	state := testStateStore(t)
	tk1 := mock.ACLToken()
	tk2 := mock.ACLToken()
	tk3 := mock.ACLToken()
	tk4 := mock.ACLToken()
	tk3.Global = true

	if err := state.UpsertACLTokens(1000,
		[]*structs.ACLToken{tk1, tk2, tk3, tk4}); err != nil {
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

func TestStateStore_RestoreACLToken(t *testing.T) {
	state := testStateStore(t)
	token := mock.ACLToken()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.ACLTokenRestore(token)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.ACLTokenByAccessorID(ws, token.AccessorID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, token, out)
}

func TestStateStore_SchedulerConfig(t *testing.T) {
	state := testStateStore(t)
	schedConfig := &structs.SchedulerConfiguration{
		PreemptionConfig: structs.PreemptionConfig{
			SystemSchedulerEnabled: false,
		},
		CreateIndex: 100,
		ModifyIndex: 200,
	}

	require := require.New(t)
	restore, err := state.Restore()

	require.Nil(err)

	err = restore.SchedulerConfigRestore(schedConfig)
	require.Nil(err)

	restore.Commit()

	modIndex, out, err := state.SchedulerConfig()
	require.Nil(err)
	require.Equal(schedConfig.ModifyIndex, modIndex)

	require.Equal(schedConfig, out)
}

func TestStateStore_Abandon(t *testing.T) {
	s := testStateStore(t)
	abandonCh := s.AbandonCh()
	s.Abandon()
	select {
	case <-abandonCh:
	default:
		t.Fatalf("bad")
	}
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
