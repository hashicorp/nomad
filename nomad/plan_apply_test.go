// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// workerPoolSize is the size of the worker pool
	workerPoolSize = 2
)

// planWaitFuture is used to wait for the Raft future to complete
func planWaitFuture(future raft.ApplyFuture) (uint64, error) {
	if err := future.Error(); err != nil {
		return 0, err
	}
	return future.Index(), nil
}

func testRegisterNode(t *testing.T, s *Server, n *structs.Node) {
	// Create the register request
	req := &structs.NodeRegisterRequest{
		Node:         n,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.NodeUpdateResponse
	if err := s.RPC("Node.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}
}

func testRegisterJob(t *testing.T, s *Server, j *structs.Job) {
	// Create the register request
	req := &structs.JobRegisterRequest{
		Job:          j,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	if err := s.RPC("Job.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}
}

// COMPAT 0.11: Tests the older unoptimized code path for applyPlan
func TestPlanApply_applyPlan(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Register node
	node := mock.Node()
	testRegisterNode(t, s1, node)

	// Register a fake deployment
	oldDeployment := mock.Deployment()
	if err := s1.State().UpsertDeployment(900, oldDeployment); err != nil {
		t.Fatalf("UpsertDeployment failed: %v", err)
	}

	// Create a deployment
	dnew := mock.Deployment()

	// Create a deployment update for the old deployment id
	desiredStatus, desiredStatusDescription := "foo", "bar"
	updates := []*structs.DeploymentStatusUpdate{
		{
			DeploymentID:      oldDeployment.ID,
			Status:            desiredStatus,
			StatusDescription: desiredStatusDescription,
		},
	}

	// Register alloc, deployment and deployment update
	alloc := mock.Alloc()
	s1.State().UpsertJobSummary(1000, mock.JobSummary(alloc.JobID))
	// Create an eval
	eval := mock.Eval()
	eval.JobID = alloc.JobID
	if err := s1.State().UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("err: %v", err)
	}

	planRes := &structs.PlanResult{
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc},
		},
		Deployment:        dnew,
		DeploymentUpdates: updates,
	}

	// Snapshot the state
	snap, err := s1.State().Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create the plan with a deployment
	plan := &structs.Plan{
		Job:               alloc.Job,
		Deployment:        dnew,
		DeploymentUpdates: updates,
		EvalID:            eval.ID,
	}

	// Apply the plan
	future, err := s1.applyPlan(plan, planRes, snap)
	assert := assert.New(t)
	assert.Nil(err)

	// Verify our optimistic snapshot is updated
	ws := memdb.NewWatchSet()
	allocOut, err := snap.AllocByID(ws, alloc.ID)
	assert.Nil(err)
	assert.NotNil(allocOut)

	deploymentOut, err := snap.DeploymentByID(ws, plan.Deployment.ID)
	assert.Nil(err)
	assert.NotNil(deploymentOut)

	// Check plan does apply cleanly
	index, err := planWaitFuture(future)
	assert.Nil(err)
	assert.NotEqual(0, index)

	// Lookup the allocation
	fsmState := s1.fsm.State()
	allocOut, err = fsmState.AllocByID(ws, alloc.ID)
	assert.Nil(err)
	assert.NotNil(allocOut)
	assert.True(allocOut.CreateTime > 0)
	assert.True(allocOut.ModifyTime > 0)
	assert.Equal(allocOut.CreateTime, allocOut.ModifyTime)

	// Lookup the new deployment
	dout, err := fsmState.DeploymentByID(ws, plan.Deployment.ID)
	assert.Nil(err)
	assert.NotNil(dout)

	// Lookup the updated deployment
	dout2, err := fsmState.DeploymentByID(ws, oldDeployment.ID)
	assert.Nil(err)
	assert.NotNil(dout2)
	assert.Equal(desiredStatus, dout2.Status)
	assert.Equal(desiredStatusDescription, dout2.StatusDescription)

	// Lookup updated eval
	evalOut, err := fsmState.EvalByID(ws, eval.ID)
	assert.Nil(err)
	assert.NotNil(evalOut)
	assert.Equal(index, evalOut.ModifyIndex)

	// Evict alloc, Register alloc2
	allocEvict := new(structs.Allocation)
	*allocEvict = *alloc
	allocEvict.DesiredStatus = structs.AllocDesiredStatusEvict
	job := allocEvict.Job
	allocEvict.Job = nil
	alloc2 := mock.Alloc()
	s1.State().UpsertJobSummary(1500, mock.JobSummary(alloc2.JobID))
	planRes = &structs.PlanResult{
		NodeUpdate: map[string][]*structs.Allocation{
			node.ID: {allocEvict},
		},
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc2},
		},
	}

	// Snapshot the state
	snap, err = s1.State().Snapshot()
	assert.Nil(err)

	// Apply the plan
	plan = &structs.Plan{
		Job:    job,
		EvalID: eval.ID,
	}
	future, err = s1.applyPlan(plan, planRes, snap)
	assert.Nil(err)

	// Check that our optimistic view is updated
	out, _ := snap.AllocByID(ws, allocEvict.ID)
	if out.DesiredStatus != structs.AllocDesiredStatusEvict && out.DesiredStatus != structs.AllocDesiredStatusStop {
		assert.Equal(structs.AllocDesiredStatusEvict, out.DesiredStatus)
	}

	// Verify plan applies cleanly
	index, err = planWaitFuture(future)
	assert.Nil(err)
	assert.NotEqual(0, index)

	// Lookup the allocation
	allocOut, err = s1.fsm.State().AllocByID(ws, alloc.ID)
	assert.Nil(err)
	if allocOut.DesiredStatus != structs.AllocDesiredStatusEvict && allocOut.DesiredStatus != structs.AllocDesiredStatusStop {
		assert.Equal(structs.AllocDesiredStatusEvict, allocOut.DesiredStatus)
	}

	assert.NotNil(allocOut.Job)
	assert.True(allocOut.ModifyTime > 0)

	// Lookup the allocation
	allocOut, err = s1.fsm.State().AllocByID(ws, alloc2.ID)
	assert.Nil(err)
	assert.NotNil(allocOut)
	assert.NotNil(allocOut.Job)

	// Lookup updated eval
	evalOut, err = fsmState.EvalByID(ws, eval.ID)
	assert.Nil(err)
	assert.NotNil(evalOut)
	assert.Equal(index, evalOut.ModifyIndex)
}

