// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestBatchQueueManager_configForPool(t *testing.T) {
	t.Run("returns default config when state config is nil", func(t *testing.T) {
		// Setup manager with default config
		defaultConf := structs.BatchQueue{Type: "default-type"}
		mgr := NewBatchQueueMgr(t.Context(), defaultConf, nil, hclog.Default())

		ss := state.TestStateStore(t)
		mgr.state = ss

		// Create a test node pool
		testPool := &structs.NodePool{Name: "test-pool"}

		// Call configForPool
		conf, isDefault, err := mgr.configForPool(testPool)

		must.NoError(t, err)
		must.True(t, isDefault)
		must.Eq(t, "default-type", conf.Type)
	})

	t.Run("returns state config when available", func(t *testing.T) {
		// Setup manager
		defaultConf := structs.BatchQueue{Type: "default-type"}
		mgr := NewBatchQueueMgr(t.Context(), defaultConf, nil, hclog.Default())

		ss := state.TestStateStore(t)
		// Insert scheduler config with batch queue settings
		schedConf := &structs.SchedulerConfiguration{
			BatchQueue: structs.BatchQueue{Type: "state-type"},
		}
		must.NoError(t, ss.SchedulerSetConfig(1, schedConf))
		mgr.state = ss

		// Create a test node pool
		testPool := &structs.NodePool{Name: "test-pool"}

		// Call configForPool
		conf, isDefault, err := mgr.configForPool(testPool)

		must.NoError(t, err)
		must.True(t, isDefault)
		must.Eq(t, "state-type", conf.Type)
	})
}
