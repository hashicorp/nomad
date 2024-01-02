// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestLoginCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Build a test server with ACLs enabled.
	srv, _, agentURL := testServer(t, false, func(c *agent.Config) {
		c.ACL.Enabled = true
	})
	defer srv.Shutdown()

	// Wait for the server to start fully.
	testutil.WaitForLeader(t, srv.Agent.RPC)

	ui := cli.NewMockUi()
	cmd := &LoginCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: agentURL,
		},
	}

	// Test the basic validation on the command.
	must.Eq(t, 1, cmd.Run([]string{"-address=" + agentURL, "this-command-does-not-take-args"}))
	must.StrContains(t, ui.ErrorWriter.String(), "This command takes no arguments")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Attempt to run the command without specifying a method name, when there's no default available
	must.Eq(t, 1, cmd.Run([]string{"-address=" + agentURL}))
	must.StrContains(t, ui.ErrorWriter.String(), "Must specify an auth method name, no default found")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Attempt to login using a non-existing method
	must.Eq(t, 1, cmd.Run([]string{"-address=" + agentURL, "-method", "there-is-no-such-method"}))
	must.StrContains(t, ui.ErrorWriter.String(), "Error: method there-is-no-such-method not found in the state store. ")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Store a default auth method
	state := srv.Agent.Server().State()
	method := &structs.ACLAuthMethod{
		Name:    "test-auth-method",
		Default: true,
		Type:    "JWT",
		Config: &structs.ACLAuthMethodConfig{
			OIDCDiscoveryURL: "http://example.com",
		},
	}
	method.SetHash()
	must.NoError(t, state.UpsertACLAuthMethods(1000, []*structs.ACLAuthMethod{method}))

	// Try logging in with non-OIDC method and no token (expected error)
	must.Eq(t, 1, cmd.Run([]string{"-address=" + agentURL}))
	must.StrContains(t, ui.ErrorWriter.String(), "You need to provide a login token.")
	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// TODO(jrasell) find a way to test the full login flow from the CLI
	//  perspective.
}
