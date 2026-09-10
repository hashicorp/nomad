// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"github.com/hashicorp/go-hclog"
	dynamic "github.com/hashicorp/nomad/nomad/queues/dynamic_priority"
	"github.com/hashicorp/nomad/nomad/queues/fifo"
	"github.com/hashicorp/nomad/nomad/queues/passthrough"
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

func NewQueue(logger hclog.Logger, ss *state.StateStore, conf *structs.BatchQueueConfig, broker queue.Broker) queue.Queue {
	qType := structs.BatchQueueTypePassthrough
	if conf != nil {
		qType = conf.Type()
	}

	switch qType {
	case structs.BatchQueueTypeDynamic:
		return dynamic.NewDynamicPriorityQueue(logger, ss, broker, conf)
	case structs.BatchQueueTypeFifo:
		return fifo.NewFifoQueue(logger, ss, broker)
	}

	return passthrough.NewPassthroughQueue(broker)
}
