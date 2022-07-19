package command

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.Contains(t, out, "Expiry Time  = never")
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

// Attempting to bootstrap the server with an operator provided token in a file should
// return the same token in the result.
func TestACLBootstrapCommand_WithOperatorFileBootstrapToken(t *testing.T) {
	ci.Parallel(t)
	// create a acl-enabled server without bootstrapping the token
	config := func(c *agent.Config) {
		c.ACL.Enabled = true
		c.ACL.PolicyTTL = 0
	}

	// create a valid token
	mockToken := mock.ACLToken()

	// Create temp file
	f, err := ioutil.TempFile("", "nomad-token.token")
	assert.Nil(t, err)
	defer os.Remove(f.Name())

	// Write the token to the file
	err = ioutil.WriteFile(f.Name(), []byte(mockToken.SecretID), 0700)
	assert.Nil(t, err)

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	require.Nil(t, srv.RootToken)

	ui := cli.NewMockUi()
	cmd := &ACLBootstrapCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	code := cmd.Run([]string{"-address=" + url, f.Name()})
	assert.Equal(t, 0, code)

	out := ui.OutputWriter.String()
	assert.Contains(t, out, mockToken.SecretID)
	require.Contains(t, out, "Expiry Time  = never")
}

// Attempting to bootstrap the server with an invalid operator provided token in a file should
// fail.
func TestACLBootstrapCommand_WithBadOperatorFileBootstrapToken(t *testing.T) {
	ci.Parallel(t)

	// create a acl-enabled server without bootstrapping the token
	config := func(c *agent.Config) {
		c.ACL.Enabled = true
		c.ACL.PolicyTTL = 0
	}

	// create a invalid token
	invalidToken := "invalid-token"

	// Create temp file
	f, err := ioutil.TempFile("", "nomad-token.token")
	assert.Nil(t, err)
	defer os.Remove(f.Name())

	// Write the token to the file
	err = ioutil.WriteFile(f.Name(), []byte(invalidToken), 0700)
	assert.Nil(t, err)

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	assert.Nil(t, srv.RootToken)

	ui := cli.NewMockUi()
	cmd := &ACLBootstrapCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	code := cmd.Run([]string{"-address=" + url, f.Name()})
	assert.Equal(t, 1, code)

	out := ui.OutputWriter.String()
	assert.NotContains(t, out, invalidToken)
}
