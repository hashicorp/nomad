package volumewatcher

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

// encoding a 100 claim batch is about 31K on the wire, which
// is a reasonable batch size
const maxBatchSize = 100

// VolumeUpdateBatcher is used to batch the updates for volume claims
type VolumeUpdateBatcher struct {
	// batchDuration is the batching duration
	batchDuration time.Duration

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
		batchDuration: batchDuration,
		raft:          raft,
		ctx:           ctx,
		workCh:        make(chan *updateWrapper, 10),
	}

	go b.batcher()
	return b
}

// CreateUpdate batches the volume claim update and returns a future
// that can be used to track the completion of the batch. Note we
// only return the *last* future if the claims gets broken up across
// multiple batches because only the last one has useful information
// for the caller.
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

type claimBatch struct {
	claims map[string]structs.CSIVolumeClaimRequest
	future *BatchFuture
}

// batcher is the long lived batcher goroutine
func (b *VolumeUpdateBatcher) batcher() {
	id, start := uuid.Generate(), time.Now()
	fmt.Println("volume update batcher goroutine created", "id", id)
	defer fmt.Println("volume update batcher goroutine ended", "id", id, "duration", time.Since(start))

	// we track claimBatches rather than a slice of
	// CSIVolumeClaimBatchRequest so that we can deduplicate updates
	// for the same volume
	batches := []*claimBatch{{
		claims: make(map[string]structs.CSIVolumeClaimRequest),
		future: NewBatchFuture(),
	}}
	ticker := time.NewTicker(b.batchDuration)
	defer ticker.Stop()
	for {
		select {
		case <-b.ctx.Done():
			// note: we can't flush here because we're likely no
			// longer the leader
			return
		case w := <-b.workCh:
			future := NewBatchFuture()

		NEXT_CLAIM:
			// de-dupe and store the claim update, and attach the future
			for _, upd := range w.claims {
				id := upd.VolumeID + upd.RequestNamespace()

				for _, batch := range batches {
					// first see if we can dedupe the update
					_, ok := batch.claims[id]
					if ok {
						batch.claims[id] = upd
						future = batch.future
						continue NEXT_CLAIM
					}
					// otherwise append to the first non-full batch
					if len(batch.claims) < maxBatchSize {
						batch.claims[id] = upd
						future = batch.future
						continue NEXT_CLAIM
					}
				}
				// all batches were full, so add a new batch
				newBatch := &claimBatch{
					claims: map[string]structs.CSIVolumeClaimRequest{id: upd},
					future: NewBatchFuture(),
				}
				batches = append(batches, newBatch)
				future = newBatch.future
			}

			// we send batches to raft FIFO, so we return the last
			// future to the caller so that it can wait until the
			// last batch has been sent
			w.f <- future

		case <-ticker.C:
			if len(batches) > 0 && len(batches[0].claims) > 0 {
				batch := batches[0]

				f := batch.future

				// Create the batch request for the oldest batch
				req := structs.CSIVolumeClaimBatchRequest{}
				for _, claim := range batch.claims {
					req.Claims = append(req.Claims, claim)
				}

				// Upsert the claims in a go routine
				go f.Set(b.raft.UpsertVolumeClaims(&req))

				// Reset the batches list
				batches = batches[1:]
			}
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
