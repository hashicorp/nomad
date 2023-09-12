// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// timeoutHookName is the name of this hook as appears in logs
	timeoutHookName = "task_timeout_hook"
)

// timeoutHook manages timeouts
type timeoutHook struct {
	logger  hclog.Logger
	message string
	tr *TaskRunner

	// fields that get re-initialized on allocation update
	lock      sync.RWMutex
	ctx       context.Context
	stop      context.CancelFunc

	stopTimerChannel chan bool
}

type saveTimeoutOpts struct {
	Namespace    string
	Region       string
}

func newTimeoutHook(
	runner *TaskRunner,
	logger hclog.Logger,
) *timeoutHook {
	h := &timeoutHook{
		logger:  logger.Named(timeoutHookName),
		tr: runner,
		stopTimerChannel: make(chan bool),
	}

	h.ctx, h.stop = context.WithCancel(h.tr.shutdownDelayCtx)
	return h
}

func (h *timeoutHook) Name() string {
	return timeoutHookName
}

func (h *timeoutHook) Prestart(_ context.Context,
	_ *interfaces.TaskPrestartRequest, _ *interfaces.TaskPrestartResponse) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	timeoutConfig := h.tr.task.Timeout

	if timeoutConfig == nil {
		return nil
	}

	var timeString string
	if timeoutConfig.Time != nil {
		timeString = *timeoutConfig.Time
	}

	var deadline time.Time

	if timeString != "" {
		t, err := time.Parse("15:04:05", timeString)
		if err != nil {
			return fmt.Errorf("error parsing time: %w", err)
		}

		currentTime := time.Now()
		deadline = time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), t.Hour(), t.Minute(), t.Second(), 0, currentTime.Location())

		// handle time's into the next day
		if deadline.Before(currentTime) {
			deadline = deadline.Add(24 * time.Hour)
		}

		h.message = fmt.Sprintf("Timed out because time deadline reached: %v", timeString)
	} else {
		deadline = time.Now().Add(*timeoutConfig.TTL)
		h.message = fmt.Sprintf("Timed out %v from task start", *timeoutConfig.TTL)
	}

	existingOrNewDeadline, err := h.saveDeadline(deadline)
	if err != nil {
		return err
	}

	// overwrite deadline with new one
	deadline = *existingOrNewDeadline

	// Now that the deadline is saved
	// Kick off goroutine to watch for timeout
	go h.stopTaskAfterDeadline(deadline)

	return nil
}

func (h *timeoutHook) Stop(_ context.Context, _ *interfaces.TaskStopRequest, _ *interfaces.TaskStopResponse) error{
	h.lock.Lock()
	defer h.lock.Unlock()

	// TODO: Bail out if unused

	h.removeDeadline()
	h.stop()

	return nil
}

func (h *timeoutHook) getUniqueId() string {
	var uniqueId string

	if h.tr.alloc.Job.Type == "system" || h.tr.alloc.Job.Type == "sysbatch" {
		uniqueId = fmt.Sprintf("system-%v-%v-%v-%v", h.tr.alloc.JobID, h.tr.taskName, h.tr.alloc.NodeID, h.tr.alloc.Job.CreateIndex)
		} else {
		uniqueId = fmt.Sprintf("standard-%v-%v-%v-%v", h.tr.alloc.JobID, h.tr.taskName, h.tr.alloc.Index(), h.tr.alloc.Job.CreateIndex)
	}

	return uniqueId
}

func (h *timeoutHook) removeDeadline() error {
	opts := saveTimeoutOpts{
		Namespace:    h.tr.alloc.Job.Namespace,
		Region:       h.tr.alloc.Job.Region,
	}

	uniqueId := h.getUniqueId()
	var Variable structs.VariableDecrypted
	Variable.Path = fmt.Sprintf("internal/timeouts/tasks/%s", uniqueId)
	Variable.VariableMetadata = structs.VariableMetadata{
		Path: Variable.Path,
	}

	args := structs.VariablesApplyRequest{
		Op:  structs.VarOpDelete,
		Var: &Variable,
		WriteRequest: structs.WriteRequest{
			Region:    opts.Region,
			Namespace: opts.Namespace,
		},
	}

	var out structs.VariablesApplyResponse
	if err := h.tr.rpcClient.RPC(structs.VariablesApplyRPCMethod, &args, &out); err != nil {
		return err
	}

	return nil
}

