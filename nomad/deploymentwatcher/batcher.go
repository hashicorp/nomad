// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package deploymentwatcher

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// AllocUpdateBatcher is used to batch the updates to the desired transitions
// of allocations and the creation of evals.
type AllocUpdateBatcher struct {
	// batch is the batching duration
	batch time.Duration

	// raft is used to actually commit the updates
	raft DeploymentRaftEndpoints

	// workCh is used to pass evaluations to the daemon process
	workCh chan *updateWrapper

	// ctx is used to exit the daemon batcher
	ctx context.Context
}

// NewAllocUpdateBatcher returns an AllocUpdateBatcher that uses the passed raft endpoints to
// create the allocation desired transition updates and new evaluations and
// exits the batcher when the passed exit channel is closed.
func NewAllocUpdateBatcher(ctx context.Context, batchDuration time.Duration, raft DeploymentRaftEndpoints) *AllocUpdateBatcher {
	b := &AllocUpdateBatcher{
		batch:  batchDuration,
		raft:   raft,
		ctx:    ctx,
		workCh: make(chan *updateWrapper, 10),
	}

	go b.batcher()
	return b
}

// CreateUpdate batches the allocation desired transition update and returns a
// future that tracks the completion of the request.
func (b *AllocUpdateBatcher) CreateUpdate(allocs map[string]*structs.DesiredTransition, eval *structs.Evaluation) *BatchFuture {
	wrapper := &updateWrapper{
		allocs: allocs,
		e:      eval,
		f:      make(chan *BatchFuture, 1),
	}

	b.workCh <- wrapper
	return <-wrapper.f
}

type updateWrapper struct {
	allocs map[string]*structs.DesiredTransition
	e      *structs.Evaluation
	f      chan *BatchFuture
}

// batcher is the long lived batcher goroutine
func (b *AllocUpdateBatcher) batcher() {
	var timerCh <-chan time.Time
	allocs := make(map[string]*structs.DesiredTransition)
	evals := make(map[string]*structs.Evaluation)
	future := NewBatchFuture()
	for {
		select {
		case <-b.ctx.Done():
			return
		case w := <-b.workCh:
			if timerCh == nil {
				timerCh = time.After(b.batch)
			}

			// Store the eval and alloc updates, and attach the future
			evals[w.e.DeploymentID] = w.e
			for id, upd := range w.allocs {
				allocs[id] = upd
			}

			w.f <- future
		case <-timerCh:
			// Capture the future and create a new one
			f := future
			future = NewBatchFuture()

			// Shouldn't be possible
			if f == nil {
				panic("no future")
			}

			// Create the request
			req := &structs.AllocUpdateDesiredTransitionRequest{
				Allocs: allocs,
				Evals:  make([]*structs.Evaluation, 0, len(evals)),
			}

			for _, e := range evals {
				req.Evals = append(req.Evals, e)
			}

			// Upsert the evals in a go routine
			go f.Set(b.raft.UpdateAllocDesiredTransition(req))

			// Reset the evals list and timer
			evals = make(map[string]*structs.Evaluation)
			allocs = make(map[string]*structs.DesiredTransition)
			timerCh = nil
		}
	}
}

// BatchFuture is a future that can be used to retrieve the index the eval was
// created at or any error in the creation process
type BatchFuture struct {
	index  uint64
	err    error
	waitCh chan struct{}
}

// NewBatchFuture returns a new BatchFuture
func NewBatchFuture() *BatchFuture {
	return &BatchFuture{
		waitCh: make(chan struct{}),
	}
}

// Set sets the results of the future, unblocking any client.
func (f *BatchFuture) Set(index uint64, err error) {
	f.index = index
	f.err = err
	close(f.waitCh)
}

// Results returns the creation index and any error.
func (f *BatchFuture) Results() (uint64, error) {
	<-f.waitCh
	return f.index, f.err
}
