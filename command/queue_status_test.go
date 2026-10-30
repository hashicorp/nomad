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

func TestQueueStatusCommand_printStatusFormatted(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &QueueStatusCommand{Meta: Meta{Ui: ui}}
	testRespTenant := []api.DynamicPriorityTenant{
		{
			TenantID:        "testTenant1",
			PercentageUsed:  100,
			TenantFairshare: map[string]float64{"cpu": 10, "memory": 20},
			TotalFairshare:  map[string]float64{"cpu": 10, "memory": 20},
		},
	}
	statusResp := &api.BatchJobQueueStatusResponse{
		Status: testRespTenant,
	}
	cmd.printStatus(statusResp, false)

	expectFormatted := "Batch Queue Status\n" +
		"Tenant       Resource  Current / Total  Percentage\n" +
		"testTenant1                             100\n" +
		"             cpu       10.00 / 10.00    \n" +
		"             memory    20.00 / 20.00    \n"
	must.Eq(t, expectFormatted, ui.OutputWriter.String())

}

func TestQueueStatusCommand_printStatusJSON(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &QueueStatusCommand{Meta: Meta{Ui: ui}}
	testRespTenant := []api.DynamicPriorityTenant{
		{
			TenantID:        "testTenant1",
			PercentageUsed:  100,
			TenantFairshare: map[string]float64{"cpu": 10, "memory": 20},
			TotalFairshare:  map[string]float64{"cpu": 10, "memory": 20},
		},
	}
	statusResp := &api.BatchJobQueueStatusResponse{
		Status: testRespTenant,
	}

	cmd.printStatus(statusResp, true)

	expectJson := `[{"TenantID":"testTenant1","PercentageUsed":100,"TenantFairshare":{"cpu":10,"memory":20},"TotalFairshare":{"cpu":10,"memory":20}}]` + "\n"
	must.Eq(t, expectJson, ui.OutputWriter.String())
}
