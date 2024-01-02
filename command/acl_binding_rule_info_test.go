// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestACLBindingRuleInfoCommand_Run(t *testing.T) {
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
	cmd := &ACLBindingRuleInfoCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: url,
		},
	}

	// Perform a lookup without specifying an ID.
	must.Eq(t, 1, cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID}))
	must.StrContains(t, ui.ErrorWriter.String(), "This command takes one argument: <acl_binding_rule_id>")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Perform a lookup specifying a random ID.
	must.Eq(t, 1, cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID, uuid.Generate()}))
	must.StrContains(t, ui.ErrorWriter.String(), "ACL binding rule not found")

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

	// Look up the ACL binding rule.
	must.Eq(t, 0, cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID, aclBindingRule.ID}))
	s := ui.OutputWriter.String()
	must.StrContains(t, s, "auth0")
	must.StrContains(t, s, "role")
	must.StrContains(t, s, "engineering")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Test that the JSON flag works in return a string that has JSON markers.
	must.Eq(t, 0, cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID, "-json", aclBindingRule.ID}))
	s = ui.OutputWriter.String()
	must.StrContains(t, s, "{")
	must.StrContains(t, s, "}")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Test that we can pass in a custom go template to format the output.
	must.Eq(t, 0, cmd.Run([]string{
		"-address=" + url, "-token=" + rootACLToken.SecretID, "-t", "Custom: {{ .ID }}", aclBindingRule.ID}))
	s = ui.OutputWriter.String()
	must.StrContains(t, s, fmt.Sprintf("Custom: %s", aclBindingRule.ID))

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}
