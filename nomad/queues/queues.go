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

func NewQueue(ss *state.StateStore, sconf *structs.BatchQueue, broker queue.Broker, logger hclog.Logger) (queue.Queue, error) {
	switch sconf.Type {
	case structs.BatchQueueTypeDynamic:
		qconf := &structs.DynamicQueueConfig{}
		if err := structs.DecodeBatchQueueConf(sconf.Config, qconf); err != nil {
			return nil, err
		}
		return dynamic.NewDynamicPriorityQueue(ss, broker, sconf, qconf, logger), nil
	case structs.BatchQueueTypeFifo:
		return fifo.NewFifoQueue(ss, broker, logger), nil
	default:
		return passthrough.NewPassthroughQueue(broker), nil
	}
}
