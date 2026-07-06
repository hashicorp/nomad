// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	tmock "github.com/stretchr/testify/mock"
)

func TestBatchQueueManager_Enqueue(t *testing.T) {
	t.Run("does not enqueue if not enabled", func(t *testing.T) {
		// Test will fail if an eval is given to the mock broker
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, hclog.Default())
		mgr.enabled.Store(false)
		mgr.Enqueue(&structs.Evaluation{})
	})

	t.Run("enqueues when enabled", func(t *testing.T) {
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, hclog.Default())

		mgr.enabled.Store(true)
		ss := state.TestStateStore(t)
		mockJob := mock.Job()
		must.NoError(t, ss.UpsertJob(structs.MsgTypeTestSetup, 1, nil, mockJob))
		mgr.state = ss

		mockQueue := &MockQueue{}
		mockQueue.On("Enqueue", tmock.Anything).Return()
		mgr.defaultQueue = mockQueue

		mockEval := &structs.Evaluation{
			JobID:     mockJob.ID,
			Namespace: mockJob.Namespace,
		}

		mgr.Enqueue(mockEval)
	})
}

func TestBatchQueueManager_SetEnabled(t *testing.T) {
	t.Run("creates queue when enabled", func(t *testing.T) {
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, hclog.Default())
		ss := state.TestStateStore(t)
		ss.UpsertNodePools(structs.MsgTypeTestSetup, 1, []*structs.NodePool{
			{
				Name: "test",
				SchedulerConfiguration: &structs.NodePoolSchedulerConfiguration{
					BatchQueue: structs.BatchQueue{
						Type: "test",
					},
				},
			},
		})

		mgr.SetEnabled(true, ss)

		must.NotNil(t, mgr.defaultQueue)
	})

	t.Run("stops queues when disabled", func(t *testing.T) {
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, hclog.Default())

		mockQueue := &MockQueue{}
		mockQueue.On("Stop").Return()
		mgr.defaultQueue = mockQueue

		mgr.SetEnabled(false, nil)

		must.Eq(t, len(mockQueue.Calls), 1)
		must.Eq(t, mockQueue.Calls[0].Method, "Stop")
	})
}

func TestBatchQueueManager_Update(t *testing.T) {
	t.Run("updates default queue given empty node pool", func(t *testing.T) {
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, hclog.Default())

		mockQueue := &MockQueue{}
		mockQueue.On("Stop").Return()
		mgr.defaultQueue = mockQueue

		before := mgr.defaultQueue
		mgr.Update(&structs.BatchQueue{})
		after := mgr.defaultQueue

		must.EqOp(t, before, after) // not enabled so should skip update

		mgr.enabled.Store(true) // set enabled so update happens
		before = mgr.defaultQueue
		mgr.Update(&structs.BatchQueue{})
		after = mgr.defaultQueue

		must.NotEqOp(t, before, after)
	})
}
