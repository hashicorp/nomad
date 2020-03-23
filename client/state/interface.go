package state

import (
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	dmstate "github.com/hashicorp/nomad/client/devicemanager/state"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	driverstate "github.com/hashicorp/nomad/client/pluginmanager/drivermanager/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// StateDB implementations store and load Nomad client state.
type StateDB interface {
	// Name of implementation.
	Name() string

	// Upgrade ensures the layout of the database is at the latest version
	// or returns an error. Corrupt data will be dropped when possible.
	// Errors should be considered critical and unrecoverable.
	Upgrade() error

	// GetAllAllocations returns all valid allocations and a map of
	// allocation IDs to retrieval errors.
	//
	// If a single error is returned then both allocations and the map will be nil.
	GetAllAllocations() ([]*structs.Allocation, map[string]error, error)

	// PulAllocation stores an allocation or returns an error if it could
	// not be stored.
	PutAllocation(*structs.Allocation) error

	// Get/Put DeploymentStatus get and put the allocation's deployment
	// status. It may be nil.
	GetDeploymentStatus(allocID string) (*structs.AllocDeploymentStatus, error)
	PutDeploymentStatus(allocID string, ds *structs.AllocDeploymentStatus) error

	// GetTaskRunnerState returns the LocalState and TaskState for a
	// TaskRunner. Either state may be nil if it is not found, but if an
	// error is encountered only the error will be non-nil.
	GetTaskRunnerState(allocID, taskName string) (*state.LocalState, *structs.TaskState, error)

	// PutTaskRunnerLocalTask stores the LocalState for a TaskRunner or
	// returns an error.
	PutTaskRunnerLocalState(allocID, taskName string, val *state.LocalState) error

	// PutTaskState stores the TaskState for a TaskRunner or returns an
	// error.
	PutTaskState(allocID, taskName string, state *structs.TaskState) error

	// DeleteTaskBucket deletes a task's state bucket if it exists. No
	// error is returned if it does not exist.
	DeleteTaskBucket(allocID, taskName string) error

	// DeleteAllocationBucket deletes an allocation's state bucket if it
	// exists. No error is returned if it does not exist.
	DeleteAllocationBucket(allocID string) error

	// GetDevicePluginState is used to retrieve the device manager's plugin
	// state.
	GetDevicePluginState() (*dmstate.PluginState, error)

	// PutDevicePluginState is used to store the device manager's plugin
	// state.
	PutDevicePluginState(state *dmstate.PluginState) error

	// GetDriverPluginState is used to retrieve the driver manager's plugin
	// state.
	GetDriverPluginState() (*driverstate.PluginState, error)

	// PutDriverPluginState is used to store the driver manager's plugin
	// state.
	PutDriverPluginState(state *driverstate.PluginState) error

	// GetDynamicPluginRegistryState is used to retrieve a dynamic plugin manager's state.
	GetDynamicPluginRegistryState() (*dynamicplugins.RegistryState, error)

	// PutDynamicPluginRegistryState is used to store the dynamic plugin managers's state.
	PutDynamicPluginRegistryState(state *dynamicplugins.RegistryState) error

	// Close the database. Unsafe for further use after calling regardless
	// of return value.
	Close() error
}
