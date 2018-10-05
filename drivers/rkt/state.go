package rkt

import (
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/driver/executor"
	"github.com/hashicorp/nomad/plugins/drivers"
)

type taskStore struct {
	store map[string]*rktTaskHandle
	lock  sync.RWMutex
}

func newTaskStore() *taskStore {
	return &taskStore{store: map[string]*rktTaskHandle{}}
}

func (ts *taskStore) Set(id string, handle *rktTaskHandle) {
	ts.lock.Lock()
	defer ts.lock.Unlock()
	ts.store[id] = handle
}

func (ts *taskStore) Get(id string) (*rktTaskHandle, bool) {
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

type rktTaskHandle struct {
	exec         executor.Executor
	pid          int
	pluginClient *plugin.Client
	logger       hclog.Logger

	// stateLock syncs access to all fields below
	stateLock sync.RWMutex

	task        *drivers.TaskConfig
	procState   drivers.TaskState
	startedAt   time.Time
	completedAt time.Time
	exitResult  *drivers.ExitResult
}

func (h *rktTaskHandle) IsRunning() bool {
	return h.procState == drivers.TaskStateRunning
}

func (h *rktTaskHandle) run() {
	if h.exitResult == nil {
		h.exitResult = &drivers.ExitResult{}
	}

	ps, err := h.exec.Wait()
	if err != nil {
		h.exitResult.Err = err
		h.procState = drivers.TaskStateUnknown
		h.completedAt = time.Now()
		return
	}
	h.procState = drivers.TaskStateExited
	h.exitResult.ExitCode = ps.ExitCode
	h.exitResult.Signal = ps.Signal
	h.completedAt = ps.Time

}
