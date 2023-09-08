// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocwatcher

import (
	"context"
	"sync"

	multierror "github.com/hashicorp/go-multierror"

	"github.com/hashicorp/nomad/client/config"
)

type groupPrevAllocWatcher struct {
	prevAllocs []config.PrevAllocWatcher
	wg         sync.WaitGroup

	// waiting and migrating are true when alloc runner is waiting on the
	// prevAllocWatcher. Writers must acquire the waitingLock and readers
	// should use the helper methods IsWaiting and IsMigrating.
	waiting     bool
	waitingLock sync.RWMutex
}

func NewGroupAllocWatcher(watchers ...config.PrevAllocWatcher) config.PrevAllocWatcher {
	return &groupPrevAllocWatcher{
		prevAllocs: watchers,
	}
}

// Wait on the previous allocs to become terminal, exit, or, return due to
// context termination. Usage of the groupPrevAllocWatcher requires that all
// sub-watchers correctly handle context cancellation.
// We may need to adjust this to use channels rather than a wait group, if we
// wish to more strictly enforce timeouts.
func (g *groupPrevAllocWatcher) Wait(ctx context.Context) error {
	g.waitingLock.Lock()
	g.waiting = true
	g.waitingLock.Unlock()
	defer func() {
		g.waitingLock.Lock()
		g.waiting = false
		g.waitingLock.Unlock()
	}()

	var merr multierror.Error
	var errmu sync.Mutex

	g.wg.Add(len(g.prevAllocs))

	for _, alloc := range g.prevAllocs {
		go func(ctx context.Context, alloc config.PrevAllocWatcher) {
			defer g.wg.Done()
			err := alloc.Wait(ctx)
			if err != nil {
				errmu.Lock()
				merr.Errors = append(merr.Errors, err)
				errmu.Unlock()
			}
		}(ctx, alloc)
	}

	g.wg.Wait()

	// Check ctx.Err first, to avoid returning an mErr of ctx.Err from prevAlloc
	// Wait routines.
	if err := ctx.Err(); err != nil {
		return err
	}

	return merr.ErrorOrNil()
}

func (g *groupPrevAllocWatcher) IsWaiting() bool {
	g.waitingLock.RLock()
	defer g.waitingLock.RUnlock()

	return g.waiting
}
