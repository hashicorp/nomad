// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/helper/users/dynamic"
)

const (
	dynamicUsersHookName = "workload_users"
	dynamicUsersStateKey = "dynamic_user_ugid"
)

// dynamicUsersHook is used for allocating a one-time use UID/GID on behalf of
// a single workload (task). No other task will be assigned the same UID/GID
// while this task is running.
type dynamicUsersHook struct {
	shutdownCtx context.Context
	logger      hclog.Logger
	usable      bool

	lock *sync.Mutex
	pool dynamic.Pool
}

func newDynamicUsersHook(ctx context.Context, usable bool, logger hclog.Logger, pool dynamic.Pool) *dynamicUsersHook {
	return &dynamicUsersHook{
		shutdownCtx: ctx,
		logger:      logger.Named(dynamicUsersHookName),
		lock:        new(sync.Mutex),
		pool:        pool,
		usable:      usable,
	}
}

func (*dynamicUsersHook) Name() string {
	return dynamicUsersHookName
}

// Prestart runs on both initial start and on restart.
func (h *dynamicUsersHook) Prestart(_ context.Context, request *interfaces.TaskPrestartRequest, response *interfaces.TaskPrestartResponse) error {
	// if the task driver does not support the DynamicWorkloadUsers capability,
	// do nothing
	if !h.usable {
		return nil
	}

	// if the task has a user set, do nothing
	//
	// it's up to the job-submitter to set a user that exists on the system
	if request.Task.User != "" {
		return nil
	}

	// if this is the restart case, the UGID will already be acquired and we
	// just need to read it back out of the hook's state
	if request.PreviousState != nil {
		ugid, exists := request.PreviousState[dynamicUsersStateKey]
		if exists {
			response.State[dynamicUsersStateKey] = ugid
			return nil
		}
	}

	// otherwise we will acquire a dynamic UGID from the pool.
	h.lock.Lock()
	defer h.lock.Unlock()

	// allocate an unused UID/GID from the pool
	ugid, err := h.pool.Acquire()
	if err != nil {
		h.logger.Error("unable to acquire anonymous UID/GID: %v", err)
		return err
	}

	h.logger.Trace("acquired dynamic workload user", "ugid", ugid)

	// set the special user of the task
	request.Task.User = dynamic.String(ugid)

	// set the user on the hook so we may release it later
	response.State = make(map[string]string, 1)
	response.State[dynamicUsersStateKey] = request.Task.User

	return nil
}

func (h *dynamicUsersHook) Stop(_ context.Context, request *interfaces.TaskStopRequest, response *interfaces.TaskStopResponse) error {
	// if the task driver does not support the DWU capability, nothing to do
	if !h.usable {
		return nil
	}

	// if we did not store a user for this task; nothing to release
	user, exists := request.ExistingState[dynamicUsersStateKey]
	if !exists {
		return nil
	}

	// otherwise we need to release the UGID back to the pool
	h.lock.Lock()
	defer h.lock.Unlock()

	// parse the UID/GID from the pseudo username
	ugid, err := dynamic.Parse(user)
	if err != nil {
		return fmt.Errorf("unable to release dynamic workload user: %w", err)
	}

	// release the UID/GID to the pool
	if err = h.pool.Release(ugid); err != nil {
		return fmt.Errorf("unable to release dynamic workload user: %w", err)
	}

	h.logger.Trace("released dynamic workload user", "ugid", ugid)
	return nil
}
