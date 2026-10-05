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
