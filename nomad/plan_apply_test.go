package nomad

import (
	"reflect"
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
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

func TestPlanApply_applyPlan(t *testing.T) {
	t.Parallel()
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
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
	if err := s1.State().UpsertEvals(1, []*structs.Evaluation{eval}); err != nil {
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
	assert.Equal(structs.AllocDesiredStatusEvict, out.DesiredStatus)

	// Verify plan applies cleanly
	index, err = planWaitFuture(future)
	assert.Nil(err)
	assert.NotEqual(0, index)

	// Lookup the allocation
	allocOut, err = s1.fsm.State().AllocByID(ws, alloc.ID)
	assert.Nil(err)
	assert.Equal(structs.AllocDesiredStatusEvict, allocOut.DesiredStatus)
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

func TestPlanApply_EvalPlan_Simple(t *testing.T) {
	t.Parallel()
	state := testStateStore(t)
	node := mock.Node()
	state.UpsertNode(1000, node)
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

	result, err := evaluatePlan(pool, snap, plan, testlog.Logger(t))
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

func TestPlanApply_EvalPlan_Partial(t *testing.T) {
	t.Parallel()
	state := testStateStore(t)
	node := mock.Node()
	state.UpsertNode(1000, node)
	node2 := mock.Node()
	state.UpsertNode(1001, node2)
	snap, _ := state.Snapshot()

	alloc := mock.Alloc()
	alloc2 := mock.Alloc() // Ensure alloc2 does not fit
	alloc2.Resources = node2.Resources

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

	result, err := evaluatePlan(pool, snap, plan, testlog.Logger(t))
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
	t.Parallel()
	state := testStateStore(t)
	node := mock.Node()
	state.UpsertNode(1000, node)
	node2 := mock.Node()
	state.UpsertNode(1001, node2)
	snap, _ := state.Snapshot()

	alloc := mock.Alloc()
	alloc2 := mock.Alloc() // Ensure alloc2 does not fit
	alloc2.Resources = node2.Resources
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

	result, err := evaluatePlan(pool, snap, plan, testlog.Logger(t))
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
	t.Parallel()
	state := testStateStore(t)
	node := mock.Node()
	state.UpsertNode(1000, node)
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
	t.Parallel()
	state := testStateStore(t)
	node := mock.Node()
	node.Status = structs.NodeStatusInit
	state.UpsertNode(1000, node)
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
	t.Parallel()
	state := testStateStore(t)
	node := mock.Node()
	node.Drain = true
	state.UpsertNode(1000, node)
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
	t.Parallel()
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
	t.Parallel()
	alloc := mock.Alloc()
	state := testStateStore(t)
	node := mock.Node()
	alloc.NodeID = node.ID
	node.Resources = alloc.Resources
	node.Reserved = nil
	state.UpsertJobSummary(999, mock.JobSummary(alloc.JobID))
	state.UpsertNode(1000, node)
	state.UpsertAllocs(1001, []*structs.Allocation{alloc})

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

func TestPlanApply_EvalNodePlan_UpdateExisting(t *testing.T) {
	t.Parallel()
	alloc := mock.Alloc()
	state := testStateStore(t)
	node := mock.Node()
	alloc.NodeID = node.ID
	node.Resources = alloc.Resources
	node.Reserved = nil
	state.UpsertNode(1000, node)
	state.UpsertAllocs(1001, []*structs.Allocation{alloc})
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
	t.Parallel()
	alloc := mock.Alloc()
	state := testStateStore(t)
	node := mock.Node()
	alloc.NodeID = node.ID
	node.Resources = alloc.Resources
	node.Reserved = nil
	state.UpsertNode(1000, node)
	state.UpsertAllocs(1001, []*structs.Allocation{alloc})
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
	t.Parallel()
	alloc := mock.Alloc()
	state := testStateStore(t)
	node := mock.Node()
	alloc.NodeID = node.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusEvict
	node.Resources = alloc.Resources
	node.Reserved = nil
	state.UpsertNode(1000, node)
	state.UpsertAllocs(1001, []*structs.Allocation{alloc})
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
	t.Parallel()
	alloc := mock.Alloc()
	state := testStateStore(t)
	node := mock.Node()
	alloc.NodeID = node.ID
	node.Resources = alloc.Resources
	node.Reserved = nil
	node.Status = structs.NodeStatusDown
	state.UpsertNode(1000, node)
	state.UpsertAllocs(1001, []*structs.Allocation{alloc})
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
