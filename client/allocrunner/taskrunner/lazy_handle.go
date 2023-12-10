// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
	bstructs "github.com/hashicorp/nomad/plugins/base/structs"
)

const (
	// retrieveBackoffBaseline is the baseline time for exponential backoff while
	// retrieving a handle.
	retrieveBackoffBaseline = 250 * time.Millisecond

	// retrieveBackoffLimit is the limit of the exponential backoff for
	// retrieving a handle.
	retrieveBackoffLimit = 5 * time.Second

	// retrieveFailureLimit is how many times we will attempt to retrieve a
	// new handle before giving up.
	retrieveFailureLimit = 5
)

// retrieveHandleFn is used to retrieve the latest driver handle
type retrieveHandleFn func() *DriverHandle

// LazyHandle is used to front calls to a DriverHandle where it is expected the
// existing handle may no longer be valid because the backing plugin has
// shutdown. LazyHandle detects the plugin shutting down and retrieves a new
// handle so that the consumer does not need to worry whether the handle is to
// the latest driver instance.
type LazyHandle struct {
	// retrieveHandle is used to retrieve the latest handle
	retrieveHandle retrieveHandleFn

	// h is the current handle and may be nil
	h *DriverHandle

	// shutdownCtx is used to cancel retries if the agent is shutting down
	shutdownCtx context.Context

	logger log.Logger
	sync.Mutex
}

// NewLazyHandle takes the function to receive the latest handle and a logger
// and returns a LazyHandle
func NewLazyHandle(shutdownCtx context.Context, fn retrieveHandleFn, logger log.Logger) *LazyHandle {
	return &LazyHandle{
		retrieveHandle: fn,
		h:              fn(),
		shutdownCtx:    shutdownCtx,
		logger:         logger.Named("lazy_handle"),
	}
}

// getHandle returns the current handle or retrieves a new one
func (l *LazyHandle) getHandle() (*DriverHandle, error) {
	l.Lock()
	defer l.Unlock()

	if l.h != nil {
		return l.h, nil
	}

	return l.refreshHandleLocked()
}

// refreshHandle retrieves a new handle
func (l *LazyHandle) refreshHandle() (*DriverHandle, error) {
	l.Lock()
	defer l.Unlock()
	return l.refreshHandleLocked()
}

// refreshHandleLocked retrieves a new handle and should be called with the lock
// held. It will retry to give the client time to restart the driver and restore
// the handle.
func (l *LazyHandle) refreshHandleLocked() (*DriverHandle, error) {
	for i := 0; i < retrieveFailureLimit; i++ {
		l.h = l.retrieveHandle()
		if l.h != nil {
			return l.h, nil
		}

		// Calculate the new backoff
		backoff := (1 << (2 * uint64(i))) * retrieveBackoffBaseline
		if backoff > retrieveBackoffLimit {
			backoff = retrieveBackoffLimit
		}

		l.logger.Debug("failed to retrieve handle", "backoff", backoff)

		select {
		case <-l.shutdownCtx.Done():
			return nil, l.shutdownCtx.Err()
		case <-time.After(backoff):
		}
	}

	return nil, fmt.Errorf("no driver handle")
}

func (l *LazyHandle) Exec(timeout time.Duration, cmd string, args []string) ([]byte, int, error) {
	h, err := l.getHandle()
	if err != nil {
		return nil, 0, err
	}

	// Only retry once
	first := true

TRY:
	out, c, err := h.Exec(timeout, cmd, args)
	if err == bstructs.ErrPluginShutdown && first {
		first = false

		h, err = l.refreshHandle()
		if err == nil {
			goto TRY
		}
	}

	return out, c, err
}

func (l *LazyHandle) Stats(ctx context.Context, interval time.Duration) (<-chan *cstructs.TaskResourceUsage, error) {
	h, err := l.getHandle()
	if err != nil {
		return nil, err
	}

	// Only retry once
	first := true

TRY:
	out, err := h.Stats(ctx, interval)
	if err == bstructs.ErrPluginShutdown && first {
		first = false

		h, err = l.refreshHandle()
		if err == nil {
			goto TRY
		}
	}

	return out, err
}
