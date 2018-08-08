package state

import (
	"github.com/hashicorp/nomad/client/allocrunnerv2/taskrunner/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// StateDB implementations store and load Nomad client state.
type StateDB interface {
	GetAllAllocations() ([]*structs.Allocation, map[string]error, error)
	PutAllocation(*structs.Allocation) error
	GetTaskRunnerState(allocID, taskName string) (*state.LocalState, *structs.TaskState, error)
	PutTaskRunnerLocalState(allocID, taskName string, buf []byte) error
	PutTaskState(allocID, taskName string, state *structs.TaskState) error
	Close() error
}
