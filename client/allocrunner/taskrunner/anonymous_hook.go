// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
)

const (
	anonymousHookName = "anonymous"
)

// anonymousHook is used for allocating an anonymous one time use UID/GID on
// behalf of a task.
type anonymousHook struct {
	shutdownCtx context.Context
	logger      hclog.Logger
	lock        sync.Mutex
}

func newAnonymousHook(ctx context.Context, logger hclog.Logger) *anonymousHook {
	return &anonymousHook{
		shutdownCtx: ctx,
		logger:      logger.Named(anonymousHookName),
	}
}

func (*anonymousHook) Name() string {
	return anonymousHookName
}

func (h *anonymousHook) Prestart(_ context.Context, request *interfaces.TaskPrestartRequest, response *interfaces.TaskPrestartResponse) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	// TODO gimmie a UID/GID

	return nil
}

func (h *anonymousHook) Stop(_ context.Context, request *interfaces.TaskStopRequest, response *interfaces.TaskStopResponse) error {
	h.lock.Lock()
	defer h.lock.Unlock()

	// TODO return the UID/GID

	return nil
}
