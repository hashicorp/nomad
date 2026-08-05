// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/queues/passthrough"
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

const DefaultQueue = structs.NodePoolDefault

type QueueData struct {
	queue.Queue
	isDefaultQueue bool
}

type BatchQueueManager struct {
	queues      map[string]*QueueData
	defaultConf structs.BatchQueue
	broker      queue.Broker
	state       *state.StateStore
	enabled     atomic.Bool
	shutdownCtx context.Context
	mux         sync.Mutex
	logger      hclog.Logger
}

type QueueMgrOpt func(*BatchQueueManager)

// WithQueue allows passing in a queue in the constructor
func WithQueue(pool string, q queue.Queue) QueueMgrOpt {
	return func(b *BatchQueueManager) {
		b.queues[pool] = &QueueData{q, true}
	}
}

func NewBatchQueueMgr(ctx context.Context, defaultConf structs.BatchQueue, broker queue.Broker, logger hclog.Logger, opt ...QueueMgrOpt) *BatchQueueManager {
	mgr := &BatchQueueManager{
		queues:      make(map[string]*QueueData),
		defaultConf: defaultConf,
		broker:      broker,
		shutdownCtx: ctx,
		mux:         sync.Mutex{},
		logger:      logger,
	}

	for _, fn := range opt {
		fn(mgr)
	}

	return mgr
}

// Enqueue takes an evaluation and passes it to the respective queue.
// Happens in Raft
func (b *BatchQueueManager) Enqueue(e *structs.Evaluation) {
	b.mux.Lock()
	defer b.mux.Unlock()

	if !b.enabled.Load() {
		return
	}

	// If an enqueue happens before SetEnabled = true, throw it away,
	// it will be processed during eval restore
	if b.state == nil {
		return
	}

	job, err := b.state.JobByID(nil, e.Namespace, e.JobID)
	if err != nil {
		return
	}

	q, ok := b.queues[job.NodePool]
	if !ok {
		// If there was an error creating a queue and it does not
		// exist, just pass the job to the eval broker for.
		b.broker.Enqueue(e)
		return
	}
	q.Enqueue(e)
}

// SetEnabled is called during leadership transfers and is responsible for starting
// and stopping queues.
func (b *BatchQueueManager) SetEnabled(enabled bool, state *state.StateStore) {
	b.mux.Lock()
	defer b.mux.Unlock()

	if enabled {
		// already enabled is a noop
		if b.enabled.Load() {
			return
		}

		if b.state == nil {
			b.state = state
		}
		if err := b.startQueues(); err != nil {
			b.logger.Error("failed to start batch queues, batch jobs will be processed normally", "err", err)
		}
	} else {
		// stop default queue
		for p := range b.queues {
			b.stopQueue(p)
		}
		b.queues = make(map[string]*QueueData)
	}

	b.enabled.Store(enabled)
}

// Queue returns a pointer to a queue. This is used by RPC handlers
// to get the jobs or tenants in a queue.
func (b *BatchQueueManager) Queue(pool string) queue.Queue {
	b.mux.Lock()
	defer b.mux.Unlock()

	// if the queue is currently nil of some update
	// just return the default passthrough queue. This
	// is unlikely to happen, but guards against a nil
	// value being returned
	if b.queues == nil || b.queues[pool] == nil {
		return &passthrough.PassthroughQueue{}
	}

	return b.queues[pool]
}

