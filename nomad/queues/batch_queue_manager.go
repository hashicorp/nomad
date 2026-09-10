// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/queues/passthrough"
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type BatchQueueManager struct {
	// qk keeps track of the queues for each node pool.
	qk *queueKeeper
	// broker is passed to queues to forward to when ready.
	broker queue.Broker
	// passthrough is a fallback queue that passes evals straight to the broker.
	passthrough queue.Queue

	// enabled and state are set when the server runs SetEnabled(true)
	enabled atomic.Bool
	state   *state.StateStore

	// newQueueFn is called to create a new queue, to be overridden in tests.
	newQueueFn newQueueFn

	shutdownCtx context.Context
	logger      hclog.Logger

	// coarse lock mainly to prevent concurrent enqueue/update calls.
	mut sync.Mutex
}

// newQueueFn matches the signature of NewQueue
type newQueueFn func(hclog.Logger, *state.StateStore, *structs.BatchQueueConfig, queue.Broker) queue.Queue

type QueueMgrOpt func(*BatchQueueManager)

// NewBatchQueueMgr returns a BatchQueueManager. It must be enabled via
// SetEnabled(true) before it will start processing jobs.
func NewBatchQueueMgr(ctx context.Context, logger hclog.Logger,
	broker queue.Broker, opt ...QueueMgrOpt) *BatchQueueManager {

	qm := &BatchQueueManager{
		qk:          newQueueKeeper(),
		broker:      broker,
		passthrough: passthrough.NewPassthroughQueue(broker),
		newQueueFn:  NewQueue,
		shutdownCtx: ctx,
		logger:      logger.Named("batch_queue"),
	}
	for _, fn := range opt {
		fn(qm)
	}
	return qm
}

// SetEnabled is called during leadership transfers to start and stop queues.
func (qm *BatchQueueManager) SetEnabled(enabled bool, state *state.StateStore) {
	if qm.enabled.Load() == enabled {
		return
	}

	// prevent concurrent calls to other methods.
	qm.mut.Lock()
	defer qm.mut.Unlock()

	if enabled {
		if err := qm.enable(state); err != nil {
			qm.logger.Warn("failed to enable batch queues, batch jobs will be processed normally", "err", err)
			qm.qk.Wipe()
			return
		}
	} else {
		// eval broker will be shutting down, too, so no need to flush queues.
		// we're just reclaiming some memory here.
		qm.qk.Wipe()
	}

	qm.enabled.Store(enabled)
}

// enable creates, restores, and starts all queues. caller should hold a lock.
func (qm *BatchQueueManager) enable(store *state.StateStore) error {
	if store == nil {
		return errors.New("empty state (this is a bug)")
	}
	qm.state = store

	// set up queues for each node pool.
	if err := qm.initQueues(); err != nil {
		qm.qk.Wipe()
		return fmt.Errorf("init queue error: %w", err)
	}

	// now that the queues are set up, restore evals into them.
	if err := qm.restoreAllQueues(); err != nil {
		qm.qk.Wipe()
		return fmt.Errorf("restore queue error: %w", err)
	}

	for _, q := range qm.qk.Iter() {
		q.Start(qm.shutdownCtx)
	}
	return nil
}

// Enqueue takes a pending evaluation, finds its job, and passes them to the
// appropriate queue for the job's node pool.
func (qm *BatchQueueManager) Enqueue(e *structs.Evaluation) {
	// If an enqueue somehow happens before SetEnabled = true, throw it away;
	// it will be processed during eval restore.
	if e == nil || !qm.enabled.Load() {
		return
	}

	// lock prevents evals being somehow dropped during queue changes
	qm.mut.Lock()
	defer qm.mut.Unlock()

	job, err := qm.state.JobByID(nil, e.Namespace, e.JobID)
	// the eval broker probably can't do anything if the job can't be found,
	// but we send the eval anyway out of an abundance of caution.
	if err != nil {
		qm.logger.Error("couldn't get job to enqueue, passing to eval broker", "eval_id", e.ID)
		qm.broker.Enqueue(e)
		return
	}
	if job == nil {
		qm.logger.Error("job for eval not found to enqueue, passing to eval broker", "eval_id", e.ID)
		qm.broker.Enqueue(e)
		return
	}

	qm.Queue(job.NodePool).Enqueue(e, job)
}

func (qm *BatchQueueManager) Dequeue(job *structs.Job) *structs.Evaluation {
	if job == nil {
		return nil
	}

	if !qm.enabled.Load() {
		return nil
	}

	qm.mut.Lock()
	defer qm.mut.Unlock()

	return qm.Queue(job.NodePool).Dequeue(job)
}

