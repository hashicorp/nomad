// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package allocrunner

import (
	"context"
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
	timeoutHookName = "alloc_timeout_hook"
)

// timeoutHook manages timeouts
type timeoutHook struct {
	logger  hclog.Logger
	allocID string
	message string
	runner *allocRunner

	// fields that get re-initialized on allocation update
	lock      sync.RWMutex
	ctx       context.Context
	stop      context.CancelFunc
	alloc     *structs.Allocation

	stopTimerChannel chan bool
}

type saveTimeoutOpts struct {
	Namespace    string
	Region       string
}

func newTimeoutHook(
	logger hclog.Logger,
	alloc *structs.Allocation,
	runner *allocRunner,
) *timeoutHook {
	h := &timeoutHook{
		logger:  logger.Named(timeoutHookName),
		allocID: alloc.ID,
		alloc:   alloc,
		runner: runner,
		stopTimerChannel: make(chan bool),
	}
	h.initialize(alloc)
	return h
}

func (h *timeoutHook) initialize(alloc *structs.Allocation) {
	h.lock.Lock()
	defer h.lock.Unlock()

	// fresh context and stop function for this allocation
	h.ctx, h.stop = context.WithCancel(h.runner.shutdownDelayCtx)

	// set the initial alloc
	h.alloc = alloc
}

func (h *timeoutHook) Name() string {
	return timeoutHookName
}

func (h *timeoutHook) Prerun() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	// get task group config
	tg := h.alloc.Job.LookupTaskGroup(h.alloc.TaskGroup)

	if tg.Timeout == nil {
		return nil
	}

	var timeString string
	if tg.Timeout.Time != nil {
		timeString = *tg.Timeout.Time
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

		if tg.Timeout.StartFrom == nil {
			deadline = time.Now().Add(*tg.Timeout.TTL)
			h.message = fmt.Sprintf("Timed out %v from task start", *tg.Timeout.TTL)
		} else if *tg.Timeout.StartFrom == "scheduled" {
			deadline = time.Unix(0, h.alloc.CreateTime).Add(*tg.Timeout.TTL)
			h.message = fmt.Sprintf("Timed out %v from allocation scheduled", *tg.Timeout.TTL)
		} else if *tg.Timeout.StartFrom == "submit" {
			deadline = time.Unix(0, h.alloc.Job.SubmitTime).Add(*tg.Timeout.TTL)
			h.message = fmt.Sprintf("Timed out %v from job submit", *tg.Timeout.TTL)
		}	else {
			deadline = time.Now().Add(*tg.Timeout.TTL)
			h.message = fmt.Sprintf("Timed out %v from task start", *tg.Timeout.TTL)
		}
	}

	// save deadline to variable
	existingOrNewDeadline, err := h.saveDeadline(deadline)
	if err != nil {
		return err
	}

	// overwrite deadline with new one
	deadline = *existingOrNewDeadline

	// Now that the deadline is saved
	// Kick off goroutine to watch for timeout
	go h.stopAllocAfterDeadline(deadline)

	return nil
}

func (h *timeoutHook) Update(request *interfaces.RunnerUpdateRequest) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	// TODO: Nothing happens here?

	return nil
}

func (h *timeoutHook) PreKill() {
	h.lock.Lock()
	defer h.lock.Unlock()

	// TODO: Bail out if not used

	h.removeDeadline()
	h.stop()
}

func (h *timeoutHook) getUniqueId() string {
	var uniqueId string
	if h.alloc.Job.Type == "system" || h.alloc.Job.Type == "sysbatch" {
		uniqueId = fmt.Sprintf("system-%v-%v-%v", h.alloc.JobID, h.alloc.NodeID, h.alloc.Job.CreateIndex)
		} else {
		uniqueId = fmt.Sprintf("standard-%v-%v-%v", h.alloc.JobID, h.alloc.Index(), h.alloc.Job.CreateIndex)
	}

	return uniqueId
}

func (h *timeoutHook) removeDeadline() error {
	opts := saveTimeoutOpts{
		Namespace:    h.alloc.Namespace,
		Region:       h.alloc.Job.Region,
	}

	uniqueId := h.getUniqueId()
	var Variable structs.VariableDecrypted
	Variable.Path = fmt.Sprintf("internal/timeouts/allocs/%s", uniqueId)
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
	if err := h.runner.rpcClient.RPC(structs.VariablesApplyRPCMethod, &args, &out); err != nil {
		return err
	}

	return nil
}

func (h *timeoutHook) saveDeadline(deadline time.Time) (*time.Time, error) {
	opts := saveTimeoutOpts{
		Namespace:    h.alloc.Namespace,
		Region:       h.alloc.Job.Region,
	}

	var Variable structs.VariableDecrypted

	uniqueId := h.getUniqueId()
	Variable.Path = fmt.Sprintf("internal/timeouts/allocs/%s", uniqueId)
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
	if err := h.runner.rpcClient.RPC(structs.VariablesApplyRPCMethod, &args, &out); err != nil {
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

func (h *timeoutHook) stopAllocAfterDeadline(deadline time.Time) {
	timer, cancel := helper.NewSafeTimer(time.Second)
	defer cancel()

	for {
		select {
		case <-time.After(time.Minute):
			fmt.Println("Stopping because of time.After")
		case <-timer.C:
			if time.Now().After(deadline) {
				h.stopAllocForTimeout()
				return
			}

			fmt.Println("timer: tick")
			timer.Reset(time.Second)
		case <- h.ctx.Done():
			fmt.Println("Stopping because context is done")
			return
		}
	}

}

func (h *timeoutHook) stopAllocForTimeout() error {
	for _, tr := range h.runner.tasks {
		// Don't emit the event if its dead already
		state := tr.TaskState().State
		if state == structs.TaskStatePending || state == structs.TaskStateRunning {
			e := structs.NewTaskEvent(structs.TaskTimedout)
			e.Message = h.message
			tr.EmitEvent(e)
		}
	}

	h.runner.Destroy()

	return nil
}