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

func TestACLRoleInfoCommand_Run(t *testing.T) {
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
	cmd := &ACLRoleInfoCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: url,
		},
	}

	// Perform a lookup without specifying an ID.
	must.One(t, cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID}))
	must.StrContains(t, ui.ErrorWriter.String(), "This command takes one argument: <acl_role_id>")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Perform a lookup specifying a random ID.
	must.One(t, cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID, uuid.Generate()}))
	must.StrContains(t, ui.ErrorWriter.String(), "ACL role not found")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Create an ACL policy that can be referenced within the ACL role.
	aclPolicy := structs.ACLPolicy{
		Name: "acl-role-policy-cli-test",
		Rules: `namespace "default" {
			policy = "read"
		}
		`,
	}
	err := srv.Agent.Server().State().UpsertACLPolicies(
		structs.MsgTypeTestSetup, 10, []*structs.ACLPolicy{&aclPolicy})
	must.NoError(t, err)

	// Create an ACL role referencing the previously created policy.
	aclRole := structs.ACLRole{
		ID:       uuid.Generate(),
		Name:     "acl-role-cli-test",
		Policies: []*structs.ACLRolePolicyLink{{Name: aclPolicy.Name}},
	}
	err = srv.Agent.Server().State().UpsertACLRoles(
		structs.MsgTypeTestSetup, 20, []*structs.ACLRole{&aclRole}, false)
	must.NoError(t, err)

	// Look up the ACL role using its ID.
	must.Zero(t, cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID, aclRole.ID}))
	s := ui.OutputWriter.String()
	must.StrContains(t, s, fmt.Sprintf("ID           = %s", aclRole.ID))
	must.StrContains(t, s, fmt.Sprintf("Name         = %s", aclRole.Name))
	must.StrContains(t, s, "Description  = <none>")
	must.StrContains(t, s, fmt.Sprintf("Policies     = %s", aclPolicy.Name))

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Look up the ACL role using its Name.
	must.Zero(t, cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID, "-by-name", aclRole.Name}))
	s = ui.OutputWriter.String()
	must.StrContains(t, s, fmt.Sprintf("ID           = %s", aclRole.ID))
	must.StrContains(t, s, fmt.Sprintf("Name         = %s", aclRole.Name))
	must.StrContains(t, s, "Description  = <none>")
	must.StrContains(t, s, fmt.Sprintf("Policies     = %s", aclPolicy.Name))

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}
