// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestQueueTenantsCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &QueueTenantsCommand{}
}

func TestQueueTenantsCommand_printTenantsFormatted(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &QueueTenantsCommand{Meta: Meta{Ui: ui}}
	testRespTenant := []api.DynamicPriorityTenant{
		{
			TenantID:        "testTenant1",
			PercentageUsed:  100,
			TenantFairshare: map[string]float64{"cpu": 10, "memory": 20},
			TotalFairshare:  map[string]float64{"cpu": 10, "memory": 20},
		},
	}
	statusResp := &api.BatchJobQueueTenantsResponse{
		Tenants: testRespTenant,
	}
	cmd.printTenants(statusResp, false)

	expectFormatted := `Batch Queue Tenants
Tenant       Resource  Current / Total  Percentage
testTenant1                             100
             cpu       10.00 / 10.00    
             memory    20.00 / 20.00    
`
	must.Eq(t, expectFormatted, ui.OutputWriter.String())

}

func TestQueueTenantsCommand_printTenantsJSON(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &QueueTenantsCommand{Meta: Meta{Ui: ui}}
	testRespTenant := []api.DynamicPriorityTenant{
		{
			TenantID:        "testTenant1",
			PercentageUsed:  100,
			TenantFairshare: map[string]float64{"cpu": 10, "memory": 20},
			TotalFairshare:  map[string]float64{"cpu": 10, "memory": 20},
		},
	}
	statusResp := &api.BatchJobQueueTenantsResponse{
		Tenants: testRespTenant,
	}

	cmd.printTenants(statusResp, true)

	expectJson := `[{"TenantID":"testTenant1","PercentageUsed":100,"TenantFairshare":{"cpu":10,"memory":20},"TotalFairshare":{"cpu":10,"memory":20}}]` + "\n"
	must.Eq(t, expectJson, ui.OutputWriter.String())
}
