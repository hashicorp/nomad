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
