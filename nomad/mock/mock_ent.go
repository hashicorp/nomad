// +build ent

package mock

import (
	"fmt"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

func SentinelPolicy() *structs.SentinelPolicy {
	sp := &structs.SentinelPolicy{
		Name:             fmt.Sprintf("sent-policy-%s", uuid.Generate()),
		Description:      "Super cool policy!",
		EnforcementLevel: "advisory",
		Scope:            "submit-job",
		Policy:           "main = rule { true }",
		CreateIndex:      10,
		ModifyIndex:      20,
	}
	sp.SetHash()
	return sp
}

func QuotaSpec() *structs.QuotaSpec {
	qs := &structs.QuotaSpec{
		Name:        fmt.Sprintf("quota-spec-%s", uuid.Generate()),
		Description: "Super cool quota!",
		Limits: []*structs.QuotaLimit{
			{
				Region: "global",
				RegionLimit: &structs.Resources{
					CPU:      2000,
					MemoryMB: 2000,
				},
			},
			{
				Region: "europe",
				RegionLimit: &structs.Resources{
					CPU:      0,
					MemoryMB: 0,
				},
			},
		},
	}
	qs.SetHash()
	return qs
}

func QuotaUsage() *structs.QuotaUsage {
	qs := QuotaSpec()
	l1 := qs.Limits[0]
	l2 := qs.Limits[1]

	l1.RegionLimit.CPU = 4000
	l1.RegionLimit.MemoryMB = 5000
	l2.RegionLimit.CPU = 40000
	l2.RegionLimit.MemoryMB = 50000
	qs.SetHash()

	qu := &structs.QuotaUsage{
		Name: fmt.Sprintf("quota-usage-%s", uuid.Generate()),
		Used: map[string]*structs.QuotaLimit{
			string(l1.Hash): l1,
			string(l2.Hash): l2,
		},
	}

	return qu
}
