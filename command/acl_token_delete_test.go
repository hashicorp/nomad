package command

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

func TestACLTokenDeleteCommand_ViaEnvVariable(t *testing.T) {
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
	cmd := &ACLTokenDeleteCommand{Meta: Meta{Ui: ui, flagAddress: url}}
	state := srv.Agent.Server().State()

	// Create a valid token
	mockToken := mock.ACLToken()
	mockToken.Policies = []string{acl.PolicyWrite}
	mockToken.SetHash()
	assert.Nil(state.UpsertACLTokens(1000, []*structs.ACLToken{mockToken}))

	// Attempt to delete a token without providing a valid token with delete
	// permissions
	os.Setenv("NOMAD_TOKEN", "foo")
	code := cmd.Run([]string{"-address=" + url, mockToken.AccessorID})
	assert.Equal(1, code)

	// Delete a token using a valid management token set via an environment
	// variable
	os.Setenv("NOMAD_TOKEN", token.SecretID)
	code = cmd.Run([]string{"-address=" + url, mockToken.AccessorID})
	assert.Equal(0, code)

	// Check the output
	out := ui.OutputWriter.String()
	if !strings.Contains(out, fmt.Sprintf("Token %s successfully deleted", mockToken.AccessorID)) {
		t.Fatalf("bad: %v", out)
	}
}
