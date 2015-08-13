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
	plan := new(structs.Plan)

	logger := log.New(os.Stderr, "", log.LstdFlags)

	ctx := NewEvalContext(state, plan, logger)
	return state, ctx
}
