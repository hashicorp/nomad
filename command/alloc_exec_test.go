// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

// static check
var _ cli.Command = &AllocExecCommand{}

func TestAllocExecCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	cases := []struct {
		name          string
		args          []string
		expectedError string
	}{
		{
			"alloc id missing",
			[]string{},
			`An allocation ID is required`,
		},
		{
			"alloc id too short",
			[]string{"-address=" + url, "2", "/bin/bash"},
			`Alloc ID must contain at least two characters`,
		},
		{
			"alloc not found",
			[]string{"-address=" + url, "26470238-5CF2-438F-8772-DC67CFB0705C", "/bin/bash"},
			`No allocation(s) with prefix or id "26470238-5CF2-438F-8772-DC67CFB0705C"`,
		},
		{
			"alloc not found with odd-length prefix",
			[]string{"-address=" + url, "26470238-5CF", "/bin/bash"},
			`No allocation(s) with prefix or id "26470238-5CF"`,
		},
		{
			"job id missing",
			[]string{"-job"},
			`A job ID is required`,
		},
		{
			"job not found",
			[]string{"-address=" + url, "-job", "example", "/bin/bash"},
			`No job(s) with prefix or ID "example" found`,
		},
		{
			"command missing",
			[]string{"-address=" + url, "26470238-5CF2-438F-8772-DC67CFB0705C"},
			`A command is required`,
		},
		{
			"connection failure",
			[]string{"-address=nope", "26470238-5CF2-438F-8772-DC67CFB0705C", "/bin/bash"},
			`Error querying allocation`,
		},
		{
			"escape char too long",
			[]string{"-address=" + url, "-e", "es", "26470238-5CF2-438F-8772-DC67CFB0705C", "/bin/bash"},
			`-e requires 'none' or a single character`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &AllocExecCommand{Meta: Meta{Ui: ui}}

			code := cmd.Run(c.args)
			must.One(t, code)

			out := ui.ErrorWriter.String()
			must.StrContains(t, out, c.expectedError)

			ui.ErrorWriter.Reset()
			ui.OutputWriter.Reset()
		})
	}

	// Wait for a node to be ready
	waitForNodes(t, client)

	t.Run("non existent task", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &AllocExecCommand{Meta: Meta{Ui: ui}}

		jobID := "job1_sfx"
		job1 := testJob(jobID)

		resp, _, err := client.Jobs().Register(job1, nil)
		must.NoError(t, err)

		code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
		must.Zero(t, code)

		// get an alloc id
		allocId1 := ""
		if allocs, _, err := client.Jobs().Allocations(jobID, false, nil); err == nil {
			if len(allocs) > 0 {
				allocId1 = allocs[0].ID
			}
		}
		must.NotEq(t, "", allocId1)

		// by alloc
		code = cmd.Run([]string{"-address=" + url, "-task=nonexistenttask1", allocId1, "/bin/bash"})
		must.One(t, code)

		out := ui.ErrorWriter.String()
		must.StrContains(t, out, "Could not find task named: nonexistenttask1")

		ui.ErrorWriter.Reset()

		// by jobID
		code = cmd.Run([]string{"-address=" + url, "-task=nonexistenttask2", "-job", jobID, "/bin/bash"})
		must.One(t, code)

		out = ui.ErrorWriter.String()
		must.StrContains(t, out, "Could not find task named: nonexistenttask2")

		ui.ErrorWriter.Reset()
	})

}

func TestAllocExecCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AllocExecCommand{Meta: Meta{Ui: ui, flagAddress: url}}

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

func TestAllocExecCommand_Run(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait for a node to be ready
	waitForNodes(t, client)

	jobID := uuid.Generate()
	job := testJob(jobID)
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "10s",
		"exec_command": map[string]interface{}{
			"run_for":       "1ms",
			"exit_code":     21,
			"stdout_string": "sample stdout output\n",
			"stderr_string": "sample stderr output\n",
		},
	}
	resp, _, err := client.Jobs().Register(job, nil)
	must.NoError(t, err)

	evalUi := cli.NewMockUi()
	code := waitForSuccess(evalUi, client, fullId, t, resp.EvalID)
	must.Zero(t, code)

	allocId := ""

	testutil.WaitForResult(func() (bool, error) {
		allocs, _, err := client.Jobs().Allocations(jobID, false, nil)
		if err != nil {
			return false, fmt.Errorf("failed to get allocations: %v", err)
		}

		if len(allocs) < 0 {
			return false, fmt.Errorf("no allocations yet")
		}

		alloc := allocs[0]
		if alloc.ClientStatus != "running" {
			return false, fmt.Errorf("alloc is not running yet: %v", alloc.ClientStatus)
		}

		allocId = alloc.ID
		return true, nil
	}, func(err error) { must.NoError(t, err) })

	cases := []struct {
		name    string
		command string
		stdin   string

		stdout   string
		stderr   string
		exitCode int
	}{
		{
			name:     "basic stdout/err",
			command:  "simplecommand",
			stdin:    "",
			stdout:   "sample stdout output",
			stderr:   "sample stderr output",
			exitCode: 21,
		},
		{
			name:     "notty: streamining input",
			command:  "showinput",
			stdin:    "hello from stdin",
			stdout:   "TTY: false\nStdin:\nhello from stdin",
			exitCode: 0,
		},
	}

	for _, c := range cases {
		t.Run("by id: "+c.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			var stdout, stderr bufferCloser

			cmd := &AllocExecCommand{
				Meta:   Meta{Ui: ui},
				Stdin:  strings.NewReader(c.stdin),
				Stdout: &stdout,
				Stderr: &stderr,
			}

			code = cmd.Run([]string{"-address=" + url, allocId, c.command})
			must.Eq(t, c.exitCode, code)
			must.Eq(t, c.stdout, strings.TrimSpace(stdout.String()))
			must.Eq(t, c.stderr, strings.TrimSpace(stderr.String()))
		})
		t.Run("by job: "+c.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			var stdout, stderr bufferCloser

			cmd := &AllocExecCommand{
				Meta:   Meta{Ui: ui},
				Stdin:  strings.NewReader(c.stdin),
				Stdout: &stdout,
				Stderr: &stderr,
			}

			code = cmd.Run([]string{"-address=" + url, "-job", jobID, c.command})
			must.Eq(t, c.exitCode, code)
			must.Eq(t, c.stdout, strings.TrimSpace(stdout.String()))
			must.Eq(t, c.stderr, strings.TrimSpace(stderr.String()))
		})
	}
}

type bufferCloser struct {
	bytes.Buffer
}

func (b *bufferCloser) Close() error {
	return nil
}
