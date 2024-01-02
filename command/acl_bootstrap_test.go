// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"os"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestACLBootstrapCommand(t *testing.T) {
	ci.Parallel(t)

	// create a acl-enabled server without bootstrapping the token
	config := func(c *agent.Config) {
		c.ACL.Enabled = true
		c.ACL.PolicyTTL = 0
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	must.Nil(t, srv.RootToken)

	ui := cli.NewMockUi()
	cmd := &ACLBootstrapCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	code := cmd.Run([]string{"-address=" + url})
	must.Zero(t, code)

	out := ui.OutputWriter.String()
	must.StrContains(t, out, "Secret ID")
	require.Contains(t, out, "Expiry Time  = <none>")
}

// If a bootstrap token has already been created, attempts to create more should
// fail.
func TestACLBootstrapCommand_ExistingBootstrapToken(t *testing.T) {
	ci.Parallel(t)

	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	must.NotNil(t, srv.RootToken)

	ui := cli.NewMockUi()
	cmd := &ACLBootstrapCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	code := cmd.Run([]string{"-address=" + url})
	must.One(t, code)

	out := ui.OutputWriter.String()
	must.StrNotContains(t, out, "Secret ID")
}

// Attempting to bootstrap a token on a non-ACL enabled server should fail.
func TestACLBootstrapCommand_NonACLServer(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &ACLBootstrapCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	code := cmd.Run([]string{"-address=" + url})
	must.One(t, code)

	out := ui.OutputWriter.String()
	must.StrNotContains(t, out, "Secret ID")
}

// Attempting to bootstrap the server with an operator provided token in a file should
// return the same token in the result.
func TestACLBootstrapCommand_WithOperatorFileBootstrapToken(t *testing.T) {
	ci.Parallel(t)
	// create a acl-enabled server without bootstrapping the token
	config := func(c *agent.Config) {
		c.ACL.Enabled = true
		c.ACL.PolicyTTL = 0
	}

	// create a valid token
	mockToken := mock.ACLToken()

	// Create temp file
	file, rm := getTempFile(t, "nomad-token.token")
	t.Cleanup(rm)

	// Write the token to the file
	err := os.WriteFile(file, []byte(mockToken.SecretID), 0700)
	must.NoError(t, err)

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	must.Nil(t, srv.RootToken)

	ui := cli.NewMockUi()
	cmd := &ACLBootstrapCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	code := cmd.Run([]string{"-address=" + url, file})
	must.Zero(t, code)

	out := ui.OutputWriter.String()
	must.StrContains(t, out, mockToken.SecretID)
	require.Contains(t, out, "Expiry Time  = <none>")
}

// Attempting to bootstrap the server with an invalid operator provided token in a file should
// fail.
func TestACLBootstrapCommand_WithBadOperatorFileBootstrapToken(t *testing.T) {
	ci.Parallel(t)

	// create a acl-enabled server without bootstrapping the token
	config := func(c *agent.Config) {
		c.ACL.Enabled = true
		c.ACL.PolicyTTL = 0
	}

	// create a invalid token
	invalidToken := "invalid-token"

	// Create temp file
	file, cleanup := getTempFile(t, "nomad-token.token")
	t.Cleanup(cleanup)

	// Write the token to the file
	err := os.WriteFile(file, []byte(invalidToken), 0700)
	must.NoError(t, err)

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	must.Nil(t, srv.RootToken)

	ui := cli.NewMockUi()
	cmd := &ACLBootstrapCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	code := cmd.Run([]string{"-address=" + url, file})
	must.One(t, code)

	out := ui.OutputWriter.String()
	must.StrNotContains(t, out, invalidToken)
}
