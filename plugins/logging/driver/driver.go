package driver

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/helper"
	loglib "github.com/hashicorp/nomad/plugins/logging/lib"
)

// Driver is a stripped-down version of the raw_exec driver. It has no resource
// isolation (yet), and just fork/execs. It doesn't implement the full driver
// spec or user-configurable tasks because it only needs to start/stop the log
// shipper tasks.
//
// Log shipper tasks read off the FIFO and should exit if the task they are
// monitoring sends EOF on that FIFO, similar to how Apache rotatelogs works.
// Therefore this driver interface starts the task and restarts it only if it
// fails unexpectedly.
type Driver struct {
	newCmd NewCommandFunc

	// tasks is the in memory datastore mapping taskIDs to driverHandles
	tasks *taskStore

	// ctx is the context for the driver for coordinating shutdown.
	ctx context.Context

	// logger will log to the Nomad agent
	logger hclog.Logger
}

type NewCommandFunc func(*TaskHandle, *loglib.LogConfig) (*exec.Cmd, error)

func NewDriver(ctx context.Context, logger hclog.Logger, newCmd NewCommandFunc) *Driver {
	return &Driver{
		newCmd: newCmd,
		ctx:    ctx,
		logger: logger,
		tasks:  newTaskStore(),
	}
}

func (d *Driver) Stop(config *loglib.LogConfig) error {
	id := config.ID()
	handle, ok := d.tasks.Get(id)
	if !ok {
		return nil
	}
	err := handle.Kill()
	if err != nil {
		d.logger.Debug("error stopping log shipper task", "error", err)
	}
	return nil
}

// Start starts the task and starts the monitor that will restart it if it stops
// unexpectedly. It returns an error only if the driver is disabled or the
// initial start fails.
func (d *Driver) Start(config *loglib.LogConfig) error {
	handle, err := d.start(config)
	if err != nil {
		return err
	}
	go d.monitor(handle)
	return nil
}

// start creates the initial task handle and runs the task. It returns a handle
// that can be monitored or an error if the config is invalid or the task can't
// be started after a few retries
func (d *Driver) start(config *loglib.LogConfig) (*TaskHandle, error) {

	id := config.ID()

	maxRetries := 5
	retries := 0
	for {
		handleCtx, handleCancel := context.WithCancel(d.ctx)
		handle := &TaskHandle{
			id:            id,
			config:        config,
			TaskLogWriter: newTaskLogWriter(handleCtx, d.logger),
			cancelFn:      handleCancel,
			exitCh:        make(chan *exitResult),
		}

		cmd, err := d.newCmd(handle, config)
		if err != nil {
			return nil, err
		}
		err = handle.run(d.ctx, cmd)
		if err == nil {
			d.tasks.Set(handle.id, handle)
			return handle, nil
		}

		d.logger.Warn("could not run log shipper task", "error", err)
		if retries >= maxRetries {
			return nil, err
		}
		// backoff before retry
		t, stopFn := helper.NewSafeTimer(1 * time.Second)
		defer stopFn()
		select {
		case <-t.C:
			stopFn()
			retries++
			continue
		case <-d.ctx.Done():
			return nil, nil
		}
	}

}

type taskLogWriter struct {
	ctx    context.Context
	logger hclog.Logger
}

func (w taskLogWriter) Write(p []byte) (int, error) {
	// TODO: do a buffered scanner over newlines on this
	w.logger.Debug(string(p))
	return len(p), nil
}

func newTaskLogWriter(ctx context.Context, logger hclog.Logger) *taskLogWriter {
	return &taskLogWriter{
		ctx:    ctx,
		logger: logger,
	}
}

// monitor waits for the task handle to exit and restarts it if it stops
// unexpectedly, creating a new handle in the process
func (d *Driver) monitor(handle *TaskHandle) {
	var err error
	for {
		select {
		case result := <-handle.exitCh:
			d.logger.Debug("log shipper task exited", "result", result)
			// If the task exits, wait 5s to see if the logging_hook sends a
			// Stop (which will delete the handle from the store). If not,
			// restart the log shipper
			t, stopFn := helper.NewSafeTimer(5 * time.Second)
			defer stopFn()
			select {
			case <-t.C:
				if _, ok := d.tasks.Get(handle.id); ok {
					d.tasks.Delete(handle.id)
					handle, err = d.start(handle.config)
					if err != nil {
						return
					}
					stopFn()
					continue
				}
				return
			case <-d.ctx.Done():
				return
			}
		case <-d.ctx.Done():
			return
		}
	}
}

// TaskHandle is the abstraction over the log shipper process. Log shipper tasks
// read off the FIFO and should exit if the task they are monitoring sends EOF
// on that FIFO, similar to how Apache rotatelogs works.
type TaskHandle struct {
	id       string
	config   *loglib.LogConfig
	pid      int
	process  *os.Process
	cancelFn context.CancelFunc
	exitCh   chan *exitResult

	TaskLogWriter io.Writer
}

func (h *TaskHandle) run(ctx context.Context, cmd *exec.Cmd) error {
	// TODO: this is failing with "operation not permitted", even if we're
	// running as root?
	// cmd.SysProcAttr = &syscall.SysProcAttr{
	// 	Setpgid: true, // ignore signals sent to the Nomad parent

	// 	// TODO: it would be nice if we could start as root and then drop privs
	// 	// inside the subprocess
	// 	//
	// 	// Credential: &syscall.Credential{Uid: uid, Gid: gid},
	// }

	// platform-specific isolation
	isolateCommand(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start log shipper: %w", err)
	}

	h.pid = cmd.Process.Pid
	h.process = cmd.Process

	go h.wait()
	return nil
}

func (h *TaskHandle) Kill() error {
	return h.process.Kill()
}

// wait blocks for the process to exit and must be called *after* there's a valid
// process on the handle
func (h *TaskHandle) wait() {
	ps, err := h.process.Wait()
	status := ps.Sys().(syscall.WaitStatus)
	code := ps.ExitCode()

	h.exitCh <- &exitResult{
		code:   code,
		status: int(status),
		err:    err,
	}
}

type exitResult struct {
	code   int
	status int
	err    error
}
