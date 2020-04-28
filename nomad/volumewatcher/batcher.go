package volumewatcher

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// VolumeUpdateBatcher is used to batch the updates for volume claims
type VolumeUpdateBatcher struct {
	// batch is the batching duration
	batch time.Duration

	// raft is used to actually commit the updates
	raft VolumeRaftEndpoints

	// workCh is used to pass evaluations to the daemon process
	workCh chan *updateWrapper

	// ctx is used to exit the daemon batcher
	ctx context.Context
}

// NewVolumeUpdateBatcher returns an VolumeUpdateBatcher that uses the
// passed raft endpoints to create the updates to volume claims, and
// exits the batcher when the passed exit channel is closed.
func NewVolumeUpdateBatcher(batchDuration time.Duration, raft VolumeRaftEndpoints, ctx context.Context) *VolumeUpdateBatcher {
	b := &VolumeUpdateBatcher{
		batch:  batchDuration,
		raft:   raft,
		ctx:    ctx,
		workCh: make(chan *updateWrapper, 10),
	}

	go b.batcher()
	return b
}

// CreateUpdate batches the volume claim update and returns a future
// that tracks the completion of the request.
func (b *VolumeUpdateBatcher) CreateUpdate(claims []structs.CSIVolumeClaimRequest) *BatchFuture {
	wrapper := &updateWrapper{
		claims: claims,
		f:      make(chan *BatchFuture, 1),
	}

	b.workCh <- wrapper
	return <-wrapper.f
}

type updateWrapper struct {
	claims []structs.CSIVolumeClaimRequest
	f      chan *BatchFuture
}

// batcher is the long lived batcher goroutine
func (b *VolumeUpdateBatcher) batcher() {
	var timerCh <-chan time.Time
	claims := make(map[string]structs.CSIVolumeClaimRequest)
	future := NewBatchFuture()
	for {
		select {
		case <-b.ctx.Done():
			// note: we can't flush here because we're likely no
			// longer the leader
			return
		case w := <-b.workCh:
			if timerCh == nil {
				timerCh = time.After(b.batch)
			}

			// de-dupe and store the claim update, and attach the future
			for _, upd := range w.claims {
				claims[upd.VolumeID+upd.RequestNamespace()] = upd
			}
			w.f <- future
		case <-timerCh:
			// Capture the future and create a new one
			f := future
			future = NewBatchFuture()

			// Create the batch request
			req := structs.CSIVolumeClaimBatchRequest{}
			for _, claim := range claims {
				req.Claims = append(req.Claims, claim)
			}

			// Upsert the claims in a go routine
			go f.Set(b.raft.UpsertVolumeClaims(&req))

			// Reset the claims list and timer
			claims = make(map[string]structs.CSIVolumeClaimRequest)
			timerCh = nil
		}
	}
}

// BatchFuture is a future that can be used to retrieve the index for
// the update or any error in the update process
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
