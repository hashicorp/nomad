// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dynamic

import "github.com/hashicorp/nomad/nomad/structs"

type Tenant struct {
	tid                TenantID
	placedWorkloadById map[structs.NamespacedID]*dynamicPriorityWorkload
	fairshare          *FairshareResources
}

func (t *Tenant) totalPercentageUsed(total *FairshareResources) int {
	if total.Total() == 0 {
		return 0
	}

	return int((t.fairshare.Total() / total.Total()) * 100)
}
