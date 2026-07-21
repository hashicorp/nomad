// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dynamic

import "github.com/hashicorp/nomad/nomad/structs"

type dynamicPriorityWorkload struct {
	// id uniquely identifies this workload
	// and is set to the evaluation ID.
	id string

	tid                TenantID
	priority           int
	eval               *structs.Evaluation
	requestedResources *UsageList

	sizeAdjustment  int
	ageAdjustment   int
	usageAdjustment int

	// waitOnRestore signals this workload was previously popped off the
	// queue and is waiting to be placed. This is detected on restore
	// and this workload will be pushed to the front of the queue and
	// waited on before processing regular workloads.
	// By doing this, we can ensure at most 1 queue workloads blocked
	// due to resource contraints even in the event of queue restores.
	waitOnRestore bool
}

func (w *dynamicPriorityWorkload) GetEval() *structs.Evaluation {
	return w.eval
}

func (w *dynamicPriorityWorkload) WaitOnRestore() bool {
	return w.waitOnRestore
}

func (w *dynamicPriorityWorkload) SetEval(e *structs.Evaluation) {
	w.eval = e
}
