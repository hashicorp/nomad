package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestLoginCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Build a test server with ACLs enabled.
	srv, _, agentURL := testServer(t, false, func(c *agent.Config) {
		c.ACL.Enabled = true
	})
	defer srv.Shutdown()

	// Wait for the server to start fully.
	testutil.WaitForLeader(t, srv.Agent.RPC)

	ui := cli.NewMockUi()
	cmd := &LoginCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: agentURL,
		},
	}

	// Test the basic validation on the command.
	must.Eq(t, 1, cmd.Run([]string{"-address=" + agentURL, "this-command-does-not-take-args"}))
	must.StrContains(t, ui.ErrorWriter.String(), "This command takes no arguments")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Attempt to call it with an unsupported method type.
	must.Eq(t, 1, cmd.Run([]string{"-address=" + agentURL, "-type=SAML"}))
	must.StrContains(t, ui.ErrorWriter.String(), `Unsupported authentication type "SAML"`)

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Use a valid method type but with incorrect casing so we can ensure this
	// is handled.
	must.Eq(t, 1, cmd.Run([]string{"-address=" + agentURL, "-type=oIdC"}))
	must.StrContains(t, ui.ErrorWriter.String(), "Must specify an auth method name and type, no default found")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Store a default auth method
	state := srv.Agent.Server().State()
	method := &structs.ACLAuthMethod{
		Name:    "test-auth-method",
		Default: true,
		Type:    "JWT",
		Config: &structs.ACLAuthMethodConfig{
			OIDCDiscoveryURL: "http://example.com",
		},
	}
	method.SetHash()
	must.NoError(t, state.UpsertACLAuthMethods(1000, []*structs.ACLAuthMethod{method}))

	// Specify an incorrect type of default method
	must.Eq(t, 1, cmd.Run([]string{"-address=" + agentURL, "-type=OIDC"}))
	must.StrContains(t, ui.ErrorWriter.String(), "Specified type: OIDC does not match the type of the default method: JWT")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// TODO(jrasell) find a way to test the full login flow from the CLI
	//  perspective.
}
