// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

func TestAllocChecksCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = (*AllocChecksCommand)(nil)
}

func TestAllocChecksCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AllocChecksCommand{Meta: Meta{Ui: ui}}

	// fails on misuse t.Run("fails on misuse", func(t *testing.T) {
	code := cmd.Run([]string{"some", "bad", "args"})
	must.One(t, code)
	out := ui.ErrorWriter.String()
	must.StrContains(t, out, commandErrorText(cmd))

	ui.ErrorWriter.Reset()

	// fails on connection failure
	code = cmd.Run([]string{"-address=nope", "foobar"})
	must.One(t, code)
	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "Error querying allocation")

	ui.ErrorWriter.Reset()

	// fails on missing allocation
	code = cmd.Run([]string{"-address=" + url, "26470238-5CF2-438F-8772-DC67CFB0705C"})
	must.One(t, code)
	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "No allocation(s) with prefix or id")

	ui.ErrorWriter.Reset()

	// fails on prefix with too few characters
	code = cmd.Run([]string{"-address=" + url, "2"})
	must.One(t, code)
	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "must contain at least two characters.")

	ui.ErrorWriter.Reset()
}

func TestAllocChecksCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AllocChecksCommand{Meta: Meta{Ui: ui, flagAddress: url}}

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

func TestAllocChecksCommand_Run(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)

	defer srv.Shutdown()

	// wait for nodes
	waitForNodes(t, client)

	jobID := "job1_checks"
	job1 := testNomadServiceJob(jobID)

	resp, _, err := client.Jobs().Register(job1, nil)
	must.NoError(t, err)

	// wait for registration success
	ui := cli.NewMockUi()
	code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
	must.Zero(t, code)

	// Get an alloc id
	allocID := getAllocFromJob(t, client, jobID)

	// do not wait for alloc running - it will stay pending because the
	// health-check will never pass

	// Run command
	cmd := &AllocChecksCommand{Meta: Meta{Ui: ui, flagAddress: url}}
	code = cmd.Run([]string{"-address=" + url, allocID})
	must.Zero(t, code)

	// check output
	out := ui.OutputWriter.String()
	must.StrContains(t, out, `Name       =  check1`)
	must.StrContains(t, out, `Group      =  job1_checks.group1[0]`)
	must.StrContains(t, out, `Task       =  (group)`)
	must.StrContains(t, out, `Service    =  service1`)
	must.StrContains(t, out, `Mode       =  healthiness`)

	ui.OutputWriter.Reset()

	// List json
	cmd = &AllocChecksCommand{Meta: Meta{Ui: ui, flagAddress: url}}
	must.Zero(t, cmd.Run([]string{"-address=" + url, "-json", allocID}))

	outJson := api.AllocCheckStatuses{}
	err = json.Unmarshal(ui.OutputWriter.Bytes(), &outJson)
	must.NoError(t, err)

	ui.OutputWriter.Reset()

	// Go template to format the output
	code = cmd.Run([]string{"-address=" + url, "-t", "{{range .}}{{ .Status }}{{end}}", allocID})
	must.Zero(t, code)

	out = ui.OutputWriter.String()
	must.StrContains(t, out, "failure")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}
