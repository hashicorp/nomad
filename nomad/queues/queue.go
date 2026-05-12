// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

func NewQueue(state *state.StateStore, sconf *structs.BatchQueue, broker Broker, logger hclog.Logger) (Queue, error) {
	switch sconf.Type {
	case structs.BatchQueueTypeDynamic:
		qconf := &structs.DynamicQueueConfig{}
		if err := structs.DecodeBatchQueueConf(sconf.Config, qconf); err != nil {
			return nil, err
		}
		return NewDynamicPriorityQueue(state, broker, sconf, qconf, logger), nil
	default:
		return NewPassthroughQueue(broker), nil
	}
}
