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
		testQueue := NewFifoQueue(ss, nil, &structs.BatchQueue{}, hclog.New(hclog.DefaultOptions))

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
		testQueue := NewFifoQueue(ss, nil, &structs.BatchQueue{}, hclog.New(hclog.DefaultOptions))

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
	q := NewFifoQueue(ss, broker, &structs.BatchQueue{}, hclog.New(hclog.DefaultOptions))

	ctx := t.Context()

	must.NoError(t, q.Start(ctx))

	job1 := mock.Job()
	eval1 := mock.Eval()
	eval1.Type = structs.JobTypeBatch
	eval1.Status = structs.EvalStatusComplete
	job2 := mock.Job()
	eval2 := mock.Eval()
	eval2.Type = structs.JobTypeBatch
	eval2.Status = structs.EvalStatusComplete

	ss.UpsertEvals(structs.MsgTypeTestSetup, 1, []*structs.Evaluation{eval1})
	ss.UpsertEvals(structs.MsgTypeTestSetup, 5, []*structs.Evaluation{eval2})

	q.Enqueue(eval1, job1)
	q.Enqueue(eval2, job2)

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

func TestFifoQueue_Jobs_WithStatus(t *testing.T) {
	t.Run("queued workloads have status and position", func(t *testing.T) {
		ss := state.TestStateStore(t)
		broker := newTestBroker()
		q := NewFifoQueue(ss, broker, &structs.BatchQueue{}, hclog.Default())

		// Directly push to queue without starting (to avoid dequeuing)
		eval1 := mock.Eval()
		eval2 := mock.Eval()
		eval1.CreateIndex = 1
		eval2.CreateIndex = 2

		q.qMux.Lock()
		q.queue.Push(newFifoWorkload(eval1))
		q.queue.Push(newFifoWorkload(eval2))
		q.qMux.Unlock()

		// Get jobs
		iter := q.Jobs(structs.SortByPriority)
		workloads := []*structs.Workload{}
		for {
			w := iter.Next()
			if w == nil {
				break
			}
			workloads = append(workloads, w.(*structs.Workload))
		}

		// Should have 2 workloads
		must.Eq(t, 2, len(workloads))

		// Both should be queued with positions 1 and 2
		must.Eq(t, "queued", workloads[0].Status)
		must.Eq(t, 1, workloads[0].Position)

		must.Eq(t, "queued", workloads[1].Status)
		must.Eq(t, 2, workloads[1].Position)
	})

	t.Run("in-progress workloads have placing status and position 0", func(t *testing.T) {
		ss := state.TestStateStore(t)
		broker := newTestBroker()
		q := NewFifoQueue(ss, broker, &structs.BatchQueue{}, hclog.Default())

		// Manually track workloads to simulate in-progress state
		eval1 := mock.Eval()
		eval2 := mock.Eval()
		eval3 := mock.Eval()

		w1 := newFifoWorkload(eval1)
		w2 := newFifoWorkload(eval2)
		w3 := newFifoWorkload(eval3)

		// Add one to queue
		q.qMux.Lock()
		q.queue.Push(w3)
		q.qMux.Unlock()

		// Track two as in-progress
		q.watcher.TrackPlacement(w1)
		q.watcher.TrackPlacement(w2)

		// Get jobs
		iter := q.Jobs(structs.SortByPriority)
		workloads := []*structs.Workload{}
		for {
			w := iter.Next()
			if w == nil {
				break
			}
			workloads = append(workloads, w.(*structs.Workload))
		}

		// Should have 3 workloads total (2 placing + 1 queued)
		must.Eq(t, 3, len(workloads))

		// Count by status
		placingCount := 0
		queuedCount := 0
		for _, wl := range workloads {
			if wl.Status == "placing" {
				placingCount++
				must.Eq(t, 0, wl.Position)
			} else if wl.Status == "queued" {
				queuedCount++
				must.True(t, wl.Position > 0)
			}
		}

		must.Eq(t, 2, placingCount)
		must.Eq(t, 1, queuedCount)
	})

	t.Run("completed placements are removed from in-progress", func(t *testing.T) {
		ss := state.TestStateStore(t)
		broker := newTestBroker()
		q := NewFifoQueue(ss, broker, &structs.BatchQueue{}, hclog.Default())

		eval := mock.Eval()
		w := newFifoWorkload(eval)

		// Track as in-progress
		q.watcher.TrackPlacement(w)

		// Should have 1 placing workload
		iter := q.Jobs(structs.SortByPriority)
		workloads := []*structs.Workload{}
		for {
			w := iter.Next()
			if w == nil {
				break
			}
			workloads = append(workloads, w.(*structs.Workload))
		}
		must.Eq(t, 1, len(workloads))
		must.Eq(t, "placing", workloads[0].Status)

		// Untrack (simulating completion)
		q.watcher.UntrackPlacement(w)

		// Should have no workloads now
		iter = q.Jobs(structs.SortByPriority)
		workloads = []*structs.Workload{}
		for {
			w := iter.Next()
			if w == nil {
				break
			}
			workloads = append(workloads, w.(*structs.Workload))
		}
		must.Eq(t, 0, len(workloads))
	})
}
