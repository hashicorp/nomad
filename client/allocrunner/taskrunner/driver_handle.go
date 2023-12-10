// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"time"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// NewDriverHandle returns a handle for task operations on a specific task
func NewDriverHandle(
	driver drivers.DriverPlugin,
	taskID string,
	task *structs.Task,
	maxKillTimeout time.Duration,
	net *drivers.DriverNetwork) *DriverHandle {
	return &DriverHandle{
		driver:      driver,
		net:         net,
		taskID:      taskID,
		killSignal:  task.KillSignal,
		killTimeout: min(task.KillTimeout, maxKillTimeout),
	}
}

// DriverHandle encapsulates a driver plugin client and task identifier and exposes
// an api to perform driver operations on the task
type DriverHandle struct {
	driver      drivers.DriverPlugin
	net         *drivers.DriverNetwork
	taskID      string
	killSignal  string
	killTimeout time.Duration
}

func (h *DriverHandle) ID() string {
	return h.taskID
}

func (h *DriverHandle) WaitCh(ctx context.Context) (<-chan *drivers.ExitResult, error) {
	return h.driver.WaitTask(ctx, h.taskID)
}

// SetKillSignal allows overriding the signal sent to kill the task.
func (h *DriverHandle) SetKillSignal(signal string) {
	h.killSignal = signal
}

func (h *DriverHandle) Kill() error {
	return h.driver.StopTask(h.taskID, h.killTimeout, h.killSignal)
}

func (h *DriverHandle) Stats(ctx context.Context, interval time.Duration) (<-chan *cstructs.TaskResourceUsage, error) {
	return h.driver.TaskStats(ctx, h.taskID, interval)
}

func (h *DriverHandle) Signal(s string) error {
	return h.driver.SignalTask(h.taskID, s)
}

// Exec is the handled used by client endpoint handler to invoke the appropriate task driver exec.
func (h *DriverHandle) Exec(timeout time.Duration, cmd string, args []string) ([]byte, int, error) {
	command := append([]string{cmd}, args...)
	res, err := h.driver.ExecTask(h.taskID, command, timeout)
	if err != nil {
		return nil, 0, err
	}
	return res.Stdout, res.ExitResult.ExitCode, res.ExitResult.Err
}

// ExecStreaming is the handled used by client endpoint handler to invoke the appropriate task driver exec.
// while allowing to stream input and output
func (h *DriverHandle) ExecStreaming(ctx context.Context,
	command []string,
	tty bool,
	stream drivers.ExecTaskStream) error {

	if impl, ok := h.driver.(drivers.ExecTaskStreamingRawDriver); ok {
		return impl.ExecTaskStreamingRaw(ctx, h.taskID, command, tty, stream)
	}

	d, ok := h.driver.(drivers.ExecTaskStreamingDriver)
	if !ok {
		return fmt.Errorf("task driver does not support exec")
	}

	execOpts, doneCh := drivers.StreamToExecOptions(
		ctx, command, tty, stream)

	result, err := d.ExecTaskStreaming(ctx, h.taskID, execOpts)
	if err != nil {
		return err
	}

	execOpts.Stdout.Close()
	execOpts.Stderr.Close()

	select {
	case err = <-doneCh:
	case <-ctx.Done():
		err = fmt.Errorf("exec task timed out: %v", ctx.Err())
	}

	if err != nil {
		return err
	}

	return stream.Send(drivers.NewExecStreamingResponseExit(result.ExitCode))
}

func (h *DriverHandle) Network() *drivers.DriverNetwork {
	return h.net
}
