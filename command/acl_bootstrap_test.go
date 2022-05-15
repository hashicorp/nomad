package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

func TestACLBootstrapCommand(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	// create a acl-enabled server without bootstrapping the token
	config := func(c *agent.Config) {
		c.ACL.Enabled = true
		c.ACL.PolicyTTL = 0
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	assert.Nil(srv.RootToken)

	ui := cli.NewMockUi()
	cmd := &ACLBootstrapCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	code := cmd.Run([]string{"-address=" + url})
	assert.Equal(0, code)

	out := ui.OutputWriter.String()
	assert.Contains(out, "Secret ID")
}

// If a bootstrap token has already been created, attempts to create more should
// fail.
func TestACLBootstrapCommand_ExistingBootstrapToken(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	assert.NotNil(srv.RootToken)

	ui := cli.NewMockUi()
	cmd := &ACLBootstrapCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	code := cmd.Run([]string{"-address=" + url})
	assert.Equal(1, code)

	out := ui.OutputWriter.String()
	assert.NotContains(out, "Secret ID")
}

// Attempting to bootstrap a token on a non-ACL enabled server should fail.
func TestACLBootstrapCommand_NonACLServer(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &ACLBootstrapCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	code := cmd.Run([]string{"-address=" + url})
	assert.Equal(1, code)

	out := ui.OutputWriter.String()
	assert.NotContains(out, "Secret ID")
}

// Attempting to bootstrap the server with an operator provided token should
// return the same token in the result.
func TestACLBootstrapCommand_WithOperatorBootstrapToken(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	// create a acl-enabled server without bootstrapping the token
	config := func(c *agent.Config) {
		c.ACL.Enabled = true
		c.ACL.PolicyTTL = 0
	}

	// create a valid token
	mockToken := mock.ACLToken()

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	assert.Nil(srv.RootToken)

	ui := cli.NewMockUi()
	cmd := &ACLBootstrapCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	code := cmd.Run([]string{"-address=" + url, "-bootstrap-token=" + mockToken.SecretID})
	assert.Equal(0, code)

	out := ui.OutputWriter.String()
	if !strings.Contains(out, mockToken.SecretID) {
		t.Fatalf("expected "+mockToken.SecretID+" output, got: %s", out)
	}
}

// Attempting to bootstrap the server with an invalid operator provided token should
// fail.
func TestACLBootstrapCommand_WithBadOperatorBootstrapToken(t *testing.T) {
	ci.Parallel(t)
	assert := assert.New(t)

	// create a acl-enabled server without bootstrapping the token
	config := func(c *agent.Config) {
		c.ACL.Enabled = true
		c.ACL.PolicyTTL = 0
	}

	// create a valid token
	invalidToken := "invalid-token"

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	assert.Nil(srv.RootToken)

	ui := cli.NewMockUi()
	cmd := &ACLBootstrapCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	code := cmd.Run([]string{"-address=" + url, "-bootstrap-token=" + invalidToken})
	assert.Equal(1, code)

	out := ui.OutputWriter.String()
	if strings.Contains(out, invalidToken) {
		t.Fatalf("expected error output, got: %s", out)
	}
}
