// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

var (
	// TODO: move these into the test?
	enabledConfig = &structs.BatchQueueConfig{
		Fifo: &structs.FifoQueueConfig{},
	}
	poolWithoutConfig = &structs.NodePool{
		Name: "pool-wo-config",
	}
	poolWithConfig = &structs.NodePool{
		Name:             "pool-with-config",
		BatchQueueConfig: enabledConfig,
	}
)

func testJobEval(pool *structs.NodePool, jobType, status string) (*structs.Evaluation, *structs.Job) {
	job := mock.MinJob()
	job.ID = fmt.Sprintf("%s-%s-job-in-%s", status, jobType, pool.Name)
	job.Type = jobType
	job.NodePool = pool.Name

	eval := mock.Eval()
	eval.TriggeredBy = structs.EvalTriggerJobRegister
	eval.Type = job.Type
	eval.JobID = job.ID
	eval.Namespace = job.Namespace
	eval.Status = status

	return eval, job
}

func testStateStore(t *testing.T) *state.StateStore {
	t.Helper()

	store := state.TestStateStore(t)
	must.NoError(t, store.UpsertNodePools(structs.MsgTypeTestSetup, 1, []*structs.NodePool{
		poolWithConfig, poolWithoutConfig,
	}))

	evalWithQueue, jobWithQueue := testJobEval(poolWithConfig, structs.JobTypeBatch, structs.EvalStatusPending)
	evalWithoutQueue, jobWithoutQueue := testJobEval(poolWithoutConfig, structs.JobTypeBatch, structs.EvalStatusPending)

	// TODO: non-batch, non-pending evals

	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 1, nil, jobWithQueue))
	must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 1, nil, jobWithoutQueue))
	must.NoError(t, store.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{
		evalWithQueue, evalWithoutQueue,
	}))

	return store
}

func TestBatchQueueManager_Disable(t *testing.T) {
	qm := NewBatchQueueMgr(t.Context(), testlog.HCLogger(t), &MockBroker{})

	must.False(t, qm.enabled.Load(), must.Sprint("should be disabled by default"))
	qm.SetEnabled(false, nil) // disable again does nothing

	t.Run("dont do stuff", func(t *testing.T) {
		// no errors from the mock broker is good
		qm.Enqueue(&structs.Evaluation{ID: "eval", JobID: "job"})

		qm.UpdateQueue(poolWithConfig)
		must.MapEmpty(t, qm.qk.m, must.Sprint("should not create any queues"))
	})

	t.Run("set disabled", func(t *testing.T) {
		// enable so we can test disable behavior; there are more thorough tests
		// in Test..Enabled, so here we fake that out.
		qm.enabled.Store(true)
		poolQ := &MockQueue{name: "disable-me"}
		poolQ.On("Stop").Once()
		qm.qk.Set("my-pool", poolQ, false)

		qm.SetEnabled(false, nil)

		must.False(t, qm.enabled.Load(), must.Sprint("should be disabled"))

		poolQ.AssertExpectations(t)
		must.MapEmpty(t, qm.qk.m, must.Sprint("should wipe queues"))
	})
}

func getNewQueueFn(q *MockQueue) newQueueFn {
	return func(_ hclog.Logger, _ *state.StateStore, _ *structs.BatchQueueConfig, _ queue.Broker) queue.Queue {
		return q
	}
}

