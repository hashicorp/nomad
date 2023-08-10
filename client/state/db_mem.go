// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"sync"

	"github.com/hashicorp/go-hclog"
	arstate "github.com/hashicorp/nomad/client/allocrunner/state"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	dmstate "github.com/hashicorp/nomad/client/devicemanager/state"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	driverstate "github.com/hashicorp/nomad/client/pluginmanager/drivermanager/state"
	"github.com/hashicorp/nomad/client/serviceregistration/checks"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/exp/maps"
)

// MemDB implements a StateDB that stores data in memory and should only be
// used for testing. All methods are safe for concurrent use.
type MemDB struct {
	// alloc_id -> value
	allocs map[string]*structs.Allocation

	// alloc_id -> value
	deployStatus map[string]*structs.AllocDeploymentStatus

	// alloc_id -> value
	networkStatus map[string]*structs.AllocNetworkStatus

	// alloc_id -> value
	acknowledgedState map[string]*arstate.State

	// alloc_id -> value
	allocVolumeStates map[string]*arstate.AllocVolumes

	// alloc_id -> task_name -> value
	localTaskState map[string]map[string]*state.LocalState
	taskState      map[string]map[string]*structs.TaskState

	// alloc_id -> check_id -> result
	checks checks.ClientResults

	// devicemanager -> plugin-state
	devManagerPs *dmstate.PluginState

	// drivermanager -> plugin-state
	driverManagerPs *driverstate.PluginState

	// dynamicmanager -> registry-state
	dynamicManagerPs *dynamicplugins.RegistryState

	// key -> value or nil
	nodeMeta map[string]*string

	nodeRegistration *cstructs.NodeRegistration

	logger hclog.Logger

	mu sync.RWMutex
}

func NewMemDB(logger hclog.Logger) *MemDB {
	logger = logger.Named("memdb")
	return &MemDB{
		allocs:            make(map[string]*structs.Allocation),
		deployStatus:      make(map[string]*structs.AllocDeploymentStatus),
		networkStatus:     make(map[string]*structs.AllocNetworkStatus),
		acknowledgedState: make(map[string]*arstate.State),
		localTaskState:    make(map[string]map[string]*state.LocalState),
		taskState:         make(map[string]map[string]*structs.TaskState),
		checks:            make(checks.ClientResults),
		logger:            logger,
	}
}

func (m *MemDB) Name() string {
	return "memdb"
}

func (m *MemDB) Upgrade() error {
	return nil
}

func (m *MemDB) GetAllAllocations() ([]*structs.Allocation, map[string]error, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	allocs := make([]*structs.Allocation, 0, len(m.allocs))
	for _, v := range m.allocs {
		allocs = append(allocs, v)
	}

	return allocs, map[string]error{}, nil
}

func (m *MemDB) PutAllocation(alloc *structs.Allocation, _ ...WriteOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allocs[alloc.ID] = alloc
	return nil
}

func (m *MemDB) GetDeploymentStatus(allocID string) (*structs.AllocDeploymentStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.deployStatus[allocID], nil
}

func (m *MemDB) PutDeploymentStatus(allocID string, ds *structs.AllocDeploymentStatus) error {
	m.mu.Lock()
	m.deployStatus[allocID] = ds
	defer m.mu.Unlock()
	return nil
}

func (m *MemDB) GetNetworkStatus(allocID string) (*structs.AllocNetworkStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.networkStatus[allocID], nil
}

func (m *MemDB) PutNetworkStatus(allocID string, ns *structs.AllocNetworkStatus, _ ...WriteOption) error {
	m.mu.Lock()
	m.networkStatus[allocID] = ns
	defer m.mu.Unlock()
	return nil
}

func (m *MemDB) PutAcknowledgedState(allocID string, state *arstate.State, opts ...WriteOption) error {
	m.mu.Lock()
	m.acknowledgedState[allocID] = state
	defer m.mu.Unlock()
	return nil
}

func (m *MemDB) GetAcknowledgedState(allocID string) (*arstate.State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.acknowledgedState[allocID], nil
}

