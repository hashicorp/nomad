// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestACLBindingRuleUpdateCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Build a test server with ACLs enabled.
	srv, _, url := testServer(t, false, func(c *agent.Config) {
		c.ACL.Enabled = true
	})
	defer srv.Shutdown()

	// Wait for the server to start fully and ensure we have a bootstrap token.
	testutil.WaitForLeader(t, srv.Agent.RPC)
	rootACLToken := srv.RootToken
	require.NotNil(t, rootACLToken)

	ui := cli.NewMockUi()
	cmd := &ACLBindingRuleUpdateCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: url,
		},
	}

	// Try calling the command without setting an ACL binding rule ID arg.
	must.Eq(t, 1, cmd.Run([]string{"-address=" + url}))
	must.StrContains(t, ui.ErrorWriter.String(), "This command takes one argument")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Try calling the command with an ACL binding rule ID that does not exist.
	code := cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID, "catch-me-if-you-can"})
	must.Eq(t, 1, code)
	must.StrContains(t, ui.ErrorWriter.String(), "ACL binding rule not found")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Create an ACL auth method that can be referenced within the ACL binding
	// rule.
	aclAuthMethod := structs.ACLAuthMethod{
		Name:          "acl-binding-rule-cli-test",
		Type:          "OIDC",
		TokenLocality: "local",
		MaxTokenTTL:   10 * time.Hour,
	}
	err := srv.Agent.Server().State().UpsertACLAuthMethods(10, []*structs.ACLAuthMethod{&aclAuthMethod})
	must.NoError(t, err)

	// Create an ACL binding rule.
	aclBindingRule := structs.ACLBindingRule{
		ID:         uuid.Generate(),
		AuthMethod: "acl-binding-rule-cli-test",
		BindType:   "role",
		BindName:   "engineering",
	}
	err = srv.Agent.Server().State().UpsertACLBindingRules(20,
		[]*structs.ACLBindingRule{&aclBindingRule}, false)
	must.NoError(t, err)

	// Try a merge update without setting any parameters to update.
	code = cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID, aclBindingRule.ID})
	must.Eq(t, 1, code)
	must.StrContains(t, ui.ErrorWriter.String(), "Please provide at least one update for the ACL binding rule")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Update the description using the merge method.
	code = cmd.Run([]string{
		"-address=" + url, "-token=" + rootACLToken.SecretID, "-description=badger-badger-badger", aclBindingRule.ID})
	must.Eq(t, 0, code)
	s := ui.OutputWriter.String()
	must.StrContains(t, s, aclBindingRule.ID)
	must.StrContains(t, s, "badger-badger-badger")
	must.StrContains(t, s, "acl-binding-rule-cli-test")
	must.StrContains(t, s, "role")
	must.StrContains(t, s, "engineering")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{
		"-address=" + url, "-token=" + rootACLToken.SecretID, "-no-merge", "-bind-name=engineering-updated", aclBindingRule.ID})
	must.Eq(t, 1, code)
	must.StrContains(t, ui.ErrorWriter.String(), "ACL binding rule bind type must be specified using the -bind-type flag")

	// Update the binding-rule using no-merge with all required flags set.
	code = cmd.Run([]string{
		"-address=" + url, "-token=" + rootACLToken.SecretID, "-no-merge", "-description=badger-badger-badger",
		"-bind-name=engineering-updated", "-bind-type=policy", aclBindingRule.ID})
	must.Eq(t, 0, code)
	s = ui.OutputWriter.String()
	must.StrContains(t, s, aclBindingRule.ID)
	must.StrContains(t, s, "badger-badger-badger")
	must.StrContains(t, s, "acl-binding-rule-cli-test")
	must.StrContains(t, s, "policy")
	must.StrContains(t, s, "engineering-updated")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}
