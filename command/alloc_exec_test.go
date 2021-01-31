package command

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// static check
var _ cli.Command = &AllocExecCommand{}

func TestAllocExecCommand_Fails(t *testing.T) {
	t.Parallel()
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
			`job "example" doesn't exist`,
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
			require.Equal(t, 1, code)

			require.Contains(t, ui.ErrorWriter.String(), c.expectedError)

			ui.ErrorWriter.Reset()
			ui.OutputWriter.Reset()

		})
	}

	// Wait for a node to be ready
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		for _, node := range nodes {
			if _, ok := node.Drivers["mock_driver"]; ok &&
				node.Status == structs.NodeStatusReady {
				return true, nil
			}
		}
		return false, fmt.Errorf("no ready nodes")
	}, func(err error) {
		require.NoError(t, err)
	})

	t.Run("non existent task", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &AllocExecCommand{Meta: Meta{Ui: ui}}

		jobID := "job1_sfx"
		job1 := testJob(jobID)
		resp, _, err := client.Jobs().Register(job1, nil)
		require.NoError(t, err)
		code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
		require.Zero(t, code, "status code not zero")

		// get an alloc id
		allocId1 := ""
		if allocs, _, err := client.Jobs().Allocations(jobID, false, nil); err == nil {
			if len(allocs) > 0 {
				allocId1 = allocs[0].ID
			}
		}
		require.NotEmpty(t, allocId1, "unable to find allocation")

		// by alloc
		require.Equal(t, 1, cmd.Run([]string{"-address=" + url, "-task=nonexistenttask1", allocId1, "/bin/bash"}))
		require.Contains(t, ui.ErrorWriter.String(), "Could not find task named: nonexistenttask1")
		ui.ErrorWriter.Reset()

		// by jobID
		require.Equal(t, 1, cmd.Run([]string{"-address=" + url, "-task=nonexistenttask2", "-job", jobID, "/bin/bash"}))
		require.Contains(t, ui.ErrorWriter.String(), "Could not find task named: nonexistenttask2")
		ui.ErrorWriter.Reset()
	})

}

func TestAllocExecCommand_AutocompleteArgs(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AllocExecCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake alloc
	state := srv.Agent.Server().State()
	a := mock.Alloc()
	assert.Nil(state.UpsertAllocs(structs.MsgTypeTestSetup, 1000, []*structs.Allocation{a}))

	prefix := a.ID[:5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	assert.Equal(1, len(res))
	assert.Equal(a.ID, res[0])
}

func TestAllocExecCommand_Run(t *testing.T) {
	t.Parallel()
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait for a node to be ready
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}

		for _, node := range nodes {
			if _, ok := node.Drivers["mock_driver"]; ok &&
				node.Status == structs.NodeStatusReady {
				return true, nil
			}
		}
		return false, fmt.Errorf("no ready nodes")
	}, func(err error) {
		require.NoError(t, err)
	})

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
	require.NoError(t, err)

	evalUi := cli.NewMockUi()
	code := waitForSuccess(evalUi, client, fullId, t, resp.EvalID)
	require.Equal(t, 0, code, "failed to get status - output: %v", evalUi.ErrorWriter.String())

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
	}, func(err error) {
		require.NoError(t, err)

	})

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
			assert.Equal(t, c.exitCode, code)
			assert.Equal(t, c.stdout, strings.TrimSpace(stdout.String()))
			assert.Equal(t, c.stderr, strings.TrimSpace(stderr.String()))
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
			assert.Equal(t, c.exitCode, code)
			assert.Equal(t, c.stdout, strings.TrimSpace(stdout.String()))
			assert.Equal(t, c.stderr, strings.TrimSpace(stderr.String()))
		})
	}
}

type bufferCloser struct {
	bytes.Buffer
}

func (b *bufferCloser) Close() error {
	return nil
}
