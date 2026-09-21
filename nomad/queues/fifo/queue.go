// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fifo

import (
	"cmp"
	"context"
	"errors"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type FifoQueue struct {
	// This is using a TreeSet from Hashicorp's go-set module due to it's
	// ability for log(n) insert and delete and allows for Top(k) lookups
	queue queue.WorkloadQueue[*fifoWorkload]

	// evalBroker is the injected broker for passing an evaluation
	// on to be scheduled by Nomad
	evalBroker queue.Broker

	// state is the in-memory state store used for both reconciling tenant
	// workload usages, and polling submitted evaluations for placement
	state  *state.StateStore
	logger hclog.Logger

	// enqueueCh is used to buffer workloads before they
	// are processed by the manager and pushed onto the queue
	enqueueCh chan *fifoWorkload

	// qNotify allows for notifying the consumer that workloads
	// have been added to the queue
	qNotify chan struct{}

	evalCancelFn queue.EvalCancelFn

	cancel context.CancelFunc
	wg     sync.WaitGroup

	watcher *queue.WorkloadWatcher
}

func NewFifoQueue(
	logger hclog.Logger,
	ss *state.StateStore,
	broker queue.Broker,
	cancelFn queue.EvalCancelFn,
) *FifoQueue {
	return &FifoQueue{
		queue:        queue.NewWorkloadQueue(workloadSortFn()),
		enqueueCh:    make(chan *fifoWorkload, 8192),
		qNotify:      make(chan struct{}, 1),
		evalBroker:   broker,
		state:        ss,
		evalCancelFn: cancelFn,
		logger:       logger.Named("fifo_queue"),
		watcher:      queue.NewWorkloadWatcher(ss, logger),
	}
}

func workloadSortFn() func(i, j *fifoWorkload) int {
	return func(i, j *fifoWorkload) int {
		wait := queue.CmpWaitOnRestore(i, j)
		if wait != 0 {
			return wait
		}

		return cmp.Compare(i.Eval().CreateIndex, j.Eval().CreateIndex)
	}
}

func (f *FifoQueue) Enqueue(e *structs.Evaluation, j *structs.Job) {
	f.enqueueCh <- newFifoWorkload(e, j)
}

func (f *FifoQueue) Dequeue(id structs.NamespacedID) *structs.Evaluation {
	wl := f.queue.Remove(id)

	if wl != nil {
		return wl.Eval()
	}

	return nil
}

func (f *FifoQueue) Start(ctx context.Context) error {
	rCtx, cancel := context.WithCancel(ctx)
	f.cancel = cancel

	f.wg.Go(func() {
		f.runProducer(rCtx)
	})
	f.wg.Go(func() {
		f.runConsumer(rCtx)
	})

	return nil
}

func (f *FifoQueue) Stop() {
	f.cancel()
	f.wg.Wait()
}

func (f *FifoQueue) runProducer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case w := <-f.enqueueCh:

			// check if a workload with the same ID exists on the queue. If so,
			// either swap out the existing if the new one has a higher job version,
			// and cancel the existing's eval, or cancel the new workload's eval
			// if it has a job version that not greater than the existing one.
			existing, ok := f.queue.Get(w.ID())
			if ok {
				// use an update here because it removes and replaces the workload
				// in the same locking transaction.
				if w.JobVersion() > existing.JobVersion() {
					f.queue.UpdateByID(w)
					f.evalCancelFn(existing.Eval())
				} else {
					f.evalCancelFn(w.Eval())
				}
				continue
			}

			f.queue.Push(w)
			select {
			case f.qNotify <- struct{}{}:
			default:
			}
		}
	}
}

func (f *FifoQueue) runConsumer(ctx context.Context) {

	for {
		select {
		case <-ctx.Done():
			return
		case <-f.qNotify:
			w := f.queue.Pop()

			if !w.WaitOnRestore() {
				f.evalBroker.Enqueue(w.Eval())
			}

			err := f.watcher.WaitForPlacement(ctx, w, memdb.NewWatchSet())
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				f.logger.Error("failure waiting for workload placement", "evalID", w.Eval().ID)
			}

			l := f.queue.Len()

			if l > 0 {
				select {
				case f.qNotify <- struct{}{}:
				default:
				}
			}
		}
	}
}

func (f *FifoQueue) Restore(eval *structs.Evaluation, j *structs.Job) error {
	w := newFifoWorkload(eval, j)

	placed, err := f.watcher.IsSchedulingComplete(w)
	if err != nil {
		return err
	}
	if !placed {
		w.SetWaitOnRestore(true)
		f.enqueueCh <- w
	}
	return nil
}

func (f *FifoQueue) Type() structs.BatchQueueType {
	return structs.BatchQueueTypeFifo
}

func (f *FifoQueue) Jobs(sortOrder structs.SortOrder) *queue.WorkloadIter {
	pos := 0
	workloads := []structs.QueueWorkload{}

	for _, workload := range f.watcher.GetInProgressWorkloads() {
		w := workload.(*fifoWorkload)
		eval := w.Eval()
		workloads = append(workloads, &structs.Workload{
			JobID:       eval.JobID,
			Namespace:   eval.Namespace,
			Position:    0,
			Status:      w.Status(),
			CreatedAt:   eval.CreateTime,
			CreateIndex: eval.CreateIndex,
		})
	}

	f.queue.Iterate(func(workload queue.Workload) {
		w := workload.(*fifoWorkload)
		// waitOnRestore does not count towards position in queue
		if w.WaitOnRestore() {
			return
		}
		pos++

		eval := w.Eval()
		workloads = append(workloads, &structs.Workload{
			JobID:       eval.JobID,
			Namespace:   eval.Namespace,
			Position:    pos,
			Status:      w.Status(),
			CreatedAt:   eval.CreateTime,
			CreateIndex: eval.CreateIndex,
		})
	})

	iter := queue.NewWorkloadIter(workloads)

	if sortOrder != structs.SortByPriority {
		iter.SortByJobId()
	}

	return iter
}

func (f *FifoQueue) Tenants() structs.QueueTenantsResponse {
	return structs.QueueTenantsResponse{Type: structs.BatchQueueTypeFifo}
}
