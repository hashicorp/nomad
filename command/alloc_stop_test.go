// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

var _ cli.Command = (*AllocStopCommand)(nil)

func TestAllocStop_Fails(t *testing.T) {
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AllocStopCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "garbage", "args"})
	must.One(t, code)

	out := ui.ErrorWriter.String()
	must.StrContains(t, out, commandErrorText(cmd))

	ui.ErrorWriter.Reset()

	// Fails on connection failure
	code = cmd.Run([]string{"-address=nope", "foobar"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "Error querying allocation")

	ui.ErrorWriter.Reset()

	// Fails on missing alloc
	code = cmd.Run([]string{"-address=" + url, "26470238-5CF2-438F-8772-DC67CFB0705C"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "No allocation(s) with prefix or id")
	ui.ErrorWriter.Reset()

	// Fail on identifier with too few characters
	code = cmd.Run([]string{"-address=" + url, "2"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "must contain at least two characters")
	ui.ErrorWriter.Reset()

	// Identifiers with uneven length should produce a query result
	code = cmd.Run([]string{"-address=" + url, "123"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "No allocation(s) with prefix or id")
}

func TestAllocStop_Run(t *testing.T) {
	ci.Parallel(t)

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait for a node to be ready
	waitForNodes(t, client)

	ui := cli.NewMockUi()
	cmd := &AllocStopCommand{Meta: Meta{Ui: ui}}

	jobID := "job1_sfx"
	job1 := testJob(jobID)
	resp, _, err := client.Jobs().Register(job1, nil)
	must.NoError(t, err)

	code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
	must.Zero(t, code)

	// get an alloc id
	allocID := ""
	if allocs, _, err := client.Jobs().Allocations(jobID, false, nil); err == nil {
		if len(allocs) > 0 {
			allocID = allocs[0].ID
		}
	}
	must.NotEq(t, "", allocID)

	// Wait for alloc to be running
	waitForAllocRunning(t, client, allocID)

	code = cmd.Run([]string{"-address=" + url, allocID})
	must.Zero(t, code)
}
