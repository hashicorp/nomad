// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestACLAuthMethodCreateCommand_Run(t *testing.T) {
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
	cmd := &ACLAuthMethodCreateCommand{
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
	must.StrContains(t, ui.ErrorWriter.String(), "ACL auth method name must be specified using the -name flag")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	must.Eq(t, 1, cmd.Run([]string{"-address=" + url, "-name=foobar", "-token-locality=global", "-max-token-ttl=3600s"}))
	must.StrContains(t, ui.ErrorWriter.String(), "ACL auth method type must be set to 'OIDC'")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	must.Eq(t, 1, cmd.Run([]string{"-address=" + url, "-name=foobar", "-type=OIDC", "-token-locality=global", "-max-token-ttl=3600s"}))
	must.StrContains(t, ui.ErrorWriter.String(), "Must provide ACL auth method config in JSON format")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Create an auth method
	args := []string{
		"-address=" + url, "-token=" + rootACLToken.SecretID, "-name=acl-auth-method-cli-test",
		"-type=OIDC", "-token-locality=global", "-default=true", "-max-token-ttl=3600s",
		"-config={\"OIDCDiscoveryURL\":\"http://example.com\", \"ExpirationLeeway\": \"1h\"}",
	}
	must.Eq(t, 0, cmd.Run(args))
	s := ui.OutputWriter.String()
	must.StrContains(t, s, "acl-auth-method-cli-test")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Create an auth method with a config from file
	configFile, err := os.CreateTemp("", "config.json")
	defer os.Remove(configFile.Name())
	must.Nil(t, err)

	conf := map[string]interface{}{"OIDCDiscoveryURL": "http://example.com"}
	jsonData, err := json.Marshal(conf)
	must.Nil(t, err)

	_, err = configFile.Write(jsonData)
	must.Nil(t, err)

	args = []string{
		"-address=" + url, "-token=" + rootACLToken.SecretID, "-name=acl-auth-method-cli-test",
		"-type=OIDC", "-token-locality=global", "-default=false", "-max-token-ttl=3600s",
		fmt.Sprintf("-config=@%s", configFile.Name()),
	}
	must.Eq(t, 0, cmd.Run(args))
	s = ui.OutputWriter.String()
	must.StrContains(t, s, "acl-auth-method-cli-test")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}
