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

func TestJobQueue_printOutput(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobQueueCommand{Meta: Meta{Ui: ui}}

	testResp := &api.BatchQueueStatusResponse{
		Workloads: []api.Workload{
			{
				JobID:    "123",
				Tenant:   "testTenant1",
				Priority: 5,
			},
		},
	}
	cmd.printOutput(testResp)

	expect := "JobID   |   Tenant        |   Priority\n" +
		"-----   |   ------        |   --------\n" +
		"123     |   testTenant1   |   5\n"

	must.Eq(t, expect, ui.OutputWriter.String())
}
