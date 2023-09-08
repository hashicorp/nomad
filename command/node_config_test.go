// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/mitchellh/cli"
)

func TestClientConfigCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NodeConfigCommand{}
}

func TestClientConfigCommand_UpdateServers(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, true, func(c *agent.Config) {
		c.Server.BootstrapExpect = 0
	})
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &NodeConfigCommand{Meta: Meta{Ui: ui}}

	// Fails if trying to update with no servers
	code := cmd.Run([]string{"-update-servers"})
	if code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Set the servers list with bad addresses
	code = cmd.Run([]string{"-address=" + url, "-update-servers", "127.0.0.42", "198.18.5.5"})
	if code != 1 {
		t.Fatalf("expected exit 1, got: %d", code)
	}

	// Set the servers list with good addresses
	code = cmd.Run([]string{"-address=" + url, "-update-servers", srv.Config.AdvertiseAddrs.RPC})
	if code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
}

func TestClientConfigCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &NodeConfigCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails if no valid flags given
	if code := cmd.Run([]string{}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope", "-servers"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying server list") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
}
