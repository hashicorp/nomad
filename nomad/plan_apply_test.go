package nomad

import (
	"testing"

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
	var resp structs.GenericResponse
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
	node := mockNode()
	testRegisterNode(t, s1, node)

	// Register alloc
	alloc := mockAlloc()
	plan := &structs.PlanResult{
		NodeEvict: map[string][]string{
			node.ID: []string{},
		},
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: []*structs.Allocation{alloc},
		},
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

	// Evict alloc, Register alloc2
	alloc2 := mockAlloc()
	plan = &structs.PlanResult{
		NodeEvict: map[string][]string{
			node.ID: []string{alloc.ID},
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
	if out != nil {
		t.Fatalf("should be missing alloc")
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

func TestPlanApply_EvalNodePlan_Simple(t *testing.T) {
	state := testStateStore(t)
	node := mockNode()
	state.RegisterNode(1000, node)
	snap, _ := state.Snapshot()

	alloc := mockAlloc()
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
	node := mockNode()
	node.Status = structs.NodeStatusInit
	state.RegisterNode(1000, node)
	snap, _ := state.Snapshot()

	alloc := mockAlloc()
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
	alloc := mockAlloc()
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
	alloc := mockAlloc()
	state := testStateStore(t)
	node := mockNode()
	alloc.NodeID = node.ID
	node.Resources = alloc.Resources
	node.Reserved = nil
	state.RegisterNode(1000, node)
	state.UpdateAllocations(1001, nil,
		[]*structs.Allocation{alloc})
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
	if fit {
		t.Fatalf("bad")
	}
}

func TestPlanApply_EvalNodePlan_NodeFull_Evict(t *testing.T) {
	alloc := mockAlloc()
	state := testStateStore(t)
	node := mockNode()
	alloc.NodeID = node.ID
	node.Resources = alloc.Resources
	node.Reserved = nil
	state.RegisterNode(1000, node)
	state.UpdateAllocations(1001, nil,
		[]*structs.Allocation{alloc})
	snap, _ := state.Snapshot()

	alloc2 := mockAlloc()
	plan := &structs.Plan{
		NodeEvict: map[string][]string{
			node.ID: []string{alloc.ID},
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
