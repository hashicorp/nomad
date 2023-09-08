// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestACLAuthMethodDeleteCommand(t *testing.T) {
	ci.Parallel(t)
	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	state := srv.Agent.Server().State()

	// Bootstrap an initial ACL token
	token := srv.RootToken
	must.NotNil(t, token)

	// Create a test auth method
	method := &structs.ACLAuthMethod{
		Name: "test-auth-method",
	}
	method.SetHash()
	must.NoError(t, state.UpsertACLAuthMethods(1000, []*structs.ACLAuthMethod{method}))

	ui := cli.NewMockUi()
	cmd := &ACLAuthMethodDeleteCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Delete the method without a valid token fails
	invalidToken := mock.ACLToken()
	code := cmd.Run([]string{"-address=" + url, "-token=" + invalidToken.SecretID, method.Name})
	must.One(t, code)

	// Delete the method with a valid management token
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, method.Name})
	must.Zero(t, code)

	// Check the output
	out := ui.OutputWriter.String()
	must.StrContains(t, out, fmt.Sprintf("%s successfully deleted", method.Name))
}
