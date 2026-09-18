// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dynamic

import "github.com/hashicorp/nomad/nomad/structs"

type Tenant struct {
	tid                TenantID
	placedWorkloadById map[structs.NamespacedID]*dynamicPriorityWorkload
	totalUsage         *ResourceUsage
}

func (t *Tenant) totalPercentageUsed(totalUsage *ResourceUsage) int {
	if totalUsage.Total() == 0 {
		return 0
	}

	return int((t.totalUsage.Total() / totalUsage.Total()) * 100)
}
