package qemu

import (
	"sync"
)

type taskStore struct {
	store map[string]*qemuTaskHandle
	lock  sync.RWMutex
}

func newTaskStore() *taskStore {
	return &taskStore{store: map[string]*qemuTaskHandle{}}
}

func (ts *taskStore) Set(id string, handle *qemuTaskHandle) {
	ts.lock.Lock()
	defer ts.lock.Unlock()
	ts.store[id] = handle
}

func (ts *taskStore) Get(id string) (*qemuTaskHandle, bool) {
	ts.lock.RLock()
	defer ts.lock.RUnlock()
	t, ok := ts.store[id]
	return t, ok
}

func (ts *taskStore) Delete(id string) {
	ts.lock.Lock()
	defer ts.lock.Unlock()
	delete(ts.store, id)
}
