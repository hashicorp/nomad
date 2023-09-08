// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"

	arstate "github.com/hashicorp/nomad/client/allocrunner/state"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	dmstate "github.com/hashicorp/nomad/client/devicemanager/state"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	driverstate "github.com/hashicorp/nomad/client/pluginmanager/drivermanager/state"
	"github.com/hashicorp/nomad/client/serviceregistration/checks"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ErrDB implements a StateDB that returns errors on restore methods, used for testing
type ErrDB struct {
	// Allocs is a preset slice of allocations used in GetAllAllocations
	Allocs []*structs.Allocation
}

func (m *ErrDB) Name() string {
	return "errdb"
}

func (m *ErrDB) Upgrade() error {
	return nil
}

func (m *ErrDB) GetAllAllocations() ([]*structs.Allocation, map[string]error, error) {
	return m.Allocs, nil, nil
}

func (m *ErrDB) PutAllocation(alloc *structs.Allocation, opts ...WriteOption) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) GetDeploymentStatus(allocID string) (*structs.AllocDeploymentStatus, error) {
	return nil, fmt.Errorf("Error!")
}

func (m *ErrDB) PutDeploymentStatus(allocID string, ds *structs.AllocDeploymentStatus) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) GetNetworkStatus(allocID string) (*structs.AllocNetworkStatus, error) {
	return nil, fmt.Errorf("Error!")
}

func (m *ErrDB) PutNetworkStatus(allocID string, ns *structs.AllocNetworkStatus, opts ...WriteOption) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) PutAcknowledgedState(allocID string, state *arstate.State, opts ...WriteOption) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) GetAcknowledgedState(allocID string) (*arstate.State, error) {
	return nil, fmt.Errorf("Error!")
}

func (m *ErrDB) PutAllocVolumes(allocID string, state *arstate.AllocVolumes, opts ...WriteOption) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) GetAllocVolumes(allocID string) (*arstate.AllocVolumes, error) {
	return nil, fmt.Errorf("Error!")
}

func (m *ErrDB) GetTaskRunnerState(allocID string, taskName string) (*state.LocalState, *structs.TaskState, error) {
	return nil, nil, fmt.Errorf("Error!")
}

func (m *ErrDB) PutTaskRunnerLocalState(allocID string, taskName string, val *state.LocalState) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) PutTaskState(allocID string, taskName string, state *structs.TaskState) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) DeleteTaskBucket(allocID, taskName string) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) DeleteAllocationBucket(allocID string, opts ...WriteOption) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) PutDevicePluginState(ps *dmstate.PluginState) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) GetDynamicPluginRegistryState() (*dynamicplugins.RegistryState, error) {
	return nil, fmt.Errorf("Error!")
}

func (m *ErrDB) PutDynamicPluginRegistryState(state *dynamicplugins.RegistryState) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) GetDevicePluginState() (*dmstate.PluginState, error) {
	return nil, fmt.Errorf("Error!")
}

func (m *ErrDB) GetDriverPluginState() (*driverstate.PluginState, error) {
	return nil, fmt.Errorf("Error!")
}

func (m *ErrDB) PutDriverPluginState(ps *driverstate.PluginState) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) PutCheckResult(allocID string, qr *structs.CheckQueryResult) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) GetCheckResults() (checks.ClientResults, error) {
	return nil, fmt.Errorf("Error!")
}

func (m *ErrDB) DeleteCheckResults(allocID string, checkIDs []structs.CheckID) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) PurgeCheckResults(allocID string) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) PutNodeMeta(map[string]*string) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) GetNodeMeta() (map[string]*string, error) {
	return nil, fmt.Errorf("Error!")
}

func (m *ErrDB) PutNodeRegistration(reg *cstructs.NodeRegistration) error {
	return fmt.Errorf("Error!")
}

func (m *ErrDB) GetNodeRegistration() (*cstructs.NodeRegistration, error) {
	return nil, fmt.Errorf("Error!")
}

func (m *ErrDB) Close() error {
	return fmt.Errorf("Error!")
}
