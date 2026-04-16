package queues

import (
	"container/heap"
	"context"
	"errors"
	"sync"
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
	tenants map[TenantID]Tenant

	// queue is the main datastructure that contains all pending workloads
	queue WorkloadQueue

	// qMux locks the queue during concurrent access
	qMux *sync.Mutex

	// qNotify allows for notifying the consumer that workloads
	// have been added to the queue
	qNotify chan struct{}

	// enqueueCh is used to buffer workloads before they
	// are processed by the manager and pushed onto the queue
	enqueueCh chan *Workload

	// totalUsage is the sum of all tenant usages
	totalUsage int

	// conf contains user configurations for tuning the behavior of the queue
	conf *DynamicPriorityConfig

	// evalBroker is the injected broker for passing an evaluation
	// on to be scheduled by Nomad
	evalBroker Queue

	// state is the in-memory state store used for both reconciling tenant
	// workload usages, and polling submitted evaluations for placement
	state  *state.StateStore
	logger hclog.Logger
}

type DynamicPriorityConfig struct {
	TenantType   string
	MetadataKey  string
	CalcInterval time.Duration
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

func NewDynamicPriorityQueue(state *state.StateStore, broker Queue, conf *DynamicPriorityConfig, logger hclog.Logger) *DynamicPriorityQueue {
	return &DynamicPriorityQueue{
		tenants:    map[TenantID]Tenant{},
		queue:      WorkloadQueue{},
		state:      state,
		enqueueCh:  make(chan *Workload, 8096),
		evalBroker: broker,
		qMux:       &sync.Mutex{},
		qNotify:    make(chan struct{}, 1),
		conf:       conf,
		logger:     logger.Named("Dynamic Priority Queue"),
	}
}

func (d *DynamicPriorityQueue) Start(ctx context.Context) {
	// rebuild internal state from statestore, unimplemented

	go d.runManager(ctx)
	go d.runConsumer(ctx)
}

// Enqueue is used to produce a message on the queue by taking
func (d *DynamicPriorityQueue) Enqueue(e *structs.Evaluation) {
	w := d.generateWorkload(e)
	// in the event of an empty workload, just pass eval to eval broker
	if w == nil {
		d.evalBroker.Enqueue(e)
		return
	}

	d.enqueueCh <- w
}

// produce pushes workloads onto the queue and notifies the consumer
// goroutine. It also updates priorities on the configured interval.
func (d *DynamicPriorityQueue) runManager(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case w := <-d.enqueueCh:
			w.calculatePriority(w.eval.CreateTime)

			d.qMux.Lock()
			heap.Push(&d.queue, w)
			d.qMux.Unlock()

			// Notify Workload processor of new workload
			select {
			case d.qNotify <- struct{}{}:
			default:
			}
		case <-time.After(d.conf.CalcInterval):
			d.qMux.Lock()
			d.calculatePriorities(time.Now().UnixNano())
			heap.Init(&d.queue) // priorities were updated, reinit
			d.qMux.Unlock()
		}
	}
}

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
			d.waitForPlacement(ctx, workload.eval)

			d.qMux.Lock()
			l := d.queue.Len()
			d.qMux.Unlock()

			// If the queue still has work, notify it
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

// generateWorkload is used to create an initial workload from a given evaluation.
// It creates the tenantID from the queues config which is
func (d *DynamicPriorityQueue) generateWorkload(e *structs.Evaluation) *Workload {
	job, err := d.state.JobByID(nil, e.Namespace, e.JobID)
	if err != nil {
		return nil
	}

	tid := ""
	switch d.conf.TenantType {
	case "namespace":
		tid = job.Namespace
	case "metadata":
		tenantID, ok := job.Meta[d.conf.MetadataKey]
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
	// Decay tenant workload usages first
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
// TODO: search codebase to see if there is already an established way to do this.
func (d *DynamicPriorityQueue) waitForPlacement(ctx context.Context, eval *structs.Evaluation) error {
	for !eval.TerminalStatus() || eval.BlockedEval != "" || eval.NextEval != "" {
		id := eval.ID
		eval.TerminalStatus()

		if eval.BlockedEval != "" {
			id = eval.BlockedEval
		} else if eval.NextEval != "" {
			id = eval.NextEval
		}

		snap, err := d.state.Snapshot()
		if err != nil {
			return err
		}

		ws := memdb.NewWatchSet()
		eval, err = snap.EvalByID(ws, id)
		if err != nil {
			return err
		}
		if eval == nil {
			return ErrWatchedEvalNotFound
		}

		// Wait for the eval to be marked complete or context to cancel.
		// This keeps the loop from spinning.
		for !eval.TerminalStatus() {
			if err := ws.WatchCtx(ctx); err != nil {
				return err
			}
			snap, err = d.state.Snapshot()
			if err != nil {
				return err
			}
			ws = memdb.NewWatchSet()
			eval, err = snap.EvalByID(ws, eval.ID)
			if err != nil {
				return err
			}
			if eval == nil {
				return ErrWatchedEvalNotFound
			}
		}
	}

	return nil
}
