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

func TestACLTokenUpdateCommand(t *testing.T) {
	ci.Parallel(t)

	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	// Bootstrap an initial ACL token
	token := srv.RootToken
	must.NotNil(t, token)

	ui := cli.NewMockUi()
	cmd := &ACLTokenUpdateCommand{Meta: Meta{Ui: ui, flagAddress: url}}
	state := srv.Agent.Server().State()

	// Create a valid token
	mockToken := mock.ACLToken()
	mockToken.Policies = []string{acl.PolicyWrite}
	mockToken.SetHash()
	must.NoError(t, state.UpsertACLTokens(structs.MsgTypeTestSetup, 1000, []*structs.ACLToken{mockToken}))

	// Request to update a new token without providing a valid management token
	invalidToken := mock.ACLToken()
	code := cmd.Run([]string{"--token=" + invalidToken.SecretID, "-address=" + url, "-name=bar", mockToken.AccessorID})
	must.One(t, code)

	// Request to update a new token with a valid management token
	code = cmd.Run([]string{"--token=" + token.SecretID, "-address=" + url, "-name=bar", mockToken.AccessorID})
	must.Zero(t, code)

	// Check the output
	out := ui.OutputWriter.String()
	must.StrContains(t, out, mockToken.AccessorID)
	must.StrContains(t, out, "bar")
}
