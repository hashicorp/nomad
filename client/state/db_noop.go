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

// NoopDB implements a StateDB that does not persist any data.
type NoopDB struct{}

func (n NoopDB) Name() string {
	return "noopdb"
}

func (n NoopDB) Upgrade() error {
	return nil
}

func (n NoopDB) GetAllAllocations() ([]*structs.Allocation, map[string]error, error) {
	return nil, nil, nil
}

func (n NoopDB) PutAllocation(alloc *structs.Allocation, opts ...WriteOption) error {
	return nil
}

func (n NoopDB) GetDeploymentStatus(allocID string) (*structs.AllocDeploymentStatus, error) {
	return nil, nil
}

func (n NoopDB) PutDeploymentStatus(allocID string, ds *structs.AllocDeploymentStatus) error {
	return nil
}

func (n NoopDB) GetNetworkStatus(allocID string) (*structs.AllocNetworkStatus, error) {
	return nil, nil
}

func (n NoopDB) PutNetworkStatus(allocID string, ds *structs.AllocNetworkStatus, opts ...WriteOption) error {
	return nil
}

func (n NoopDB) PutAcknowledgedState(allocID string, state *arstate.State, opts ...WriteOption) error {
	return nil
}

func (n NoopDB) GetAcknowledgedState(allocID string) (*arstate.State, error) { return nil, nil }

func (n NoopDB) PutAllocVolumes(allocID string, state *arstate.AllocVolumes, opts ...WriteOption) error {
	return nil
}

func (n NoopDB) GetAllocVolumes(allocID string) (*arstate.AllocVolumes, error) { return nil, nil }

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

func (n NoopDB) DeleteAllocationBucket(allocID string, opts ...WriteOption) error {
	return nil
}

func (n NoopDB) PutDevicePluginState(ps *dmstate.PluginState) error {
	return nil
}

func (n NoopDB) GetDevicePluginState() (*dmstate.PluginState, error) {
	return nil, nil
}

func (n NoopDB) PutDriverPluginState(ps *driverstate.PluginState) error {
	return nil
}

func (n NoopDB) GetDriverPluginState() (*driverstate.PluginState, error) {
	return nil, nil
}

func (n NoopDB) PutDynamicPluginRegistryState(ps *dynamicplugins.RegistryState) error {
	return nil
}

func (n NoopDB) GetDynamicPluginRegistryState() (*dynamicplugins.RegistryState, error) {
	return nil, nil
}

func (n NoopDB) PutCheckResult(allocID string, qr *structs.CheckQueryResult) error {
	return nil
}

func (n NoopDB) GetCheckResults() (checks.ClientResults, error) {
	return nil, nil
}

func (n NoopDB) DeleteCheckResults(allocID string, checkIDs []structs.CheckID) error {
	return nil
}

func (n NoopDB) PurgeCheckResults(allocID string) error {
	return nil
}

func (n NoopDB) PutNodeMeta(map[string]*string) error {
	return nil
}

func (n NoopDB) GetNodeMeta() (map[string]*string, error) {
	return nil, nil
}

func (n NoopDB) PutNodeRegistration(reg *cstructs.NodeRegistration) error {
	return nil
}

func (n NoopDB) GetNodeRegistration() (*cstructs.NodeRegistration, error) {
	return nil, nil
}

func (n NoopDB) Close() error {
	return nil
}