// Verifies that applyPlan properly updates the constituent objects in MemDB,
// when the plan contains normalized allocs.
func TestPlanApply_applyPlanWithNormalizedAllocs(t *testing.T) {
	ci.Parallel(t)

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.Build = "1.4.0"
	})
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	// Register node
	node := mock.Node()
	testRegisterNode(t, s1, node)

	// Register a fake deployment
	oldDeployment := mock.Deployment()
	if err := s1.State().UpsertDeployment(900, oldDeployment); err != nil {
		t.Fatalf("UpsertDeployment failed: %v", err)
	}

	// Create a deployment
	dnew := mock.Deployment()

	// Create a deployment update for the old deployment id
	desiredStatus, desiredStatusDescription := "foo", "bar"
	updates := []*structs.DeploymentStatusUpdate{
		{
			DeploymentID:      oldDeployment.ID,
			Status:            desiredStatus,
			StatusDescription: desiredStatusDescription,
		},
	}

	// Register allocs, deployment and deployment update
	alloc := mock.Alloc()
	stoppedAlloc := mock.Alloc()
	stoppedAllocDiff := &structs.Allocation{
		ID:                 stoppedAlloc.ID,
		DesiredDescription: "Desired Description",
		ClientStatus:       structs.AllocClientStatusLost,
	}
	preemptedAlloc := mock.Alloc()
	preemptedAllocDiff := &structs.Allocation{
		ID:                    preemptedAlloc.ID,
		PreemptedByAllocation: alloc.ID,
	}
	s1.State().UpsertJobSummary(1000, mock.JobSummary(alloc.JobID))
	s1.State().UpsertAllocs(structs.MsgTypeTestSetup, 1100, []*structs.Allocation{stoppedAlloc, preemptedAlloc})
	// Create an eval
	eval := mock.Eval()
	eval.JobID = alloc.JobID
	if err := s1.State().UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval}); err != nil {
		t.Fatalf("err: %v", err)
	}

	timestampBeforeCommit := time.Now().UTC().UnixNano()
	planRes := &structs.PlanResult{
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc},
		},
		NodeUpdate: map[string][]*structs.Allocation{
			stoppedAlloc.NodeID: {stoppedAllocDiff},
		},
		NodePreemptions: map[string][]*structs.Allocation{
			preemptedAlloc.NodeID: {preemptedAllocDiff},
		},
		Deployment:        dnew,
		DeploymentUpdates: updates,
	}

	// Snapshot the state
	snap, err := s1.State().Snapshot()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create the plan with a deployment
	plan := &structs.Plan{
		Job:               alloc.Job,
		Deployment:        dnew,
		DeploymentUpdates: updates,
		EvalID:            eval.ID,
	}

	require := require.New(t)
	assert := assert.New(t)

	// Apply the plan
	future, err := s1.applyPlan(plan, planRes, snap)
	require.NoError(err)

	// Verify our optimistic snapshot is updated
	ws := memdb.NewWatchSet()
	allocOut, err := snap.AllocByID(ws, alloc.ID)
	require.NoError(err)
	require.NotNil(allocOut)

	deploymentOut, err := snap.DeploymentByID(ws, plan.Deployment.ID)
	require.NoError(err)
	require.NotNil(deploymentOut)

	// Check plan does apply cleanly
	index, err := planWaitFuture(future)
	require.NoError(err)
	assert.NotEqual(0, index)

	// Lookup the allocation
	fsmState := s1.fsm.State()
	allocOut, err = fsmState.AllocByID(ws, alloc.ID)
	require.NoError(err)
	require.NotNil(allocOut)
	assert.True(allocOut.CreateTime > 0)
	assert.True(allocOut.ModifyTime > 0)
	assert.Equal(allocOut.CreateTime, allocOut.ModifyTime)

	// Verify stopped alloc diff applied cleanly
	updatedStoppedAlloc, err := fsmState.AllocByID(ws, stoppedAlloc.ID)
	require.NoError(err)
	require.NotNil(updatedStoppedAlloc)
	assert.True(updatedStoppedAlloc.ModifyTime > timestampBeforeCommit)
	assert.Equal(updatedStoppedAlloc.DesiredDescription, stoppedAllocDiff.DesiredDescription)
	assert.Equal(updatedStoppedAlloc.ClientStatus, stoppedAllocDiff.ClientStatus)
	assert.Equal(updatedStoppedAlloc.DesiredStatus, structs.AllocDesiredStatusStop)

	// Verify preempted alloc diff applied cleanly
	updatedPreemptedAlloc, err := fsmState.AllocByID(ws, preemptedAlloc.ID)
	require.NoError(err)
	require.NotNil(updatedPreemptedAlloc)
	assert.True(updatedPreemptedAlloc.ModifyTime > timestampBeforeCommit)
	assert.Equal(updatedPreemptedAlloc.DesiredDescription,
		"Preempted by alloc ID "+preemptedAllocDiff.PreemptedByAllocation)
	assert.Equal(updatedPreemptedAlloc.DesiredStatus, structs.AllocDesiredStatusEvict)

	// Lookup the new deployment
	dout, err := fsmState.DeploymentByID(ws, plan.Deployment.ID)
	require.NoError(err)
	require.NotNil(dout)

	// Lookup the updated deployment
	dout2, err := fsmState.DeploymentByID(ws, oldDeployment.ID)
	require.NoError(err)
	require.NotNil(dout2)
	assert.Equal(desiredStatus, dout2.Status)
	assert.Equal(desiredStatusDescription, dout2.StatusDescription)

	// Lookup updated eval
	evalOut, err := fsmState.EvalByID(ws, eval.ID)
	require.NoError(err)
	require.NotNil(evalOut)
	assert.Equal(index, evalOut.ModifyIndex)
}

func TestPlanApply_EvalPlan_Simple(t *testing.T) {
	ci.Parallel(t)
	state := testStateStore(t)
	node := mock.Node()
	state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	snap, _ := state.Snapshot()

	alloc := mock.Alloc()
	plan := &structs.Plan{
		Job: alloc.Job,
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc},
		},
		Deployment: mock.Deployment(),
		DeploymentUpdates: []*structs.DeploymentStatusUpdate{
			{
				DeploymentID:      uuid.Generate(),
				Status:            "foo",
				StatusDescription: "bar",
			},
		},
	}

	pool := NewEvaluatePool(workerPoolSize, workerPoolBufferSize)
	defer pool.Shutdown()

	result, err := evaluatePlan(pool, snap, plan, testlog.HCLogger(t))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if result == nil {
		t.Fatalf("missing result")
	}
	if !reflect.DeepEqual(result.NodeAllocation, plan.NodeAllocation) {
		t.Fatalf("incorrect node allocations")
	}
	if !reflect.DeepEqual(result.Deployment, plan.Deployment) {
		t.Fatalf("incorrect deployment")
	}
	if !reflect.DeepEqual(result.DeploymentUpdates, plan.DeploymentUpdates) {
		t.Fatalf("incorrect deployment updates")
	}
}

