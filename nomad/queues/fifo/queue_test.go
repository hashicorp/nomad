// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package fifo

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestFifoQueue_workloadSortFn(t *testing.T) {
	sortedQ := queue.NewWorkloadQueue(workloadSortFn())

	first := newFifoWorkload(mock.Eval(), mock.Job())
	second := newFifoWorkload(mock.Eval(), mock.Job())

	first.Eval().CreateIndex = 1
	second.Eval().CreateIndex = 5

	sortedQ.Push(second)
	sortedQ.Push(first)

	got, ok := sortedQ.Pop()
	must.True(t, ok)
	must.Eq(t, first, got.(*fifoWorkload))
	got, ok = sortedQ.Pop()
	must.True(t, ok)
	must.Eq(t, second, got.(*fifoWorkload))
}

func TestFifoQueue_queueOps(t *testing.T) {
	q := New()

	newWorkload := func(createIndex, version uint64) *fifoWorkload {
		job := mock.BatchJob()
		job.Version = version
		eval := mock.Eval()
		eval.JobID = job.ID
		eval.CreateIndex = createIndex
		w, ok := q.NewWorkload(eval, job)
		must.True(t, ok)
		return w.(*fifoWorkload)
	}

	first := newWorkload(1, 0)
	second := newWorkload(2, 0)
	q.Push(second)
	q.Push(first)

	got, ok := q.Get(second.ID())
	must.True(t, ok)
	must.Eq[queue.Workload](t, second, got)

	secondV2 := newFifoWorkload(mock.Eval(), second.Job().Copy())
	secondV2.Eval().CreateIndex = 3
	replaced, ok := q.Update(secondV2)
	must.True(t, ok)
	must.Eq[queue.Workload](t, second, replaced)

	_, ok = q.Update(newWorkload(4, 0))
	must.False(t, ok, must.Sprint("update should not insert missing workloads"))

	got, ok = q.Pop()
	must.True(t, ok)
	must.Eq[queue.Workload](t, first, got)

	got, ok = q.Remove(second.ID())
	must.True(t, ok)
	must.Eq[queue.Workload](t, secondV2, got)

	got, ok = q.Pop()
	must.False(t, ok)
	must.True(t, got == nil, must.Sprint("expected untyped nil workload"))
}

func TestFifoQueue_Jobs(t *testing.T) {
	collect := func(iter *queue.WorkloadIter) []*structs.Workload {
		workloads := []*structs.Workload{}
		for w := iter.Next(); w != nil; w = iter.Next() {
			workloads = append(workloads, w.(*structs.Workload))
		}
		return workloads
	}

	t.Run("queued workloads have status and position", func(t *testing.T) {
		q := New()

		eval1 := mock.Eval()
		eval2 := mock.Eval()
		eval1.CreateIndex = 1
		eval2.CreateIndex = 2

		q.Push(newFifoWorkload(eval2, mock.Job()))
		q.Push(newFifoWorkload(eval1, mock.Job()))

		workloads := collect(q.Jobs(structs.SortByPriority, nil))
		must.Len(t, 2, workloads)

		must.Eq(t, eval1.JobID, workloads[0].JobID)
		must.Eq(t, queue.WorkloadStatusQueued, workloads[0].Status)
		must.Eq(t, 1, workloads[0].Position)

		must.Eq(t, eval2.JobID, workloads[1].JobID)
		must.Eq(t, queue.WorkloadStatusQueued, workloads[1].Status)
		must.Eq(t, 2, workloads[1].Position)
	})

	t.Run("in-progress workloads have position 0", func(t *testing.T) {
		q := New()

		w1 := newFifoWorkload(mock.Eval(), mock.Job())
		w2 := newFifoWorkload(mock.Eval(), mock.Job())
		w3 := newFifoWorkload(mock.Eval(), mock.Job())
		w1.SetStatus(queue.WorkloadStatusPlacing, "")
		w2.SetStatus(queue.WorkloadStatusPlacing, "")

		q.Push(w3)

		// workloads of other types are ignored
		other := &otherWorkload{BaseWorkload: queue.NewBaseWorkload(mock.Eval(), mock.Job(), queue.WorkloadStatusPlacing)}

		workloads := collect(q.Jobs(structs.SortByPriority, []queue.Workload{w1, other, w2}))
		must.Len(t, 3, workloads)

		must.Eq(t, queue.WorkloadStatusPlacing, workloads[0].Status)
		must.Eq(t, 0, workloads[0].Position)
		must.Eq(t, queue.WorkloadStatusPlacing, workloads[1].Status)
		must.Eq(t, 0, workloads[1].Position)
		must.Eq(t, queue.WorkloadStatusQueued, workloads[2].Status)
		must.Eq(t, 1, workloads[2].Position)
	})
}

type otherWorkload struct {
	queue.BaseWorkload
}
