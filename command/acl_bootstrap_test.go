package command

import (
	"testing"

	"github.com/hashicorp/nomad/command/agent"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

func TestACLBootstrapCommand_Implements(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	// create a acl-enabled server without bootstrapping the token
	config := func(c *agent.Config) {
		c.ACL.Enabled = true
		c.ACL.PolicyTTL = 0
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	assert.Nil(srv.RootToken)

	ui := new(cli.MockUi)
	cmd := &ACLBootstrapCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	code := cmd.Run([]string{"-address=" + url})
	assert.Equal(0, code)

	out := ui.OutputWriter.String()
	assert.Contains(out, "Secret ID")
}
