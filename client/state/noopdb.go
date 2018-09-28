package state

import (
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	"github.com/hashicorp/nomad/client/devicemanager"
	"github.com/hashicorp/nomad/nomad/structs"
)

// NoopDB implements a StateDB that does not persist any data.
type NoopDB struct{}

func (n NoopDB) Name() string {
	return "noopdb"
}

func (n NoopDB) GetAllAllocations() ([]*structs.Allocation, map[string]error, error) {
	return nil, nil, nil
}

func (n NoopDB) PutAllocation(*structs.Allocation) error {
	return nil
}

func (n NoopDB) GetTaskRunnerState(allocID string, taskName string) (*state.LocalState, *structs.TaskState, error) {
	return nil, nil, nil
}

func (n NoopDB) PutTaskRunnerLocalState(allocID string, taskName string, val *state.LocalState) error {
	return nil
}

func (n NoopDB) PutTaskState(allocID string, taskName string, state *structs.TaskState) error {
	return nil
}

func (n NoopDB) DeleteTaskBucket(allocID, taskName string) error {
	return nil
}

func (n NoopDB) DeleteAllocationBucket(allocID string) error {
	return nil
}

func (n NoopDB) PutDevicePluginState(ps *devicemanager.PluginState) error {
	return nil
}

func (n NoopDB) GetDevicePluginState() (*devicemanager.PluginState, error) {
	return nil, nil
}

func (n NoopDB) Close() error {
	return nil
}
