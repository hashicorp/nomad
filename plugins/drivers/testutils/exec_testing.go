// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testutils

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/plugins/drivers"
	dproto "github.com/hashicorp/nomad/plugins/drivers/proto"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func ExecTaskStreamingConformanceTests(t *testing.T, driver *DriverHarness, taskID string) {
	t.Helper()

	if runtime.GOOS == "windows" {
		// tests assume unix-ism now
		t.Skip("test assume unix tasks")
	}

	TestExecTaskStreamingBasicResponses(t, driver, taskID)
	TestExecFSIsolation(t, driver, taskID)
}

var ExecTaskStreamingBasicCases = []struct {
	Name     string
	Command  string
	Tty      bool
	Stdin    string
	Stdout   interface{}
	Stderr   interface{}
	ExitCode int
}{
	{
		Name:     "notty: basic",
		Command:  "echo hello stdout; echo hello stderr >&2; exit 43",
		Tty:      false,
		Stdout:   "hello stdout\n",
		Stderr:   "hello stderr\n",
		ExitCode: 43,
	},
	{
		Name:     "notty: streaming",
		Command:  "for n in 1 2 3; do echo $n; sleep 1; done",
		Tty:      false,
		Stdout:   "1\n2\n3\n",
		ExitCode: 0,
	},
	{
		Name:     "notty: stty check",
		Command:  "stty size",
		Tty:      false,
		Stderr:   regexp.MustCompile("stty: .?standard input.?: Inappropriate ioctl for device\n"),
		ExitCode: 1,
	},
	{
		Name:     "notty: stdin passing",
		Command:  "echo hello from command; head -n1",
		Tty:      false,
		Stdin:    "hello from stdin\n",
		Stdout:   "hello from command\nhello from stdin\n",
		ExitCode: 0,
	},
	// TTY cases - difference is new lines add `\r` and child process waiting is different
	{
		Name:     "tty: basic",
		Command:  "echo hello stdout; echo hello stderr >&2; exit 43",
		Tty:      true,
		Stdout:   "hello stdout\r\nhello stderr\r\n",
		ExitCode: 43,
	},
	{
		Name:     "tty: streaming",
		Command:  "for n in 1 2 3; do echo $n; sleep 1; done",
		Tty:      true,
		Stdout:   "1\r\n2\r\n3\r\n",
		ExitCode: 0,
	},
	{
		Name:     "tty: stty check",
		Command:  "sleep 1; stty size",
		Tty:      true,
		Stdout:   "100 100\r\n",
		ExitCode: 0,
	},
	{
		Name:    "tty: stdin passing",
		Command: "head -n1",
		Tty:     true,
		Stdin:   "hello from stdin\n",
		// in tty mode, we emit line twice: once for tty echoing and one for the actual head output
		Stdout:   "hello from stdin\r\nhello from stdin\r\n",
		ExitCode: 0,
	},
	{
		Name:    "tty: children processes",
		Command: "(( sleep 3; echo from background ) & ); echo from main; exec sleep 1",
		Tty:     true,
		// when using tty; wait for lead process only, like `docker exec -it`
		Stdout:   "from main\r\n",
		ExitCode: 0,
	},
}

func TestExecTaskStreamingBasicResponses(t *testing.T, driver *DriverHarness, taskID string) {
	for _, c := range ExecTaskStreamingBasicCases {
		t.Run("basic: "+c.Name, func(t *testing.T) {

			result := execTask(t, driver, taskID, c.Command, c.Tty, c.Stdin)

			require.Equal(t, c.ExitCode, result.exitCode)

			switch s := c.Stdout.(type) {
			case string:
				require.Equal(t, s, result.stdout)
			case *regexp.Regexp:
				require.Regexp(t, s, result.stdout)
			case nil:
				require.Empty(t, result.stdout)
			default:
				require.Fail(t, "unexpected stdout type", "found %v (%v), but expected string or regexp", s, reflect.TypeOf(s))
			}

			switch s := c.Stderr.(type) {
			case string:
				require.Equal(t, s, result.stderr)
			case *regexp.Regexp:
				require.Regexp(t, s, result.stderr)
			case nil:
				require.Empty(t, result.stderr)
			default:
				require.Fail(t, "unexpected stderr type", "found %v (%v), but expected string or regexp", s, reflect.TypeOf(s))
			}

		})
	}
}

// TestExecFSIsolation asserts that exec occurs inside chroot/isolation environment rather than
// on host
func TestExecFSIsolation(t *testing.T, driver *DriverHarness, taskID string) {
	t.Run("isolation", func(t *testing.T) {
		caps, err := driver.Capabilities()
		require.NoError(t, err)

		isolated := (caps.FSIsolation != drivers.FSIsolationNone)

		text := "hello from the other side"

		// write to a file and check it presence in host
		w := execTask(t, driver, taskID,
			fmt.Sprintf(`FILE=$(mktemp); echo "$FILE"; echo %q >> "${FILE}"`, text),
			false, "")
		require.Zero(t, w.exitCode)

		tempfile := strings.TrimSpace(w.stdout)
		if !isolated {
			defer os.Remove(tempfile)
		}

		t.Logf("created file in task: %v", tempfile)

		// read from host
		b, err := os.ReadFile(tempfile)
		if !isolated {
			require.NoError(t, err)
			require.Equal(t, text, strings.TrimSpace(string(b)))
		} else {
			require.Error(t, err)
			require.True(t, os.IsNotExist(err))
		}

		// read should succeed from task again
		r := execTask(t, driver, taskID,
			fmt.Sprintf("cat %q", tempfile),
			false, "")
		require.Zero(t, r.exitCode)
		require.Equal(t, text, strings.TrimSpace(r.stdout))

		// we always run in a cgroup - testing freezer cgroup
		r = execTask(t, driver, taskID,
			"cat /proc/self/cgroup",
			false, "",
		)
		require.Zero(t, r.exitCode)

		switch cgroupslib.GetMode() {

		case cgroupslib.CG1:
			acceptable := []string{":freezer:/nomad", ":freezer:/docker"}
			if testutil.IsCI() {
				// github actions freezer cgroup
				acceptable = append(acceptable, ":freezer:/actions_job")
			}

			ok := false
			for _, freezerGroup := range acceptable {
				if strings.Contains(r.stdout, freezerGroup) {
					ok = true
					break
				}
			}
			if !ok {
				require.Fail(t, "unexpected freezer cgroup", "expected freezer to be /nomad/ or /docker/, but found:\n%s", r.stdout)
			}
		case cgroupslib.CG2:
			info, _ := driver.PluginInfo()
			if info.Name == "docker" {
				// Note: docker on cgroups v2 now returns nothing
				// root@97b4d3d33035:/# cat /proc/self/cgroup
				// 0::/
				t.Skip("/proc/self/cgroup not useful in docker cgroups.v2")
			}
			// e.g. 0::/testing.slice/5bdbd6c2-8aba-3ab2-728b-0ff3a81727a9.sleep.scope
			require.True(t, strings.HasSuffix(strings.TrimSpace(r.stdout), ".scope"), "actual stdout %q", r.stdout)
		}
	})
}

