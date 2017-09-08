// +build ent

package mock

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

func SentinelPolicy() *structs.SentinelPolicy {
	sp := &structs.SentinelPolicy{
		Name:             fmt.Sprintf("sent-policy-%s", structs.GenerateUUID()),
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
