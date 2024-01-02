// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

var _ cli.Command = (*JobRunCommand)(nil)

func TestRunCommand_Output_Json(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobRunCommand{Meta: Meta{Ui: ui}}

	fh, err := os.CreateTemp("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh.Name())
	_, err = fh.WriteString(`
job "job1" {
	type = "service"
	datacenters = [ "dc1" ]
	group "group1" {
		count = 1
		task "task1" {
			driver = "exec"
			resources {
				cpu = 1000
				memory = 512
			}
		}
	}
}`)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := cmd.Run([]string{"-output", fh.Name()}); code != 0 {
		t.Fatalf("expected exit code 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, `"Type": "service",`) {
		t.Fatalf("Expected JSON output: %v", out)
	}
}

func TestRunCommand_hcl1_hcl2_strict(t *testing.T) {
	ci.Parallel(t)

	_, _, addr := testServer(t, false, nil)

	t.Run("-hcl1 implies -hcl2-strict is false", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobRunCommand{Meta: Meta{Ui: ui}}
		got := cmd.Run([]string{
			"-hcl1", "-hcl2-strict",
			"-address", addr,
			"-detach",
			"asset/example-short.nomad.hcl",
		})
		require.Equal(t, 0, got, ui.ErrorWriter.String())
	})
}

func TestRunCommand_Fails(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	s := testutil.NewTestServer(t, nil)
	defer s.Stop()

	ui := cli.NewMockUi()
	cmd := &JobRunCommand{Meta: Meta{Ui: ui, flagAddress: "http://" + s.HTTPAddr}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails when specified file does not exist
	if code := cmd.Run([]string{"/unicorns/leprechauns"}); code != 1 {
		t.Fatalf("expect exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error getting job struct") {
		t.Fatalf("expect getting job struct error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on invalid HCL
	fh1, err := os.CreateTemp("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh1.Name())
	if _, err := fh1.WriteString("nope"); err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := cmd.Run([]string{fh1.Name()}); code != 1 {
		t.Fatalf("expect exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error getting job struct") {
		t.Fatalf("expect parsing error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on invalid job spec
	fh2, err := os.CreateTemp("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh2.Name())
	if _, err := fh2.WriteString(`job "job1" {}`); err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := cmd.Run([]string{fh2.Name()}); code != 1 {
		t.Fatalf("expect exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error submitting job") {
		t.Fatalf("expect validation error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure (requires a valid job)
	fh3, err := os.CreateTemp("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh3.Name())
	_, err = fh3.WriteString(`
job "job1" {
	type = "service"
	datacenters = [ "dc1" ]
	group "group1" {
		count = 1
		task "task1" {
			driver = "exec"
			resources {
				cpu = 1000
				memory = 512
			}
		}
	}
}`)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if code := cmd.Run([]string{"-address=nope", fh3.Name()}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error submitting job") {
		t.Fatalf("expected failed query error, got: %s", out)
	}

	// Fails on invalid check-index (requires a valid job)
	if code := cmd.Run([]string{"-check-index=bad", fh3.Name()}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "parsing check-index") {
		t.Fatalf("expected parse error, got: %s", out)
	}
	ui.ErrorWriter.Reset()

}

func TestRunCommand_From_STDIN(t *testing.T) {
	ci.Parallel(t)
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	ui := cli.NewMockUi()
	cmd := &JobRunCommand{
		Meta:      Meta{Ui: ui},
		JobGetter: JobGetter{testStdin: stdinR},
	}

	go func() {
		stdinW.WriteString(`
job "job1" {
  type = "service"
  datacenters = [ "dc1" ]
  group "group1" {
		count = 1
		task "task1" {
			driver = "exec"
			resources {
				cpu = 1000
				memory = 512
			}
		}
	}
}`)
		stdinW.Close()
	}()

	args := []string{"-address=nope", "-"}
	if code := cmd.Run(args); code != 1 {
		t.Fatalf("expected exit code 1, got %d: %q", code, ui.ErrorWriter.String())
	}

	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error submitting job") {
		t.Fatalf("expected submission error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestRunCommand_From_URL(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobRunCommand{
		Meta: Meta{Ui: ui},
	}

	args := []string{"https://example.com/foo/bar"}
	if code := cmd.Run(args); code != 1 {
		t.Fatalf("expected exit code 1, got %d: %q", code, ui.ErrorWriter.String())
	}

	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error getting jobfile") {
		t.Fatalf("expected error getting jobfile, got: %s", out)
	}
}

// TestRunCommand_JSON asserts that `nomad job run -json` accepts JSON jobs
// with or without a top level Job key.
func TestRunCommand_JSON(t *testing.T) {
	ci.Parallel(t)
	run := func(args ...string) (stdout string, stderr string, code int) {
		ui := cli.NewMockUi()
		cmd := &JobRunCommand{
			Meta: Meta{Ui: ui},
		}
		t.Logf("run: nomad job run %s", strings.Join(args, " "))
		code = cmd.Run(args)
		return ui.OutputWriter.String(), ui.ErrorWriter.String(), code
	}

	// Agent startup is slow, do some work while we wait
	agentReady := make(chan string)
	go func() {
		_, _, addr := testServer(t, false, nil)
		agentReady <- addr
	}()

	// First convert HCL -> JSON with -output
	stdout, stderr, code := run("-output", "asset/example-short.nomad.hcl")
	require.Zero(t, code, stderr)
	require.Empty(t, stderr)
	require.NotEmpty(t, stdout)
	t.Logf("run -output==> %s...", stdout[:12])

	jsonFile := filepath.Join(t.TempDir(), "redis.json")
	require.NoError(t, os.WriteFile(jsonFile, []byte(stdout), 0o640))

	// Wait for agent to start and get its address
	addr := ""
	select {
	case addr = <-agentReady:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for agent to start")
	}

	// Submit JSON
	stdout, stderr, code = run("-detach", "-address", addr, "-json", jsonFile)
	require.Zero(t, code, stderr)
	require.Empty(t, stderr)

	// Read the JSON from the API as it omits the Job envelope and
	// therefore differs from -output
	resp, err := http.Get(addr + "/v1/job/example")
	require.NoError(t, err)
	buf, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.NotEmpty(t, buf)
	t.Logf("/v1/job/example==> %s...", string(buf[:12]))
	require.NoError(t, os.WriteFile(jsonFile, buf, 0o640))

	// Submit JSON
	stdout, stderr, code = run("-detach", "-address", addr, "-json", jsonFile)
	require.Zerof(t, code, "stderr: %s\njson: %s\n", stderr, string(buf))
	require.Empty(t, stderr)
	require.NotEmpty(t, stdout)
}
