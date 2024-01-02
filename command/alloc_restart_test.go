// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

func TestAllocRestartCommand_Implements(t *testing.T) {
	var _ cli.Command = &AllocRestartCommand{}
}

func TestAllocRestartCommand_Fails(t *testing.T) {
	ci.Parallel(t)

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AllocRestartCommand{Meta: Meta{Ui: ui}}

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

	ui.ErrorWriter.Reset()

	// Wait for a node to be ready
	waitForNodes(t, client)

	jobID := "job1_sfx"
	job1 := testJob(jobID)
	resp, _, err := client.Jobs().Register(job1, nil)
	must.NoError(t, err)

	code = waitForSuccess(ui, client, fullId, t, resp.EvalID)
	must.Zero(t, code)

	// get an alloc id
	allocId1 := ""
	if allocs, _, err := client.Jobs().Allocations(jobID, false, nil); err == nil {
		if len(allocs) > 0 {
			allocId1 = allocs[0].ID
		}
	}
	must.NotEq(t, "", allocId1)

	// Fails on not found task
	code = cmd.Run([]string{"-address=" + url, allocId1, "fooooobarrr"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "Could not find task named")

	ui.ErrorWriter.Reset()
}

func TestAllocRestartCommand_Run(t *testing.T) {
	ci.Parallel(t)

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait for a node to be ready
	waitForNodes(t, client)

	ui := cli.NewMockUi()
	cmd := &AllocRestartCommand{Meta: Meta{Ui: ui}}

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

	ui.OutputWriter.Reset()
}

func TestAllocRestartCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AllocRestartCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake alloc
	state := srv.Agent.Server().State()
	a := mock.Alloc()
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{a}))

	prefix := a.ID[:5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.Len(t, 1, res)
	must.Eq(t, a.ID, res[0])
}
