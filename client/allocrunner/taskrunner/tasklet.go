// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/interfaces"
)

// contextExec allows canceling a interfaces.ScriptExecutor with a context.
type contextExec struct {
	// pctx is the parent context. A subcontext will be created with Exec's
	// timeout.
	pctx context.Context

	// exec to be wrapped in a context
	exec interfaces.ScriptExecutor
}

func newContextExec(ctx context.Context, exec interfaces.ScriptExecutor) *contextExec {
	return &contextExec{
		pctx: ctx,
		exec: exec,
	}
}

// execResult are the outputs of an Exec
type execResult struct {
	output []byte
	code   int
	err    error
}

// Exec a command until the timeout expires, the context is canceled, or the
// underlying Exec returns.
func (c *contextExec) Exec(timeout time.Duration, cmd string, args []string) ([]byte, int, error) {
	resCh := make(chan execResult, 1)

	// Don't trust the underlying implementation to obey timeout
	ctx, cancel := context.WithTimeout(c.pctx, timeout)
	defer cancel()

	go func() {
		output, code, err := c.exec.Exec(timeout, cmd, args)
		select {
		case resCh <- execResult{output, code, err}:
		case <-ctx.Done():
		}
	}()

	select {
	case res := <-resCh:
		return res.output, res.code, res.err
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	}
}

// tasklet is an abstraction around periodically running a script within
// the context of a Task. The interfaces.ScriptExecutor is fired at least
// once and on each interval, and fires a callback whenever the script
// is complete.
type tasklet struct {
	Command    string        // Command is the command to run for tasklet
	Args       []string      // Args is a list of arguments for tasklet
	Interval   time.Duration // Interval of the tasklet
	Timeout    time.Duration // Timeout of the tasklet
	exec       interfaces.ScriptExecutor
	callback   taskletCallback
	logger     log.Logger
	shutdownCh <-chan struct{}
}

// taskletHandle is returned by tasklet.run by cancelling a tasklet and
// waiting for it to shutdown.
type taskletHandle struct {
	// cancel the script
	cancel func()
	exitCh chan struct{}
}

// wait returns a chan that's closed when the tasklet exits
func (t taskletHandle) wait() <-chan struct{} {
	return t.exitCh
}

// taskletCallback is called with a cancellation context and the output of a
// tasklet's Exec whenever it runs.
type taskletCallback func(context.Context, execResult)

// run this tasklet check and return its cancel func. The tasklet's
// callback will be called each time it completes. If the shutdownCh is
// closed the check will be run once more before exiting.
func (t *tasklet) run() *taskletHandle {
	ctx, cancel := context.WithCancel(context.Background())
	exitCh := make(chan struct{})

	// Wrap the original interfaces.ScriptExecutor in one that obeys context
	// cancelation.
	ctxExec := newContextExec(ctx, t.exec)

	go func() {
		defer close(exitCh)
		timer := time.NewTimer(0)
		defer timer.Stop()
		for {
			// Block until tasklet is removed, Nomad is shutting
			// down, or the tasklet interval is up
			select {
			case <-ctx.Done():
				// tasklet has been removed
				return
			case <-t.shutdownCh:
				// unblock but don't exit until after we run once more
			case <-timer.C:
				timer.Reset(t.Interval)
			}

			metrics.IncrCounter([]string{
				"client", "allocrunner", "taskrunner", "tasklet_runs"}, 1)

			// Execute check script with timeout
			t.logger.Trace("tasklet executing")
			output, code, err := ctxExec.Exec(t.Timeout, t.Command, t.Args)
			switch err {
			case context.Canceled:
				// check removed during execution; exit
				return
			case context.DeadlineExceeded:
				metrics.IncrCounter([]string{
					"client", "allocrunner", "taskrunner",
					"tasklet_timeouts"}, 1)
				// If no error was returned, set one to make sure the tasklet
				// is marked as failed
				if err == nil {
					err = context.DeadlineExceeded
				}

				// Log deadline exceeded every time as it's a
				// distinct issue from the tasklet returning failure
				t.logger.Warn("tasklet timed out", "timeout", t.Timeout)
			}

			t.callback(ctx, execResult{output, code, err})

			select {
			case <-t.shutdownCh:
				// We've been told to exit and just ran so exit
				return
			default:
			}
		}
	}()
	return &taskletHandle{cancel: cancel, exitCh: exitCh}
}