func TestPlanApply_EvalPlan_Preemption(t *testing.T) {
	ci.Parallel(t)
	state := testStateStore(t)
	node := mock.Node()
	node.NodeResources = &structs.NodeResources{
		Cpu: structs.NodeCpuResources{
			CpuShares: 2000,
		},
		Memory: structs.NodeMemoryResources{
			MemoryMB: 4192,
		},
		Disk: structs.NodeDiskResources{
			DiskMB: 30 * 1024,
		},
		Networks: []*structs.NetworkResource{
			{
				Device: "eth0",
				CIDR:   "192.168.0.100/32",
				MBits:  1000,
			},
		},
	}
	state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)

	preemptedAlloc := mock.Alloc()
	preemptedAlloc.NodeID = node.ID
	preemptedAlloc.AllocatedResources = &structs.AllocatedResources{
		Shared: structs.AllocatedSharedResources{
			DiskMB: 25 * 1024,
		},
		Tasks: map[string]*structs.AllocatedTaskResources{
			"web": {
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 1500,
				},
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 4000,
				},
				Networks: []*structs.NetworkResource{
					{
						Device:        "eth0",
						IP:            "192.168.0.100",
						ReservedPorts: []structs.Port{{Label: "admin", Value: 5000}},
						MBits:         800,
						DynamicPorts:  []structs.Port{{Label: "http", Value: 9876}},
					},
				},
			},
		},
	}

	// Insert a preempted alloc such that the alloc will fit only after preemption
	state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{preemptedAlloc})

	alloc := mock.Alloc()
	alloc.AllocatedResources = &structs.AllocatedResources{
		Shared: structs.AllocatedSharedResources{
			DiskMB: 24 * 1024,
		},
		Tasks: map[string]*structs.AllocatedTaskResources{
			"web": {
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 1500,
				},
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 3200,
				},
				Networks: []*structs.NetworkResource{
					{
						Device:        "eth0",
						IP:            "192.168.0.100",
						ReservedPorts: []structs.Port{{Label: "admin", Value: 5000}},
						MBits:         800,
						DynamicPorts:  []structs.Port{{Label: "http", Value: 9876}},
					},
				},
			},
		},
	}
	plan := &structs.Plan{
		Job: alloc.Job,
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc},
		},
		NodePreemptions: map[string][]*structs.Allocation{
			node.ID: {preemptedAlloc},
		},
		Deployment: mock.Deployment(),
		DeploymentUpdates: []*structs.DeploymentStatusUpdate{
			{
				DeploymentID:      uuid.Generate(),
				Status:            "foo",
				StatusDescription: "bar",
			},
		},
	}
	snap, _ := state.Snapshot()

	pool := NewEvaluatePool(workerPoolSize, workerPoolBufferSize)
	defer pool.Shutdown()

	result, err := evaluatePlan(pool, snap, plan, testlog.HCLogger(t))

	require := require.New(t)
	require.NoError(err)
	require.NotNil(result)

	require.Equal(result.NodeAllocation, plan.NodeAllocation)
	require.Equal(result.Deployment, plan.Deployment)
	require.Equal(result.DeploymentUpdates, plan.DeploymentUpdates)
	require.Equal(result.NodePreemptions, plan.NodePreemptions)

}

func TestPlanApply_EvalPlan_Partial(t *testing.T) {
	ci.Parallel(t)
	state := testStateStore(t)
	node := mock.Node()
	state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	node2 := mock.Node()
	state.UpsertNode(structs.MsgTypeTestSetup, 1001, node2)
	snap, _ := state.Snapshot()

	alloc := mock.Alloc()
	alloc.Name = "example.cache[0]"

	alloc2 := mock.Alloc() // Ensure alloc2 does not fit
	alloc2.AllocatedResources = structs.NodeResourcesToAllocatedResources(node2.NodeResources)
	alloc2.Name = "example.cache[1]"

	// Create a deployment where the allocs are markeda as canaries
	d := mock.Deployment()
	d.TaskGroups["web"].PlacedCanaries = []string{alloc.ID, alloc2.ID}

	plan := &structs.Plan{
		Job: alloc.Job,
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID:  {alloc},
			node2.ID: {alloc2},
		},
		Deployment: d,
	}

	pool := NewEvaluatePool(workerPoolSize, workerPoolBufferSize)
	defer pool.Shutdown()

	result, err := evaluatePlan(pool, snap, plan, testlog.HCLogger(t))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if result == nil {
		t.Fatalf("missing result")
	}

	if _, ok := result.NodeAllocation[node.ID]; !ok {
		t.Fatalf("should allow alloc")
	}
	if _, ok := result.NodeAllocation[node2.ID]; ok {
		t.Fatalf("should not allow alloc2")
	}

	// Check the deployment was updated
	if result.Deployment == nil || len(result.Deployment.TaskGroups) == 0 {
		t.Fatalf("bad: %v", result.Deployment)
	}
	placedCanaries := result.Deployment.TaskGroups["web"].PlacedCanaries
	if len(placedCanaries) != 1 || placedCanaries[0] != alloc.ID {
		t.Fatalf("bad: %v", placedCanaries)
	}

	if result.RefreshIndex != 1001 {
		t.Fatalf("bad: %d", result.RefreshIndex)
	}
}

func TestPlanApply_EvalPlan_Partial_AllAtOnce(t *testing.T) {
	ci.Parallel(t)
	state := testStateStore(t)
	node := mock.Node()
	state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	node2 := mock.Node()
	state.UpsertNode(structs.MsgTypeTestSetup, 1001, node2)
	snap, _ := state.Snapshot()

	alloc := mock.Alloc()
	alloc.Name = "example.cache[0]"

	alloc2 := mock.Alloc() // Ensure alloc2 does not fit
	alloc2.AllocatedResources = structs.NodeResourcesToAllocatedResources(node2.NodeResources)
	alloc2.Name = "example.cache[1]"

	plan := &structs.Plan{
		Job:       alloc.Job,
		AllAtOnce: true, // Require all to make progress
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID:  {alloc},
			node2.ID: {alloc2},
		},
		Deployment: mock.Deployment(),
		DeploymentUpdates: []*structs.DeploymentStatusUpdate{
			{
				DeploymentID:      uuid.Generate(),
				Status:            "foo",
				StatusDescription: "bar",
			},
		},
	}

	pool := NewEvaluatePool(workerPoolSize, workerPoolBufferSize)
	defer pool.Shutdown()

	result, err := evaluatePlan(pool, snap, plan, testlog.HCLogger(t))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if result == nil {
		t.Fatalf("missing result")
	}

	if len(result.NodeAllocation) != 0 {
		t.Fatalf("should not alloc: %v", result.NodeAllocation)
	}
	if result.RefreshIndex != 1001 {
		t.Fatalf("bad: %d", result.RefreshIndex)
	}
	if result.Deployment != nil || len(result.DeploymentUpdates) != 0 {
		t.Fatalf("bad: %v", result)
	}
}

