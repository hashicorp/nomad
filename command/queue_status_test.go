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

func TestQueueStatusCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &QueueStatusCommand{}
}

func TestQueueStatusCommand_printDynamicQueueFormatted(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &QueueStatusCommand{Meta: Meta{Ui: ui}}

	testResp := []api.DynamicPriorityWorkload{
		{
			JobID:            "123",
			Tenant:           "testTenant1",
			AdjustedPriority: 10,
			BasePriority:     10,
			UsageAdjustment:  10,
			AgeAdjustment:    5,
			SizeAdjustment:   6,
		},
	}
	cmd.printDynamicQueueFormatted(testResp)

	expect := "Batch Queue Workloads\n" +
		"JobID  Tenant       Adjusted Priority  Base Priority  Usage  Age  Size\n" +
		"123    testTenant1  10                 10             10     5    6\n"

	must.Eq(t, expect, ui.OutputWriter.String())
}

func TestQueueStatusCommand_printDynamicQueueJSON(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &QueueStatusCommand{Meta: Meta{Ui: ui}}

	testResp := []api.DynamicPriorityWorkload{
		{
			JobID:            "123",
			Tenant:           "testTenant1",
			AdjustedPriority: 10,
			BasePriority:     10,
			UsageAdjustment:  10,
			AgeAdjustment:    5,
			SizeAdjustment:   6,
		},
	}
	cmd.printDynamicQueueJSON(testResp)

	expect := `[{"JobID":"123","Tenant":"testTenant1","AdjustedPriority":10,"BasePriority":10,"UsageAdjustment":10,"AgeAdjustment":5,"SizeAdjustment":6}]` + "\n"

	must.Eq(t, expect, ui.OutputWriter.String())
}

func TestQueueStatusCommand_printTenantsFormatted(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &QueueStatusCommand{Meta: Meta{Ui: ui}}
	testRespTenant := []api.DynamicPriorityTenant{
		{
			TenantID:       "testTenant1",
			PercentageUsed: 100,
			TenantUsage:    map[string]float64{"cpu": 10, "memory": 20},
			TotalUsage:     map[string]float64{"cpu": 10, "memory": 20},
		},
	}
	statusResp := &api.BatchJobQueueStatusResponse{
		Results: testRespTenant,
	}
	cmd.printTenants(statusResp, false)

	expectFormatted := `Batch Queue Tenants
Tenant       Resource  Usage / Total  Percentage
testTenant1                           100
             cpu       10.00 / 10.00  
             memory    20.00 / 20.00  
`
	must.Eq(t, expectFormatted, ui.OutputWriter.String())

}

func TestQueueStatusCommand_printTenantsJSON(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &QueueStatusCommand{Meta: Meta{Ui: ui}}
	testRespTenant := []api.DynamicPriorityTenant{
		{
			TenantID:       "testTenant1",
			PercentageUsed: 100,
			TenantUsage:    map[string]float64{"cpu": 10, "memory": 20},
			TotalUsage:     map[string]float64{"cpu": 10, "memory": 20},
		},
	}
	statusResp := &api.BatchJobQueueStatusResponse{
		Results: testRespTenant,
	}

	cmd.printTenants(statusResp, true)

	expectJson := `[{"TenantID":"testTenant1","PercentageUsed":100,"TenantUsage":{"cpu":10,"memory":20},"TotalUsage":{"cpu":10,"memory":20}}]` + "\n"
	must.Eq(t, expectJson, ui.OutputWriter.String())
}
