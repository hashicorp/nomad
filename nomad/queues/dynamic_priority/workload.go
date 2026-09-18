// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dynamic

import (
	"github.com/hashicorp/nomad/nomad/queues/queue"
)

type dynamicPriorityWorkload struct {
	queue.BaseWorkload
	// id uniquely identifies this workload
	// and is set to the evaluation ID.
	// id string

	tid      TenantID
	priority int

	requestedResources *UsageList

	cpuAdjustment   int
	memAdjustment   int
	ageAdjustment   int
	usageAdjustment int
}

func (w *dynamicPriorityWorkload) GetStatus() string {
	if w.description != "" {
		return fmt.Sprintf("%s (%s)", w.status, w.description)
	}
	return w.status
}

func (w *dynamicPriorityWorkload) SetStatus(s, description string) {
	w.status = s
	w.description = description
}

func (w *dynamicPriorityWorkload) toStruct(position int) *structs.DynamicPriorityWorkload {
	return &structs.DynamicPriorityWorkload{
		JobID:            w.eval.JobID,
		Tenant:           string(w.tid),
		Namespace:        w.eval.Namespace,
		Position:         position,
		Status:           w.GetStatus(),
		AdjustedPriority: w.priority,
		BasePriority:     w.eval.Priority,
		UsageAdjustment:  w.usageAdjustment,
		AgeAdjustment:    w.ageAdjustment,
		CpuAdjustment:    w.cpuAdjustment,
		MemoryAdjustment: w.memAdjustment,
		CreatedAt:        w.eval.CreateTime,
		CreateIndex:      w.eval.CreateIndex,
	}
}
