// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	tmock "github.com/stretchr/testify/mock"
)

type MockBroker struct {
	tmock.Mock
}

func (m *MockBroker) Enqueue(e *structs.Evaluation) {
	m.Called(e)
}

func TestBatchQueueManager_Enqueue(t *testing.T) {
	t.Run("does not enqueue if not enabled", func(t *testing.T) {
		// Test will fail if an eval is given to the mock broker
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, &MockBroker{}, hclog.Default())
		mgr.enabled.Store(false)
		mgr.Enqueue(&structs.Evaluation{})
	})

	t.Run("enqueues on node pool queue if available", func(t *testing.T) {
		mockBroker := &MockBroker{}
		mockBroker.On("Enqueue", tmock.Anything).Return()
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, mockBroker, hclog.Default())

		mgr.enabled.Store(true)
		ss := state.TestStateStore(t)
		mockJob := mock.Job()
		must.NoError(t, ss.UpsertJob(structs.MsgTypeTestSetup, 1, nil, mockJob))
		mgr.state = ss

		mgr.nodePoolQueues[mockJob.NodePool] = NewPassthroughQueue(mockBroker)
		// give a not setup mock to the defaultQueue so test will fail if an eval
		// is given to this queue
		mgr.defaultQueue = NewPassthroughQueue(&MockBroker{})

		mockEval := &structs.Evaluation{
			JobID:     mockJob.ID,
			Namespace: mockJob.Namespace,
		}

		mgr.Enqueue(mockEval)
		must.Eq(t, len(mockBroker.Calls), 1)
	})

	t.Run("enqueues on default queue if no node pool queue", func(t *testing.T) {
		mockBroker := &MockBroker{}
		mockBroker.On("Enqueue", tmock.Anything).Return()
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, mockBroker, hclog.Default())

		mgr.enabled.Store(true)
		ss := state.TestStateStore(t)
		mockJob := mock.Job()
		must.NoError(t, ss.UpsertJob(structs.MsgTypeTestSetup, 1, nil, mockJob))
		mgr.state = ss

		// only setup the default queue, test will fail if it tries to add
		// to a specific nodePool queue
		mgr.defaultQueue = NewPassthroughQueue(mockBroker)

		mockEval := &structs.Evaluation{
			JobID:     mockJob.ID,
			Namespace: mockJob.Namespace,
		}

		mgr.Enqueue(mockEval)
		must.Eq(t, 1, len(mockBroker.Calls))
	})
}

func TestBatchQueueManager_SetEnabled(t *testing.T) {
	t.Run("creates queues when enabled", func(t *testing.T) {
		mockBroker := &MockBroker{}
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, mockBroker, hclog.Default())
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
		must.Eq(t, 1, len(mgr.nodePoolQueues))
	})

	t.Run("stops queues when disabled", func(t *testing.T) {
		mockBroker := &MockBroker{}
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, mockBroker, hclog.Default())
		stopCh1 := make(chan struct{})
		stopCh2 := make(chan struct{})
		// DynamicPriorityQueue closes the stop chan on Stop(), so we can use it to assert stop is called
		_, cancel := context.WithCancel(t.Context())
		mgr.defaultQueue = &DynamicPriorityQueue{cancelCtx: cancel}
		mgr.nodePoolQueues["test"] = &DynamicPriorityQueue{}

		mgr.SetEnabled(false, nil)

		select {
		case <-stopCh1:
		case <-time.After(50 * time.Millisecond):
			t.FailNow()
		}

		select {
		case <-stopCh2:
		case <-time.After(50 * time.Millisecond):
			t.FailNow()
		}
	})
}

func TestBatchQueueManager_Update(t *testing.T) {
	t.Run("updates default queue given empty node pool", func(t *testing.T) {
		mockBroker := &MockBroker{}
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, mockBroker, hclog.Default())
		_, cancel := context.WithCancel(t.Context())
		mgr.defaultQueue = &DynamicPriorityQueue{cancelCtx: cancel}
		before := mgr.defaultQueue
		mgr.Update("", &structs.BatchQueue{})
		after := mgr.defaultQueue

		must.EqOp(t, before, after) // not enabled so should skip update

		mgr.enabled.Store(true) // set enabled so update happens
		before = mgr.defaultQueue
		mgr.Update("", &structs.BatchQueue{})
		after = mgr.defaultQueue

		must.NotEqOp(t, before, after)
	})

	t.Run("updates specific queue given node pool", func(t *testing.T) {
		mockBroker := &MockBroker{}
		mgr := NewBatchQueueMgr(t.Context(), structs.BatchQueue{}, mockBroker, hclog.Default())
		_, cancel := context.WithCancel(t.Context())
		mgr.nodePoolQueues["test"] = &DynamicPriorityQueue{cancelCtx: cancel}

		before := mgr.nodePoolQueues["test"]
		mgr.Update("test", &structs.BatchQueue{})
		after := mgr.nodePoolQueues["test"]

		must.EqOp(t, before, after) // not enabled so should skip update

		mgr.enabled.Store(true) // set enabled so update happens
		before = mgr.nodePoolQueues["test"]
		mgr.Update("test", &structs.BatchQueue{})
		after = mgr.nodePoolQueues["test"]

		must.NotEqOp(t, before, after)
	})
}
