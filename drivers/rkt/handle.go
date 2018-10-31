// +build linux

package rkt

import (
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/driver/executor"
	"github.com/hashicorp/nomad/plugins/drivers"
)

type rktTaskHandle struct {
	exec         executor.Executor
	env          *env.TaskEnv
	uuid         string
	pid          int
	pluginClient *plugin.Client
	logger       hclog.Logger

	// stateLock syncs access to all fields below
	stateLock sync.RWMutex

	taskConfig  *drivers.TaskConfig
	procState   drivers.TaskState
	startedAt   time.Time
	completedAt time.Time
	exitResult  *drivers.ExitResult
}

func (h *rktTaskHandle) IsRunning() bool {
	h.stateLock.RLock()
	defer h.stateLock.RUnlock()
	return h.procState == drivers.TaskStateRunning
}

func (h *rktTaskHandle) run() {

	// Since run is called immediately after the handle is created this
	// ensures the exitResult is initialized so we avoid a nil pointer
	// thus it does not need to be included in the lock
	h.stateLock.Lock()
	if h.exitResult == nil {
		h.exitResult = &drivers.ExitResult{}
	}
	h.stateLock.Unlock()

	ps, err := h.exec.Wait()
	h.stateLock.Lock()
	defer h.stateLock.Unlock()

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

	// TODO: detect if the taskConfig OOMed
}
