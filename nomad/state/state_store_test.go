package state

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func testStateStore(t *testing.T) *StateStore {
	state, err := NewStateStore(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if state == nil {
		t.Fatalf("missing state")
	}
	return state
}

func TestStateStore_Blocking_Error(t *testing.T) {
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
	noopFn := func(memdb.WatchSet, *StateStore) (interface{}, uint64, error) {
		return nil, 5, nil
	}

	state := testStateStore(t)
	timeout := time.Now().Add(50 * time.Millisecond)
	deadlineCtx, cancel := context.WithDeadline(context.Background(), timeout)
	defer cancel()

	_, idx, err := state.BlockingQuery(noopFn, 10, deadlineCtx)
	assert.EqualError(t, err, context.DeadlineExceeded.Error())
	assert.EqualValues(t, 5, idx)
	assert.WithinDuration(t, timeout, time.Now(), 25*time.Millisecond)
}

func TestStateStore_Blocking_MinQuery(t *testing.T) {
	job := mock.Job()
	count := 0
	queryFn := func(ws memdb.WatchSet, s *StateStore) (interface{}, uint64, error) {
		_, err := s.JobByID(ws, job.Namespace, job.ID)
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
	timeout := time.Now().Add(10 * time.Millisecond)
	deadlineCtx, cancel := context.WithDeadline(context.Background(), timeout)
	defer cancel()

	time.AfterFunc(5*time.Millisecond, func() {
		state.UpsertJob(11, job)
	})

	resp, idx, err := state.BlockingQuery(queryFn, 10, deadlineCtx)
	assert.Nil(t, err)
	assert.Equal(t, 2, count)
	assert.EqualValues(t, 11, idx)
	assert.True(t, resp.(bool))
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

	// Create a plan result
	res := structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Alloc: []*structs.Allocation{alloc},
			Job:   job,
		},
	}

	err := state.UpsertPlanResults(1000, &res)
	if err != nil {
		t.Fatalf("err: %v", err)
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

	if watchFired(ws) {
		t.Fatalf("bad")
	}
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

	// Create a plan result
	res := structs.ApplyPlanResultsRequest{
		AllocUpdateRequest: structs.AllocUpdateRequest{
			Alloc: []*structs.Allocation{alloc, alloc2},
			Job:   job,
		},
		Deployment: d,
	}

	err := state.UpsertPlanResults(1000, &res)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(alloc, out) {
		t.Fatalf("bad: %#v %#v", alloc, out)
	}

	dout, err := state.DeploymentByID(ws, d.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if dout == nil {
		t.Fatalf("bad: nil deployment")
	}

	tg, ok := dout.TaskGroups[alloc.TaskGroup]
	if !ok {
		t.Fatalf("bad: nil deployment state")
	}
	if tg == nil || tg.PlacedAllocs != 2 {
		t.Fatalf("bad: %v", dout)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}

	// Update the allocs to be part of a new deployment
	d2 := d.Copy()
	d2.ID = structs.GenerateUUID()

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
	}

	err = state.UpsertPlanResults(1001, &res)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	dout, err = state.DeploymentByID(ws, d2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if dout == nil {
		t.Fatalf("bad: nil deployment")
	}

	tg, ok = dout.TaskGroups[alloc.TaskGroup]
	if !ok {
		t.Fatalf("bad: nil deployment state")
	}
	if tg == nil || tg.PlacedAllocs != 2 {
		t.Fatalf("bad: %v", dout)
	}
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
	}

	err := state.UpsertPlanResults(1000, &res)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()

	// Check the deployments are correctly updated.
	dout, err := state.DeploymentByID(ws, dnew.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if dout == nil {
		t.Fatalf("bad: nil deployment")
	}

	tg, ok := dout.TaskGroups[alloc.TaskGroup]
	if !ok {
		t.Fatalf("bad: nil deployment state")
	}
	if tg == nil || tg.PlacedAllocs != 1 {
		t.Fatalf("bad: %v", dout)
	}

	doutstandingout, err := state.DeploymentByID(ws, doutstanding.ID)
	if err != nil || doutstandingout == nil {
		t.Fatalf("bad: %v %v", err, doutstandingout)
	}
	if doutstandingout.Status != update.Status || doutstandingout.StatusDescription != update.StatusDescription || doutstandingout.ModifyIndex != 1000 {
		t.Fatalf("bad: %v", doutstandingout)
	}

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

func TestStateStore_Deployments_Namespace(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	ns1 := "namespaced"
	deploy1 := mock.Deployment()
	deploy2 := mock.Deployment()
	deploy1.Namespace = ns1
	deploy2.Namespace = ns1

	ns2 := "new-namespace"
	deploy3 := mock.Deployment()
	deploy4 := mock.Deployment()
	deploy3.Namespace = ns2
	deploy4.Namespace = ns2

	// Create watchsets so we can test that update fires the watch
	watches := []memdb.WatchSet{memdb.NewWatchSet(), memdb.NewWatchSet()}
	_, err := state.DeploymentsByNamespace(watches[0], ns1)
	assert.Nil(err)
	_, err = state.DeploymentsByNamespace(watches[1], ns2)
	assert.Nil(err)

	assert.Nil(state.UpsertDeployment(1001, deploy1))
	assert.Nil(state.UpsertDeployment(1002, deploy2))
	assert.Nil(state.UpsertDeployment(1003, deploy3))
	assert.Nil(state.UpsertDeployment(1004, deploy4))
	assert.True(watchFired(watches[0]))
	assert.True(watchFired(watches[1]))

	ws := memdb.NewWatchSet()
	iter1, err := state.DeploymentsByNamespace(ws, ns1)
	assert.Nil(err)
	iter2, err := state.DeploymentsByNamespace(ws, ns2)
	assert.Nil(err)

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

	assert.Len(out1, 2)
	assert.Len(out2, 2)

	for _, deploy := range out1 {
		assert.Equal(ns1, deploy.Namespace)
	}
	for _, deploy := range out2 {
		assert.Equal(ns2, deploy.Namespace)
	}

	index, err := state.Index("deployment")
	assert.Nil(err)
	assert.EqualValues(1004, index)
	assert.False(watchFired(ws))
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

func TestStateStore_DeploymentsByIDPrefix_Namespaces(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	deploy1 := mock.Deployment()
	deploy1.ID = "aabbbbbb-7bfb-395d-eb95-0685af2176b2"
	deploy2 := mock.Deployment()
	deploy2.ID = "aabbcbbb-7bfb-395d-eb95-0685af2176b2"
	sharedPrefix := "aabb"

	ns1, ns2 := "namespace1", "namespace2"
	deploy1.Namespace = ns1
	deploy2.Namespace = ns2

	assert.Nil(state.UpsertDeployment(1000, deploy1))
	assert.Nil(state.UpsertDeployment(1001, deploy2))

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
	iter1, err := state.DeploymentsByIDPrefix(ws, ns1, sharedPrefix)
	assert.Nil(err)
	iter2, err := state.DeploymentsByIDPrefix(ws, ns2, sharedPrefix)
	assert.Nil(err)

	deploysNs1 := gatherDeploys(iter1)
	deploysNs2 := gatherDeploys(iter2)
	assert.Len(deploysNs1, 1)
	assert.Len(deploysNs2, 1)

	iter1, err = state.DeploymentsByIDPrefix(ws, ns1, deploy1.ID[:8])
	assert.Nil(err)

	deploysNs1 = gatherDeploys(iter1)
	assert.Len(deploysNs1, 1)
	assert.False(watchFired(ws))
}

func TestStateStore_UpsertNode_Node(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()

	// Create a watchset so we can test that upsert fires the watch
	ws := memdb.NewWatchSet()
	_, err := state.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("bad: %v", err)
	}

	err = state.UpsertNode(1000, node)
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

	if !reflect.DeepEqual(node, out) {
		t.Fatalf("bad: %#v %#v", node, out)
	}

	index, err := state.Index("nodes")
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
	state := testStateStore(t)
	node := mock.Node()

	err := state.UpsertNode(800, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watchset so we can test that update node status fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.NodeByID(ws, node.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	err = state.UpdateNodeStatus(801, node.ID, structs.NodeStatusReady)
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

	if out.Status != structs.NodeStatusReady {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 801 {
		t.Fatalf("bad: %#v", out)
	}

	index, err := state.Index("nodes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 801 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_UpdateNodeDrain_Node(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()

	err := state.UpsertNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watchset so we can test that update node drain fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.NodeByID(ws, node.ID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	err = state.UpdateNodeDrain(1001, node.ID, true)
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

	if !out.Drain {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1001 {
		t.Fatalf("bad: %#v", out)
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

// This test ensures that UpsertJob creates the EphemeralDisk is a job doesn't have
// one and clear out the task's disk resource asks
// COMPAT 0.4.1 -> 0.5
func TestStateStore_UpsertJob_NoEphemeralDisk(t *testing.T) {
	state := testStateStore(t)
	job := mock.Job()

	// Set the EphemeralDisk to nil and set the tasks's DiskMB to 150
	job.TaskGroups[0].EphemeralDisk = nil
	job.TaskGroups[0].Tasks[0].Resources.DiskMB = 150

	err := state.UpsertJob(1000, job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Expect the state store to create the EphemeralDisk and clear out Tasks's
	// DiskMB
	expected := job.Copy()
	expected.TaskGroups[0].EphemeralDisk = &structs.EphemeralDisk{
		SizeMB: 150,
	}
	expected.TaskGroups[0].Tasks[0].Resources.DiskMB = 0

	if !reflect.DeepEqual(expected, out) {
		t.Fatalf("bad: %#v %#v", expected, out)
	}
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
	job.Priority = 0

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
	for i := 1; i < 20; i++ {
		finalJob = mock.Job()
		finalJob.ID = job.ID
		finalJob.Priority = i
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
	if out.ModifyIndex != 1019 {
		t.Fatalf("bad: %#v", out)
	}
	if out.Version != 19 {
		t.Fatalf("bad: %#v", out)
	}

	index, err := state.Index("job_version")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1019 {
		t.Fatalf("bad: %d", index)
	}

	// Check the job versions
	allVersions, err := state.JobVersionsByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(allVersions) != structs.JobTrackedVersions {
		t.Fatalf("got %d; want 1", len(allVersions))
	}

	if a := allVersions[0]; a.ID != job.ID || a.Version != 19 || a.Priority != 19 {
		t.Fatalf("bad: %+v", a)
	}
	if a := allVersions[1]; a.ID != job.ID || a.Version != 18 || a.Priority != 18 {
		t.Fatalf("bad: %+v", a)
	}

	// Ensure we didn't delete the stable job
	if a := allVersions[structs.JobTrackedVersions-1]; a.ID != job.ID ||
		a.Version != 0 || a.Priority != 0 || !a.Stable {
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

func TestStateStore_JobsByNamespace(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	ns1 := "new"
	job1 := mock.Job()
	job2 := mock.Job()
	job1.Namespace = ns1
	job2.Namespace = ns1

	ns2 := "new-namespace"
	job3 := mock.Job()
	job4 := mock.Job()
	job3.Namespace = ns2
	job4.Namespace = ns2

	// Create watchsets so we can test that update fires the watch
	watches := []memdb.WatchSet{memdb.NewWatchSet(), memdb.NewWatchSet()}
	_, err := state.JobsByNamespace(watches[0], ns1)
	assert.Nil(err)
	_, err = state.JobsByNamespace(watches[1], ns2)
	assert.Nil(err)

	assert.Nil(state.UpsertJob(1001, job1))
	assert.Nil(state.UpsertJob(1002, job2))
	assert.Nil(state.UpsertJob(1003, job3))
	assert.Nil(state.UpsertJob(1004, job4))
	assert.True(watchFired(watches[0]))
	assert.True(watchFired(watches[1]))

	ws := memdb.NewWatchSet()
	iter1, err := state.JobsByNamespace(ws, ns1)
	assert.Nil(err)
	iter2, err := state.JobsByNamespace(ws, ns2)
	assert.Nil(err)

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

	assert.Len(out1, 2)
	assert.Len(out2, 2)

	for _, job := range out1 {
		assert.Equal(ns1, job.Namespace)
	}
	for _, job := range out2 {
		assert.Equal(ns2, job.Namespace)
	}

	index, err := state.Index("jobs")
	assert.Nil(err)
	assert.EqualValues(1004, index)
	assert.False(watchFired(ws))
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

func TestStateStore_JobsByIDPrefix_Namespaces(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	job1 := mock.Job()
	job2 := mock.Job()

	jobID := "redis"
	ns1, ns2 := "namespace1", "namespace2"
	job1.ID = jobID
	job2.ID = jobID
	job1.Namespace = ns1
	job2.Namespace = ns2

	assert.Nil(state.UpsertJob(1000, job1))
	assert.Nil(state.UpsertJob(1001, job2))

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
	iter1, err := state.JobsByIDPrefix(ws, ns1, jobID)
	assert.Nil(err)
	iter2, err := state.JobsByIDPrefix(ws, ns2, jobID)
	assert.Nil(err)

	jobsNs1 := gatherJobs(iter1)
	assert.Len(jobsNs1, 1)

	jobsNs2 := gatherJobs(iter2)
	assert.Len(jobsNs2, 1)

	// Try prefix
	iter1, err = state.JobsByIDPrefix(ws, ns1, "re")
	assert.Nil(err)
	iter2, err = state.JobsByIDPrefix(ws, ns2, "re")
	assert.Nil(err)

	jobsNs1 = gatherJobs(iter1)
	jobsNs2 = gatherJobs(iter2)
	assert.Len(jobsNs1, 1)
	assert.Len(jobsNs2, 1)

	job3 := mock.Job()
	job3.ID = "riak"
	job3.Namespace = ns1
	assert.Nil(state.UpsertJob(1003, job3))
	assert.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	iter1, err = state.JobsByIDPrefix(ws, ns1, "r")
	assert.Nil(err)
	iter2, err = state.JobsByIDPrefix(ws, ns2, "r")
	assert.Nil(err)

	jobsNs1 = gatherJobs(iter1)
	jobsNs2 = gatherJobs(iter2)
	assert.Len(jobsNs1, 2)
	assert.Len(jobsNs2, 1)

	iter1, err = state.JobsByIDPrefix(ws, ns1, "ri")
	assert.Nil(err)

	jobsNs1 = gatherJobs(iter1)
	assert.Len(jobsNs1, 1)
	assert.False(watchFired(ws))
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

// This test ensures that the state restore creates the EphemeralDisk for a job if
// it doesn't have one
// COMPAT 0.4.1 -> 0.5
func TestStateStore_Jobs_NoEphemeralDisk(t *testing.T) {
	state := testStateStore(t)
	job := mock.Job()

	// Set EphemeralDisk to nil and set the DiskMB to 150
	job.TaskGroups[0].EphemeralDisk = nil
	job.TaskGroups[0].Tasks[0].Resources.DiskMB = 150

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

	// Expect job to have local disk and clear out the task's disk resource ask
	expected := job.Copy()
	expected.TaskGroups[0].EphemeralDisk = &structs.EphemeralDisk{
		SizeMB: 150,
	}
	expected.TaskGroups[0].Tasks[0].Resources.DiskMB = 0
	if !reflect.DeepEqual(out, expected) {
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
			"web": structs.TaskGroupSummary{
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

	expect := []*IndexEntry{
		&IndexEntry{"nodes", 1000},
	}

	if !reflect.DeepEqual(expect, out) {
		t.Fatalf("bad: %#v %#v", expect, out)
	}
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

func TestStateStore_UpsertEvals_Namespace(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	ns1 := "namespaced"
	eval1 := mock.Eval()
	eval2 := mock.Eval()
	eval1.Namespace = ns1
	eval2.Namespace = ns1

	ns2 := "new-namespace"
	eval3 := mock.Eval()
	eval4 := mock.Eval()
	eval3.Namespace = ns2
	eval4.Namespace = ns2

	// Create watchsets so we can test that update fires the watch
	watches := []memdb.WatchSet{memdb.NewWatchSet(), memdb.NewWatchSet()}
	_, err := state.EvalsByNamespace(watches[0], ns1)
	assert.Nil(err)
	_, err = state.EvalsByNamespace(watches[1], ns2)
	assert.Nil(err)

	assert.Nil(state.UpsertEvals(1001, []*structs.Evaluation{eval1, eval2, eval3, eval4}))
	assert.True(watchFired(watches[0]))
	assert.True(watchFired(watches[1]))

	ws := memdb.NewWatchSet()
	iter1, err := state.EvalsByNamespace(ws, ns1)
	assert.Nil(err)
	iter2, err := state.EvalsByNamespace(ws, ns2)
	assert.Nil(err)

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

	assert.Len(out1, 2)
	assert.Len(out2, 2)

	for _, eval := range out1 {
		assert.Equal(ns1, eval.Namespace)
	}
	for _, eval := range out2 {
		assert.Equal(ns2, eval.Namespace)
	}

	index, err := state.Index("evals")
	assert.Nil(err)
	assert.EqualValues(1001, index)
	assert.False(watchFired(ws))
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

func TestStateStore_EvalsByIDPrefix_Namespaces(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	eval1 := mock.Eval()
	eval1.ID = "aabbbbbb-7bfb-395d-eb95-0685af2176b2"
	eval2 := mock.Eval()
	eval2.ID = "aabbcbbb-7bfb-395d-eb95-0685af2176b2"
	sharedPrefix := "aabb"

	ns1, ns2 := "namespace1", "namespace2"
	eval1.Namespace = ns1
	eval2.Namespace = ns2

	assert.Nil(state.UpsertEvals(1000, []*structs.Evaluation{eval1, eval2}))

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
	iter1, err := state.EvalsByIDPrefix(ws, ns1, sharedPrefix)
	assert.Nil(err)
	iter2, err := state.EvalsByIDPrefix(ws, ns2, sharedPrefix)
	assert.Nil(err)

	evalsNs1 := gatherEvals(iter1)
	evalsNs2 := gatherEvals(iter2)
	assert.Len(evalsNs1, 1)
	assert.Len(evalsNs2, 1)

	iter1, err = state.EvalsByIDPrefix(ws, ns1, eval1.ID[:8])
	assert.Nil(err)

	evalsNs1 = gatherEvals(iter1)
	assert.Len(evalsNs1, 1)
	assert.False(watchFired(ws))
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
	ts := map[string]*structs.TaskState{"web": &structs.TaskState{State: structs.TaskStateRunning}}
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
	ts := map[string]*structs.TaskState{"web": &structs.TaskState{State: structs.TaskStatePending}}
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
	ts := map[string]*structs.TaskState{"web": &structs.TaskState{State: structs.TaskStatePending}}
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
			"web": structs.TaskGroupSummary{
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

func TestStateStore_UpsertAlloc_AllocsByNamespace(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	ns1 := "namespaced"
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc1.Namespace = ns1
	alloc1.Job.Namespace = ns1
	alloc2.Namespace = ns1
	alloc2.Job.Namespace = ns1

	ns2 := "new-namespace"
	alloc3 := mock.Alloc()
	alloc4 := mock.Alloc()
	alloc3.Namespace = ns2
	alloc3.Job.Namespace = ns2
	alloc4.Namespace = ns2
	alloc4.Job.Namespace = ns2

	assert.Nil(state.UpsertJob(999, alloc1.Job))
	assert.Nil(state.UpsertJob(1000, alloc3.Job))

	// Create watchsets so we can test that update fires the watch
	watches := []memdb.WatchSet{memdb.NewWatchSet(), memdb.NewWatchSet()}
	_, err := state.AllocsByNamespace(watches[0], ns1)
	assert.Nil(err)
	_, err = state.AllocsByNamespace(watches[1], ns2)
	assert.Nil(err)

	assert.Nil(state.UpsertAllocs(1001, []*structs.Allocation{alloc1, alloc2, alloc3, alloc4}))
	assert.True(watchFired(watches[0]))
	assert.True(watchFired(watches[1]))

	ws := memdb.NewWatchSet()
	iter1, err := state.AllocsByNamespace(ws, ns1)
	assert.Nil(err)
	iter2, err := state.AllocsByNamespace(ws, ns2)
	assert.Nil(err)

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

	assert.Len(out1, 2)
	assert.Len(out2, 2)

	for _, alloc := range out1 {
		assert.Equal(ns1, alloc.Namespace)
	}
	for _, alloc := range out2 {
		assert.Equal(ns2, alloc.Namespace)
	}

	index, err := state.Index("allocs")
	assert.Nil(err)
	assert.EqualValues(1001, index)
	assert.False(watchFired(ws))
}

func TestStateStore_UpsertAlloc_Deployment(t *testing.T) {
	state := testStateStore(t)
	deployment := mock.Deployment()
	alloc := mock.Alloc()
	alloc.DeploymentID = deployment.ID

	if err := state.UpsertJob(999, alloc.Job); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := state.UpsertDeployment(1000, deployment); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watch set so we can test that update fires the watch
	ws := memdb.NewWatchSet()
	if _, err := state.AllocsByDeployment(ws, alloc.DeploymentID); err != nil {
		t.Fatalf("bad: %v", err)
	}

	err := state.UpsertAllocs(1001, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !watchFired(ws) {
		t.Fatalf("watch not fired")
	}

	ws = memdb.NewWatchSet()
	allocs, err := state.AllocsByDeployment(ws, alloc.DeploymentID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(allocs) != 1 {
		t.Fatalf("bad: %#v", allocs)
	}

	if !reflect.DeepEqual(alloc, allocs[0]) {
		t.Fatalf("bad: %#v %#v", alloc, allocs[0])
	}

	index, err := state.Index("allocs")
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

func TestStateStore_UpsertAlloc_NoEphemeralDisk(t *testing.T) {
	state := testStateStore(t)
	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].EphemeralDisk = nil
	alloc.Job.TaskGroups[0].Tasks[0].Resources.DiskMB = 120

	if err := state.UpsertJob(999, alloc.Job); err != nil {
		t.Fatalf("err: %v", err)
	}

	err := state.UpsertAllocs(1000, []*structs.Allocation{alloc})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	expected := alloc.Copy()
	expected.Job.TaskGroups[0].EphemeralDisk = &structs.EphemeralDisk{SizeMB: 120}
	if !reflect.DeepEqual(expected, out) {
		t.Fatalf("bad: %#v %#v", expected, out)
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
			"web": structs.TaskGroupSummary{
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
			"web": structs.TaskGroupSummary{},
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
			"web": structs.TaskGroupSummary{
				Running: 1,
			},
			"db": structs.TaskGroupSummary{
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
			"web": structs.TaskGroupSummary{},
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

func TestStateStore_AllocsByIDPrefix_Namespaces(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	alloc1 := mock.Alloc()
	alloc1.ID = "aabbbbbb-7bfb-395d-eb95-0685af2176b2"
	alloc2 := mock.Alloc()
	alloc2.ID = "aabbcbbb-7bfb-395d-eb95-0685af2176b2"
	sharedPrefix := "aabb"

	ns1, ns2 := "namespace1", "namespace2"
	alloc1.Namespace = ns1
	alloc2.Namespace = ns2

	assert.Nil(state.UpsertAllocs(1000, []*structs.Allocation{alloc1, alloc2}))

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
	iter1, err := state.AllocsByIDPrefix(ws, ns1, sharedPrefix)
	assert.Nil(err)
	iter2, err := state.AllocsByIDPrefix(ws, ns2, sharedPrefix)
	assert.Nil(err)

	allocsNs1 := gatherAllocs(iter1)
	allocsNs2 := gatherAllocs(iter2)
	assert.Len(allocsNs1, 1)
	assert.Len(allocsNs2, 1)

	iter1, err = state.AllocsByIDPrefix(ws, ns1, alloc1.ID[:8])
	assert.Nil(err)

	allocsNs1 = gatherAllocs(iter1)
	assert.Len(allocsNs1, 1)
	assert.False(watchFired(ws))
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

func TestStateStore_RestoreAlloc_NoEphemeralDisk(t *testing.T) {
	state := testStateStore(t)
	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].EphemeralDisk = nil
	alloc.Job.TaskGroups[0].Tasks[0].Resources.DiskMB = 120

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

	expected := alloc.Copy()
	expected.Job.TaskGroups[0].EphemeralDisk = &structs.EphemeralDisk{SizeMB: 120}
	expected.Job.TaskGroups[0].Tasks[0].Resources.DiskMB = 0

	if !reflect.DeepEqual(out, expected) {
		t.Fatalf("Bad: %#v %#v", out, expected)
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

// Test that non-existent deployment can't be updated
func TestStateStore_UpsertDeploymentStatusUpdate_NonExistent(t *testing.T) {
	state := testStateStore(t)

	// Update the non-existent deployment
	req := &structs.DeploymentStatusUpdateRequest{
		DeploymentUpdate: &structs.DeploymentStatusUpdate{
			DeploymentID: structs.GenerateUUID(),
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

// Test that non-existent deployment can't be promoted
func TestStateStore_UpsertDeploymentPromotion_NonExistent(t *testing.T) {
	state := testStateStore(t)

	// Promote the non-existent deployment
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: structs.GenerateUUID(),
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

	// Create a job
	j := mock.Job()
	if err := state.UpsertJob(1, j); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create a deployment
	d := mock.Deployment()
	d.JobID = j.ID
	if err := state.UpsertDeployment(2, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create a set of allocations
	c1 := mock.Alloc()
	c1.JobID = j.ID
	c1.DeploymentID = d.ID
	d.TaskGroups[c1.TaskGroup].PlacedCanaries = append(d.TaskGroups[c1.TaskGroup].PlacedCanaries, c1.ID)
	c2 := mock.Alloc()
	c2.JobID = j.ID
	c2.DeploymentID = d.ID
	d.TaskGroups[c2.TaskGroup].PlacedCanaries = append(d.TaskGroups[c2.TaskGroup].PlacedCanaries, c2.ID)

	if err := state.UpsertAllocs(3, []*structs.Allocation{c1, c2}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Promote the canaries
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
	}
	err := state.UpdateDeploymentPromotion(4, req)
	if err == nil {
		t.Fatalf("bad: %v", err)
	}
	if !strings.Contains(err.Error(), c1.ID) {
		t.Fatalf("expect canary %q to be listed as unhealth: %v", c1.ID, err)
	}
	if !strings.Contains(err.Error(), c2.ID) {
		t.Fatalf("expect canary %q to be listed as unhealth: %v", c2.ID, err)
	}
}

// Test promoting a deployment with no canaries
func TestStateStore_UpsertDeploymentPromotion_NoCanaries(t *testing.T) {
	state := testStateStore(t)

	// Create a job
	j := mock.Job()
	if err := state.UpsertJob(1, j); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Create a deployment
	d := mock.Deployment()
	d.JobID = j.ID
	if err := state.UpsertDeployment(2, d); err != nil {
		t.Fatalf("bad: %v", err)
	}

	// Promote the canaries
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
	}
	err := state.UpdateDeploymentPromotion(4, req)
	if err == nil {
		t.Fatalf("bad: %v", err)
	}
	if !strings.Contains(err.Error(), "no canaries to promote") {
		t.Fatalf("expect error promoting non-existent canaries: %v", err)
	}
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
		"web": &structs.DeploymentState{
			DesiredTotal:    10,
			DesiredCanaries: 1,
		},
		"foo": &structs.DeploymentState{
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
	d.JobID = j.ID
	d.TaskGroups = map[string]*structs.DeploymentState{
		"web": &structs.DeploymentState{
			DesiredTotal:    10,
			DesiredCanaries: 1,
		},
		"foo": &structs.DeploymentState{
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
			Groups:       []string{"web"},
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
	if len(dout.TaskGroups) != 2 {
		t.Fatalf("bad: %#v", dout.TaskGroups)
	}
	stateout, ok := dout.TaskGroups["web"]
	if !ok {
		t.Fatalf("bad: no state for task group web")
	}
	if !stateout.Promoted {
		t.Fatalf("bad: task group web not promoted: %#v", stateout)
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

// Test that allocation health can't be set against a non-existent deployment
func TestStateStore_UpsertDeploymentAllocHealth_NonExistent(t *testing.T) {
	state := testStateStore(t)

	// Set health against the non-existent deployment
	req := &structs.ApplyDeploymentAllocHealthRequest{
		DeploymentAllocHealthRequest: structs.DeploymentAllocHealthRequest{
			DeploymentID:         structs.GenerateUUID(),
			HealthyAllocationIDs: []string{structs.GenerateUUID()},
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
			HealthyAllocationIDs: []string{structs.GenerateUUID()},
		},
	}
	err := state.UpdateDeploymentAllocHealth(2, req)
	if err == nil || !strings.Contains(err.Error(), "has terminal status") {
		t.Fatalf("expected error because the deployment is terminal: %v", err)
	}
}

// Test that allocation health can't be set against a non-existent alloc
func TestStateStore_UpsertDeploymentAllocHealth_BadAlloc_NonExistent(t *testing.T) {
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
			HealthyAllocationIDs: []string{structs.GenerateUUID()},
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
