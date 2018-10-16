package state

import (
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// StateDB implementations store and load Nomad client state.
type StateDB interface {
	GetAllAllocations() ([]*structs.Allocation, map[string]error, error)
	PutAllocation(*structs.Allocation) error
	GetTaskRunnerState(allocID, taskName string) (*state.LocalState, *structs.TaskState, error)
	PutTaskRunnerLocalState(allocID, taskName string, val *state.LocalState) error
	PutTaskState(allocID, taskName string, state *structs.TaskState) error
	DeleteTaskBucket(allocID, taskName string) error
	DeleteAllocationBucket(allocID string) error
	Close() error
}
