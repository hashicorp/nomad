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

func TestJobQueue_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobQueueCommand{}
}

func TestJobQueue_printDynamicQueueFormatted(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobQueueCommand{Meta: Meta{Ui: ui}}

	testResp := []api.DynamicPriorityWorkload{
		{
			JobID:            "123",
			Tenant:           "testTenant1",
			AdjustedPriority: 10,
			BasePriority:     10,
		},
	}
	cmd.printDynamicQueueFormatted(testResp)

	expect := "Batch Queue Workloads\n" +
		"JobID  Tenant       Adjusted Priority  Base Priority  Usage  Age  Size\n" +
		"123    testTenant1  10                 10             0      0    0\n"

	must.Eq(t, expect, ui.OutputWriter.String())
}

func TestJobQueue_printDynamicQueueJSON(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobQueueCommand{Meta: Meta{Ui: ui}}

	testResp := []api.DynamicPriorityWorkload{
		{
			JobID:            "123",
			Tenant:           "testTenant1",
			AdjustedPriority: 10,
			BasePriority:     10,
		},
	}
	cmd.printDynamicQueueJSON(testResp)

	expect := "[{\"JobID\":\"123\",\"Tenant\":\"testTenant1\",\"AdjustedPriority\":10,\"BasePriority\":10,\"UsageAdjustment\":0,\"AgeAdjustment\":0,\"SizeAdjustment\":0}]\n"

	must.Eq(t, expect, ui.OutputWriter.String())
}
