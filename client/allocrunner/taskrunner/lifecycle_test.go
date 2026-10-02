// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"sync"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/taskrunner/restarts"
	cstate "github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/shoenig/test/must"
)

// restartTestDriver implements only the DriverPlugin methods restartImpl
// touches; the embedded interface panics if anything else is called.
type restartTestDriver struct {
	drivers.DriverPlugin

	logger   hclog.Logger
	exitCh   chan struct{} // closed when the task "process" exits
	stopOnce sync.Once
}

// WaitTask mirrors the docker driver's handleWait: a result is sent when the
// task exits *or* when ctx is done.
func (d *restartTestDriver) WaitTask(ctx context.Context, taskID string) (<-chan *drivers.ExitResult, error) {
	if deadline, ok := ctx.Deadline(); ok {
		d.logger.Trace("WaitTask called", "task_id", taskID, "ctx_deadline_in", time.Until(deadline).Round(time.Millisecond))
	} else {
		d.logger.Trace("WaitTask called", "task_id", taskID, "ctx_deadline_in", "none")
	}

	ch := make(chan *drivers.ExitResult, 1)
	go func() {
		defer close(ch)
		select {
		case <-d.exitCh:
			d.logger.Trace("WaitTask: task exited, sending exit result", "task_id", taskID)
			ch <- &drivers.ExitResult{}
		case <-ctx.Done():
			// This is the fake "exit" that unblocks restartImpl's select
			// even though the task is still running.
			d.logger.Trace("WaitTask: ctx done before task exited, sending ctx error as exit result",
				"task_id", taskID, "error", ctx.Err())
			ch <- &drivers.ExitResult{Err: ctx.Err()}
		}
	}()
	return ch, nil
}

func (d *restartTestDriver) StopTask(taskID string, timeout time.Duration, signal string) error {
	stopped := false
	d.stopOnce.Do(func() {
		close(d.exitCh)
		stopped = true
	})
	if stopped {
		d.logger.Trace("StopTask: task killed", "task_id", taskID, "timeout", timeout, "signal", signal)
	} else {
		d.logger.Trace("StopTask: task already stopped", "task_id", taskID)
	}
	return nil
}

func (d *restartTestDriver) killed() bool {
	select {
	case <-d.exitCh:
		return true
	default:
		return false
	}
}

// newRestartTestTaskRunner returns a TaskRunner whose task is running under
// restartTestDriver, without starting the Run loop.
func newRestartTestTaskRunner(t *testing.T, shutdownDelay time.Duration) (*TaskRunner, *restartTestDriver) {
	alloc := mock.Alloc()
	tg := alloc.Job.TaskGroups[0]
	task := tg.Tasks[0]
	task.ShutdownDelay = shutdownDelay

	logger := testlog.HCLogger(t)
	drv := &restartTestDriver{
		logger: logger.Named("restart_test_driver"),
		exitCh: make(chan struct{}),
	}

	tr := &TaskRunner{
		allocID:          alloc.ID,
		taskName:         task.Name,
		alloc:            alloc,
		task:             task,
		logger:           logger,
		state:            structs.NewTaskState(),
		stateDB:          cstate.NoopDB{},
		stateUpdater:     NewMockTaskStateUpdater(),
		restartTracker:   restarts.NewRestartTracker(tg.RestartPolicy, alloc.Job.Type, task.Lifecycle),
		killCtx:          context.Background(),
		shutdownDelayCtx: context.Background(),
		maxEvents:        10,
		handle:           &DriverHandle{driver: drv, taskID: "restart-test"},
	}
	tr.state.State = structs.TaskStateRunning

	return tr, drv
}

// TestTaskRunner_Restart_ShutdownDelayVsCtx asserts a restart kills the task
// even when the caller's ctx expires during shutdown_delay. check_restart
// calls Restart with a 10s ctx (serviceregistration.asyncRestart), so a task
// with shutdown_delay >= 10s must not be left running and marked pending.
func TestTaskRunner_Restart_ShutdownDelayVsCtx(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name          string
		restartCtx    time.Duration
		shutdownDelay time.Duration
	}{
		{
			// e.g. shutdown_delay = 5s against the 10s check_restart ctx
			name:          "delay shorter than ctx",
			restartCtx:    300 * time.Millisecond,
			shutdownDelay: 100 * time.Millisecond,
		},
		{
			// e.g. connect sidecar shutdown_delay = 13s against the 10s ctx
			name:          "delay longer than ctx",
			restartCtx:    100 * time.Millisecond,
			shutdownDelay: 300 * time.Millisecond,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)

			tr, drv := newRestartTestTaskRunner(t, tc.shutdownDelay)

			ctx, cancel := context.WithTimeout(context.Background(), tc.restartCtx)
			defer cancel()

			event := structs.NewTaskEvent(structs.TaskRestartSignal).
				SetRestartReason(`healthcheck: check "service: \"catalog\" check" unhealthy`)
			must.NoError(t, tr.Restart(ctx, event, true))

			// Allow well past shutdown_delay for the kill to land.
			select {
			case <-drv.exitCh:
			case <-time.After(3 * tc.shutdownDelay):
			}

			// Without a kill the Run loop never sees an exit, so the task
			// stays in whatever state restartImpl left it in; if that's
			// pending, every later Restart returns ErrTaskNotRunning.
			ts := tr.TaskState()
			var retryErr error
			if !drv.killed() {
				retryErr = tr.Restart(ctx, event, true)
			}
			must.True(t, drv.killed(), must.Sprintf(
				"task was never killed: state=%q events=%v follow-up restart err=%v",
				ts.State, eventTypes(ts), retryErr))
		})
	}
}

func eventTypes(ts *structs.TaskState) []string {
	types := make([]string, 0, len(ts.Events))
	for _, ev := range ts.Events {
		types = append(types, ev.Type)
	}
	return types
}
