package state

import (
	"github.com/hashicorp/nomad/client/allocrunnerv2/taskrunner/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type noopDB struct{}

func (n noopDB) GetAllAllocations() ([]*structs.Allocation, map[string]error, error) {
	return nil, nil, nil
}

func (n noopDB) PutAllocation(*structs.Allocation) error {
	return nil
}

func (n noopDB) GetTaskRunnerState(allocID string, taskName string) (*state.LocalState, *structs.TaskState, error) {
	return nil, nil, nil
}

func (n noopDB) PutTaskRunnerLocalState(allocID string, taskName string, buf []byte) error {
	return nil
}

func (n noopDB) PutTaskState(allocID string, taskName string, state *structs.TaskState) error {
	return nil
}

func (n noopDB) Close() error {
	return nil
}
