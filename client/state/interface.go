// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	arstate "github.com/hashicorp/nomad/client/allocrunner/state"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	dmstate "github.com/hashicorp/nomad/client/devicemanager/state"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	driverstate "github.com/hashicorp/nomad/client/pluginmanager/drivermanager/state"
	"github.com/hashicorp/nomad/client/serviceregistration/checks"
	cstructs "github.com/hashicorp/nomad/client/structs"
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

	// GetDeploymentStatus gets the allocation's deployment status. It may be nil.
	GetDeploymentStatus(allocID string) (*structs.AllocDeploymentStatus, error)

	// PutDeploymentStatus sets the allocation's deployment status. It may be nil.
	PutDeploymentStatus(allocID string, ds *structs.AllocDeploymentStatus) error

	// GetNetworkStatus gets the allocation's network status. It may be nil.
	GetNetworkStatus(allocID string) (*structs.AllocNetworkStatus, error)

	// PutNetworkStatus puts the allocation's network status. It may be nil.
	PutNetworkStatus(allocID string, ns *structs.AllocNetworkStatus, opts ...WriteOption) error

	// PutAcknowledgedState stores an allocation's last acknowledged state or
	// returns an error if it could not be stored.
	PutAcknowledgedState(string, *arstate.State, ...WriteOption) error

	// GetAcknowledgedState retrieves an allocation's last acknowledged
	// state. It may be nil even if there's no error
	GetAcknowledgedState(string) (*arstate.State, error)

	// PutAllocVolumes stores stubs of an allocation's dynamic volume mounts so
	// they can be restored.
	PutAllocVolumes(allocID string, state *arstate.AllocVolumes, opts ...WriteOption) error

	// GetAllocVolumes retrieves stubs of an allocation's dynamic volume mounts
	// so they can be restored.
	GetAllocVolumes(allocID string) (*arstate.AllocVolumes, error)

	// GetTaskRunnerState returns the LocalState and TaskState for a
	// TaskRunner. Either state may be nil if it is not found, but if an
	// error is encountered only the error will be non-nil.
	GetTaskRunnerState(allocID, taskName string) (*state.LocalState, *structs.TaskState, error)

	// PutTaskRunnerLocalState stores the LocalState for a TaskRunner or
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

	// PutCheckResult sets the query result for the check implied in qr.
	PutCheckResult(allocID string, qr *structs.CheckQueryResult) error

	// DeleteCheckResults removes the given set of check results.
	DeleteCheckResults(allocID string, checkIDs []structs.CheckID) error

	// PurgeCheckResults removes all check results of the given allocation.
	PurgeCheckResults(allocID string) error

	// GetCheckResults is used to restore the set of check results on this Client.
	GetCheckResults() (checks.ClientResults, error)

	// PutNodeMeta sets dynamic node metadata for merging with the copy from the
	// Client's config.
	//
	// This overwrites existing dynamic node metadata entirely.
	PutNodeMeta(map[string]*string) error

	// GetNodeMeta retrieves node metadata for merging with the copy from
	// the Client's config.
	GetNodeMeta() (map[string]*string, error)

	PutNodeRegistration(*cstructs.NodeRegistration) error
	GetNodeRegistration() (*cstructs.NodeRegistration, error)

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
