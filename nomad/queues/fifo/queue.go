// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fifo

import (
	"cmp"
	"context"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type FifoQueue struct {
	// This is using a TreeSet from Hashicorp's go-set module due to it's
	// ability for log(n) insert and delete and allows for Top(k) lookups
	queue queue.WorkloadQueue

	// qMux locks the queue during concurrent access
	qMux sync.Mutex

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

	counter atomic.Uint64

	// qNotify allows for notifying the consumer that workloads
	// have been added to the queue
	qNotify chan struct{}

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewFifoQueue(ss *state.StateStore, broker queue.Broker, logger hclog.Logger) *FifoQueue {
	return &FifoQueue{
		queue:      queue.NewWorkloadQueue(workloadSortFn()),
		enqueueCh:  make(chan *fifoWorkload, 8192),
		qNotify:    make(chan struct{}, 1),
		evalBroker: broker,
		state:      ss,
		qMux:       sync.Mutex{},
		logger:     logger.Named("Fifo Queue"),
	}
}

func workloadSortFn() func(i, j queue.Workload) int {
	return func(i, j queue.Workload) int {
		a := i.(*fifoWorkload)
		b := j.(*fifoWorkload)

		wait := queue.CmpWaitOnRestore(a, b)
		if wait != 0 {
			return wait
		}

		return cmp.Compare(a.counter, b.counter)
	}
}

func (f *FifoQueue) Enqueue(e *structs.Evaluation) {
	f.enqueueCh <- newFifoWorkload(e)
}

func (f *FifoQueue) Start(ctx context.Context) error {
	rCtx, cancel := context.WithCancel(ctx)
	f.cancel = cancel

	snap, err := f.state.Snapshot()
	if err != nil {
		f.logger.Error("failed to get state snapshot", "err", err)
		return err
	}

	if err := f.restore(snap); err != nil {
		return err
	}

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
			f.qMux.Lock()
			w.counter = f.incrementCounter()
			f.queue.Push(w)
			f.qMux.Unlock()
			f.qNotify <- struct{}{}
		}
	}
}

func (f *FifoQueue) incrementCounter() uint64 {
	return f.counter.Add(1)
}

func (f *FifoQueue) runConsumer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-f.qNotify:
			f.qMux.Lock()
			w := f.queue.Pop()
			f.qMux.Unlock()

			f.evalBroker.Enqueue(w.GetEval())

			err := queue.WaitForPlacement(ctx, w, f.state, memdb.NewWatchSet())
			if err != nil {
				f.logger.Error("failure waiting for workload placement", "evalID", w.GetEval().ID)
			}

			f.qMux.Lock()
			l := f.queue.Len()
			f.qMux.Unlock()

			if l > 0 {
				select {
				case f.qNotify <- struct{}{}:
				default:
				}
			}
		}
	}
}

func (f *FifoQueue) restore(snap *state.StateSnapshot) error {
	f.qMux.Lock()
	defer f.qMux.Unlock()

	ws := memdb.NewWatchSet()
	iter, err := snap.Evals(ws, state.SortDefault)
	if err != nil {
		f.logger.Error("failed to get evals while enabling queue", "err", err)
		return err
	}

	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		eval, ok := raw.(*structs.Evaluation)
		if !ok {
			f.logger.Error("object from eval table not an eval")
			continue
		}

		// Skip non batch jobs
		if eval.Type != structs.JobTypeBatch {
			continue
		}
		// If the eval was not a job register, skip it
		if eval.TriggeredBy != structs.EvalTriggerJobRegister {
			continue
		}
		// Pending evals will be enqueued later in leadership transfer
		if eval.Status == structs.EvalStatusPending {
			continue
		}

		w := newFifoWorkload(eval)

		placed, err := queue.IsSchedulingComplete(w, f.state)
		if err != nil {
			f.logger.Error("failed to wait for placement while enabling queue", "err", err)
		}
		if !placed {
			w.waitOnRestore = true
			f.enqueueCh <- w
		}
	}

	return nil

}

func (f *FifoQueue) Type() structs.BatchQueueType {
	return structs.BatchQueueTypeFifo
}

func (f *FifoQueue) Jobs(sortOrder structs.SortOrder) *queue.WorkloadIter {
	f.qMux.Lock()
	sortedWorkloads := f.queue.Slice()
	defer f.qMux.Unlock()

	workloads := []structs.QueueWorkload{}
	for pos, workload := range sortedWorkloads {
		eval := workload.GetEval()
		workloads = append(workloads, &structs.Workload{
			JobID:       eval.JobID,
			Namespace:   eval.Namespace,
			Position:    pos + 1,
			CreatedAt:   eval.CreateTime,
			CreateIndex: eval.CreateIndex,
		})
	}
	iter := queue.NewWorkloadIter(workloads)

	if sortOrder != structs.SortByPriority {
		iter.SortByJobId()
	}

	return iter
}

func (f *FifoQueue) Tenants() structs.QueueTenantsResponse {
	return structs.QueueTenantsResponse{Type: structs.BatchQueueTypeFifo}
}
