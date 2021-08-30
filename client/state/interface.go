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

	// PutAllocation stores an allocation or returns an error if it could
	// not be stored.
	PutAllocation(*structs.Allocation, ...WriteOption) error

	// Get/Put DeploymentStatus get and put the allocation's deployment
	// status. It may be nil.
	GetDeploymentStatus(allocID string) (*structs.AllocDeploymentStatus, error)
	PutDeploymentStatus(allocID string, ds *structs.AllocDeploymentStatus) error

	// Get/Put NetworkStatus get and put the allocation's network
	// status. It may be nil.
	GetNetworkStatus(allocID string) (*structs.AllocNetworkStatus, error)
	PutNetworkStatus(allocID string, ns *structs.AllocNetworkStatus, opts ...WriteOption) error

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
	DeleteAllocationBucket(allocID string, opts ...WriteOption) error

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

	// PutDynamicPluginRegistryState is used to store the dynamic plugin manager's state.
	PutDynamicPluginRegistryState(state *dynamicplugins.RegistryState) error

	// Close the database. Unsafe for further use after calling regardless
	// of return value.
	Close() error
}

// WriteOptions adjusts the way the data is persisted by the StateDB above. Default is
// zero/false values for all fields. To provide different values, use With* functions
// below, like this: statedb.PutAllocation(alloc, WithBatchMode())
type WriteOptions struct {
	// In Batch mode, concurrent writes (Put* and Delete* operations above) are
	// coalesced into a single transaction, increasing write performance. To benefit
	// from this mode, writes must happen concurrently in goroutines, as every write
	// request still waits for the shared transaction to commit before returning.
	// See https://github.com/boltdb/bolt#batch-read-write-transactions for details.
	// This mode is only supported for BoltDB state backend and is ignored in other backends.
	BatchMode bool
}

// WriteOption is a function that modifies WriteOptions struct above.
type WriteOption func(*WriteOptions)

// mergeWriteOptions creates a final WriteOptions struct to be used by the write methods above
// from a list of WriteOption-s provided as variadic arguments.
func mergeWriteOptions(opts []WriteOption) WriteOptions {
	writeOptions := WriteOptions{} // Default WriteOptions is zero value.
	for _, opt := range opts {
		opt(&writeOptions)
	}
	return writeOptions
}

// WithBatchMode enables Batch mode for write requests (Put* and Delete*
// operations above).
func WithBatchMode() WriteOption {
	return func(s *WriteOptions) {
		s.BatchMode = true
	}
}
