// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package executor

import (
	"bytes"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/stretchr/testify/require"
)

func TestUtils_IsolationMode(t *testing.T) {
	private := IsolationModePrivate
	host := IsolationModeHost
	blank := ""

	for _, tc := range []struct {
		plugin, task, exp string
	}{
		{plugin: private, task: private, exp: private},
		{plugin: private, task: host, exp: host},
		{plugin: private, task: blank, exp: private}, // default to private

		{plugin: host, task: private, exp: private},
		{plugin: host, task: host, exp: host},
		{plugin: host, task: blank, exp: host}, // default to host
	} {
		result := IsolationMode(tc.plugin, tc.task)
		require.Equal(t, tc.exp, result)
	}
}

type testExecCmd struct {
	command  *ExecCommand
	allocDir *allocdir.AllocDir

	stdout         *bytes.Buffer
	stderr         *bytes.Buffer
	outputCopyDone *sync.WaitGroup
}

// configureTLogging configures a test command executor with buffer as
// Std{out|err} but using os.Pipe so it mimics non-test case where cmd is set
// with files as Std{out|err} the buffers can be used to read command output
func configureTLogging(t *testing.T, testcmd *testExecCmd) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var copyDone sync.WaitGroup

	stdoutPr, stdoutPw, err := os.Pipe()
	require.NoError(t, err)

	stderrPr, stderrPw, err := os.Pipe()
	require.NoError(t, err)

	copyDone.Add(2)
	go func() {
		defer copyDone.Done()
		io.Copy(&stdout, stdoutPr)
	}()
	go func() {
		defer copyDone.Done()
		io.Copy(&stderr, stderrPr)
	}()

	testcmd.stdout = &stdout
	testcmd.stderr = &stderr
	testcmd.outputCopyDone = &copyDone

	testcmd.command.stdout = stdoutPw
	testcmd.command.stderr = stderrPw
	return
}
