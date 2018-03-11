package command

import (
	"testing"

	"github.com/hashicorp/nomad/command/agent"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

func TestACLBootstrapCommand(t *testing.T) {
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

// If a bootstrap token has already been created, attempts to create more should
// fail.
func TestACLBootstrapCommand_ExistingBootstrapToken(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	assert.NotNil(srv.RootToken)

	ui := new(cli.MockUi)
	cmd := &ACLBootstrapCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	code := cmd.Run([]string{"-address=" + url})
	assert.Equal(1, code)

	out := ui.OutputWriter.String()
	assert.NotContains(out, "Secret ID")
}

// Attempting to bootstrap a token on a non-ACL enabled server should fail.
func TestACLBootstrapCommand_NonACLServer(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &ACLBootstrapCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	code := cmd.Run([]string{"-address=" + url})
	assert.Equal(1, code)

	out := ui.OutputWriter.String()
	assert.NotContains(out, "Secret ID")
}
