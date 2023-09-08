// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"reflect"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
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
	alloc2 := mock.Alloc() // Ensure alloc2 does not fit
	alloc2.AllocatedResources = structs.NodeResourcesToAllocatedResources(node2.NodeResources)

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
	alloc2 := mock.Alloc() // Ensure alloc2 does not fit
	alloc2.AllocatedResources = structs.NodeResourcesToAllocatedResources(node2.NodeResources)
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
	require := require.New(t)
	alloc := mock.Alloc()
	state := testStateStore(t)
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

	state.UpsertJobSummary(999, mock.JobSummary(alloc.JobID))
	state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc})

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
	state.UpsertJobSummary(1200, mock.JobSummary(alloc2.JobID))

	snap, _ := state.Snapshot()
	plan := &structs.Plan{
		Job: alloc.Job,
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: {alloc2},
		},
	}

	fit, reason, err := evaluateNodePlan(snap, plan, node.ID)
	require.NoError(err)
	require.False(fit)
	require.Equal("device oversubscribed", reason)
}

func TestPlanApply_EvalNodePlan_UpdateExisting(t *testing.T) {
	ci.Parallel(t)
	alloc := mock.Alloc()
	state := testStateStore(t)
	node := mock.Node()
	node.ReservedResources = nil
	node.Reserved = nil
	alloc.NodeID = node.ID
	alloc.AllocatedResources = structs.NodeResourcesToAllocatedResources(node.NodeResources)
	state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc})
	snap, _ := state.Snapshot()

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

func TestPlanApply_EvalNodePlan_UpdateExisting_Ineligible(t *testing.T) {
	ci.Parallel(t)
	alloc := mock.Alloc()
	state := testStateStore(t)
	node := mock.Node()
	node.ReservedResources = nil
	node.Reserved = nil
	node.SchedulingEligibility = structs.NodeSchedulingIneligible
	alloc.NodeID = node.ID
	alloc.AllocatedResources = structs.NodeResourcesToAllocatedResources(node.NodeResources)
	state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc})
	snap, _ := state.Snapshot()

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

func TestPlanApply_EvalNodePlan_NodeFull_Evict(t *testing.T) {
	ci.Parallel(t)
	alloc := mock.Alloc()
	state := testStateStore(t)
	node := mock.Node()
	node.ReservedResources = nil
	alloc.NodeID = node.ID
	alloc.AllocatedResources = structs.NodeResourcesToAllocatedResources(node.NodeResources)
	state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc})
	snap, _ := state.Snapshot()

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

func TestPlanApply_EvalNodePlan_NodeFull_AllocEvict(t *testing.T) {
	ci.Parallel(t)
	alloc := mock.Alloc()
	state := testStateStore(t)
	node := mock.Node()
	node.ReservedResources = nil
	alloc.NodeID = node.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusEvict
	alloc.AllocatedResources = structs.NodeResourcesToAllocatedResources(node.NodeResources)
	state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc})
	snap, _ := state.Snapshot()

	alloc2 := mock.Alloc()
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
	if !fit {
		t.Fatalf("bad")
	}
	if reason != "" {
		t.Fatalf("bad")
	}
}

func TestPlanApply_EvalNodePlan_NodeDown_EvictOnly(t *testing.T) {
	ci.Parallel(t)
	alloc := mock.Alloc()
	state := testStateStore(t)
	node := mock.Node()
	alloc.NodeID = node.ID
	alloc.AllocatedResources = structs.NodeResourcesToAllocatedResources(node.NodeResources)
	node.ReservedResources = nil
	node.Status = structs.NodeStatusDown
	state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc})
	snap, _ := state.Snapshot()

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

// TestPlanApply_EvalNodePlan_Node_Disconnected tests that plans for disconnected
// nodes can only contain allocs with client status unknown.
func TestPlanApply_EvalNodePlan_Node_Disconnected(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	node := mock.Node()
	node.Status = structs.NodeStatusDisconnected
	_ = state.UpsertNode(structs.MsgTypeTestSetup, 1000, node)
	snap, _ := state.Snapshot()

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
			require.NoError(t, err)
			require.Equal(t, tc.expectedFit, fit)
			require.Equal(t, tc.expectedReason, reason)
		})
	}
}
