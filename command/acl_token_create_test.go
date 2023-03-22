package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestACLTokenCreateCommand(t *testing.T) {
	ci.Parallel(t)

	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	defer stopTestAgent(srv)

	// Bootstrap an initial ACL token
	token := srv.RootToken
	must.NotNil(t, token)

	ui := cli.NewMockUi()
	cmd := &ACLTokenCreateCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Request to create a new token without providing a valid management token
	code := cmd.Run([]string{"-address=" + url, "-token=foo", "-policy=foo", "-type=client"})
	must.One(t, code)

	// Request to create a new token with a valid management token
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "-policy=foo", "-type=client"})
	must.Zero(t, code)

	// Check the output
	out := ui.OutputWriter.String()
	must.StrContains(t, out, "[foo]")
}
