// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package queues

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (b *BatchQueueManager) configForPool(_ *structs.NodePool) (*structs.BatchQueue, bool, error) {
	conf := b.defaultConf
	_, stateConf, err := b.state.SchedulerConfig()
	if err != nil {
		return nil, false, fmt.Errorf("failed to get scheduler config from state, skipping queue creation: %w", err)
	}

	if stateConf != nil {
		conf = stateConf.BatchQueue
	}

	return &conf, true, nil
}
