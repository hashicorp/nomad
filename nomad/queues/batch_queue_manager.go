// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/queues/passthrough"
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type BatchQueueManager struct {
	defaultQueue queue.Queue
	defaultConf  structs.BatchQueue
	broker       queue.Broker
	state        *state.StateStore
	enabled      atomic.Bool
	shutdownCtx  context.Context
	mux          sync.Mutex
	logger       hclog.Logger
}

type QueueMgrOpt func(*BatchQueueManager)

// WithQueue allows passing in a default queue in the constructor
func WithQueue(q queue.Queue) QueueMgrOpt {
	return func(b *BatchQueueManager) {
		b.defaultQueue = q
	}
}

func NewBatchQueueMgr(ctx context.Context, defaultConf structs.BatchQueue, broker queue.Broker, logger hclog.Logger, opt ...QueueMgrOpt) *BatchQueueManager {
	mgr := &BatchQueueManager{
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
	if b.state == nil || b.defaultQueue == nil {
		return
	}

	b.defaultQueue.Enqueue(e)
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
		b.startQueues()
	} else if b.defaultQueue != nil {
		// stop default queue
		b.defaultQueue.Stop()
		b.defaultQueue = nil
	}

	b.enabled.Store(enabled)
}

// Queue returns a pointer to a queue. This is used by RPC handlers
// to get the jobs or tenants in a queue.
func (b *BatchQueueManager) Queue() queue.Queue {
	b.mux.Lock()
	defer b.mux.Unlock()

	// if the queue is currently nil of some update
	// just return the default passthrough queue. This
	// is unlikely to happen, but guards against a nil
	// value being returned
	if b.defaultQueue == nil {
		return &passthrough.PassthroughQueue{}
	}

	return b.defaultQueue
}

// Update is used to either update the default queue or a specific node pools queue.
func (b *BatchQueueManager) Update(conf *structs.BatchQueue) {
	b.mux.Lock()
	defer b.mux.Unlock()

	if !b.enabled.Load() {
		return
	}

	if b.defaultQueue != nil {
		b.defaultQueue.Stop()
	}

	queue, err := NewQueue(b.state, conf, b.broker, b.logger)
	if err != nil {
		b.logger.Error("failed to update batch queue", "err", err)
		return
	}

	b.defaultQueue = queue
	b.defaultQueue.Start(b.shutdownCtx)
}

func (b *BatchQueueManager) startQueues() {
	defaultConf := b.defaultConf
	_, conf, err := b.state.SchedulerConfig()
	if err != nil {
		b.logger.Error("failed to get scheduler config from state, skipping queue creation", "err", err)
		return
	}

	if conf != nil {
		defaultConf = conf.BatchQueue
	}

	queue, err := NewQueue(b.state, &defaultConf, b.broker, b.logger)
	if err != nil {
		b.logger.Error("failed to create default batch queue", "err", err)
		return
	}

	b.defaultQueue = queue
	b.defaultQueue.Start(b.shutdownCtx)
}
