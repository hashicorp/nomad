// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestACLRoleCreateCommand_Run(t *testing.T) {
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
	cmd := &ACLRoleCreateCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: url,
		},
	}

	// Test the basic validation on the command.
	must.One(t, cmd.Run([]string{"-address=" + url, "this-command-does-not-take-args"}))
	must.StrContains(t, ui.ErrorWriter.String(), uiMessageNoArguments)

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	must.One(t, cmd.Run([]string{"-address=" + url}))
	must.StrContains(t, ui.ErrorWriter.String(), "ACL role name must be specified using the -name flag")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	must.One(t, cmd.Run([]string{"-address=" + url, `-name="foobar"`}))
	must.StrContains(t, ui.ErrorWriter.String(), "At least one policy name must be specified using the -policy flag")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Create an ACL policy that can be referenced within the ACL role.
	aclPolicy := structs.ACLPolicy{
		Name: "acl-role-cli-test-policy",
		Rules: `namespace "default" {
			policy = "read"
		}
		`,
	}
	err := srv.Agent.Server().State().UpsertACLPolicies(
		structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{&aclPolicy})
	must.NoError(t, err)

	// Create an ACL role.
	args := []string{
		"-address=" + url, "-token=" + rootACLToken.SecretID, "-name=acl-role-cli-test",
		"-policy=acl-role-cli-test-policy", "-description=acl-role-all-the-things",
	}
	must.Zero(t, cmd.Run(args))
	s := ui.OutputWriter.String()
	must.StrContains(t, s, "Name         = acl-role-cli-test")
	must.StrContains(t, s, "Description  = acl-role-all-the-things")
	must.StrContains(t, s, "Policies     = acl-role-cli-test-policy")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}
