package command

import (
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

func TestACLTokenSelfCommand_ViaEnvVar(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()
	state := srv.Agent.Server().State()

	// Bootstrap an initial ACL token
	token := srv.RootToken
	assert.NotNil(token, "failed to bootstrap ACL token")

	ui := new(cli.MockUi)
	cmd := &ACLTokenSelfCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a valid token
	mockToken := mock.ACLToken()
	mockToken.Policies = []string{acl.PolicyWrite}
	mockToken.SetHash()
	assert.Nil(state.UpsertACLTokens(1000, []*structs.ACLToken{mockToken}))

	// Attempt to fetch info on a token without providing a valid management
	// token
	invalidToken := mock.ACLToken()
	os.Setenv("NOMAD_TOKEN", invalidToken.SecretID)
	code := cmd.Run([]string{"-address=" + url})
	assert.Equal(1, code)

	// Fetch info on a token with a valid token
	os.Setenv("NOMAD_TOKEN", mockToken.SecretID)
	code = cmd.Run([]string{"-address=" + url})
	assert.Equal(0, code)

	// Check the output
	out := ui.OutputWriter.String()
	if !strings.Contains(out, mockToken.AccessorID) {
		t.Fatalf("bad: %v", out)
	}
}
