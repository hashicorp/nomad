// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package feasible

import "github.com/hashicorp/nomad/nomad/structs"

type customResourceChecker struct {
	ask       *structs.CustomResources
	proposed  *structs.CustomResources
	available *structs.CustomResources
}

func (crc *customResourceChecker) addProposed(proposed []*structs.Allocation) {
	for _, alloc := range proposed {
		for _, task := range alloc.AllocatedResources.Tasks {
			proposedResources := []*structs.CustomResource(*crc.proposed)
			proposedResources = append(proposedResources, task.Custom...)
		}
	}
}

func (crc *customResourceChecker) Select(ask *structs.CustomResources) error {
	crc.available.Subtract(crc.proposed)
	return ask.Select(*crc.available)
}
