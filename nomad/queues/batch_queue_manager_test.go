// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/uuid"
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

	t.Run("enqueues on matching node pool queue", func(t *testing.T) {
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, hclog.Default())

		mgr.enabled.Store(true)
		ss := state.TestStateStore(t)
		testNodePool := mock.NodePool()
		must.NoError(t, ss.UpsertNodePools(structs.MsgTypeTestSetup, 1, []*structs.NodePool{testNodePool}))
		mockJob := mock.Job()
		mockJob.NodePool = testNodePool.Name
		must.NoError(t, ss.UpsertJob(structs.MsgTypeTestSetup, 2, nil, mockJob))
		mgr.state = ss

		mockDefaultQueue := &MockQueue{}
		mockTestQueue := &MockQueue{}
		mockTestQueue.On("Enqueue", tmock.Anything).Return()
		mgr.queues = map[string]*QueueData{
			"default":         {mockDefaultQueue, true},
			testNodePool.Name: {mockTestQueue, false},
		}

		mockEval := &structs.Evaluation{
			JobID:     mockJob.ID,
			Namespace: mockJob.Namespace,
		}

		mgr.Enqueue(mockEval)
		must.Eq(t, len(mockDefaultQueue.Calls), 0)
		must.Eq(t, len(mockTestQueue.Calls), 1)
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

		must.NotNil(t, mgr.queues)
	})

	t.Run("stops queues when disabled", func(t *testing.T) {
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, hclog.Default())

		mockQueue := &MockQueue{}
		mockQueue.On("Stop").Return()
		mgr.queues = map[string]*QueueData{"default": {mockQueue, true}}

		mgr.SetEnabled(false, nil)

		must.Eq(t, len(mockQueue.Calls), 1)
		must.Eq(t, mockQueue.Calls[0].Method, "Stop")
	})
}

func TestBatchQueueManager_UpdateDefaultQueues(t *testing.T) {
	t.Run("returns early when not enabled", func(t *testing.T) {
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, hclog.Default())
		mgr.enabled.Store(false)

		err := mgr.UpdateDefaultQueues()
		must.NoError(t, err)
		must.MapEmpty(t, mgr.queues)
	})

	t.Run("stops and recreates default queues", func(t *testing.T) {
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, hclog.Default())
		mgr.enabled.Store(true)

		ss := state.TestStateStore(t)
		testPool := mock.NodePool()
		must.NoError(t, ss.UpsertNodePools(structs.MsgTypeTestSetup, 1, []*structs.NodePool{testPool}))
		mgr.state = ss

		mockQueue := &MockQueue{}
		mockQueue.On("Stop").Return()
		mgr.queues = map[string]*QueueData{testPool.Name: {mockQueue, true}}

		err := mgr.UpdateDefaultQueues()
		must.NoError(t, err)
		must.Eq(t, 1, len(mockQueue.Calls))
		must.NotNil(t, mgr.queues)
	})

	t.Run("re-enqueues batch queue evals", func(t *testing.T) {
		broker := &MockBroker{}
		broker.On("Enqueue", tmock.Anything).Return()
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, broker, hclog.Default())
		mgr.enabled.Store(true)

		ss := state.TestStateStore(t)
		testPool := mock.NodePool()
		must.NoError(t, ss.UpsertNodePools(structs.MsgTypeTestSetup, 1, []*structs.NodePool{testPool}))

		// Create a batch job
		job := mock.Job()
		job.Type = structs.JobTypeBatch
		job.NodePool = testPool.Name
		must.NoError(t, ss.UpsertJob(structs.MsgTypeTestSetup, 2, nil, job))

		// Create pending eval for job register
		batchEval := &structs.Evaluation{
			ID:          uuid.Generate(),
			JobID:       job.ID,
			Namespace:   job.Namespace,
			Type:        structs.JobTypeBatch,
			TriggeredBy: structs.EvalTriggerJobRegister,
			Status:      structs.EvalStatusPending,
		}
		// Create eval that is not a batch queue eval
		nonBatchEval := &structs.Evaluation{
			ID:          uuid.Generate(),
			JobID:       job.ID,
			Namespace:   job.Namespace,
			Type:        structs.JobTypeService,
			TriggeredBy: structs.EvalTriggerJobRegister,
			Status:      structs.EvalStatusPending,
		}
		must.NoError(t, ss.UpsertEvals(
			structs.MsgTypeTestSetup,
			3,
			[]*structs.Evaluation{batchEval, nonBatchEval}),
		)

		mgr.state = ss

		mockQueue := &MockQueue{}
		mockQueue.On("Stop").Return()
		mockQueue.On("Enqueue", tmock.Anything).Return()
		mgr.queues = map[string]*QueueData{testPool.Name: {mockQueue, true}}

		err := mgr.UpdateDefaultQueues()
		must.NoError(t, err)

		must.Eq(t, 1, len(mockQueue.Calls))
	})

	t.Run("only stops default scheduler conf queues", func(t *testing.T) {
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, hclog.Default())
		mgr.enabled.Store(true)

		ss := state.TestStateStore(t)
		defaultPool := mock.NodePool()
		defaultPool.Name = "default-pool"
		customPool := mock.NodePool()
		customPool.Name = "custom-pool"
		must.NoError(t, ss.UpsertNodePools(structs.MsgTypeTestSetup, 1, []*structs.NodePool{defaultPool, customPool}))
		mgr.state = ss

		mockDefaultQueue := &MockQueue{}
		mockDefaultQueue.On("Stop").Return()

		mockCustomQueue := &MockQueue{}

		mgr.queues = map[string]*QueueData{
			defaultPool.Name: {mockDefaultQueue, true},
			customPool.Name:  {mockCustomQueue, false},
		}

		err := mgr.UpdateDefaultQueues()
		must.NoError(t, err)

		// Verify Stop was called only on default queue
		must.Eq(t, 1, len(mockDefaultQueue.Calls))
		must.Eq(t, "Stop", mockDefaultQueue.Calls[0].Method)

		// Verify Stop was not called on custom queue
		must.Eq(t, 0, len(mockCustomQueue.Calls))
	})
}

func TestBatchQueueManager_UpdateQueue(t *testing.T) {
	t.Run("returns early when not enabled", func(t *testing.T) {
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, hclog.Default())
		mgr.enabled.Store(false)

		testPool := mock.NodePool()
		err := mgr.UpdateQueue(testPool)
		must.NoError(t, err)
		must.MapEmpty(t, mgr.queues)
	})

	t.Run("stops and recreates queue for specific pool", func(t *testing.T) {
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, nil, hclog.Default())
		mgr.enabled.Store(true)

		ss := state.TestStateStore(t)
		testPool := mock.NodePool()
		must.NoError(t, ss.UpsertNodePools(structs.MsgTypeTestSetup, 1, []*structs.NodePool{testPool}))
		mgr.state = ss

		mockQueue := &MockQueue{}
		mockQueue.On("Stop").Return()
		mgr.queues = map[string]*QueueData{testPool.Name: {mockQueue, false}}

		err := mgr.UpdateQueue(testPool)
		must.NoError(t, err)
		must.Eq(t, 1, len(mockQueue.Calls))
		must.NotNil(t, mgr.queues)
	})
}
