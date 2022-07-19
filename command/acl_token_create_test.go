package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestACLTokenCreateCommand(t *testing.T) {
	ci.Parallel(t)

	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	// Bootstrap an initial ACL token
	token := srv.RootToken
	require.NotNil(t, token, "failed to bootstrap ACL token")

	ui := cli.NewMockUi()
	cmd := &ACLTokenCreateCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Request to create a new token without providing a valid management token
	code := cmd.Run([]string{"-address=" + url, "-token=foo", "-policy=foo", "-type=client"})
	require.Equal(t, 1, code)

	// Request to create a new token with a valid management token that does
	// not have an expiry set.
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "-policy=foo", "-type=client"})
	require.Equal(t, 0, code)

	// Check the output
	out := ui.OutputWriter.String()
	require.Contains(t, out, "[foo]")
	require.Contains(t, out, "Expiry Time  = never")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Create a new token that has an expiry TTL set and check the response.
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "-type=management", "-ttl=10m"})
	require.Equal(t, 0, code)

	out = ui.OutputWriter.String()
	require.NotContains(t, out, "Expiry Time  = never")
}