func TestPlanApply_EvalNodePlan_Simple(t *testing.T) {
	ci.Parallel(t)
	state := testStateStore(t)
	node := mock.Node()
	state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	snap, _ := state.Snapshot()

	alloc := mock.Alloc()
	plan := &structs.Plan{
		Job: alloc.Job,
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc},
		},
	}

	fit, reason, err := evaluateNodePlan(snap, plan, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !fit {
		t.Fatalf("bad")
	}
	if reason != "" {
		t.Fatalf("bad")
	}
}

func TestPlanApply_EvalNodePlan_NodeNotReady(t *testing.T) {
	ci.Parallel(t)
	state := testStateStore(t)
	node := mock.Node()
	node.Status = structs.NodeStatusInit
	state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	snap, _ := state.Snapshot()

	alloc := mock.Alloc()
	plan := &structs.Plan{
		Job: alloc.Job,
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc},
		},
	}

	fit, reason, err := evaluateNodePlan(snap, plan, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fit {
		t.Fatalf("bad")
	}
	if reason == "" {
		t.Fatalf("bad")
	}
}

func TestPlanApply_EvalNodePlan_NodeDrain(t *testing.T) {
	ci.Parallel(t)
	state := testStateStore(t)
	node := mock.DrainNode()
	state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	snap, _ := state.Snapshot()

	alloc := mock.Alloc()
	plan := &structs.Plan{
		Job: alloc.Job,
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc},
		},
	}

	fit, reason, err := evaluateNodePlan(snap, plan, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fit {
		t.Fatalf("bad")
	}
	if reason == "" {
		t.Fatalf("bad")
	}
}

func TestPlanApply_EvalNodePlan_NodeNotExist(t *testing.T) {
	ci.Parallel(t)
	state := testStateStore(t)
	snap, _ := state.Snapshot()

	nodeID := "12345678-abcd-efab-cdef-123456789abc"
	alloc := mock.Alloc()
	plan := &structs.Plan{
		Job: alloc.Job,
		NodeAllocation: map[string][]*structs.Allocation{
			nodeID: {alloc},
		},
	}

	fit, reason, err := evaluateNodePlan(snap, plan, nodeID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fit {
		t.Fatalf("bad")
	}
	if reason == "" {
		t.Fatalf("bad")
	}
}

func TestPlanApply_EvalNodePlan_NodeFull(t *testing.T) {
	ci.Parallel(t)
	alloc := mock.Alloc()
	state := testStateStore(t)
	node := mock.Node()
	node.ReservedResources = nil
	alloc.NodeID = node.ID
	alloc.AllocatedResources = structs.NodeResourcesToAllocatedResources(node.NodeResources)
	state.UpsertJobSummary(999, mock.JobSummary(alloc.JobID))
	state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc})

	alloc2 := mock.Alloc()
	alloc2.NodeID = node.ID
	state.UpsertJobSummary(1200, mock.JobSummary(alloc2.JobID))

	snap, _ := state.Snapshot()
	plan := &structs.Plan{
		Job: alloc.Job,
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc2},
		},
	}

	fit, reason, err := evaluateNodePlan(snap, plan, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fit {
		t.Fatalf("bad")
	}
	if reason == "" {
		t.Fatalf("bad")
	}
}

// Test that we detect device oversubscription
func TestPlanApply_EvalNodePlan_NodeFull_Device(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	testState := testStateStore(t)
	node := mock.NvidiaNode()
	node.ReservedResources = nil

	nvidia0 := node.NodeResources.Devices[0].Instances[0].ID

	// Have the allocation use a Nvidia device
	alloc.NodeID = node.ID
	alloc.AllocatedResources.Tasks["web"].Devices = []*structs.AllocatedDeviceResource{
		{
			Type:      "gpu",
			Vendor:    "nvidia",
			Name:      "1080ti",
			DeviceIDs: []string{nvidia0},
		},
	}

	must.NoError(t, testState.UpsertJobSummary(999, mock.JobSummary(alloc.JobID)))
	must.NoError(t, testState.UpsertNode(structs.MsgTypeTestSetup, 1000, node))
	must.NoError(t, testState.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}))

	// Alloc2 tries to use the same device
	alloc2 := mock.Alloc()
	alloc2.AllocatedResources.Tasks["web"].Networks = nil
	alloc2.AllocatedResources.Tasks["web"].Devices = []*structs.AllocatedDeviceResource{
		{
			Type:      "gpu",
			Vendor:    "nvidia",
			Name:      "1080ti",
			DeviceIDs: []string{nvidia0},
		},
	}
	alloc2.NodeID = node.ID
	must.NoError(t, testState.UpsertJobSummary(1200, mock.JobSummary(alloc2.JobID)))

	snap, err := testState.Snapshot()
	must.NoError(t, err)

	plan := &structs.Plan{
		Job: alloc.Job,
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc2},
		},
	}

	fit, reason, err := evaluateNodePlan(snap, plan, node.ID)
	must.NoError(t, err)
	must.False(t, fit)
	must.Eq(t, "device oversubscribed", reason)
}

func TestPlanApply_EvalNodePlan_UpdateExisting(t *testing.T) {
	ci.Parallel(t)
	alloc := mock.Alloc()
	testState := testStateStore(t)
	node := mock.Node()
	node.ReservedResources = nil
	node.Reserved = nil
	alloc.NodeID = node.ID
	alloc.AllocatedResources = structs.NodeResourcesToAllocatedResources(node.NodeResources)
	must.NoError(t, testState.UpsertNode(structs.MsgTypeTestSetup, 1000, node))
	must.NoError(t, testState.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}))
	snap, err := testState.Snapshot()
	must.NoError(t, err)

	plan := &structs.Plan{
		Job: alloc.Job,
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc},
		},
	}

	fit, reason, err := evaluateNodePlan(snap, plan, node.ID)
	must.NoError(t, err)
	must.True(t, fit)
	must.Eq(t, "", reason)
}

