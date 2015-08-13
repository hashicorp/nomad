package scheduler

import (
	"os"
	"testing"

	"github.com/hashicorp/nomad/nomad/state"
)

func testContext(t *testing.T) (*state.StateStore, *EvalContext) {
	state, err := state.NewStateStore(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ctx := NewEvalContext(state)
	return state, ctx
}
