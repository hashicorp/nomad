// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestACLTokenListCommand(t *testing.T) {
	ci.Parallel(t)

	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	state := srv.Agent.Server().State()

	// Bootstrap an initial ACL token
	token := srv.RootToken
	must.NotNil(t, token)

	// Create a valid token
	mockToken := mock.ACLToken()
	mockToken.Policies = []string{acl.PolicyWrite}
	mockToken.SetHash()
	must.NoError(t, state.UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{mockToken}))

	ui := cli.NewMockUi()
	cmd := &ACLTokenListCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Attempt to list tokens without a valid management token
	invalidToken := mock.ACLToken()
	code := cmd.Run([]string{"-address=" + url, "-token=" + invalidToken.SecretID})
	must.One(t, code)

	// Apply a token with a valid management token
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID})
	must.Zero(t, code)

	// Check the output
	out := ui.OutputWriter.String()
	must.StrContains(t, out, mockToken.Name)

	// List json
	must.Zero(t, cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "-json"}))

	out = ui.OutputWriter.String()
	must.StrContains(t, out, "CreateIndex")
	ui.OutputWriter.Reset()
}
