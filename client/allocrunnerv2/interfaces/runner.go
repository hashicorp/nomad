package interfaces

import "github.com/hashicorp/nomad/client/allocrunnerv2/state"

// AllocRunner is the interface for an allocation runner.
type AllocRunner interface {
	// ID returns the ID of the allocation being run.
	ID() string

	// Run starts the runner and begins executing all the tasks as part of the
	// allocation.
	Run()

	// State returns a copy of the runners state object
	State() *state.State
}

// TaskRunner is the interface for a task runner.
type TaskRunner interface {
}
