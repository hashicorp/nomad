// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestValidateCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobValidateCommand{}
}

func TestValidateCommand_Files(t *testing.T) {

	// Create a Vault server
	v := testutil.NewTestVault(t)
	defer v.Stop()

	// Create a Nomad server
	s := testutil.NewTestServer(t, func(c *testutil.TestServerConfig) {
		c.Vault.Address = v.HTTPAddr
		c.Vault.Enabled = true
		c.Vault.AllowUnauthenticated = false
		c.Vault.Token = v.RootToken
	})
	defer s.Stop()

	t.Run("basic", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobValidateCommand{Meta: Meta{Ui: ui, flagAddress: "http://" + s.HTTPAddr}}
		args := []string{"testdata/example-basic.nomad"}
		code := cmd.Run(args)
		require.Equal(t, 0, code)
	})

	t.Run("vault no token", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobValidateCommand{Meta: Meta{Ui: ui}}
		args := []string{"-address", "http://" + s.HTTPAddr, "testdata/example-vault.nomad"}
		code := cmd.Run(args)
		require.Contains(t, ui.ErrorWriter.String(), "* Vault used in the job but missing Vault token")
		require.Equal(t, 1, code)
	})

	t.Run("vault bad token via flag", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobValidateCommand{Meta: Meta{Ui: ui}}
		args := []string{"-address", "http://" + s.HTTPAddr, "-vault-token=abc123", "testdata/example-vault.nomad"}
		code := cmd.Run(args)
		require.Contains(t, ui.ErrorWriter.String(), "* bad token")
		require.Equal(t, 1, code)
	})

	t.Run("vault token bad via env", func(t *testing.T) {
		t.Setenv("VAULT_TOKEN", "abc123")
		ui := cli.NewMockUi()
		cmd := &JobValidateCommand{Meta: Meta{Ui: ui}}
		args := []string{"-address", "http://" + s.HTTPAddr, "testdata/example-vault.nomad"}
		code := cmd.Run(args)
		require.Contains(t, ui.ErrorWriter.String(), "* bad token")
		require.Equal(t, 1, code)
	})
}
func TestValidateCommand_hcl1_hcl2_strict(t *testing.T) {
	ci.Parallel(t)

	_, _, addr := testServer(t, false, nil)

	t.Run("-hcl1 implies -hcl2-strict is false", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &JobValidateCommand{Meta: Meta{Ui: ui}}
		got := cmd.Run([]string{
			"-hcl1", "-hcl2-strict",
			"-address", addr,
			"asset/example-short.nomad.hcl",
		})
		require.Equal(t, 0, got, ui.ErrorWriter.String())
	})
}

func TestValidateCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobValidateCommand{Meta: Meta{Ui: ui}}

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
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Job validation errors") {
		t.Fatalf("expect validation error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestValidateCommand_From_STDIN(t *testing.T) {
	ci.Parallel(t)
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Create a server
	s := testutil.NewTestServer(t, nil)
	defer s.Stop()

	ui := cli.NewMockUi()
	cmd := &JobValidateCommand{
		Meta:      Meta{Ui: ui, flagAddress: "http://" + s.HTTPAddr},
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
						config {
							command = "/bin/echo"
						}
                        resources {
                                cpu = 1000
                                memory = 512
                        }
                }
        }
}`)
		stdinW.Close()
	}()

	args := []string{"-"}
	if code := cmd.Run(args); code != 0 {
		t.Fatalf("expected exit code 0, got %d: %q", code, ui.ErrorWriter.String())
	}
	ui.ErrorWriter.Reset()
}

func TestValidateCommand_From_URL(t *testing.T) {
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

func TestValidateCommand_JSON(t *testing.T) {
	ci.Parallel(t)

	_, _, addr := testServer(t, false, nil)

	ui := cli.NewMockUi()
	cmd := &JobValidateCommand{
		Meta: Meta{Ui: ui},
	}

	code := cmd.Run([]string{"-address", addr, "-json", "testdata/example-short.json"})

	require.Zerof(t, code, "stdout: %s\nstdout: %s\n",
		ui.OutputWriter.String(), ui.ErrorWriter.String())

	code = cmd.Run([]string{"-address", addr, "-json", "testdata/example-short-bad.json"})

	require.Equalf(t, 1, code, "stdout: %s\nstdout: %s\n",
		ui.OutputWriter.String(), ui.ErrorWriter.String())
}
