// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

/*
 * T: solid, maintainable core
 * T: consistent, correct restore (also maintainable)
 * T: dynamic queue: split cpu/mem resource weights
 * T: visibility: CLI, metrics
 * S: fifo queue: opt in/out per job
 * S: front of line blocking options
 */

package queues

import (
	"github.com/hashicorp/go-hclog"
	dynamic "github.com/hashicorp/nomad/nomad/queues/dynamic_priority"
	"github.com/hashicorp/nomad/nomad/queues/fifo"
	"github.com/hashicorp/nomad/nomad/queues/passthrough"
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/queues/runner"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

func NewQueue(
	logger hclog.Logger,
	ss *state.StateStore,
	conf *structs.BatchQueueConfig,
	broker queue.Broker,
	pool string,
	cancelFn queue.EvalCancelFn,
) queue.QueueRunner {
	qType := structs.BatchQueueTypePassthrough
	if conf != nil {
		qType = conf.Type()
	}

	var queue runner.Queue
	switch qType {
	case structs.BatchQueueTypeDynamic:
		queue = dynamic.New(logger.Named("dynamic_priority_queue"), ss, conf.DynamicPriority, pool)
	case structs.BatchQueueTypeFifo:
		queue = fifo.New()
	default:
		return passthrough.NewPassthroughQueue(broker)
	}

	return runner.New(
		logger.Named("queue_runner"),
		queue,
		broker,
		ss,
		cancelFn,
	)
}