func TestPlanApply_EvalNodePlan_UpdateExisting_Ineligible(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	testState := testStateStore(t)
	node := mock.Node()
	node.ReservedResources = nil
	node.Reserved = nil
	node.SchedulingEligibility = structs.NodeSchedulingIneligible
	alloc.NodeID = node.ID
	alloc.AllocatedResources = structs.NodeResourcesToAllocatedResources(node.NodeResources)
	must.NoError(t, testState.UpsertNode(structs.MsgTypeTestSetup, 1000, node))
	must.NoError(t, testState.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}))
	snap, err := testState.Snapshot()
	must.NoError(t, err)

	plan := &structs.Plan{
		Job: alloc.Job,
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc},
		},
	}

	fit, reason, err := evaluateNodePlan(snap, plan, node.ID)
	must.NoError(t, err)
	must.True(t, fit)
	must.Eq(t, "", reason)
}

func TestPlanApply_EvalNodePlan_NodeFull_Evict(t *testing.T) {
	ci.Parallel(t)
	alloc := mock.Alloc()
	testState := testStateStore(t)
	node := mock.Node()
	node.ReservedResources = nil
	alloc.NodeID = node.ID
	alloc.AllocatedResources = structs.NodeResourcesToAllocatedResources(node.NodeResources)
	must.NoError(t, testState.UpsertNode(structs.MsgTypeTestSetup, 1000, node))
	must.NoError(t, testState.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}))
	snap, err := testState.Snapshot()
	must.NoError(t, err)

	allocEvict := new(structs.Allocation)
	*allocEvict = *alloc
	allocEvict.DesiredStatus = structs.AllocDesiredStatusEvict
	alloc2 := mock.Alloc()
	plan := &structs.Plan{
		Job: alloc.Job,
		NodeUpdate: map[string][]*structs.Allocation{
			node.ID: {allocEvict},
		},
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc2},
		},
	}

	fit, reason, err := evaluateNodePlan(snap, plan, node.ID)
	must.NoError(t, err)
	must.True(t, fit)
	must.Eq(t, "", reason)
}

func TestPlanApply_EvalNodePlan_NodeFull_AllocEvict(t *testing.T) {
	ci.Parallel(t)
	alloc := mock.Alloc()
	testState := testStateStore(t)
	node := mock.Node()
	node.ReservedResources = nil
	alloc.NodeID = node.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusEvict
	alloc.AllocatedResources = structs.NodeResourcesToAllocatedResources(node.NodeResources)
	must.NoError(t, testState.UpsertNode(structs.MsgTypeTestSetup, 1000, node))
	must.NoError(t, testState.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}))
	snap, err := testState.Snapshot()
	must.NoError(t, err)

	alloc2 := mock.Alloc()
	plan := &structs.Plan{
		Job: alloc.Job,
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc2},
		},
	}

	fit, reason, err := evaluateNodePlan(snap, plan, node.ID)
	must.NoError(t, err)
	must.True(t, fit)
	must.Eq(t, "", reason)
}

func TestPlanApply_EvalNodePlan_NodeDown_EvictOnly(t *testing.T) {
	ci.Parallel(t)
	alloc := mock.Alloc()
	testState := testStateStore(t)
	node := mock.Node()
	alloc.NodeID = node.ID
	alloc.AllocatedResources = structs.NodeResourcesToAllocatedResources(node.NodeResources)
	node.ReservedResources = nil
	node.Status = structs.NodeStatusDown
	must.NoError(t, testState.UpsertNode(structs.MsgTypeTestSetup, 1000, node))
	must.NoError(t, testState.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}))
	snap, err := testState.Snapshot()
	must.NoError(t, err)

	allocEvict := new(structs.Allocation)
	*allocEvict = *alloc
	allocEvict.DesiredStatus = structs.AllocDesiredStatusEvict
	plan := &structs.Plan{
		Job: alloc.Job,
		NodeUpdate: map[string][]*structs.Allocation{
			node.ID: {allocEvict},
		},
	}

	fit, reason, err := evaluateNodePlan(snap, plan, node.ID)
	must.NoError(t, err)
	must.True(t, fit)
	must.Eq(t, "", reason)
}

// TestPlanApply_EvalNodePlan_Node_Disconnected tests that plans for disconnected
// nodes can only contain allocs with client status unknown.
func TestPlanApply_EvalNodePlan_Node_Disconnected(t *testing.T) {
	ci.Parallel(t)

	testState := testStateStore(t)
	node := mock.Node()
	node.Status = structs.NodeStatusDisconnected
	must.NoError(t, testState.UpsertNode(structs.MsgTypeTestSetup, 1000, node))

	snap, err := testState.Snapshot()
	must.NoError(t, err)

	unknownAlloc := mock.Alloc()
	unknownAlloc.ClientStatus = structs.AllocClientStatusUnknown

	runningAlloc := unknownAlloc.Copy()
	runningAlloc.ClientStatus = structs.AllocClientStatusRunning

	job := unknownAlloc.Job

	type testCase struct {
		name           string
		nodeAllocs     map[string][]*structs.Allocation
		expectedFit    bool
		expectedReason string
	}

	testCases := []testCase{
		{
			name: "unknown-valid",
			nodeAllocs: map[string][]*structs.Allocation{
				node.ID: {unknownAlloc},
			},
			expectedFit:    true,
			expectedReason: "",
		},
		{
			name: "running-invalid",
			nodeAllocs: map[string][]*structs.Allocation{
				node.ID: {runningAlloc},
			},
			expectedFit:    false,
			expectedReason: "node is disconnected and contains invalid updates",
		},
		{
			name: "multiple-invalid",
			nodeAllocs: map[string][]*structs.Allocation{
				node.ID: {runningAlloc, unknownAlloc},
			},
			expectedFit:    false,
			expectedReason: "node is disconnected and contains invalid updates",
		},
		{
			name: "multiple-valid",
			nodeAllocs: map[string][]*structs.Allocation{
				node.ID: {unknownAlloc, unknownAlloc.Copy()},
			},
			expectedFit:    true,
			expectedReason: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plan := &structs.Plan{
				Job:            job,
				NodeAllocation: tc.nodeAllocs,
			}

			fit, reason, err := evaluateNodePlan(snap, plan, node.ID)
			must.NoError(t, err)
			must.Eq(t, tc.expectedFit, fit)
			must.Eq(t, tc.expectedReason, reason)
		})
	}
}

