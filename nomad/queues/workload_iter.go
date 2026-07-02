package queues

import (
	"cmp"
	"slices"

	"github.com/hashicorp/nomad/nomad/structs"
)

type WorkloadIter struct {
	Workloads []structs.QueueWorkload
	index     int
}

func NewWorkloadIter(workloads []structs.QueueWorkload) *WorkloadIter {
	return &WorkloadIter{
		Workloads: workloads,
		index:     0,
	}
}

func (i *WorkloadIter) SortByCreateIndex() {
	slices.SortFunc(i.Workloads, func(a, b structs.QueueWorkload) int {
		if a.GetCreateIndex() == b.GetCreateIndex() {
			return cmp.Compare(a.GetID(), b.GetID())
		}
		return cmp.Compare(a.GetCreateIndex(), b.GetCreateIndex())
	})
}

func (i *WorkloadIter) Sort(sortFn func(i, j interface{}) int) {
	slices.SortFunc(i.Workloads, func(a, b structs.QueueWorkload) int {
		return sortFn(a, b)
	})
	i.index = 0
}

func (i *WorkloadIter) Next() interface{} {
	if i.index >= len(i.Workloads) {
		return nil
	}
	w := i.Workloads[i.index]
	i.index++
	return w
}
