package mock

import (
	"context"
	"io"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/fifo"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// mockTaskHandle is a task handler which supervises a mock task
type mockTaskHandle struct {
	logger hclog.Logger

	runFor          time.Duration
	killAfter       time.Duration
	killTimeout     time.Duration
	waitCh          chan struct{}
	exitCode        int
	exitSignal      int
	exitErr         error
	signalErr       error
	stdoutString    string
	stdoutRepeat    int
	stdoutRepeatDur time.Duration

	task        *drivers.TaskConfig
	procState   drivers.TaskState
	startedAt   time.Time
	completedAt time.Time
	exitResult  *drivers.ExitResult

	// Calling kill closes killCh if it is not already closed
	kill   context.CancelFunc
	killCh <-chan struct{}
}

func (h *mockTaskHandle) run() {
	defer close(h.waitCh)

	errCh := make(chan error, 1)

	// Setup logging output
	if h.stdoutString != "" {
		go h.handleLogging(errCh)
	}

	timer := time.NewTimer(h.runFor)
	defer timer.Stop()

	select {
	case <-timer.C:
		h.logger.Debug("run_for time elapsed; exiting", "run_for", h.runFor)
	case <-h.killCh:
		h.logger.Debug("killed; exiting")
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

func (h *mockTaskHandle) handleLogging(errCh chan<- error) {
	stdout, err := fifo.Open(h.task.StdoutPath)
	if err != nil {
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
