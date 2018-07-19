package interfaces

import (
	"github.com/hashicorp/nomad/client/allocrunnerv2/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// AllocRunner is the interface for an allocation runner.
type AllocRunner interface {
	// ID returns the ID of the allocation being run.
	ID() string

	// Run starts the runner and begins executing all the tasks as part of the
	// allocation.
	Run()

	// State returns a copy of the runners state object
	State() *state.State

	TaskStateHandler
}

// TaskStateHandler exposes a handler to be called when a task's state changes
type TaskStateHandler interface {
	// TaskStateUpdated is used to emit updated task state
	TaskStateUpdated(task string, state *structs.TaskState)
}
