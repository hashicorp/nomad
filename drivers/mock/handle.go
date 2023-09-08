// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"context"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/fifo"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// taskHandle supervises a mock task
type taskHandle struct {
	logger hclog.Logger

	pluginExitAfter time.Duration
	killAfter       time.Duration
	waitCh          chan interface{}

	taskConfig  *drivers.TaskConfig
	command     Command
	execCommand *Command

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

	var pluginExitTimer <-chan time.Time
	if h.pluginExitAfter != 0 {
		timer := time.NewTimer(h.pluginExitAfter)
		defer timer.Stop()
		pluginExitTimer = timer.C
	}

	stdout, err := fifo.OpenWriter(h.taskConfig.StdoutPath)
	if err != nil {
		h.logger.Error("failed to write to stdout", "error", err)
		h.exitResult = &drivers.ExitResult{Err: err}
		return
	}
	stderr, err := fifo.OpenWriter(h.taskConfig.StderrPath)
	if err != nil {
		h.logger.Error("failed to write to stderr", "error", err)
		h.exitResult = &drivers.ExitResult{Err: err}
		return
	}

	h.exitResult = runCommand(h.command, stdout, stderr, h.killCh, pluginExitTimer, h.logger)
}
