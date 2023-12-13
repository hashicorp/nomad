// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestACLBindingRuleCreateCommand_Run(t *testing.T) {
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
	cmd := &ACLBindingRuleCreateCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: url,
		},
	}

	// Test the basic validation on the command.
	must.Eq(t, 1, cmd.Run([]string{"-address=" + url, "this-command-does-not-take-args"}))
	must.StrContains(t, ui.ErrorWriter.String(), "This command takes no arguments")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	must.Eq(t, 1, cmd.Run([]string{"-address=" + url}))
	must.StrContains(t, ui.ErrorWriter.String(),
		"ACL binding rule auth method must be specified using the -auth-method flag")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	must.Eq(t, 1, cmd.Run([]string{"-address=" + url, "-auth-method=auth0", "-bind-name=engineering"}))
	must.StrContains(t, ui.ErrorWriter.String(),
		"ACL binding rule bind type must be specified using the -bind-type flag")

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

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Create the ACL binding rule.
	args := []string{
		"-address=" + url, "-token=" + rootACLToken.SecretID, "-auth-method=acl-binding-rule-cli-test",
		"-bind-name=engineering", "-bind-type=role", "-selector=engineering in list.groups",
	}
	must.Eq(t, 0, cmd.Run(args))
	s := ui.OutputWriter.String()
	must.StrContains(t, s, "acl-binding-rule-cli-test")
	must.StrContains(t, s, "role")
	must.StrContains(t, s, "engineering")
	must.StrContains(t, s, "engineering in list.groups")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}
