// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bytes"
	"testing"
	"time"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

var _ cli.Command = (*AllocLogsCommand)(nil)

func TestLogsCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AllocLogsCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
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
	must.StrContains(t, out, "must contain at least two characters.")

	ui.ErrorWriter.Reset()

	// Identifiers with uneven length should produce a query result
	code = cmd.Run([]string{"-address=" + url, "123"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "No allocation(s) with prefix or id")
}

func TestLogsCommand_Follow_AllocComplete(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()
	waitForNodes(t, client)

	job := testJob("follow-complete")
	job.TaskGroups[0].Tasks[0].Config["stdout_string"] = "hello\n"
	resp, _, err := client.Jobs().Register(job, nil)
	must.NoError(t, err)
	must.Zero(t, waitForSuccess(cli.NewMockUi(), client, fullId, t, resp.EvalID))

	allocs, _, err := client.Jobs().Allocations("follow-complete", false, nil)
	must.NoError(t, err)
	must.Len(t, 1, allocs)
	waitForAllocRunning(t, client, allocs[0].ID)

	// Follow both log streams, which is the default with no other flags.
	var stdout, stderr bytes.Buffer
	ui := &cli.BasicUi{Writer: &stdout, ErrorWriter: &stderr}
	cmd := &AllocLogsCommand{Meta: Meta{Ui: ui}}

	codeCh := make(chan int, 1)
	go func() {
		codeCh <- cmd.Run([]string{"-address=" + url, "-f", allocs[0].ID})
	}()

	select {
	case code := <-codeCh:
		must.Zero(t, code, must.Sprint(stderr.String()))
		must.StrContains(t, stdout.String(), "hello")
	case <-time.After(30 * time.Second * time.Duration(testutil.TestMultiplier())):
		t.Fatal("command did not exit after the allocation completed")
	}
}

func TestLogsCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AllocLogsCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake alloc
	state := srv.Agent.Server().State()
	a := mock.Alloc()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 999, nil, a.Job))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{a}))

	prefix := a.ID[:5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.Len(t, 1, res)
	must.Eq(t, a.ID, res[0])
}