func ExecTask(t *testing.T, driver *DriverHarness, taskID string, cmd string, tty bool, stdin string) (exitCode int, stdout, stderr string) {
	r := execTask(t, driver, taskID, cmd, tty, stdin)
	return r.exitCode, r.stdout, r.stderr
}

func execTask(t *testing.T, driver *DriverHarness, taskID string, cmd string, tty bool, stdin string) execResult {
	stream := newTestExecStream(t, tty, stdin)

	ctx, cancelFn := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFn()

	command := []string{"/bin/sh", "-c", cmd}

	isRaw := false
	exitCode := -2
	if raw, ok := driver.impl.(drivers.ExecTaskStreamingRawDriver); ok {
		isRaw = true
		err := raw.ExecTaskStreamingRaw(ctx, taskID,
			command, tty, stream)
		require.NoError(t, err)
	} else if d, ok := driver.impl.(drivers.ExecTaskStreamingDriver); ok {
		execOpts, errCh := drivers.StreamToExecOptions(ctx, command, tty, stream)

		r, err := d.ExecTaskStreaming(ctx, taskID, execOpts)
		require.NoError(t, err)

		select {
		case err := <-errCh:
			require.NoError(t, err)
		default:
			// all good
		}

		exitCode = r.ExitCode
	} else {
		require.Fail(t, "driver does not support exec")
	}

	result := stream.currentResult()
	require.NoError(t, result.err)

	if !isRaw {
		result.exitCode = exitCode
	}

	return result
}

type execResult struct {
	exitCode int
	stdout   string
	stderr   string

	err error
}

func newTestExecStream(t *testing.T, tty bool, stdin string) *testExecStream {

	return &testExecStream{
		t:      t,
		input:  newInputStream(tty, stdin),
		result: &execResult{exitCode: -2},
	}
}

func newInputStream(tty bool, stdin string) []*drivers.ExecTaskStreamingRequestMsg {
	input := []*drivers.ExecTaskStreamingRequestMsg{}
	if tty {
		// emit two resize to ensure we honor latest
		input = append(input, &drivers.ExecTaskStreamingRequestMsg{
			TtySize: &dproto.ExecTaskStreamingRequest_TerminalSize{
				Height: 50,
				Width:  40,
			}})
		input = append(input, &drivers.ExecTaskStreamingRequestMsg{
			TtySize: &dproto.ExecTaskStreamingRequest_TerminalSize{
				Height: 100,
				Width:  100,
			}})

	}

	input = append(input, &drivers.ExecTaskStreamingRequestMsg{
		Stdin: &dproto.ExecTaskStreamingIOOperation{
			Data: []byte(stdin),
		},
	})

	if !tty {
		// don't close stream in interactive session and risk closing tty prematurely
		input = append(input, &drivers.ExecTaskStreamingRequestMsg{
			Stdin: &dproto.ExecTaskStreamingIOOperation{
				Close: true,
			},
		})
	}

	return input
}

var _ drivers.ExecTaskStream = (*testExecStream)(nil)

type testExecStream struct {
	t *testing.T

	// input
	input      []*drivers.ExecTaskStreamingRequestMsg
	recvCalled int

	// result so far
	resultLock sync.Mutex
	result     *execResult
}

func (s *testExecStream) currentResult() execResult {
	s.resultLock.Lock()
	defer s.resultLock.Unlock()

	// make a copy
	return *s.result
}

func (s *testExecStream) Recv() (*drivers.ExecTaskStreamingRequestMsg, error) {
	if s.recvCalled >= len(s.input) {
		return nil, io.EOF
	}

	i := s.input[s.recvCalled]
	s.recvCalled++
	return i, nil
}

func (s *testExecStream) Send(m *drivers.ExecTaskStreamingResponseMsg) error {
	s.resultLock.Lock()
	defer s.resultLock.Unlock()

	switch {
	case m.Stdout != nil && m.Stdout.Data != nil:
		s.t.Logf("received stdout: %s", string(m.Stdout.Data))
		s.result.stdout += string(m.Stdout.Data)
	case m.Stderr != nil && m.Stderr.Data != nil:
		s.t.Logf("received stderr: %s", string(m.Stderr.Data))
		s.result.stderr += string(m.Stderr.Data)
	case m.Exited && m.Result != nil:
		s.result.exitCode = int(m.Result.ExitCode)
	}

	return nil
}