func (m *MemDB) PutAllocVolumes(allocID string, state *arstate.AllocVolumes, opts ...WriteOption) error {
	m.mu.Lock()
	m.allocVolumeStates[allocID] = state
	defer m.mu.Unlock()
	return nil
}

func (m *MemDB) GetAllocVolumes(allocID string) (*arstate.AllocVolumes, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.allocVolumeStates[allocID], nil
}

func (m *MemDB) GetTaskRunnerState(allocID string, taskName string) (*state.LocalState, *structs.TaskState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var ls *state.LocalState
	var ts *structs.TaskState

	// Local Task State
	allocLocalTS := m.localTaskState[allocID]
	if len(allocLocalTS) != 0 {
		ls = allocLocalTS[taskName]
	}

	// Task State
	allocTS := m.taskState[allocID]
	if len(allocTS) != 0 {
		ts = allocTS[taskName]
	}

	return ls, ts, nil
}

func (m *MemDB) PutTaskRunnerLocalState(allocID string, taskName string, val *state.LocalState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if alts, ok := m.localTaskState[allocID]; ok {
		alts[taskName] = val.Copy()
		return nil
	}

	m.localTaskState[allocID] = map[string]*state.LocalState{
		taskName: val.Copy(),
	}

	return nil
}

func (m *MemDB) PutTaskState(allocID string, taskName string, state *structs.TaskState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ats, ok := m.taskState[allocID]; ok {
		ats[taskName] = state.Copy()
		return nil
	}

	m.taskState[allocID] = map[string]*structs.TaskState{
		taskName: state.Copy(),
	}

	return nil
}

func (m *MemDB) DeleteTaskBucket(allocID, taskName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ats, ok := m.taskState[allocID]; ok {
		delete(ats, taskName)
	}

	if alts, ok := m.localTaskState[allocID]; ok {
		delete(alts, taskName)
	}

	return nil
}

func (m *MemDB) DeleteAllocationBucket(allocID string, _ ...WriteOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.allocs, allocID)
	delete(m.taskState, allocID)
	delete(m.localTaskState, allocID)

	return nil
}

func (m *MemDB) PutDevicePluginState(ps *dmstate.PluginState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.devManagerPs = ps
	return nil
}

// GetDevicePluginState stores the device manager's plugin state or returns an
// error.
func (m *MemDB) GetDevicePluginState() (*dmstate.PluginState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.devManagerPs, nil
}

func (m *MemDB) GetDriverPluginState() (*driverstate.PluginState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.driverManagerPs, nil
}

func (m *MemDB) PutDriverPluginState(ps *driverstate.PluginState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.driverManagerPs = ps
	return nil
}

func (m *MemDB) GetDynamicPluginRegistryState() (*dynamicplugins.RegistryState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dynamicManagerPs, nil
}

func (m *MemDB) PutDynamicPluginRegistryState(ps *dynamicplugins.RegistryState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dynamicManagerPs = ps
	return nil
}

func (m *MemDB) PutCheckResult(allocID string, qr *structs.CheckQueryResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.checks[allocID]; !exists {
		m.checks[allocID] = make(checks.AllocationResults)
	}

	m.checks[allocID][qr.ID] = qr
	return nil
}

func (m *MemDB) GetCheckResults() (checks.ClientResults, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return maps.Clone(m.checks), nil
}

func (m *MemDB) DeleteCheckResults(allocID string, checkIDs []structs.CheckID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range checkIDs {
		delete(m.checks[allocID], id)
	}
	return nil
}

func (m *MemDB) PurgeCheckResults(allocID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.checks, allocID)
	return nil
}

func (m *MemDB) PutNodeMeta(nm map[string]*string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodeMeta = nm
	return nil
}

func (m *MemDB) GetNodeMeta() (map[string]*string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nodeMeta, nil
}

func (m *MemDB) PutNodeRegistration(reg *cstructs.NodeRegistration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodeRegistration = reg
	return nil
}

func (m *MemDB) GetNodeRegistration() (*cstructs.NodeRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nodeRegistration, nil
}

func (m *MemDB) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Set everything to nil to blow up on further use
	m.allocs = nil
	m.taskState = nil
	m.localTaskState = nil

	return nil
}
