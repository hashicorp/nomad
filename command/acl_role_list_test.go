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
	"github.com/stretchr/testify/require"
)

func TestACLRoleListCommand_Run(t *testing.T) {
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
	cmd := &ACLRoleListCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: url,
		},
	}

	// Perform a list straight away without any roles held in state.
	require.Equal(t, 0, cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID}))
	require.Contains(t, ui.OutputWriter.String(), "No ACL roles found")

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
	require.NoError(t, err)

	// Create an ACL role referencing the previously created policy.
	aclRole := structs.ACLRole{
		ID:       uuid.Generate(),
		Name:     "acl-role-cli-test",
		Policies: []*structs.ACLRolePolicyLink{{Name: aclPolicy.Name}},
	}
	err = srv.Agent.Server().State().UpsertACLRoles(
		structs.MsgTypeTestSetup, 20, []*structs.ACLRole{&aclRole}, false)
	require.NoError(t, err)

	// Perform a listing to get the created role.
	require.Equal(t, 0, cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID}))
	s := ui.OutputWriter.String()
	require.Contains(t, s, "ID")
	require.Contains(t, s, "Name")
	require.Contains(t, s, "Policies")
	require.Contains(t, s, "acl-role-cli-test")
	require.Contains(t, s, "acl-role-policy-cli-test")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}
