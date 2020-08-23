package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/command/agent"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

func TestACLTokenCreateCommand(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	// Bootstrap an initial ACL token
	token := srv.RootToken
	assert.NotNil(token, "failed to bootstrap ACL token")

	ui := new(cli.MockUi)
	cmd := &ACLTokenCreateCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Request to create a new token without providing a valid management token
	code := cmd.Run([]string{"-address=" + url, "-token=foo", "-policy=foo", "-type=client"})
	assert.Equal(1, code)

	// Request to create a new token with a valid management token
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "-policy=foo", "-type=client"})
	assert.Equal(0, code)

	// Check the output
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "[foo]") {
		t.Fatalf("bad: %v", out)
	}
}
