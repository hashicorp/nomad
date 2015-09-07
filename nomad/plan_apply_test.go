package nomad

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func testRegisterNode(t *testing.T, s *Server, n *structs.Node) {
	// Create the register request
	req := &structs.NodeRegisterRequest{
		Node:         n,
		WriteRequest: structs.WriteRequest{Region: "region1"},
	}

	// Fetch the response
	var resp structs.NodeUpdateResponse
	if err := s.RPC("Client.Register", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Index == 0 {
		t.Fatalf("bad index: %d", resp.Index)
	}
}

func TestPlanApply_applyPlan(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	testutil.WaitForLeader(t, s1.RPC)

	// Register ndoe
	node := mock.Node()
	testRegisterNode(t, s1, node)

	// Register alloc
	alloc := mock.Alloc()
	allocFail := mock.Alloc()
	plan := &structs.PlanResult{
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: []*structs.Allocation{alloc},
		},
		FailedAllocs: []*structs.Allocation{allocFail},
	}

	// Apply the plan
	index, err := s1.applyPlan(plan)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index == 0 {
		t.Fatalf("bad: %d", index)
	}

	// Lookup the allocation
	out, err := s1.fsm.State().GetAllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("missing alloc")
	}

	// Lookup the allocation
	out, err = s1.fsm.State().GetAllocByID(allocFail.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("missing alloc")
	}

	// Evict alloc, Register alloc2
	allocEvict := new(structs.Allocation)
	*allocEvict = *alloc
	allocEvict.DesiredStatus = structs.AllocDesiredStatusEvict
	alloc2 := mock.Alloc()
	plan = &structs.PlanResult{
		NodeUpdate: map[string][]*structs.Allocation{
			node.ID: []*structs.Allocation{allocEvict},
		},
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: []*structs.Allocation{alloc2},
		},
	}

	// Apply the plan
	index, err = s1.applyPlan(plan)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index == 0 {
		t.Fatalf("bad: %d", index)
	}

	// Lookup the allocation
	out, err = s1.fsm.State().GetAllocByID(alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.DesiredStatus != structs.AllocDesiredStatusEvict {
		t.Fatalf("should be evicted alloc: %#v", out)
	}

	// Lookup the allocation
	out, err = s1.fsm.State().GetAllocByID(alloc2.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out == nil {
		t.Fatalf("missing alloc")
	}
}

func TestPlanApply_EvalPlan_Simple(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()
	state.RegisterNode(1000, node)
	snap, _ := state.Snapshot()

	alloc := mock.Alloc()
	allocFail := mock.Alloc()
	plan := &structs.Plan{
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: []*structs.Allocation{alloc},
		},
		FailedAllocs: []*structs.Allocation{allocFail},
	}

	result, err := evaluatePlan(snap, plan)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if result == nil {
		t.Fatalf("missing result")
	}
	if !reflect.DeepEqual(result.FailedAllocs, plan.FailedAllocs) {
		t.Fatalf("missing failed allocs")
	}
}

func TestPlanApply_EvalPlan_Partial(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()
	state.RegisterNode(1000, node)
	node2 := mock.Node()
	state.RegisterNode(1001, node2)
	snap, _ := state.Snapshot()

	alloc := mock.Alloc()
	alloc2 := mock.Alloc() // Ensure alloc2 does not fit
	alloc2.Resources = node2.Resources
	plan := &structs.Plan{
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID:  []*structs.Allocation{alloc},
			node2.ID: []*structs.Allocation{alloc2},
		},
	}

	result, err := evaluatePlan(snap, plan)
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
}

func TestPlanApply_EvalPlan_Partial_AllAtOnce(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()
	state.RegisterNode(1000, node)
	node2 := mock.Node()
	state.RegisterNode(1001, node2)
	snap, _ := state.Snapshot()

	alloc := mock.Alloc()
	alloc2 := mock.Alloc() // Ensure alloc2 does not fit
	alloc2.Resources = node2.Resources
	plan := &structs.Plan{
		AllAtOnce: true, // Require all to make progress
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID:  []*structs.Allocation{alloc},
			node2.ID: []*structs.Allocation{alloc2},
		},
	}

	result, err := evaluatePlan(snap, plan)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if result == nil {
		t.Fatalf("missing result")
	}

	if len(result.NodeAllocation) != 0 {
		t.Fatalf("should not alloc: %v", result.NodeAllocation)
	}
}

func TestPlanApply_EvalNodePlan_Simple(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()
	state.RegisterNode(1000, node)
	snap, _ := state.Snapshot()

	alloc := mock.Alloc()
	plan := &structs.Plan{
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: []*structs.Allocation{alloc},
		},
	}

	fit, err := evaluateNodePlan(snap, plan, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !fit {
		t.Fatalf("bad")
	}
}

func TestPlanApply_EvalNodePlan_NodeNotReady(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()
	node.Status = structs.NodeStatusInit
	state.RegisterNode(1000, node)
	snap, _ := state.Snapshot()

	alloc := mock.Alloc()
	plan := &structs.Plan{
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: []*structs.Allocation{alloc},
		},
	}

	fit, err := evaluateNodePlan(snap, plan, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fit {
		t.Fatalf("bad")
	}
}

