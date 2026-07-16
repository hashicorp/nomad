// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fifo

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

type testBroker struct {
	evalIDs chan string
}

func newTestBroker() *testBroker {
	return &testBroker{evalIDs: make(chan string, 32)}
}

func (b *testBroker) Enqueue(e *structs.Evaluation) {
	b.evalIDs <- e.ID
}

func TestFifoQueue_workloadSortFn(t *testing.T) {
	t.Run("wait_on_restore_workloads_are_prioritized", func(t *testing.T) {
		sortFn := workloadSortFn()
		sortedQ := queue.NewWorkloadQueue(sortFn)

		first := &fifoWorkload{eval: mock.Eval(), waitOnRestore: true}
		second := &fifoWorkload{eval: mock.Eval(), waitOnRestore: false}

		first.eval.CreateIndex = 3
		second.eval.CreateIndex = 1

		sortedQ.Push(second)
		sortedQ.Push(first)

		must.Eq(t, first, sortedQ.Pop().(*fifoWorkload))
		must.Eq(t, second, sortedQ.Pop().(*fifoWorkload))

	})

	t.Run("counter_orders_fifo_for_regular_workloads", func(t *testing.T) {
		sortFn := workloadSortFn()
		sortedQ := queue.NewWorkloadQueue(sortFn)

		first := &fifoWorkload{eval: mock.Eval()}
		second := &fifoWorkload{eval: mock.Eval()}

		first.eval.CreateIndex = 1
		second.eval.CreateIndex = 5

		sortedQ.Push(second)
		sortedQ.Push(first)

		must.Eq(t, first, sortedQ.Pop().(*fifoWorkload))
		must.Eq(t, second, sortedQ.Pop().(*fifoWorkload))

	})
}

func TestFifoQueue_restore(t *testing.T) {
	t.Run("unplaced workload is enqueued", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewFifoQueue(ss, nil, hclog.New(hclog.DefaultOptions))

		job := mock.Job()
		job.Type = structs.JobTypeBatch
		ss.UpsertJob(structs.MsgTypeTestSetup, 0, nil, job)

		testEval := mock.Eval()
		testEval.JobID = job.ID
		testEval.Namespace = job.Namespace
		testEval.Type = structs.JobTypeBatch
		testEval.TriggeredBy = structs.EvalTriggerJobRegister
		testEval.Status = structs.EvalStatusBlocked
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		snap, err := ss.Snapshot()
		must.NoError(t, err)

		err = testQueue.restore(snap)
		must.NoError(t, err)

		select {
		case w := <-testQueue.enqueueCh:
			must.Eq(t, testEval.ID, w.id)
			must.True(t, w.waitOnRestore)
		default:
			t.Fatal("expected workload in enqueueCh channel")
		}
	})

	t.Run("skips pending non-batch and non-register evals", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewFifoQueue(ss, nil, hclog.New(hclog.DefaultOptions))

		batchJob := mock.Job()
		batchJob.Type = structs.JobTypeBatch
		ss.UpsertJob(structs.MsgTypeTestSetup, 0, nil, batchJob)

		serviceJob := mock.Job()
		serviceJob.Type = structs.JobTypeService
		ss.UpsertJob(structs.MsgTypeTestSetup, 1, nil, serviceJob)

		pendingEval := mock.Eval()
		pendingEval.JobID = batchJob.ID
		pendingEval.Namespace = batchJob.Namespace
		pendingEval.Type = structs.JobTypeBatch
		pendingEval.TriggeredBy = structs.EvalTriggerJobRegister
		pendingEval.Status = structs.EvalStatusPending

		nonBatchEval := mock.Eval()
		nonBatchEval.JobID = serviceJob.ID
		nonBatchEval.Namespace = serviceJob.Namespace
		nonBatchEval.Type = structs.JobTypeService
		nonBatchEval.TriggeredBy = structs.EvalTriggerJobRegister
		nonBatchEval.Status = structs.EvalStatusBlocked

		nonRegisterEval := mock.Eval()
		nonRegisterEval.JobID = batchJob.ID
		nonRegisterEval.Namespace = batchJob.Namespace
		nonRegisterEval.Type = structs.JobTypeBatch
		nonRegisterEval.TriggeredBy = structs.EvalTriggerNodeUpdate
		nonRegisterEval.Status = structs.EvalStatusBlocked

		ss.UpsertEvals(structs.MsgTypeTestSetup, 2, []*structs.Evaluation{
			pendingEval,
			nonBatchEval,
			nonRegisterEval,
		})

		snap, err := ss.Snapshot()
		must.NoError(t, err)

		err = testQueue.restore(snap)
		must.NoError(t, err)
		must.Eq(t, 0, len(testQueue.enqueueCh))
	})
}

func TestFifoQueue_runConsumer_enqueueOrder(t *testing.T) {
	ss := state.TestStateStore(t)
	broker := newTestBroker()
	q := NewFifoQueue(ss, broker, hclog.New(hclog.DefaultOptions))

	ctx := t.Context()

	must.NoError(t, q.Start(ctx))

	eval1 := mock.Eval()
	eval1.Type = structs.JobTypeBatch
	eval1.Status = structs.EvalStatusComplete
	eval2 := mock.Eval()
	eval2.Type = structs.JobTypeBatch
	eval2.Status = structs.EvalStatusComplete

	ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval1})
	ss.UpsertEvals(structs.MsgTypeTestSetup, 5, []*structs.Evaluation{eval2})

	q.Enqueue(eval1)
	q.Enqueue(eval2)

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			if len(broker.evalIDs) != 2 {
				return fmt.Errorf("waiting for 2 enqueued evals")
			}
			return nil
		}),
		wait.Timeout(5*time.Second),
		wait.Gap(50*time.Millisecond),
	))

	first := <-broker.evalIDs
	second := <-broker.evalIDs

	must.Eq(t, eval1.ID, first)
	must.Eq(t, eval2.ID, second)
}
