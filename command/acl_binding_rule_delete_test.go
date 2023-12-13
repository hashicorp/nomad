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

func TestACLBindingRuleDeleteCommand_Run(t *testing.T) {
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
	cmd := &ACLBindingRuleDeleteCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: url,
		},
	}

	// Try and delete more than one ACL binding rules.
	code := cmd.Run([]string{"-address=" + url, "acl-binding-rule-1", "acl-binding-rule-2"})
	must.Eq(t, 1, code)
	must.StrContains(t, ui.ErrorWriter.String(), "This command takes one argument")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Try deleting a binding rule that does not exist.
	must.Eq(t, 1, cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID, "acl-binding-rule-1"}))
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

	// Delete the existing ACL binding rule.
	must.Eq(t, 0, cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID, aclBindingRule.ID}))
	must.StrContains(t, ui.OutputWriter.String(), "successfully deleted")
}
