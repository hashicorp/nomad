// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dynamic

import (
	"github.com/hashicorp/nomad/nomad/queues/queue"
	"github.com/hashicorp/nomad/nomad/structs"
)

type DynamicPriorityWorkload struct {
	queue.BaseWorkload

	tid      TenantID
	priority int

	requestedResources *FairshareResources

	cpuAdjustment       int
	memAdjustment       int
	ageAdjustment       int
	fairshareAdjustment int
}

func (w *DynamicPriorityWorkload) ToStruct(pos int) *structs.DynamicPriorityWorkload {
	e := w.Eval()
	return &structs.DynamicPriorityWorkload{
		JobID:               e.JobID,
		Tenant:              string(w.tid),
		Status:              w.Status(),
		Namespace:           e.Namespace,
		Position:            pos,
		AdjustedPriority:    w.priority,
		BasePriority:        e.Priority,
		FairshareAdjustment: w.fairshareAdjustment,
		AgeAdjustment:       w.ageAdjustment,
		CpuAdjustment:       w.cpuAdjustment,
		MemoryAdjustment:    w.memAdjustment,
		CreatedAt:           e.CreateTime,
		CreateIndex:         e.CreateIndex,
	}
}
