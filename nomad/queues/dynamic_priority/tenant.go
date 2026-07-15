// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dynamic

type Tenant struct {
	tid                TenantID
	placedWorkloadById map[string]*dynamicPriorityWorkload
	totalUsage         *ResourceUsage
}

func (t *Tenant) totalPercentageUsed(totalUsage *ResourceUsage) int {
	if totalUsage.Total() == 0 {
		return 0
	}

	return int((t.totalUsage.Total() / totalUsage.Total()) * 100)
}