// Queue returns a pointer to a queue. This is used by RPC handlers
// to get the jobs or tenants in a queue.
func (qm *BatchQueueManager) Queue(pool string) queue.Queue {
	if q, ok := qm.qk.Get(pool); ok {
		return q
	}
	// If a queue does not exist, pass through to eval broker.
	// This can happen with jobs that specify node_pool = "all"
	return qm.passthrough
}

// UpdateQueue updates an individual queue to use the provided config
// TODO: make sure this gets called on node register, and (flush and) delete the Q on node pool delete
func (qm *BatchQueueManager) UpdateQueue(pool *structs.NodePool) error {
	if !qm.enabled.Load() {
		return nil
	}

	qm.mut.Lock()
	defer qm.mut.Unlock()

	conf := configForPool(pool)

	// TODO: special handling for the "all" pool, become a "global" default config.
	// TODO: only restart queue if there's a change. (use Hash()?)

	// stop the queue with the old config to send evals straight to the broker.
	qm.qk.Stop(pool.Name)

	// drop the queue if the config has been removed from the node pool.
	if conf == nil || !conf.IsEnabled() {
		qm.qk.Delete(pool.Name)
		return nil
	}

	// make a new one
	queue := qm.newQueueFn(qm.logger, qm.state, conf, qm.broker)
	qm.qk.Set(pool.Name, queue, false)

	// restore from state
	if err := qm.restoreQueue(pool.Name); err != nil {
		qm.qk.Stop(pool.Name)
		qm.qk.Delete(pool.Name)
		return err
	}

	queue.Start(qm.shutdownCtx)

	return nil
}

func configForPool(p *structs.NodePool) *structs.BatchQueueConfig {
	if p.BatchQueueConfig != nil {
		return p.BatchQueueConfig
	}
	// TODO: fallback to all/global queue config, if set.
	return nil
}

// initQueues makes a new queue per node pool and store them in the queue keeper
func (qm *BatchQueueManager) initQueues() error {
	snap, err := qm.state.Snapshot()
	if err != nil {
		return err
	}
	pools, err := snap.NodePools(nil, state.SortDefault)
	if err != nil {
		return err
	}

	for raw := pools.Next(); raw != nil; raw = pools.Next() {
		pool := raw.(*structs.NodePool)

		// never create a queue for the special "all" node pool. later,
		// if we enqueue a job with the "all" pool, we won't find a queue,
		// and therefore send it straight to the eval broker.
		if pool.Name == structs.NodePoolAll {
			continue
		}

		conf := configForPool(pool)
		if conf == nil {
			continue
		}

		queue := qm.newQueueFn(qm.logger, qm.state, conf, qm.broker)
		qm.qk.Set(pool.Name, queue, false)
	}

	return nil
}

// restoreQueues restores all queues from state.
func (qm *BatchQueueManager) restoreAllQueues() error {
	// we should only do this on initial startup.
	if qm.enabled.Load() {
		return nil
	}
	return qm.restoreQueue("")
}

// restoreQueue runs Enqueue on pending evals and Restore on non-pending evals
// for a given pool's queue. If pool is empty, it will restore all queues.
func (qm *BatchQueueManager) restoreQueue(pool string) error {

	// TODO: iter jobs instead of evals, because we can't depend on the register-job eval existing in the state store.

	return qm.iterEvals(func(eval *structs.Evaluation, job *structs.Job) error {

		// skip evals on other pools.
		if pool != "" && job.NodePool != pool {
			return nil
		}

		q := qm.Queue(job.NodePool) // per-pool queue, or passthrough

		if eval.Status == structs.EvalStatusPending {
			q.Enqueue(eval, job)
		} else {
			// non-pending evals get "restored" (count resource usage, resume watching)
			if err := q.Restore(eval, job); err != nil {
				return err
			}
		}

		return nil
	})
}

// iterEvals executes fn for each batch queue evaluation in the state store.
func (qm *BatchQueueManager) iterEvals(fn func(*structs.Evaluation, *structs.Job) error) error {
	snap, err := qm.state.Snapshot()
	if err != nil {
		return err
	}
	evals, err := snap.Evals(nil, state.SortDefault)
	if err != nil {
		return err
	}

	for raw := evals.Next(); raw != nil; raw = evals.Next() {
		eval := raw.(*structs.Evaluation)

		if !eval.IsBatchQueue() {
			continue
		}

		job, err := snap.JobByID(nil, eval.Namespace, eval.JobID)
		if err != nil {
			return err
		}
		if job == nil { // job may be nil if it was purged
			continue
		}

		if err := fn(eval, job); err != nil {
			return err
		}
	}

	return nil
}
