package mock

import (
	"context"
	"io"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/fifo"
	bstructs "github.com/hashicorp/nomad/plugins/base/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// taskHandle supervises a mock task
type taskHandle struct {
	logger hclog.Logger

	runFor          time.Duration
	pluginExitAfter time.Duration
	killAfter       time.Duration
	waitCh          chan struct{}
	exitCode        int
	exitSignal      int
	exitErr         error
	signalErr       error
	stdoutString    string
	stdoutRepeat    int
	stdoutRepeatDur time.Duration

	taskConfig *drivers.TaskConfig

	// stateLock guards the procState field
	stateLock sync.RWMutex
	procState drivers.TaskState

	startedAt   time.Time
	completedAt time.Time
	exitResult  *drivers.ExitResult

	// Calling kill closes killCh if it is not already closed
	kill   context.CancelFunc
	killCh <-chan struct{}

	// Recovered is set to true if the handle was created while being recovered
	Recovered bool
}

func (h *taskHandle) TaskStatus() *drivers.TaskStatus {
	h.stateLock.RLock()
	defer h.stateLock.RUnlock()

	return &drivers.TaskStatus{
		ID:               h.taskConfig.ID,
		Name:             h.taskConfig.Name,
		State:            h.procState,
		StartedAt:        h.startedAt,
		CompletedAt:      h.completedAt,
		ExitResult:       h.exitResult,
		DriverAttributes: map[string]string{},
	}
}

func (h *taskHandle) IsRunning() bool {
	h.stateLock.Lock()
	defer h.stateLock.Unlock()
	return h.procState == drivers.TaskStateRunning
}

func (h *taskHandle) run() {
	defer func() {
		h.stateLock.Lock()
		h.procState = drivers.TaskStateExited
		h.stateLock.Unlock()

		h.completedAt = time.Now()
		close(h.waitCh)
	}()

	h.stateLock.Lock()
	h.procState = drivers.TaskStateRunning
	h.stateLock.Unlock()

	errCh := make(chan error, 1)

	// Setup logging output
	go h.handleLogging(errCh)

	timer := time.NewTimer(h.runFor)
	defer timer.Stop()

	var pluginExitTimer <-chan time.Time
	if h.pluginExitAfter != 0 {
		timer := time.NewTimer(h.pluginExitAfter)
		defer timer.Stop()
		pluginExitTimer = timer.C
	}

	select {
	case <-timer.C:
		h.logger.Debug("run_for time elapsed; exiting", "run_for", h.runFor)
	case <-h.killCh:
		h.logger.Debug("killed; exiting")
	case <-pluginExitTimer:
		h.logger.Debug("exiting plugin")
		h.exitResult = &drivers.ExitResult{
			Err: bstructs.ErrPluginShutdown,
		}

		return
	case err := <-errCh:
		h.logger.Error("error running mock task; exiting", "error", err)
		h.exitResult = &drivers.ExitResult{
			Err: err,
		}
		return
	}

	h.exitResult = &drivers.ExitResult{
		ExitCode: h.exitCode,
		Signal:   h.exitSignal,
		Err:      h.exitErr,
	}
	return
}

func (h *taskHandle) handleLogging(errCh chan<- error) {
	stdout, err := fifo.Open(h.taskConfig.StdoutPath)
	if err != nil {
		h.logger.Error("failed to write to stdout", "error", err)
		errCh <- err
		return
	}
	stderr, err := fifo.Open(h.taskConfig.StderrPath)
	if err != nil {
		h.logger.Error("failed to write to stderr", "error", err)
		errCh <- err
		return
	}
	defer stderr.Close()

	if h.stdoutString == "" {
		return
	}

	if _, err := io.WriteString(stdout, h.stdoutString); err != nil {
		h.logger.Error("failed to write to stdout", "error", err)
		errCh <- err
		return
	}

	for i := 0; i < h.stdoutRepeat; i++ {
		select {
		case <-h.waitCh:
			h.logger.Warn("exiting before done writing output", "i", i, "total", h.stdoutRepeat)
			return
		case <-time.After(h.stdoutRepeatDur):
			if _, err := io.WriteString(stdout, h.stdoutString); err != nil {
				h.logger.Error("failed to write to stdout", "error", err)
				errCh <- err
				return
			}
		}
	}
}