func TestPlanApply_EvalNodePlan_NodeDrain(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()
	node.Drain = true
	state.RegisterNode(1000, node)
	snap, _ := state.Snapshot()

	alloc := mock.Alloc()
	plan := &structs.Plan{
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: []*structs.Allocation{alloc},
		},
	}

	fit, err := evaluateNodePlan(snap, plan, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fit {
		t.Fatalf("bad")
	}
}

func TestPlanApply_EvalNodePlan_NodeNotExist(t *testing.T) {
	state := testStateStore(t)
	snap, _ := state.Snapshot()

	nodeID := "foo"
	alloc := mock.Alloc()
	plan := &structs.Plan{
		NodeAllocation: map[string][]*structs.Allocation{
			nodeID: []*structs.Allocation{alloc},
		},
	}

	fit, err := evaluateNodePlan(snap, plan, nodeID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fit {
		t.Fatalf("bad")
	}
}

func TestPlanApply_EvalNodePlan_NodeFull(t *testing.T) {
	alloc := mock.Alloc()
	state := testStateStore(t)
	node := mock.Node()
	alloc.NodeID = node.ID
	node.Resources = alloc.Resources
	node.Reserved = nil
	state.RegisterNode(1000, node)
	state.UpdateAllocations(1001, []*structs.Allocation{alloc})
	snap, _ := state.Snapshot()

	alloc2 := mock.Alloc()
	alloc2.NodeID = node.ID
	plan := &structs.Plan{
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: []*structs.Allocation{alloc2},
		},
	}

	fit, err := evaluateNodePlan(snap, plan, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fit {
		t.Fatalf("bad")
	}
}

func TestPlanApply_EvalNodePlan_UpdateExisting(t *testing.T) {
	alloc := mock.Alloc()
	state := testStateStore(t)
	node := mock.Node()
	alloc.NodeID = node.ID
	node.Resources = alloc.Resources
	node.Reserved = nil
	state.RegisterNode(1000, node)
	state.UpdateAllocations(1001, []*structs.Allocation{alloc})
	snap, _ := state.Snapshot()

	plan := &structs.Plan{
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: []*structs.Allocation{alloc},
		},
	}

	fit, err := evaluateNodePlan(snap, plan, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !fit {
		t.Fatalf("bad")
	}
}

func TestPlanApply_EvalNodePlan_NodeFull_Evict(t *testing.T) {
	alloc := mock.Alloc()
	state := testStateStore(t)
	node := mock.Node()
	alloc.NodeID = node.ID
	node.Resources = alloc.Resources
	node.Reserved = nil
	state.RegisterNode(1000, node)
	state.UpdateAllocations(1001, []*structs.Allocation{alloc})
	snap, _ := state.Snapshot()

	allocEvict := new(structs.Allocation)
	*allocEvict = *alloc
	allocEvict.DesiredStatus = structs.AllocDesiredStatusEvict
	alloc2 := mock.Alloc()
	plan := &structs.Plan{
		NodeUpdate: map[string][]*structs.Allocation{
			node.ID: []*structs.Allocation{allocEvict},
		},
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: []*structs.Allocation{alloc2},
		},
	}

	fit, err := evaluateNodePlan(snap, plan, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !fit {
		t.Fatalf("bad")
	}
}

func TestPlanApply_EvalNodePlan_NodeFull_AllocEvict(t *testing.T) {
	alloc := mock.Alloc()
	state := testStateStore(t)
	node := mock.Node()
	alloc.NodeID = node.ID
	alloc.DesiredStatus = structs.AllocDesiredStatusEvict
	node.Resources = alloc.Resources
	node.Reserved = nil
	state.RegisterNode(1000, node)
	state.UpdateAllocations(1001, []*structs.Allocation{alloc})
	snap, _ := state.Snapshot()

	alloc2 := mock.Alloc()
	plan := &structs.Plan{
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: []*structs.Allocation{alloc2},
		},
	}

	fit, err := evaluateNodePlan(snap, plan, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !fit {
		t.Fatalf("bad")
	}
}

func TestPlanApply_EvalNodePlan_NodeDown_EvictOnly(t *testing.T) {
	alloc := mock.Alloc()
	state := testStateStore(t)
	node := mock.Node()
	alloc.NodeID = node.ID
	node.Resources = alloc.Resources
	node.Reserved = nil
	node.Status = structs.NodeStatusDown
	state.RegisterNode(1000, node)
	state.UpdateAllocations(1001, []*structs.Allocation{alloc})
	snap, _ := state.Snapshot()

	allocEvict := new(structs.Allocation)
	*allocEvict = *alloc
	allocEvict.DesiredStatus = structs.AllocDesiredStatusEvict
	plan := &structs.Plan{
		NodeUpdate: map[string][]*structs.Allocation{
			node.ID: []*structs.Allocation{allocEvict},
		},
	}

	fit, err := evaluateNodePlan(snap, plan, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !fit {
		t.Fatalf("bad")
	}
}
