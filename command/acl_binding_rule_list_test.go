// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestACLBindingRuleListCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Build a test server with ACLs enabled.
	srv, _, url := testServer(t, false, func(c *agent.Config) {
		c.ACL.Enabled = true
	})
	defer srv.Shutdown()

	// Wait for the server to start fully and ensure we have a bootstrap token.
	testutil.WaitForLeader(t, srv.Agent.RPC)
	rootACLToken := srv.RootToken
	must.NotNil(t, rootACLToken)

	ui := cli.NewMockUi()
	cmd := &ACLBindingRuleListCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: url,
		},
	}

	// Perform a list straight away without any roles held in state.
	must.Eq(t, 0, cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID}))
	must.StrContains(t, ui.OutputWriter.String(), "No ACL binding rules found")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Create an ACL binding rule.
	aclBindingRule := structs.ACLBindingRule{
		ID:         uuid.Generate(),
		AuthMethod: "auth0",
		BindType:   "role",
		BindName:   "engineering",
	}
	err := srv.Agent.Server().State().UpsertACLBindingRules(10,
		[]*structs.ACLBindingRule{&aclBindingRule}, true)
	must.NoError(t, err)

	// Perform a listing to get the created binding rule.
	must.Eq(t, 0, cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID}))
	s := ui.OutputWriter.String()
	must.StrContains(t, s, "ID")
	must.StrContains(t, s, "Description")
	must.StrContains(t, s, "Auth Method")
	must.StrContains(t, s, "auth0")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}
