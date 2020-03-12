package state

import (
	"sync"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	dmstate "github.com/hashicorp/nomad/client/devicemanager/state"
	"github.com/hashicorp/nomad/client/dynamicplugins"
	driverstate "github.com/hashicorp/nomad/client/pluginmanager/drivermanager/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// MemDB implements a StateDB that stores data in memory and should only be
// used for testing. All methods are safe for concurrent use.
type MemDB struct {
	// alloc_id -> value
	allocs map[string]*structs.Allocation

	// alloc_id -> value
	deployStatus map[string]*structs.AllocDeploymentStatus

	// alloc_id -> task_name -> value
	localTaskState map[string]map[string]*state.LocalState
	taskState      map[string]map[string]*structs.TaskState

	// devicemanager -> plugin-state
	devManagerPs *dmstate.PluginState

	// drivermanager -> plugin-state
	driverManagerPs *driverstate.PluginState

	// dynamicmanager -> registry-state
	dynamicManagerPs *dynamicplugins.RegistryState

	logger hclog.Logger

	mu sync.RWMutex
}

func NewMemDB(logger hclog.Logger) *MemDB {
	logger = logger.Named("memdb")
	return &MemDB{
		allocs:         make(map[string]*structs.Allocation),
		deployStatus:   make(map[string]*structs.AllocDeploymentStatus),
		localTaskState: make(map[string]map[string]*state.LocalState),
		taskState:      make(map[string]map[string]*structs.TaskState),
		logger:         logger,
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

func (m *MemDB) PutAllocation(alloc *structs.Allocation) error {
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

func (m *MemDB) DeleteAllocationBucket(allocID string) error {
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

func (m *MemDB) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Set everything to nil to blow up on further use
	m.allocs = nil
	m.taskState = nil
	m.localTaskState = nil

	return nil
}