func (h *timeoutHook) saveDeadline(deadline time.Time) (*time.Time, error) {
	opts := saveTimeoutOpts{
		Namespace:    h.tr.alloc.Job.Namespace,
		Region:       h.tr.alloc.Job.Region,
	}

	var Variable structs.VariableDecrypted

	uniqueId := h.getUniqueId()
	Variable.Path = fmt.Sprintf("internal/timeouts/tasks/%s", uniqueId)
	Variable.Items = structs.VariableItems{
		"deadline": deadline.String(),
	}
	Variable.ModifyIndex = 0

	args := structs.VariablesApplyRequest{
		Op:  structs.VarOpCAS,
		Var: &Variable,
		WriteRequest: structs.WriteRequest{
			Region:    opts.Region,
			Namespace: opts.Namespace,
		},
	}

	var out structs.VariablesApplyResponse
	if err := h.tr.rpcClient.RPC(structs.VariablesApplyRPCMethod, &args, &out); err != nil {
		if strings.Contains(err.Error(), "cas error:") && out.Conflict != nil {
			return nil, fmt.Errorf("conflicting value: %w", err)
		}

		if out.Conflict != nil {
			existingDeadline := out.Conflict.Items["deadline"]
			deadlineMinusClockSkew := strings.Split(existingDeadline, " m=")[0]
			layout := "2006-01-02 15:04:05.999999 -0700 MST"
			time, err := time.Parse(layout, deadlineMinusClockSkew)
			return &time, err
		}

		return nil, fmt.Errorf("some write error: %w", err)
	}

	if out.Conflict != nil {
		existingDeadline := out.Conflict.Items["deadline"]
		deadlineMinusClockSkew := strings.Split(existingDeadline, " m=")[0]
		layout := "2006-01-02 15:04:05.999999 -0700 MST"
		time, err := time.Parse(layout, deadlineMinusClockSkew)
		return &time, err
	}

	return &deadline, nil
}

func (h *timeoutHook) stopTaskAfterDeadline(deadline time.Time) {
	timer, cancel := helper.NewSafeTimer(time.Second)
	defer cancel()

	for {
		select {
		case <-time.After(time.Minute):
			fmt.Println("Stopping because of time.After")
		case <-timer.C:
			if time.Now().After(deadline) {
				h.stopTaskForTimeout()
				return
			}

			fmt.Println("task timer: tick")
			timer.Reset(time.Second)
		case <- h.ctx.Done():
			fmt.Println("Stopping because context is done")
			return
		}
	}

}

func (h *timeoutHook) stopTaskForTimeout() error {
	tr := h.tr
	// Don't emit the event if its dead already
	state := tr.TaskState().State
	if state == structs.TaskStatePending || state == structs.TaskStateRunning {
		e := structs.NewTaskEvent(structs.TaskTimedout)
		e.Message = h.message
		tr.EmitEvent(e)
	}


	if h.tr.task.Timeout.FailOnTimeout {
		tr.MarkFailedKill("Failing on timeout")
	} else {
		taskEvent := structs.NewTaskEvent(structs.TaskKilling)
		taskEvent.SetKillTimeout(tr.Task().KillTimeout, tr.clientConfig.MaxKillTimeout)
		err := tr.Kill(context.TODO(), taskEvent)

		if err != nil && err != errors.New("Task not running") {
			tr.logger.Warn("error stopping leader task", "error", err, "task_name", tr.taskName)
		}
	}

	return nil
}