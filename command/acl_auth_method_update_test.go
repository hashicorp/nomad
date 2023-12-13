// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestACLAuthMethodUpdateCommand_Run(t *testing.T) {
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
	cmd := &ACLAuthMethodUpdateCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: url,
		},
	}

	// Try calling the command without setting the method name argument
	must.One(t, cmd.Run([]string{"-address=" + url}))
	must.StrContains(t, ui.ErrorWriter.String(), "This command takes one argument")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Try calling the command with a method name that doesn't exist
	code := cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID, "catch-me-if-you-can"})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "ACL auth-method not found")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Create a test auth method
	ttl, _ := time.ParseDuration("3600s")
	method := &structs.ACLAuthMethod{
		Name:          "test-auth-method",
		Type:          "OIDC",
		MaxTokenTTL:   ttl,
		TokenLocality: "local",
		Config: &structs.ACLAuthMethodConfig{
			OIDCDiscoveryURL: "http://example.com",
		},
	}
	method.SetHash()
	must.NoError(t, srv.Agent.Server().State().UpsertACLAuthMethods(1000, []*structs.ACLAuthMethod{method}))

	// Try an update without setting any parameters to update.
	code = cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID, method.Name})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "Please provide at least one flag to update the ACL auth method")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Update the token locality
	code = cmd.Run([]string{
		"-address=" + url, "-token=" + rootACLToken.SecretID, "-token-locality=global", method.Name})
	must.Zero(t, code)
	s := ui.OutputWriter.String()
	must.StrContains(t, s, method.Name)

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Update an auth method with a config from file
	configFile, err := os.CreateTemp("", "config.json")
	defer os.Remove(configFile.Name())
	must.Nil(t, err)

	conf := map[string]interface{}{"OIDCDiscoveryURL": "http://example.com"}
	jsonData, err := json.Marshal(conf)
	must.Nil(t, err)

	_, err = configFile.Write(jsonData)
	must.Nil(t, err)

	code = cmd.Run([]string{
		"-address=" + url,
		"-token=" + rootACLToken.SecretID,
		fmt.Sprintf("-config=@%s", configFile.Name()),
		method.Name,
	})
	must.Zero(t, code)
	s = ui.OutputWriter.String()
	must.StrContains(t, s, method.Name)

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Update a default auth method
	code = cmd.Run([]string{
		"-address=" + url, "-token=" + rootACLToken.SecretID, "-default=true", method.Name})
	must.Zero(t, code)
	s = ui.OutputWriter.String()
	must.StrContains(t, s, method.Name)

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}
