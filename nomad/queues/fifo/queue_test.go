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

		first := &fifoWorkload{BaseWorkload: queue.NewBaseWorkload(mock.Eval(), mock.Job())}
		first.SetWaitOnRestore(true)
		second := &fifoWorkload{BaseWorkload: queue.NewBaseWorkload(mock.Eval(), mock.Job())}

		first.Eval().CreateIndex = 3
		second.Eval().CreateIndex = 1

		sortedQ.Push(second)
		sortedQ.Push(first)

		must.Eq(t, first, sortedQ.Pop())
		must.Eq(t, second, sortedQ.Pop())
	})

	t.Run("counter_orders_fifo_for_regular_workloads", func(t *testing.T) {
		sortFn := workloadSortFn()
		sortedQ := queue.NewWorkloadQueue(sortFn)

		// first := &fifoWorkload{eval: mock.Eval()}
		// second := &fifoWorkload{eval: mock.Eval()}
		first := &fifoWorkload{BaseWorkload: queue.NewBaseWorkload(mock.Eval(), mock.Job())}
		second := &fifoWorkload{BaseWorkload: queue.NewBaseWorkload(mock.Eval(), mock.Job())}

		first.Eval().CreateIndex = 1
		second.Eval().CreateIndex = 5

		sortedQ.Push(second)
		sortedQ.Push(first)

		must.Eq(t, first, sortedQ.Pop())
		must.Eq(t, second, sortedQ.Pop())
	})
}

func TestFifoQueue_restore(t *testing.T) {
	t.Run("restore enqueues unplaced workload", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewFifoQueue(hclog.New(hclog.DefaultOptions), ss, nil, nil)

		job := mock.Job()
		job.Type = structs.JobTypeBatch
		must.NoError(t, ss.UpsertJob(structs.MsgTypeTestSetup, 0, nil, job))

		testEval := mock.Eval()
		testEval.JobID = job.ID
		testEval.Namespace = job.Namespace
		testEval.Type = structs.JobTypeBatch
		testEval.TriggeredBy = structs.EvalTriggerJobRegister
		testEval.Status = structs.EvalStatusBlocked
		must.NoError(t, ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval}))

		err := testQueue.Restore(testEval, job)
		must.NoError(t, err)

		select {
		case w := <-testQueue.enqueueCh:
			must.Eq(t, testEval.ID, w.Eval().ID)
			must.True(t, w.WaitOnRestore())
		default:
			t.Fatal("expected workload in enqueueCh channel")
		}
	})

	t.Run("restore does not enqueue placed workload", func(t *testing.T) {
		ss := state.TestStateStore(t)
		testQueue := NewFifoQueue(hclog.New(hclog.DefaultOptions), ss, nil, nil)

		// NOTE: eval type/status filtering is handled by the BatchQueueManager
		// restore path; FifoQueue.Restore only decides whether a workload is
		// already placed and should be skipped.
		job := mock.Job()
		job.Type = structs.JobTypeBatch
		ss.UpsertJob(structs.MsgTypeTestSetup, 0, nil, job)

		testEval := mock.Eval()
		testEval.JobID = job.ID
		testEval.Namespace = job.Namespace
		testEval.Type = structs.JobTypeBatch
		testEval.TriggeredBy = structs.EvalTriggerJobRegister
		testEval.Status = structs.EvalStatusComplete
		ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{testEval})

		err := testQueue.Restore(testEval, job)
		must.NoError(t, err)

		select {
		case w := <-testQueue.enqueueCh:
			t.Fatalf("expected no workload in enqueueCh, got eval %q", w.ID().ID)
		default:
		}
	})
}

func TestFifoQueue_runConsumer_enqueueOrder(t *testing.T) {
	ss := state.TestStateStore(t)
	broker := newTestBroker()
	q := NewFifoQueue(hclog.New(hclog.DefaultOptions), ss, broker, nil)

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

	// For the purposes of this test, the job doesn't matter. It just can't be nil.
	q.Enqueue(eval1, mock.Job())
	q.Enqueue(eval2, mock.Job())

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
