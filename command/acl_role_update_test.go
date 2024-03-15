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

func TestACLRoleUpdateCommand_Run(t *testing.T) {
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
	cmd := &ACLRoleUpdateCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: url,
		},
	}

	// Try calling the command without setting an ACL Role ID arg.
	must.One(t, cmd.Run([]string{"-address=" + url}))
	must.StrContains(t, ui.ErrorWriter.String(), "This command takes one argument")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Try calling the command with an ACL role ID that does not exist.
	code := cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID, "catch-me-if-you-can"})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "ACL role not found")

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

	// Create an ACL role that can be used for updating.
	aclRole := structs.ACLRole{
		ID:          uuid.Generate(),
		Name:        "acl-role-cli-test",
		Description: "my-lovely-role",
		Policies:    []*structs.ACLRolePolicyLink{{Name: aclPolicy.Name}},
	}

	err = srv.Agent.Server().State().UpsertACLRoles(
		structs.MsgTypeTestSetup, 20, []*structs.ACLRole{&aclRole}, false)
	must.NoError(t, err)

	// Try a merge update without setting any parameters to update.
	code = cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID, aclRole.ID})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "Please provide at least one flag to update the ACL role")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Update the description using the merge method.
	code = cmd.Run([]string{
		"-address=" + url, "-token=" + rootACLToken.SecretID, "-description=badger-badger-badger", aclRole.ID})
	must.Zero(t, code)
	s := ui.OutputWriter.String()
	must.StrContains(t, s, fmt.Sprintf("ID           = %s", aclRole.ID))
	must.StrContains(t, s, "Name         = acl-role-cli-test")
	must.StrContains(t, s, "Description  = badger-badger-badger")
	must.StrContains(t, s, "Policies     = acl-role-cli-test-policy")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Try updating the role using no-merge without setting the required flags.
	code = cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID, "-no-merge", aclRole.ID})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "ACL role name must be specified using the -name flag")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{
		"-address=" + url, "-token=" + rootACLToken.SecretID, "-no-merge", "-name=update-role-name", aclRole.ID})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "At least one policy name must be specified using the -policy flag")

	// Update the role using no-merge with all required flags set.
	code = cmd.Run([]string{
		"-address=" + url, "-token=" + rootACLToken.SecretID, "-no-merge", "-name=update-role-name",
		"-description=updated-description", "-policy=acl-role-cli-test-policy", aclRole.ID})
	must.Zero(t, code)
	s = ui.OutputWriter.String()
	must.StrContains(t, s, fmt.Sprintf("ID           = %s", aclRole.ID))
	must.StrContains(t, s, "Name         = update-role-name")
	must.StrContains(t, s, "Description  = updated-description")
	must.StrContains(t, s, "Policies     = acl-role-cli-test-policy")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}
