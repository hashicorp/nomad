package exec

import (
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/driver/executor"
	"github.com/hashicorp/nomad/plugins/drivers"
)

type execTaskHandle struct {
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
	exitCh      chan struct{}
}

func (h *execTaskHandle) IsRunning() bool {
	return h.procState == drivers.TaskStateRunning
}

func (h *execTaskHandle) run() {
	defer close(h.exitCh)
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
	// TODO: plumb OOM bool
}
