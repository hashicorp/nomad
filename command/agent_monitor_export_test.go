// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/shoenig/test/must"
)

func TestMonitorExportCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &MonitorExportCommand{}
}

func TestMonitorExportCommand_Fails(t *testing.T) {
	const expectedText = "log log log log log"

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.log")
	must.NoError(t, os.WriteFile(testFile, []byte(expectedText), 0777))
	config := func(c *agent.Config) {
		c.LogFile = testFile
	}

	srv, _, url := testServer(t, false, config)
	defer srv.Shutdown()
	cases := []struct {
		name       string
		cmdArgs    []string
		defaultErr bool
		errString  string
	}{
		{
			name:       "misuse",
			cmdArgs:    []string{"some", "bad", "args"},
			defaultErr: true,
		},
		{
			name:      "no address",
			cmdArgs:   []string{"-address=nope"},
			errString: "unsupported protocol scheme",
		},
		{
			name:      "invalid follow boolean",
			cmdArgs:   []string{"-address=" + url, "-follow=maybe"},
			errString: `invalid boolean value "maybe" for -follow`,
		},
		{
			name:      "invalid on-disk boolean",
			cmdArgs:   []string{"-address=" + url, "-on-disk=maybe"},
			errString: `invalid boolean value "maybe" for -on-disk`,
		},
		{
			name:      "setting both on-disk and service-name",
			cmdArgs:   []string{"-address=" + url, "-on-disk=true", "-service-name=nomad"},
			errString: "journald and nomad log file simultaneously",
		},
		{
			name:      "setting neither on-disk nor service-name",
			cmdArgs:   []string{"-address=" + url},
			errString: "One of -service-name or -on-disk must be set",
		},
		{
			name:      "requires nomad in service name",
			cmdArgs:   []string{"-address=" + url, "-service-name=docker.path"},
			errString: "does not include 'nomad'",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &MonitorExportCommand{Meta: Meta{Ui: ui}}

			code := cmd.Run(tc.cmdArgs)
			must.One(t, code)

			out := ui.ErrorWriter.String()
			if tc.defaultErr {
				must.StrContains(t, out, commandErrorText(cmd))
			} else {
				must.StrContains(t, out, tc.errString)
			}
		})
	}
}
