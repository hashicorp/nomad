package scheduler

import (
	"log"
	"os"
	"testing"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

func testContext(t *testing.T) (*state.StateStore, *EvalContext) {
	state, err := state.NewStateStore(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	plan := &structs.Plan{
		NodeUpdate:     make(map[string][]*structs.Allocation),
		NodeAllocation: make(map[string][]*structs.Allocation),
	}

	logger := log.New(os.Stderr, "", log.LstdFlags)

	ctx := NewEvalContext(state, plan, logger)
	return state, ctx
}

func TestEvalContext_ProposedAlloc(t *testing.T) {
	state, ctx := testContext(t)
	nodes := []*RankedNode{
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				ID: structs.GenerateUUID(),
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
			},
		},
		&RankedNode{
			Node: &structs.Node{
				// Perfect fit
				ID: structs.GenerateUUID(),
				Resources: &structs.Resources{
					CPU:      2048,
					MemoryMB: 2048,
				},
			},
		},
	}

	// Add existing allocations
	alloc1 := &structs.Allocation{
		ID:     structs.GenerateUUID(),
		EvalID: structs.GenerateUUID(),
		NodeID: nodes[0].Node.ID,
		JobID:  structs.GenerateUUID(),
		Resources: &structs.Resources{
			CPU:      2048,
			MemoryMB: 2048,
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
	}
	alloc2 := &structs.Allocation{
		ID:     structs.GenerateUUID(),
		EvalID: structs.GenerateUUID(),
		NodeID: nodes[1].Node.ID,
		JobID:  structs.GenerateUUID(),
		Resources: &structs.Resources{
			CPU:      1024,
			MemoryMB: 1024,
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
	}
	noErr(t, state.UpsertAllocs(1000, []*structs.Allocation{alloc1, alloc2}))

	// Add a planned eviction to alloc1
	plan := ctx.Plan()
	plan.NodeUpdate[nodes[0].Node.ID] = []*structs.Allocation{alloc1}

	// Add a planned placement to node1
	plan.NodeAllocation[nodes[1].Node.ID] = []*structs.Allocation{
		&structs.Allocation{
			Resources: &structs.Resources{
				CPU:      1024,
				MemoryMB: 1024,
			},
		},
	}

	proposed, err := ctx.ProposedAllocs(nodes[0].Node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(proposed) != 0 {
		t.Fatalf("bad: %#v", proposed)
	}

	proposed, err = ctx.ProposedAllocs(nodes[1].Node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(proposed) != 2 {
		t.Fatalf("bad: %#v", proposed)
	}
}
