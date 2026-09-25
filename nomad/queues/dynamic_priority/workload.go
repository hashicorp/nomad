// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dynamic

import (
	"github.com/hashicorp/nomad/nomad/queues/queue"
)

type dynamicPriorityWorkload struct {
	queue.BaseWorkload

	tid      TenantID
	priority int

	requestedResources *FairshareResources

	cpuAdjustment       int
	memAdjustment       int
	ageAdjustment       int
	fairshareAdjustment int
}
