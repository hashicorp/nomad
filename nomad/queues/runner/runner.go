// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package runner

import (
	"context"
	"errors"
	"maps"
	"slices"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Queue is implemented by queue implementations to store and order their
// workloads. Implementations must be safe for concurrent use.
//
// The runner only ever passes an implementation workloads that were created
// by its own NewWorkload, so implementations may type assert them to their
// concrete workload type.
type Queue interface {
	// NewWorkload builds an implementation-specific workload for the eval and
	// job. If it returns false, the eval does not belong in the queue and is
	// sent straight to the eval broker.
	NewWorkload(*structs.Evaluation, *structs.Job) (queue.Workload, bool)
	Push(queue.Workload)
	Pop() (queue.Workload, bool)
	Get(structs.NamespacedID) (queue.Workload, bool)
	Remove(structs.NamespacedID) (queue.Workload, bool)
	// Update atomically replaces the workload with the same ID, returning
	// the replaced workload. It returns false and does not insert the
	// workload if there is nothing to replace.

	// TODO: this doesnt really need to exist? Should Push just replace if
	// the workload already exists on the queue?
	Update(queue.Workload) (queue.Workload, bool)
	Type() structs.BatchQueueType
}

// Runnable may be implemented by a Queue that needs a background goroutine,
// for example to periodically re-prioritize its workloads.
type Runnable interface {
	Run(context.Context)
}

// Viewable may be implemented by a Queue to expose its contents.
type Viewable interface {
	// Jobs returns the queued workloads along with the given in-progress
	// workloads, which are no longer stored in the queue.
	Jobs(structs.SortOrder, []queue.Workload) *queue.WorkloadIter
	Tenants() structs.QueueTenantsResponse
}

var _ queue.QueueRunner = (*QueueRunner)(nil)

// QueueRunner implements queue.Queue for any Queue implementation.
type QueueRunner struct {
	queue Queue

	// enqueueCh is used to buffer workloads before they are processed by
	// the producer and pushed onto the queue.
	enqueueCh chan queue.Workload

	// qNotify allows for notifying the consumer that workloads have been
	// added to the queue.
	qNotify chan struct{}

	// restored holds workloads for evals that were already handed to the
	// eval broker before a leader election or queue update, but have not
	// finished scheduling. It is populated by Restore before Start, and
	// drained by the consumer before it processes the queue.
	restored []queue.Workload

	evalBroker   queue.Broker
	evalCancelFn queue.EvalCancelFn
	state        *state.StateStore
	watcher      *queue.WorkloadWatcher
	logger       hclog.Logger

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// New returns a QueueRunner around the given queue implementation.
func New(
	logger hclog.Logger,
	q Queue,
	broker queue.Broker,
	ss *state.StateStore,
	cancelFn queue.EvalCancelFn,
) *QueueRunner {
	return &QueueRunner{
		queue:        q,
		enqueueCh:    make(chan queue.Workload, 8192),
		qNotify:      make(chan struct{}, 1),
		evalBroker:   broker,
		evalCancelFn: cancelFn,
		state:        ss,
		watcher:      queue.NewWorkloadWatcher(ss, logger),
		logger:       logger,
	}
}

func (qr *QueueRunner) Type() structs.BatchQueueType { return qr.queue.Type() }

// Start runs the queue's goroutines. Any Restore calls must happen before
// Start.
func (qr *QueueRunner) Start(ctx context.Context) error {
	rCtx, cancel := context.WithCancel(ctx)
	qr.cancel = cancel

	if runner, ok := qr.queue.(Runnable); ok {
		qr.wg.Go(func() {
			runner.Run(rCtx)
		})
	}

	qr.wg.Go(func() {
		qr.runProducer(rCtx)
	})
	qr.wg.Go(func() {
		qr.runConsumer(rCtx)
	})

	return nil
}

func (qr *QueueRunner) Stop() {
	if qr.cancel != nil {
		qr.cancel()
	}
	qr.wg.Wait()
}

// Enqueue submits a pending eval to the queue. If the queue implementation
// does not accept the eval, it is sent directly to the eval broker.
func (qr *QueueRunner) Enqueue(e *structs.Evaluation, j *structs.Job) {
	wl, ok := qr.queue.NewWorkload(e, j)
	if !ok {
		qr.evalBroker.Enqueue(e)
		return
	}
	qr.enqueueCh <- wl
}

// Dequeue removes a job's workload from the queue, returning its eval.
func (qr *QueueRunner) Dequeue(id structs.NamespacedID) *structs.Evaluation {
	if wl, ok := qr.queue.Remove(id); ok {
		return wl.Eval()
	}
	return nil
}

// Restore takes a non-pending eval that was previously sent to the eval
// broker, and if it has not finished scheduling, records it so the consumer
// waits for its placement before sending any more evals to the broker.
//
// Restore must only be called before Start. It is not safe for concurrent
// use with the consumer goroutine, which owns the restored workloads once
// started.
func (qr *QueueRunner) Restore(e *structs.Evaluation, j *structs.Job) error {
	wl, ok := qr.queue.NewWorkload(e, j)
	if !ok {
		return nil
	}

	// this follows the eval chain and sets the latest eval on the workload.
	placed, err := qr.watcher.IsSchedulingComplete(wl)
	if err != nil {
		return err
	}

	if placed {
		return nil
	}

	qr.restored = append(qr.restored, wl)
	return nil
}

// Jobs returns the in-progress and queued workloads, if the queue
// implementation is Viewable.
func (qr *QueueRunner) Jobs(o structs.SortOrder) *queue.WorkloadIter {
	view, ok := qr.queue.(Viewable)
	if !ok {
		return &queue.WorkloadIter{}
	}

	inProgress := slices.Collect(maps.Values(qr.watcher.GetInProgressWorkloads()))
	return view.Jobs(o, inProgress)
}

// Tenants returns the queue's tenants, if the queue implementation is
// Viewable.
func (qr *QueueRunner) Tenants() structs.QueueTenantsResponse {
	if view, ok := qr.queue.(Viewable); ok {
		return view.Tenants()
	}
	return structs.QueueTenantsResponse{Type: qr.queue.Type()}
}

// runProducer pushes workloads onto the queue and notifies the consumer
// goroutine.
func (qr *QueueRunner) runProducer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case w := <-qr.enqueueCh:

			// skipQueue and cancelRedundent hold Nomad logic for processing
			// multiple evaluations for the same job, or what happens when
			// a newer version of a job is being pushed to the queue when
			// a previous version exists.
			if qr.skipQueue(w) || qr.cancelRedundant(w) {
				continue
			}

			qr.queue.Push(w)

			// Notify Workload consumer of new workload
			select {
			case qr.qNotify <- struct{}{}:
			default:
			}
		}
	}
}

