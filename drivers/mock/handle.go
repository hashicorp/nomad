package mock

import (
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
	exitCode        int
	exitSignal      int
	exitErr         error
	signalErr       error
	stdoutString    string
	stdoutRepeat    int
	stdoutRepeatDur time.Duration

	doneCh chan struct{}

	task        *drivers.TaskConfig
	procState   drivers.TaskState
	startedAt   time.Time
	completedAt time.Time
	exitResult  *drivers.ExitResult
}

func (h *mockTaskHandle) run() {

	// Setup logging output
	if h.stdoutString != "" {
		go h.handleLogging()
	}

	timer := time.NewTimer(h.runFor)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			select {
			case <-h.doneCh:
				// already closed
			default:
				close(h.doneCh)
			}
		case <-h.doneCh:
			h.logger.Debug("finished running task", "name", h.task.Name)
			h.exitResult = &drivers.ExitResult{
				ExitCode: h.exitCode,
				Signal:   h.exitSignal,
				Err:      h.exitErr,
			}
			return
		}
	}
}

func (h *mockTaskHandle) handleLogging() {
	stdout, err := fifo.Open(h.task.StdoutPath)
	if err != nil {
		h.exitErr = err
		close(h.doneCh)
		h.logger.Error("failed to write to stdout: %v", err)
		return
	}

	for i := 0; i < h.stdoutRepeat; i++ {
		select {
		case <-h.doneCh:
			return
		case <-time.After(h.stdoutRepeatDur):
			if _, err := io.WriteString(stdout, h.stdoutString); err != nil {
				h.exitErr = err
				close(h.doneCh)
				h.logger.Error("failed to write to stdout", "error", err)
				return
			}
		}
	}
}
