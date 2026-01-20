// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

package executor

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
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

func Test_memoryLimits(t *testing.T) {
	cases := []struct {
		memory      int64
		memoryMax   int64
		expReserved int64
		expHard     int64
	}{
		{
			// typical case; only 'memory' is set and that is used as the hard
			// memory limit
			memory:      100,
			memoryMax:   0,
			expReserved: 0,
			expHard:     mbToBytes(100),
		},
		{
			// oversub case; both 'memory' and 'memory_max' are set and used as
			// the reserve and hard memory limits
			memory:      100,
			memoryMax:   200,
			expReserved: mbToBytes(100),
			expHard:     mbToBytes(200),
		},
		{
			// special oversub case; 'memory' is set and 'memory_max' is set to
			// -1; which indicates there should be no hard limit (i.e. -1 / max)
			memory:      100,
			memoryMax:   MemoryNoLimit,
			expReserved: mbToBytes(100),
			expHard:     MemoryNoLimit,
		},
	}

	for _, tc := range cases {
		name := fmt.Sprintf("(%d,%d)", tc.memory, tc.memoryMax)
		t.Run(name, func(t *testing.T) {
			memory := structs.AllocatedMemoryResources{
				MemoryMB:    tc.memory,
				MemoryMaxMB: tc.memoryMax,
			}
			hard, reserved := memoryLimits(memory)
			must.Eq(t, tc.expReserved, reserved)
			must.Eq(t, tc.expHard, hard)
		})
	}
}
