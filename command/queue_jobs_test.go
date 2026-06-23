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

func TestQueueJobsCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &QueueJobsCommand{}
}

func TestQueueJobsCommand_printDynamicQueueFormatted(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &QueueJobsCommand{Meta: Meta{Ui: ui}}

	testResp := []api.DynamicPriorityWorkload{
		{
			JobID:            "123",
			Tenant:           "testTenant1",
			Position:         1,
			AdjustedPriority: 10,
			BasePriority:     10,
			UsageAdjustment:  10,
			AgeAdjustment:    5,
			SizeAdjustment:   6,
		},
	}
	cmd.printDynamicQueueFormatted(testResp)

	expect := "Batch Queue Workloads\n" +
		"JobID  Tenant       Adjusted Priority  Base Priority  Position  Usage  Age  Size\n" +
		"123    testTenant1  10                 10             1         10     5    6\n"

	must.Eq(t, expect, ui.OutputWriter.String())
}

func TestQueueJobsCommand_printDynamicQueueJSON(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &QueueJobsCommand{Meta: Meta{Ui: ui}}

	testResp := []api.DynamicPriorityWorkload{
		{
			JobID:            "123",
			Tenant:           "testTenant1",
			Position:         1,
			AdjustedPriority: 10,
			BasePriority:     10,
			UsageAdjustment:  10,
			AgeAdjustment:    5,
			SizeAdjustment:   6,
		},
	}
	cmd.printDynamicQueueJSON(testResp)

	expect := `[{"JobID":"123","Tenant":"testTenant1","Position":1,"AdjustedPriority":10,"BasePriority":10,"UsageAdjustment":10,"AgeAdjustment":5,"SizeAdjustment":6}]` + "\n"

	must.Eq(t, expect, ui.OutputWriter.String())
}
