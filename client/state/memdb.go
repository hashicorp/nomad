package state

import (
	"fmt"
	"sync"

	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// MemDB implements a StateDB that stores data in memory and should only be
// used for testing. All methods are safe for concurrent use.
type MemDB struct {
	// alloc_id -> value
	allocs map[string]*structs.Allocation

	// alloc_id -> task_name -> value
	localTaskState map[string]map[string]*state.LocalState
	taskState      map[string]map[string]*structs.TaskState

	mu sync.RWMutex
}

func NewMemDB() *MemDB {
	return &MemDB{
		allocs:         make(map[string]*structs.Allocation),
		localTaskState: make(map[string]map[string]*state.LocalState),
		taskState:      make(map[string]map[string]*structs.TaskState),
	}
}

func (m MemDB) GetAllAllocations() ([]*structs.Allocation, map[string]error, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	allocs := make([]*structs.Allocation, 0, len(m.allocs))
	for _, v := range m.allocs {
		allocs = append(allocs, v)
	}

	return allocs, nil, nil
}

func (m MemDB) PutAllocation(alloc *structs.Allocation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allocs[alloc.ID] = alloc
	return nil
}

func (m MemDB) GetTaskRunnerState(allocID string, taskName string) (*state.LocalState, *structs.TaskState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Local Task State
	allocLocalTS := m.localTaskState[allocID]
	if len(allocLocalTS) == 0 {
		return nil, nil, fmt.Errorf("alloc has no local state: %v", allocID)
	}

	ls := allocLocalTS[taskName]
	if ls == nil {
		return nil, nil, fmt.Errorf("task has no local state: %v", taskName)
	}

	// Task State
	allocTS := m.taskState[allocID]
	if len(allocTS) == 0 {
		return nil, nil, fmt.Errorf("alloc has no task state: %v", allocID)
	}

	ts := allocTS[taskName]
	if ts == nil {
		return nil, nil, fmt.Errorf("task has no state: %v", taskName)
	}

	return ls, ts, nil
}

func (m MemDB) PutTaskRunnerLocalState(allocID string, taskName string, val *state.LocalState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if alts, ok := m.localTaskState[allocID]; ok {
		alts[taskName] = val
		return nil
	}

	m.localTaskState[allocID] = map[string]*state.LocalState{
		taskName: val,
	}

	return nil
}

func (m MemDB) PutTaskState(allocID string, taskName string, state *structs.TaskState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ats, ok := m.taskState[allocID]; ok {
		ats[taskName] = state
		return nil
	}

	m.taskState[allocID] = map[string]*structs.TaskState{
		taskName: state,
	}

	return nil
}

func (m MemDB) DeleteTaskBucket(allocID, taskName string) error {
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

func (m MemDB) DeleteAllocationBucket(allocID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.allocs, allocID)
	delete(m.taskState, allocID)
	delete(m.localTaskState, allocID)

	return nil
}

func (m MemDB) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Set everything to nil to blow up on further use
	m.allocs = nil
	m.taskState = nil
	m.localTaskState = nil

	return nil
}
