package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
)

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
