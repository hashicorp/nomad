// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/anonymous"
)

const (
	anonymousHookName = "anonymous"
	anonymousStateKey = "anonymous.ugid"
)

// anonymousHook is used for allocating an anonymous one time use UID/GID on
// behalf of a task.
type anonymousHook struct {
	shutdownCtx context.Context
	logger      hclog.Logger
	usable      bool

	lock *sync.Mutex
	pool anonymous.Pool
}

func newAnonymousHook(ctx context.Context, usable bool, logger hclog.Logger, pool anonymous.Pool) *anonymousHook {
	return &anonymousHook{
		shutdownCtx: ctx,
		logger:      logger.Named(anonymousHookName),
		lock:        new(sync.Mutex),
		pool:        pool,
		usable:      usable,
	}
}

func (*anonymousHook) Name() string {
	return anonymousHookName
}

// Prestart runs on both initial start and on restart.
func (h *anonymousHook) Prestart(_ context.Context, request *interfaces.TaskPrestartRequest, response *interfaces.TaskPrestartResponse) error {
	// If the driver does not support the anonymous user capability, do nothing.
	if !h.usable {
		return nil
	}

	// If the task has a service user set, do nothing.
	if request.Task.User != "" {
		return nil
	}

	// If this is the restart case, the UGID will already be acquired and we
	// just need to read it back out of the hook's state.
	if request.PreviousState != nil {
		ugid, exists := request.PreviousState[anonymousStateKey]
		if exists {
			response.State[anonymousStateKey] = ugid
			return nil
		}
	}

	// Otherwise we will acquire an anonymous UGID from the pool.
	h.lock.Lock()
	defer h.lock.Unlock()

	// allocate an unused UID/GID from the pool
	ugid, err := h.pool.Acquire()
	if err != nil {
		h.logger.Error("unable to acquire anonymous UID/GID: %v", err)
		return err
	}

	h.logger.Info("acquired anonymous user", "ugid", ugid) // TODO debug

	// set the special user of the task
	request.Task.User = anonymous.String(ugid)

	// set the user on the hook so we may release it later
	response.State = make(map[string]string, 1)
	response.State[anonymousStateKey] = request.Task.User

	return nil
}

func (h *anonymousHook) Stop(_ context.Context, request *interfaces.TaskStopRequest, response *interfaces.TaskStopResponse) error {
	// If we did not store a user for this task; nothing to release
	user, exists := request.ExistingState[anonymousStateKey]
	if !exists {
		return nil
	}

	// Otherwise we need to release the UGID from the pool
	h.lock.Lock()
	defer h.lock.Unlock()

	// parse the UID/GID from the faux username
	ugid, err := anonymous.Parse(user)
	if err != nil {
		return fmt.Errorf("unable to release anonymous user: %v", err)
	}

	// release the UID/GID to the pool and we will never think about it again
	if err = h.pool.Release(ugid); err != nil {
		return fmt.Errorf("unable to release anonymous user: %v", err)
	}

	h.logger.Info("released anonymous user", "ugid", ugid) // TODO debug

	return nil
}