func TestBatchQueueMgr_Enable(t *testing.T) {
	// set up state
	store := state.TestStateStore(t)

	must.NoError(t, store.UpsertNodePools(structs.MsgTypeTestSetup, 1, []*structs.NodePool{
		poolWithConfig, poolWithoutConfig,
	}))

	// TODO: another node pool...
	evalWithQueue, jobWithQueue := testJobEval(poolWithConfig, structs.JobTypeBatch, structs.EvalStatusPending)
	evalWithoutQueue, jobWithoutQueue := testJobEval(poolWithoutConfig, structs.JobTypeBatch, structs.EvalStatusPending)

	// non pending eval should be restored, non batch eval should be ignored
	nonPendingEval, nonPendingJob := testJobEval(poolWithConfig, structs.JobTypeBatch, structs.EvalStatusBlocked)
	nonBatchEval, nonBatchJob := testJobEval(poolWithConfig, structs.JobTypeService, structs.EvalStatusPending)

	for _, j := range []*structs.Job{
		jobWithQueue, jobWithoutQueue, nonPendingJob, nonBatchJob,
	} {
		must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, 1, nil, j))
	}
	must.NoError(t, store.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{
		evalWithQueue, evalWithoutQueue, nonPendingEval, nonBatchEval,
	}))

	// queue manager

	broker := &MockBroker{}
	// broker.Test(t)

	qm := NewBatchQueueMgr(t.Context(), testlog.HCLogger(t), broker)

	// run tests!

	t.Log("enabling the queue manager should create queues, restore evals, then start queues")
	{
		passthrough := &MockQueue{name: "passthru"}
		passthrough.Test(t)
		passthrough.On("Enqueue", evalWithoutQueue.ID, evalWithoutQueue.JobID).Once()
		qm.passthrough = passthrough

		startingQ := &MockQueue{name: "startingQ"}
		startingQ.Test(t)
		enqueue := startingQ.On("Enqueue", evalWithQueue.ID, evalWithQueue.JobID).Once()
		restore := startingQ.On("Restore", nonPendingEval.ID, nonPendingEval.JobID).Once()
		// Start should not run before enqueue and restore are done.
		startingQ.On("Start").Once().NotBefore(enqueue, restore)
		qm.newQueueFn = getNewQueueFn(startingQ)

		qm.SetEnabled(true, store)

		must.True(t, qm.enabled.Load(), must.Sprint("should be enabled"))

		// broker.AssertExpectations(t)
		must.True(t, passthrough.AssertExpectations(t))
		must.True(t, startingQ.AssertExpectations(t))

		must.MapLen(t, 1, qm.qk.m, must.Sprint("queue keeper should have one queue, for the enabled pool"))
		must.Eq(t, "startingQ", qm.Queue(poolWithConfig.Name).Type())
		must.Eq(t, "passthru", qm.Queue(poolWithoutConfig.Name).Type())
	}

	t.Log("repeat enable should be a no-op")
	{
		// should not be called again
		qm.newQueueFn = nil

		// reset mocks
		poolQ := &MockQueue{name: "mockQ"}
		poolQ.Test(t)
		qm.qk.Set(poolWithConfig.Name, poolQ, false)

		// no errors from unexpected mock calls is good
		qm.SetEnabled(true, store)
	}

	t.Log("enqueue should go to the correct queue")
	{
		passthrough := &MockQueue{name: "passthru"}
		passthrough.Test(t)
		passthrough.On("Enqueue", evalWithoutQueue.ID, evalWithoutQueue.JobID).Once()
		qm.passthrough = passthrough

		qm.Enqueue(evalWithoutQueue)

		must.True(t, passthrough.AssertExpectations(t))

		poolQ := &MockQueue{name: "poolQ"}
		poolQ.Test(t)
		poolQ.On("Enqueue", evalWithQueue.ID, evalWithQueue.JobID).Once()
		qm.qk.Set(poolWithConfig.Name, poolQ, false)

		qm.Enqueue(evalWithQueue)
		must.True(t, poolQ.AssertExpectations(t))
	}

	t.Log("update removing config should stop and delete queue")
	{
		// enabled at first
		must.NotEq(t, "passthru", qm.Queue(poolWithConfig.Name).Type())

		update := poolWithConfig.Copy()
		update.BatchQueueConfig = nil

		poolQ := &MockQueue{name: "poolQ"}
		poolQ.Test(t)
		poolQ.On("Stop").Once()
		qm.qk.Set(poolWithConfig.Name, poolQ, false)

		must.NoError(t, qm.UpdateQueue(update))

		must.True(t, poolQ.AssertExpectations(t))

		after := qm.Queue(update.Name)
		must.Eq(t, "passthru", after.Type())
	}

	t.Log("update adding config should enable queue")
	{
		// disabled at first
		must.Eq(t, "passthru", qm.Queue(poolWithoutConfig.Name).Type())

		update := poolWithoutConfig.Copy()
		update.BatchQueueConfig = enabledConfig

		poolQ := &MockQueue{name: "newQ"}
		poolQ.Test(t)
		enqueue := poolQ.On("Enqueue", evalWithoutQueue.ID, evalWithoutQueue.JobID).Once()
		poolQ.On("Start").Once().NotBefore(enqueue)
		qm.newQueueFn = getNewQueueFn(poolQ)

		must.NoError(t, qm.UpdateQueue(update))

		must.True(t, poolQ.AssertExpectations(t))

		q := qm.Queue(update.Name)
		must.Eq(t, "newQ", q.Type())
	}

	t.Log("changing config should replace queue")
	{
		// TODO: add checks for handling "all" pool, and only restart on change

		// this existing queue should be stopped and replaced
		oldQ := &MockQueue{name: "oldQ"}
		oldQ.Test(t)
		oldQ.On("Stop").Once()
		qm.qk.Set(poolWithConfig.Name, oldQ, false)

		// expect the new queue to be restored from state
		poolQ := &MockQueue{name: "newQ"}
		poolQ.Test(t)
		enqueue := poolQ.On("Enqueue", evalWithQueue.ID, evalWithQueue.JobID).Once()
		restore := poolQ.On("Restore", nonPendingEval.ID, nonPendingEval.JobID).Once()
		poolQ.On("Start").Once().NotBefore(enqueue, restore)
		qm.newQueueFn = getNewQueueFn(poolQ)

		// update the queue
		must.NoError(t, qm.UpdateQueue(poolWithConfig))

		must.True(t, oldQ.AssertExpectations(t))
		must.True(t, poolQ.AssertExpectations(t))

		q := qm.Queue(poolWithConfig.Name)
		must.Eq(t, "newQ", q.Type())
	}
}