// runConsumer first waits for any restored workloads to be placed, then pops
// the highest priority workloads off the queue one at a time, enqueues them
// onto the eval broker, and waits for them to be placed before continuing.
func (qr *QueueRunner) runConsumer(ctx context.Context) {
	// restored evals are already in the eval broker or blocked evals
	// tracker, so only wait on them.
	for _, w := range qr.restored {
		if !qr.waitForPlacement(ctx, w) {
			return
		}
	}
	qr.restored = nil

	for {
		select {
		case <-ctx.Done():
			return
		case <-qr.qNotify:
			// Pop a workload off the queue if available
			w, ok := qr.queue.Pop()
			if !ok {
				continue
			}

			qr.evalBroker.Enqueue(w.Eval())

			if !qr.waitForPlacement(ctx, w) {
				return
			}

			select {
			case qr.qNotify <- struct{}{}:
			default:
			}
		}
	}
}

// waitForPlacement blocks until the workload finishes scheduling. It returns
// false if the context was canceled.
func (qr *QueueRunner) waitForPlacement(ctx context.Context, w queue.Workload) bool {
	err := qr.watcher.WaitForPlacement(ctx, w, memdb.NewWatchSet())
	if err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return false
		}
		qr.logger.Error("failure waiting for workload placement", "eval_id", w.Eval().ID, "error", err)
	}
	return true
}

// cancelRedundant checks if a workload with the same ID exists on the queue.
// If so, it keeps whichever job is newer, cancels the eval for the other one,
// and returns true.
func (qr *QueueRunner) cancelRedundant(w queue.Workload) bool {
	existing, ok := qr.queue.Get(w.ID())
	if !ok {
		return false
	}

	if w.JobVersion() > existing.JobVersion() {
		// Update removes and replaces the workload in the same locking
		// transaction.
		replaced, ok := qr.queue.Update(w)
		if !ok {
			// the existing workload left the queue since Get, so there is
			// nothing to replace and w should be pushed as normal.
			return false
		}
		qr.cancelEval(replaced.Eval())
	} else {
		qr.cancelEval(w.Eval())
	}

	return true
}

func (qr *QueueRunner) cancelEval(e *structs.Evaluation) {
	if err := qr.evalCancelFn(e); err != nil {
		qr.logger.Error("failed to cancel redundant eval", "eval_id", e.ID, "error", err)
	}
}

// skipQueue lets the caller know if an alloc already exists for the job, or we are currently watching the job.
// In these situations, we don't want to enqueue the workload, and the caller should directly enqueue the workload
// on the eval broker.
func (qr *QueueRunner) skipQueue(w queue.Workload) bool {
	e := w.Eval()
	j, err := qr.state.JobByIDAndVersion(nil, e.Namespace, e.JobID, w.JobVersion())
	if err != nil {
		qr.logger.Error("failed to get job by version", "job_id", e.JobID, "error", err)
	}
	if j == nil {
		return false
	}
	allocs, _ := qr.state.AllocsByJob(nil, e.Namespace, e.JobID, true)
	for _, a := range allocs {
		if a.Job.Version == j.Version {
			qr.evalBroker.Enqueue(w.Eval())
			return true
		}
	}

	// TODO this does not check the workload we are currently "watching"
	return false
}
