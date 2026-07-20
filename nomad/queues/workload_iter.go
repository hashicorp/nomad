// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

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

func (i *WorkloadIter) SortByJobId() {
	slices.SortFunc(i.Workloads, func(a, b structs.QueueWorkload) int {
		return cmp.Compare(a.GetID(), b.GetID())
	})
}

func (i *WorkloadIter) Sort(sortFn func(i, j any) int) {
	slices.SortFunc(i.Workloads, func(a, b structs.QueueWorkload) int {
		return sortFn(a, b)
	})
	i.index = 0
}

func (i *WorkloadIter) Next() any {
	if i.index >= len(i.Workloads) {
		return nil
	}
	w := i.Workloads[i.index]
	i.index++
	return w
}
