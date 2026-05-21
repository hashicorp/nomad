// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"container/heap"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

var ErrWatchedEvalNotFound = errors.New("watched evaluation not found")

type TenantID string

type DynamicPriorityQueue struct {
	// tenants is used to keep track of cluster usage for this queue.
	// When workloads are placed or the  configured interval is passed,
	// cluster usage is updated for the workloads of each tenant.
	tenants map[TenantID]*Tenant

	// queue is the main datastructure that contains all pending workloads
	//
	// TODO: at the moment, this is using the go stdlib container/heap package,
	// but we may want to switch to treeset from Hashicorp's go-set.
	// Why? Both have O(logn) push/pop. Heap has constant time peeking, but
	// we don't use that. We do want to iterate over workloads quickly, which
	// we can do with a red-black tree.
	queue WorkloadQueue

	// qMux locks the queue during concurrent access
	qMux sync.Mutex

	// qNotify allows for notifying the consumer that workloads
	// have been added to the queue
	qNotify chan struct{}

	// enqueueCh is used to buffer workloads before they
	// are processed by the manager and pushed onto the queue
	enqueueCh chan *Workload

	// totalUsage is the sum of all tenant usages
	totalUsage int

	tenantType structs.BatchQueueTenant

	metadataKey string

	// conf contains user configurations for tuning the behavior of the queue
	conf *structs.DynamicQueueConfig

	// evalBroker is the injected broker for passing an evaluation
	// on to be scheduled by Nomad
	evalBroker Broker

	// enabled tracks whether the server running the batch job queue is the leader
	// so should process evaluations
	enabled atomic.Bool

	// state is the in-memory state store used for both reconciling tenant
	// workload usages, and polling submitted evaluations for placement
	state  *state.StateStore
	logger hclog.Logger
}

type Tenant struct {
	tid       TenantID
	workloads map[string]*Workload
	usage     int
}

type Workload struct {
	id       string
	tid      TenantID
	priority int
	eval     *structs.Evaluation
	size     int
	index    int
}

func (w *Workload) calculatePriority(_ int64) {
	// unimplemented
}

func NewDynamicPriorityQueue(broker Broker, qconf *structs.BatchQueue, conf *structs.DynamicQueueConfig, logger hclog.Logger) *DynamicPriorityQueue {
	return &DynamicPriorityQueue{
		tenants:     make(map[TenantID]*Tenant),
		queue:       WorkloadQueue{},
		enqueueCh:   make(chan *Workload, 8192),
		evalBroker:  broker,
		qMux:        sync.Mutex{},
		qNotify:     make(chan struct{}, 1),
		tenantType:  qconf.TenantType,
		metadataKey: qconf.MetadataKey,
		conf:        conf,
		logger:      logger.Named("Dynamic Priority Queue"),
		totalUsage:  0,
	}
}

func (d *DynamicPriorityQueue) Start(ctx context.Context) error {
	go d.runProducer(ctx)
	go d.runConsumer(ctx)

	return nil
}

func (d *DynamicPriorityQueue) SetEnabled(val bool, state *state.StateStore) {
	// rebuild internal state from statestore, unimplemented
	d.state = state
	d.enabled.Store(val)
}

// Enqueue is the method used to put evaluations on the queue.
// It generates a workload with an empty priority, appends it
// to an internal channel to be processed and added to the actual
// heap container.
func (d *DynamicPriorityQueue) Enqueue(e *structs.Evaluation) {
	if !d.enabled.Load() {
		return
	}

	w := d.generateWorkload(e)

	// in the event of an empty workload, just pass eval to eval broker
	if w == nil {
		d.evalBroker.Enqueue(e)
		return
	}

	d.enqueueCh <- w
}

// runProducer pushes workloads onto the queue and notifies the consumer
// goroutine. It also updates priorities on the configured interval.
func (d *DynamicPriorityQueue) runProducer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case w := <-d.enqueueCh:
			w.calculatePriority(w.eval.CreateTime)

			d.qMux.Lock()
			heap.Push(&d.queue, w)
			d.qMux.Unlock()

			// Notify Workload consumer of new workload
			select {
			case d.qNotify <- struct{}{}:
			default:
			}
		case <-time.After(d.conf.CalcInterval):
			if !d.enabled.Load() {
				continue
			}

			d.qMux.Lock()
			d.calculatePriorities(time.Now().UnixNano())
			heap.Init(&d.queue)
			d.qMux.Unlock()
		}
	}
}

