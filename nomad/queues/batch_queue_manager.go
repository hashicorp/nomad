// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"context"
	"sync/atomic"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type BatchQueueManager struct {
	defaultQueue   Queue
	defaultConf    structs.BatchQueue
	nodePoolQueues map[string]Queue
	broker         Broker
	state          *state.StateStore
	enabled        atomic.Bool
	shutdownCtx    context.Context
	logger         hclog.Logger
}

func NewBatchQueueMgr(ctx context.Context, defaultConf structs.BatchQueue, broker Broker, logger hclog.Logger) *BatchQueueManager {
	return &BatchQueueManager{
		nodePoolQueues: make(map[string]Queue),
		defaultConf:    defaultConf,
		broker:         broker,
		shutdownCtx:    ctx,
		logger:         logger,
	}
}

// Enqueue takes an evaluation and passes it to the respective queue.
func (b *BatchQueueManager) Enqueue(e *structs.Evaluation) {
	if !b.enabled.Load() {
		return
	}

	// This shouldn't happen, but in the event we enqueue
	// an eval before setting state, just pass it to the broker
	if b.state == nil || b.defaultQueue == nil {
		b.broker.Enqueue(e)
		return
	}

	job, err := b.state.JobByID(nil, e.Namespace, e.JobID)
	if err != nil {
		b.logger.Error("batch queue failed to lookup job for eval", "evalID", e.ID, "err", err)
		b.broker.Enqueue(e)
		return
	}

	// if a node pool has a specific batch queue configuration, use that,
	// otherwise use the scheduler config queue.
	if queue, ok := b.nodePoolQueues[job.NodePool]; !ok {
		b.defaultQueue.Enqueue(e, job)
	} else {
		queue.Enqueue(e, job)
	}
}

func (b *BatchQueueManager) SetEnabled(enabled bool, state *state.StateStore) {
	if enabled {
		if b.state == nil {
			b.state = state
		}
		b.createQueues()
	} else {
		// stop default queue and any node pool queues
		b.defaultQueue.Stop()
		for _, q := range b.nodePoolQueues {
			q.Stop()
		}
	}

	b.enabled.Store(enabled)
}

// createQueues should create all queues for the server. It should always create a queue
// for the default scheduler config, but only create queues for node pool scheduler configs
// that have a valid batch queue configuration. This allows easy fallback to the default queue.
func (b *BatchQueueManager) createQueues() {
	defaultConf := b.defaultConf
	_, conf, err := b.state.SchedulerConfig()
	if err != nil {
		b.logger.Error("failed to get scheduler config from state, skipping queue creation", "err", err)
		return
	}

	if conf != nil {
		defaultConf = conf.BatchQueue
	}

	b.defaultQueue, err = NewQueue(b.state, &defaultConf, b.broker, b.logger)
	if err != nil {
		b.logger.Error("failed to create default batch queue", "err", err)
		return
	}
	b.defaultQueue.Start(b.shutdownCtx)

	nodePoolIter, err := b.state.NodePools(nil, state.SortDefault)
	if err != nil {
		b.logger.Error("failed to get node pools from state, skipping node pool queue creation", "err", err)
		return
	}

	for {
		raw := nodePoolIter.Next()
		if raw == nil {
			break
		}
		np := raw.(*structs.NodePool)

		conf := structs.BatchQueue{}
		if np.SchedulerConfiguration != nil {
			conf = np.SchedulerConfiguration.BatchQueue
		}

		// Do not create queue for empty node pool scheduler configs
		if conf.Type == "" {
			continue
		}

		queue, err := NewQueue(b.state, &conf, b.broker, b.logger)
		if err != nil {
			b.logger.Error("failed to create node pool queue", "err", err)
			return
		}

		b.nodePoolQueues[np.Name] = queue
		queue.Start(b.shutdownCtx)
	}
}

// Update is used to either update the default queue or a specific node pools queue.
func (b *BatchQueueManager) Update(nodePool string, conf *structs.BatchQueue) {
	if !b.enabled.Load() {
		return
	}

	queue, err := NewQueue(b.state, conf, b.broker, b.logger)
	if err != nil {
		b.logger.Error("failed to update batch queue", "err", err)
		return
	}

	if nodePool == "" {
		b.defaultQueue.Stop()
		b.defaultQueue = queue
	} else {
		b.nodePoolQueues[nodePool].Stop()
		b.nodePoolQueues[nodePool] = queue
	}

	queue.Start(b.shutdownCtx)
}
