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

func TestAllocSignalCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &AllocSignalCommand{}
}

func TestAllocSignalCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AllocSignalCommand{Meta: Meta{Ui: ui}}

	// Fails on lack of alloc ID
	code := cmd.Run([]string{})
	must.One(t, code)

	out := ui.ErrorWriter.String()
	must.StrContains(t, out, "This command takes up to two arguments")

	ui.ErrorWriter.Reset()

	// Fails on misuse
	code = cmd.Run([]string{"some", "bad", "args"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "This command takes up to two arguments")

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
	must.StrContains(t, out, "must contain at least two characters.")

	ui.ErrorWriter.Reset()
}

func TestAllocSignalCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AllocSignalCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake alloc
	state := srv.Agent.Server().State()
	a := mock.Alloc()
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{a}))

	prefix := a.ID[:5]
	args := complete.Args{All: []string{"signal", prefix}, Last: prefix}
	predictor := cmd.AutocompleteArgs()

	// Match Allocs
	res := predictor.Predict(args)
	must.Len(t, 1, res)
	must.Eq(t, a.ID, res[0])
}

func TestAllocSignalCommand_Run(t *testing.T) {
	ci.Parallel(t)

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait for a node to be ready
	waitForNodes(t, client)

	ui := cli.NewMockUi()
	cmd := &AllocSignalCommand{Meta: Meta{Ui: ui}}

	jobID := "job1_sfx"
	job1 := testJob(jobID)
	resp, _, err := client.Jobs().Register(job1, nil)
	must.NoError(t, err)

	code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
	must.Zero(t, code)

	// Get an alloc id
	allocID := getAllocFromJob(t, client, jobID)

	// Wait for alloc to be running
	waitForAllocRunning(t, client, allocID)

	code = cmd.Run([]string{"-address=" + url, allocID})
	must.Zero(t, code)
}
