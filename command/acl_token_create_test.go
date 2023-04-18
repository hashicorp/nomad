package command

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
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

	// Request to create a new token with a valid management token that does
	// not have an expiry set.
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "-policy=foo", "-type=client"})
	require.Equal(t, 0, code)

	// Check the output
	out := ui.OutputWriter.String()
	require.Contains(t, out, "[foo]")
	require.Contains(t, out, "Expiry Time  = <none>")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Create a new token that has an expiry TTL set and check the response.
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "-type=management", "-ttl=10m"})
	require.Equal(t, 0, code)

	out = ui.OutputWriter.String()
	require.NotContains(t, out, "Expiry Time  = <none>")
}

func Test_generateACLTokenRoleLinks(t *testing.T) {
	ci.Parallel(t)

	inputRoleNames := []string{
		"duplicate",
		"policy1",
		"policy2",
		"duplicate",
	}
	inputRoleIDs := []string{
		"77a780d8-2dee-7c7f-7822-6f5471c5cbb2",
		"56850b06-a343-a772-1a5c-ad083fd8a50e",
		"77a780d8-2dee-7c7f-7822-6f5471c5cbb2",
		"77a780d8-2dee-7c7f-7822-6f5471c5cbb2",
	}
	expectedOutput := []*api.ACLTokenRoleLink{
		{Name: "duplicate"},
		{Name: "policy1"},
		{Name: "policy2"},
		{ID: "77a780d8-2dee-7c7f-7822-6f5471c5cbb2"},
		{ID: "56850b06-a343-a772-1a5c-ad083fd8a50e"},
	}
	require.ElementsMatch(t, generateACLTokenRoleLinks(inputRoleNames, inputRoleIDs), expectedOutput)
}