// runConsumer pops the highest priority workloads off the queue one
// at a time, enqueues them onto the Eval Broker, and waits for them
// to be placed before continuing.
func (d *DynamicPriorityQueue) runConsumer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.qNotify:

			// Pop a workload off the queue if available
			d.qMux.Lock()
			workload := heap.Pop(&d.queue).(*Workload)
			d.qMux.Unlock()

			// Give the eval to the eval broker
			d.evalBroker.Enqueue(workload.eval)

			// Wait for the eval to be placed
			err := d.waitForPlacement(ctx, workload.eval, memdb.NewWatchSet())
			if err != nil {
				d.logger.Error("failure waiting for workload placement", "evalID", workload.eval)
			}

			d.qMux.Lock()
			l := d.queue.Len()
			d.qMux.Unlock()

			// If the queue still has work, notify self
			// to continue.
			if l > 0 {
				select {
				case d.qNotify <- struct{}{}:
				default:
				}
			}
		}
	}
}

// generateWorkload is used to create an initial workload from a given evaluation
func (d *DynamicPriorityQueue) generateWorkload(e *structs.Evaluation) *Workload {
	job, err := d.state.JobByID(nil, e.Namespace, e.JobID)
	if err != nil {
		return nil
	}

	tid := ""
	switch d.tenantType {
	case "namespace":
		tid = job.Namespace
	case "metadata":
		tenantID, ok := job.Meta[d.metadataKey]
		if !ok {
			return nil
		}
		tid = tenantID
	default:
		d.logger.Error("unknown tenant type, this is a bug.")
		return nil
	}

	return &Workload{
		tid:      TenantID(tid),
		priority: 0,
		eval:     e,
		size:     0,
	}
}

func (d *DynamicPriorityQueue) calculatePriorities(time int64) {
	// Decay tenant workload usages first, because a workload's
	// priority relies on its tenant's usage.
	for _, tenant := range d.tenants {
		for range tenant.workloads {
			// Unimplemented
			d.totalUsage -= 0
			tenant.usage -= 0
		}
	}

	// Now that we have accurate tenant usage, calculate
	// each workloads new priority
	for _, workload := range d.queue {
		workload.calculatePriority(time)
	}
}

// waitForPlacement follows a given evalutation in the state store until it, or it's nexted/blocked evals
// have been marked terminal, indicating the workload has been scheduled.
//
// Note: If a job with an unsatisfiable contraint is given to the Eval Broker, this function will block
// until a Nomad operator manually intervenes and stops the job. In the future, we can add an optional
// configurable timeout for this blocking query.
func (d *DynamicPriorityQueue) waitForPlacement(ctx context.Context, eval *structs.Evaluation, ws memdb.WatchSet) error {
	for !eval.TerminalStatus() || eval.BlockedEval != "" || eval.NextEval != "" {
		id := eval.ID

		if eval.BlockedEval != "" {
			id = eval.BlockedEval
		} else if eval.NextEval != "" {
			id = eval.NextEval
		}

		snap, err := d.state.Snapshot()
		if err != nil {
			return err
		}

		// TODO: handle snapshot restores
		abandonCh := snap.AbandonCh()
		ws.Add(abandonCh)

		eval, err = snap.EvalByID(ws, id)
		if err != nil {
			return err
		}
		if eval == nil {
			return ErrWatchedEvalNotFound
		}

		if eval.TerminalStatus() {
			continue
		}

		// If the latest version of the eval isn't terminal, wait for an update
		if err = ws.WatchCtx(ctx); err != nil {
			return err
		}

		// The watch channel will be closed, we should delete it to
		// prevent immediately firing on the next WatchCtx
		for k := range ws {
			delete(ws, k)
		}
	}

	return nil
}