func Test_evaluatePlanAllocIndexes(t *testing.T) {
	ci.Parallel(t)

	// Generate test state that will be used throughout this test. The helper
	// function can be used to pull a state snapshot which is passed to the
	// evaluatePlanAllocIndexes function.
	testState := testStateStore(t)

	testStateSnapshotFn := func(t *testing.T, testState *state.StateStore) *state.StateSnapshot {
		testStateSnapshot, testStateSnapshotErr := testState.Snapshot()
		must.NoError(t, testStateSnapshotErr)
		return testStateSnapshot
	}

	// Generate and test a non-conflicting alloc index plan which mimics an
	// initial job registration.
	t.Run("initial registration", func(t *testing.T) {
		testPlan := structs.Plan{
			Job: &structs.Job{
				ID:        "example",
				Namespace: "default",
				Version:   0,
			},
			NodeAllocation: map[string][]*structs.Allocation{
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: "example.cache[1]", ID: "bd8cf1bc-5ded-5942-5b39-078878e74e15"},
					{Name: "example.cache[3]", ID: "e3036b79-c88f-d8a5-09bd-202bc5a842c2"},
				},
				"52b3508a-c88e-fc83-74c9-829d5ef1103a": {
					{Name: "example.cache[0]", ID: "2a8243d9-c54b-ec78-8b51-e99ff3579d72"},
					{Name: "example.cache[2]", ID: "a5eee921-7d25-0e64-78dd-34c630a0ee67"},
				},
			},
		}
		must.NoError(t, evaluatePlanAllocIndexes(testStateSnapshotFn(t, testState), &testPlan))
	})

	t.Run("initial registration conflict", func(t *testing.T) {
		testPlan := structs.Plan{
			Job: &structs.Job{
				ID:        "example",
				Namespace: "default",
				Version:   0,
			},
			NodeAllocation: map[string][]*structs.Allocation{
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: "example.cache[1]", ID: "bd8cf1bc-5ded-5942-5b39-078878e74e15"},
					{Name: "example.cache[2]", ID: "e3036b79-c88f-d8a5-09bd-202bc5a842c2"},
				},
				"52b3508a-c88e-fc83-74c9-829d5ef1103a": {
					{Name: "example.cache[0]", ID: "2a8243d9-c54b-ec78-8b51-e99ff3579d72"},
					{Name: "example.cache[2]", ID: "a5eee921-7d25-0e64-78dd-34c630a0ee67"},
				},
			},
		}
		must.ErrorContains(t,
			evaluatePlanAllocIndexes(testStateSnapshotFn(t, testState), &testPlan),
			duplicateAllocIndexErrorString)
	})

	// Generate and insert 3 test allocations into state. This mimics the
	// behaviour of a job that has been registered and successfully deployed.
	// The written state does not change, and forms the basis for testing
	// plans.
	testAllocs := []*structs.Allocation{mock.Alloc(), mock.Alloc(), mock.Alloc()}

	for i, testAlloc := range testAllocs {
		testAlloc.Job = testAllocs[0].Job
		testAlloc.JobID = testAllocs[0].Job.ID
		testAlloc.ClientStatus = structs.AllocClientStatusRunning
		testAlloc.Name = fmt.Sprintf("%s.%s[%v]", testAlloc.JobID, testAlloc.TaskGroup, i)
	}

	must.NoError(t, testState.UpsertAllocs(structs.MsgTypeTestSetup, 10, testAllocs))

	// Grab the concatenation of the jobID and task group name for ease.
	testJobGroupID := testAllocs[0].JobID + "." + testAllocs[0].TaskGroup

	// Generate a plan with an incremented job version and four node
	// allocations. This represents a plan generated by an operator scaling
	// the job by 1.
	t.Run("job scale out", func(t *testing.T) {
		testPlan := structs.Plan{
			Job: &structs.Job{
				ID:        testAllocs[0].JobID,
				Namespace: testAllocs[0].Namespace,
				Version:   1,
			},
			NodeAllocation: map[string][]*structs.Allocation{
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: "example.cache[1]", ID: "bd8cf1bc-5ded-5942-5b39-078878e74e15"},
					{Name: "example.cache[3]", ID: "e3036b79-c88f-d8a5-09bd-202bc5a842c2"},
				},
				"52b3508a-c88e-fc83-74c9-829d5ef1103a": {
					{Name: "example.cache[0]", ID: "2a8243d9-c54b-ec78-8b51-e99ff3579d72"},
					{Name: "example.cache[2]", ID: "a5eee921-7d25-0e64-78dd-34c630a0ee67"},
				},
			},
		}
		must.NoError(t, evaluatePlanAllocIndexes(testStateSnapshotFn(t, testState), &testPlan))
	})

	t.Run("job scale out conflict", func(t *testing.T) {
		testPlan := structs.Plan{
			Job: &structs.Job{
				ID:        testAllocs[0].JobID,
				Namespace: testAllocs[0].Namespace,
				Version:   1,
			},
			NodeAllocation: map[string][]*structs.Allocation{
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: "example.cache[1]", ID: "bd8cf1bc-5ded-5942-5b39-078878e74e15"},
					{Name: "example.cache[3]", ID: "e3036b79-c88f-d8a5-09bd-202bc5a842c2"},
				},
				"52b3508a-c88e-fc83-74c9-829d5ef1103a": {
					{Name: "example.cache[1]", ID: "2a8243d9-c54b-ec78-8b51-e99ff3579d72"},
					{Name: "example.cache[2]", ID: "a5eee921-7d25-0e64-78dd-34c630a0ee67"},
				},
			},
		}
		must.ErrorContains(t,
			evaluatePlanAllocIndexes(testStateSnapshotFn(t, testState), &testPlan),
			duplicateAllocIndexErrorString)
	})

	// Generate a plan which represents a job scaling in. This means we have a
	// version incrementation, a node update and node allocations alongside the
	// existing state allocs.
	t.Run("job scale in", func(t *testing.T) {
		testPlan := structs.Plan{
			Job: &structs.Job{
				ID:        testAllocs[0].JobID,
				Namespace: testAllocs[0].Namespace,
				Version:   1,
			},
			NodeUpdate: map[string][]*structs.Allocation{
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: testJobGroupID + "[2]", ID: "bd8cf1bc-5ded-5942-5b39-078878e74e15"},
				},
			},
			NodeAllocation: map[string][]*structs.Allocation{
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: testJobGroupID + "[1]", ID: testAllocs[0].ID},
				},
				"52b3508a-c88e-fc83-74c9-829d5ef1103a": {
					{Name: testJobGroupID + "[0]", ID: testAllocs[1].ID},
				},
			},
		}
		must.NoError(t, evaluatePlanAllocIndexes(testStateSnapshotFn(t, testState), &testPlan))
	})

	t.Run("job scale in conflict", func(t *testing.T) {
		testPlan := structs.Plan{
			Job: &structs.Job{
				ID:        testAllocs[0].JobID,
				Namespace: testAllocs[0].Namespace,
				Version:   1,
			},
			NodeUpdate: map[string][]*structs.Allocation{
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: testJobGroupID + "[2]", ID: "bd8cf1bc-5ded-5942-5b39-078878e74e15"},
				},
			},
			NodeAllocation: map[string][]*structs.Allocation{
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: testJobGroupID + "[1]", ID: testAllocs[0].ID},
				},
				"52b3508a-c88e-fc83-74c9-829d5ef1103a": {
					{Name: testJobGroupID + "[1]", ID: testAllocs[1].ID},
				},
			},
		}
		must.ErrorContains(t,
			evaluatePlanAllocIndexes(testStateSnapshotFn(t, testState), &testPlan),
			duplicateAllocIndexErrorString)
	})

	// Generate a plan which represents a job update, such as modifying the
	// Docker tag used. This depends on the update block, but in default
	// configurations, the following plan example will be seen.
	t.Run("job update", func(t *testing.T) {
		testPlan := structs.Plan{
			Job: &structs.Job{
				ID:        testAllocs[0].JobID,
				Namespace: testAllocs[0].Namespace,
				Version:   1,
			},
			NodeUpdate: map[string][]*structs.Allocation{
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: testJobGroupID + "[2]", ID: testAllocs[2].ID},
				},
			},
			NodeAllocation: map[string][]*structs.Allocation{
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: testJobGroupID + "[2]", ID: "a5eee921-7d25-0e64-78dd-34c630a0ee67"},
				},
			},
		}
		must.NoError(t, evaluatePlanAllocIndexes(testStateSnapshotFn(t, testState), &testPlan))
	})

	t.Run("job update conflict", func(t *testing.T) {
		testPlan := structs.Plan{
			Job: &structs.Job{
				ID:        testAllocs[0].JobID,
				Namespace: testAllocs[0].Namespace,
				Version:   1,
			},
			NodeUpdate: map[string][]*structs.Allocation{
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: "example.cache[1]", ID: testAllocs[1].ID},
					{Name: "example.cache[3]", ID: testAllocs[2].ID},
				},
			},
			NodeAllocation: map[string][]*structs.Allocation{
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: testJobGroupID + "[2]", ID: "2a8243d9-c54b-ec78-8b51-e99ff3579d72"},
					{Name: testJobGroupID + "[2]", ID: "a5eee921-7d25-0e64-78dd-34c630a0ee67"},
				},
			},
		}
		must.ErrorContains(t,
			evaluatePlanAllocIndexes(testStateSnapshotFn(t, testState), &testPlan),
			duplicateAllocIndexErrorString)
	})

	// When a client that is running allocations does not heartbeat, a plan
	// will be generated to replace the allocations. The job version is NOT
	// incremented, so this test is important to ensure this functionality is
	// still working.
	t.Run("alloc lost", func(t *testing.T) {
		testPlan := structs.Plan{
			Job: &structs.Job{
				ID:        testAllocs[0].JobID,
				Namespace: testAllocs[0].Namespace,
				Version:   0,
			},
			NodeUpdate: map[string][]*structs.Allocation{
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: testJobGroupID + "[0]", ID: testAllocs[0].ID},
					{Name: testJobGroupID + "[2]", ID: testAllocs[2].ID},
				},
			},
			NodeAllocation: map[string][]*structs.Allocation{
				"52b3508a-c88e-fc83-74c9-829d5ef1103a": {
					{Name: testJobGroupID + "[0]", ID: "2a8243d9-c54b-ec78-8b51-e99ff3579d72"},
					{Name: testJobGroupID + "[2]", ID: "a5eee921-7d25-0e64-78dd-34c630a0ee67"},
				},
			},
		}
		must.NoError(t, evaluatePlanAllocIndexes(testStateSnapshotFn(t, testState), &testPlan))
	})

	t.Run("alloc lost conflict", func(t *testing.T) {
		testPlan := structs.Plan{
			Job: &structs.Job{
				ID:        testAllocs[0].JobID,
				Namespace: testAllocs[0].Namespace,
				Version:   0,
			},
			NodeUpdate: map[string][]*structs.Allocation{
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: testJobGroupID + "[0]"},
					{Name: testJobGroupID + "[2]"},
				},
			},
			NodeAllocation: map[string][]*structs.Allocation{
				"52b3508a-c88e-fc83-74c9-829d5ef1103a": {
					{Name: testJobGroupID + "[2]"},
					{Name: testJobGroupID + "[2]"},
				},
			},
		}
		must.ErrorContains(t,
			evaluatePlanAllocIndexes(testStateSnapshotFn(t, testState), &testPlan),
			duplicateAllocIndexErrorString)
	})

	// Stopping a job produces a destructive plan only, but increments the job
	// version. We cannot really test for conflicts here, so this can be used
	// to ensure correct behaviour.
	t.Run("job stop", func(t *testing.T) {
		testPlan := structs.Plan{
			Job: &structs.Job{
				ID:        testAllocs[0].JobID,
				Namespace: testAllocs[0].Namespace,
				Version:   1,
			},
			NodeUpdate: map[string][]*structs.Allocation{
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: testJobGroupID + "[0]"},
					{Name: testJobGroupID + "[1]"},
					{Name: testJobGroupID + "[2]"},
				},
			},
			NodeAllocation: nil,
		}
		must.NoError(t, evaluatePlanAllocIndexes(testStateSnapshotFn(t, testState), &testPlan))
	})

	// When a deployment finishes, a noop plan is processed which does not
	// include any allocation updates.
	t.Run("deployment finish", func(t *testing.T) {
		testPlan := structs.Plan{
			Job: &structs.Job{
				ID:        testAllocs[0].JobID,
				Namespace: testAllocs[0].Namespace,
				Version:   1,
			},
			NodeUpdate:     nil,
			NodeAllocation: nil,
		}
		must.NoError(t, evaluatePlanAllocIndexes(testStateSnapshotFn(t, testState), &testPlan))
	})

	// Canary deployments generate a unique state and plan. This test exercises
	// the plan generated by a canary deployment being promoted.
	t.Run("canary promote", func(t *testing.T) {

		// Generate a canary allocation and upset this into state. This mimics
		// a deployment state which requires manual promotion.
		canaryAlloc := mock.Alloc()
		canaryAlloc.Job.Version = 1
		canaryAlloc.Job = testAllocs[0].Job
		canaryAlloc.JobID = testAllocs[0].Job.ID
		canaryAlloc.Name = fmt.Sprintf("%s.%s[%v]", canaryAlloc.JobID, canaryAlloc.TaskGroup, 0)

		must.NoError(t, testState.UpsertAllocs(structs.MsgTypeTestSetup, 20, []*structs.Allocation{canaryAlloc}))

		testPlan := structs.Plan{
			Job: &structs.Job{
				ID:        testAllocs[0].JobID,
				Namespace: testAllocs[0].Namespace,
				Version:   1,
			},
			NodeUpdate: map[string][]*structs.Allocation{
				"52b3508a-c88e-fc83-74c9-829d5ef1103a": {
					{Name: testJobGroupID + "[0]", ID: testAllocs[0].ID},
					{Name: testJobGroupID + "[1]", ID: testAllocs[1].ID},
				},
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: testJobGroupID + "[2]", ID: testAllocs[2].ID},
				},
			},
			NodeAllocation: map[string][]*structs.Allocation{
				"52b3508a-c88e-fc83-74c9-829d5ef1103a": {
					{Name: testJobGroupID + "[1]"},
				},
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: testJobGroupID + "[2]"},
				},
			},
		}
		must.NoError(t, evaluatePlanAllocIndexes(testStateSnapshotFn(t, testState), &testPlan))

		// Delete the alloc state entry, so it doesn't impact any subsequent
		// tests.
		must.NoError(t, testState.DeleteEval(30, []string{}, []string{canaryAlloc.ID}, false))
	})

	// This test mimics what happens when a client disconnects, and an
	// allocation running on it has max_client_disconnect configured. It runs
	// both the disconnect and reconnect plan process.
	t.Run("client disconnect", func(t *testing.T) {

		// Generate a replacement allocation, which will replace the allocation
		// currently placed on a disconnected client.
		replacementAlloc := mock.Alloc()
		replacementAlloc.Job.Version = 0
		replacementAlloc.NodeID = "52b3508a-c88e-fc83-74c9-829d5ef1103a"
		replacementAlloc.Job = testAllocs[0].Job
		replacementAlloc.JobID = testAllocs[0].Job.ID
		replacementAlloc.ClientStatus = structs.AllocClientStatusRunning
		replacementAlloc.Name = fmt.Sprintf("%s.%s[%v]", replacementAlloc.JobID, replacementAlloc.TaskGroup, 2)

		disconnectTestPlan := structs.Plan{
			Job: &structs.Job{
				ID:        testAllocs[0].JobID,
				Namespace: testAllocs[0].Namespace,
				Version:   0,
			},
			NodeUpdate: nil,
			NodeAllocation: map[string][]*structs.Allocation{
				"52b3508a-c88e-fc83-74c9-829d5ef1103a": {
					{Name: replacementAlloc.Name, ID: replacementAlloc.ID},
				},
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: replacementAlloc.Name, ID: testAllocs[2].ID, ClientStatus: structs.AllocClientStatusUnknown},
				},
			},
		}
		must.NoError(t, evaluatePlanAllocIndexes(testStateSnapshotFn(t, testState), &disconnectTestPlan))

		// Write the replacement to state, to mimic a successful replacement of
		// the disconnected allocation.
		must.NoError(t, testState.UpsertAllocs(
			structs.MsgTypeTestSetup, 40, []*structs.Allocation{replacementAlloc}))

		// Update the disconnect allocation state to unknown.
		testAllocs[2].ClientStatus = structs.AllocClientStatusUnknown
		must.NoError(t, testState.UpsertAllocs(
			structs.MsgTypeTestSetup, 50, []*structs.Allocation{testAllocs[2]}))

		reconnectTestPlan := structs.Plan{
			Job: &structs.Job{
				ID:        testAllocs[0].JobID,
				Namespace: testAllocs[0].Namespace,
				Version:   0,
			},
			NodeUpdate: map[string][]*structs.Allocation{
				"52b3508a-c88e-fc83-74c9-829d5ef1103a": {
					{Name: replacementAlloc.Name, ID: replacementAlloc.ID},
				},
			},
			NodeAllocation: map[string][]*structs.Allocation{
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: testJobGroupID + "[2]", ID: testAllocs[2].ID, ClientStatus: structs.AllocClientStatusRunning},
				},
			},
		}
		must.NoError(t, evaluatePlanAllocIndexes(testStateSnapshotFn(t, testState), &reconnectTestPlan))

		// Delete the alloc state entry, so it doesn't impact any subsequent
		// tests and revert the running allocations state from unknown.
		must.NoError(t, testState.DeleteEval(60, []string{}, []string{replacementAlloc.ID}, false))

		testAllocs[2].ClientStatus = structs.AllocClientStatusRunning
		must.NoError(t, testState.UpsertAllocs(
			structs.MsgTypeTestSetup, 70, []*structs.Allocation{testAllocs[2]}))
	})

	t.Run("client disconnect conflict", func(t *testing.T) {

		// Generate a replacement allocation, which will replace the allocation
		// currently placed on a disconnected client.
		replacementAlloc := mock.Alloc()
		replacementAlloc.Job.Version = 0
		replacementAlloc.NodeID = "52b3508a-c88e-fc83-74c9-829d5ef1103a"
		replacementAlloc.Job = testAllocs[0].Job
		replacementAlloc.JobID = testAllocs[0].Job.ID
		replacementAlloc.ClientStatus = structs.AllocClientStatusRunning
		replacementAlloc.Name = fmt.Sprintf("%s.%s[%v]", replacementAlloc.JobID, replacementAlloc.TaskGroup, 1)

		testPlan := structs.Plan{
			Job: &structs.Job{
				ID:        testAllocs[0].JobID,
				Namespace: testAllocs[0].Namespace,
				Version:   0,
			},
			NodeUpdate: nil,
			NodeAllocation: map[string][]*structs.Allocation{
				"52b3508a-c88e-fc83-74c9-829d5ef1103a": {
					{Name: replacementAlloc.Name, ID: replacementAlloc.ID},
				},
				"8bdb21db-9445-3650-ca9d-0d7883cc8a73": {
					{Name: replacementAlloc.Name, ID: testAllocs[2].ID, ClientStatus: structs.AllocClientStatusUnknown},
				},
			},
		}
		must.ErrorContains(t,
			evaluatePlanAllocIndexes(testStateSnapshotFn(t, testState), &testPlan),
			duplicateAllocIndexErrorString)
	})
}
