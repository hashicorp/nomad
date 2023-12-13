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

func TestACLAuthMethodInfoCommand(t *testing.T) {
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
	cmd := &ACLAuthMethodInfoCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Attempt to get info without a valid management token
	invalidToken := mock.ACLToken()
	code := cmd.Run([]string{"-address=" + url, "-token=" + invalidToken.SecretID, method.Name})
	must.One(t, code)

	// Get info with a valid management token
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, method.Name})
	must.Zero(t, code)

	// Check the output
	out := ui.OutputWriter.String()
	must.StrContains(t, out, method.Name)
}
