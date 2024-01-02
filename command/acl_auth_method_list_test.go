// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestACLAuthMethodListCommand(t *testing.T) {
	ci.Parallel(t)

	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	state := srv.Agent.Server().State()
	defer srv.Shutdown()

	// Bootstrap an initial ACL token
	token := srv.RootToken
	must.NotNil(t, token)

	// Create a test auth method
	method := &structs.ACLAuthMethod{
		Name: "test-auth-method",
		Config: &structs.ACLAuthMethodConfig{
			OIDCDiscoveryURL: "http://example.com",
		},
	}
	method.SetHash()
	must.NoError(t, state.UpsertACLAuthMethods(1000, []*structs.ACLAuthMethod{method}))

	ui := cli.NewMockUi()
	cmd := &ACLAuthMethodListCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// List with an invalid token works fine
	invalidToken := mock.ACLToken()
	code := cmd.Run([]string{"-address=" + url, "-token=" + invalidToken.SecretID})
	must.Zero(t, code)

	// List with a valid management token
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID})
	must.Zero(t, code)

	// List with no token at all
	code = cmd.Run([]string{"-address=" + url})
	must.Zero(t, code)

	// Check the output
	out := ui.OutputWriter.String()
	must.StrContains(t, out, method.Name)

	// List json
	must.Zero(t, cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "-json"}))

	out = ui.OutputWriter.String()
	must.StrContains(t, out, "CreateIndex")
	ui.OutputWriter.Reset()
}