// UpdateDefault updates all queues to use the new default_scheduler_config.batch_queue config.
func (b *BatchQueueManager) UpdateDefaultQueues() error {
	b.mux.Lock()
	defer b.mux.Unlock()

	if !b.enabled.Load() {
		return nil
	}

	// If a pool has the default_scheduler.batch_queue config, we should restart it
	for p, q := range b.queues {
		if q.isDefaultQueue {
			b.stopQueue(p)
		}
	}

	ws := memdb.NewWatchSet()
	pools, err := b.state.NodePools(ws, state.SortDefault)
	if err != nil {
		return err
	}

	for raw := pools.Next(); raw != nil; raw = pools.Next() {
		pool := raw.(*structs.NodePool)

		// Don't create a queue for "all" node pool
		if pool.Name == structs.NodePoolAll {
			continue
		}

		_, isDefault, err := b.configForPool(pool)
		if err != nil {
			return err
		}

		if !isDefault {
			continue
		}
		if err := b.startQueue(pool); err != nil {
			return err
		}
	}

	return b.enqueuePending(withDefaultQueueFilter())
}

// UpdatePool updates an individual queues to use the provided config
func (b *BatchQueueManager) UpdateQueue(conf *structs.NodePool) error {
	b.mux.Lock()
	defer b.mux.Unlock()

	if !b.enabled.Load() {
		return nil
	}

	// stop previously running queue
	b.stopQueue(conf.Name)

	// restart queue with new config
	if err := b.startQueue(conf); err != nil {
		return err
	}

	// enqueue any previously pending evaluations
	return b.enqueuePending(withNodePoolFilter(conf.Name))
}

type filter func(name string, qdata *QueueData) bool

func withNodePoolFilter(pool string) filter {
	return func(name string, qdata *QueueData) bool {
		return pool == name
	}
}

func withDefaultQueueFilter() filter {
	return func(name string, qdata *QueueData) bool {
		return qdata.isDefaultQueue
	}
}

// enqueuePending takes any pending evaluations and enqueues them on the proper
// queue. When a queue is updates, it's state will be wiped. Queues can rebuild
// their internal state but do not currently rebuild their previously pending evals.
//
// This happens in leader.go during leadership transfer but queue updates are not
// related to leadership transfers, so the batch queue manager must do it.
func (b *BatchQueueManager) enqueuePending(filterFn filter) error {
	ws := memdb.NewWatchSet()
	iter, err := b.state.Evals(ws, state.SortDefault)
	if err != nil {
		return err
	}
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		eval, ok := raw.(*structs.Evaluation)
		if !ok {
			continue
		}

		// Skip non batch jobs
		if !eval.IsBatchQueue() {
			continue
		}

		job, err := b.state.JobByID(nil, eval.Namespace, eval.JobID)
		if err != nil {
			return err
		}

		q, ok := b.queues[job.NodePool]
		if !ok {
			b.broker.Enqueue(eval)
		}
		if !filterFn(job.NodePool, q) {
			continue
		}

		q.Enqueue(eval)
	}
	return nil
}

// startQueues encapsulates all the logic for starting every
// queue in the cluster.
func (b *BatchQueueManager) startQueues() error {
	ws := memdb.NewWatchSet()
	pools, err := b.state.NodePools(ws, state.SortDefault)
	if err != nil {
		return err
	}

	for raw := pools.Next(); raw != nil; raw = pools.Next() {
		pool := raw.(*structs.NodePool)

		// Don't create a queue for "all" node pool
		if pool.Name == structs.NodePoolAll {
			continue
		}

		if err = b.startQueue(pool); err != nil {
			return err
		}
	}
	return nil
}

// stopQueue is a helper for stopping a queue if it exists,
// and removing it from the queue map
func (b *BatchQueueManager) stopQueue(pool string) {
	if q, ok := b.queues[pool]; ok {
		q.Stop()
		delete(b.queues, pool)
	}
}

// startQueue is a helper for starting a queue for a given nodepool,
// and adding it to the queue map.
func (b *BatchQueueManager) startQueue(np *structs.NodePool) error {
	conf, isDefault, err := b.configForPool(np)
	if err != nil {
		return err
	}
	queue, err := NewQueue(b.state, conf, b.broker, b.logger)
	if err != nil {
		return err
	}

	if err := queue.Start(b.shutdownCtx); err != nil {
		return err
	}

	b.queues[np.Name] = &QueueData{queue, isDefault}

	return nil
}
