package nomad

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestEvaluatePool(t *testing.T) {
	state := testStateStore(t)
	node := mock.Node()
	state.UpsertNode(1000, node)
	snap, _ := state.Snapshot()

	alloc := mock.Alloc()
	plan := &structs.Plan{
		NodeAllocation: map[string][]*structs.Allocation{
			node.ID: []*structs.Allocation{alloc},
		},
	}

	pool := NewEvaluatePool(1, 4)
	defer pool.Shutdown()

	// Push a request
	req := pool.RequestCh()
	req <- evaluateRequest{snap, plan, node.ID}

	// Get the response
	res := <-pool.ResultCh()

	// Verify response
	if res.err != nil {
		t.Fatalf("err: %v", res.err)
	}
	if !res.fit {
		t.Fatalf("bad")
	}
}
