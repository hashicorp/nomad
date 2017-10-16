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

func TestACLPolicyInfoCommand(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	state := srv.Agent.Server().State()
	defer srv.Shutdown()

	// Bootstrap an initial ACL token
	token := srv.RootToken
	assert.NotNil(token, "failed to bootstrap ACL token")

	// Create a test ACLPolicy
	policy := &structs.ACLPolicy{
		Name:  "testPolicy",
		Rules: acl.PolicyWrite,
	}
	policy.SetHash()
	assert.Nil(state.UpsertACLPolicies(1000, []*structs.ACLPolicy{policy}))

	ui := new(cli.MockUi)
	cmd := &ACLPolicyInfoCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Attempt to apply a policy without a valid management token
	invalidToken := mock.ACLToken()
	os.Setenv("NOMAD_TOKEN", invalidToken.SecretID)
	code := cmd.Run([]string{"-address=" + url, policy.Name})
	assert.Equal(1, code)

	// Apply a policy with a valid management token
	os.Setenv("NOMAD_TOKEN", token.SecretID)
	code = cmd.Run([]string{"-address=" + url, policy.Name})
	assert.Equal(0, code)

	// Check the output
	out := ui.OutputWriter.String()
	if !strings.Contains(out, policy.Name) {
		t.Fatalf("bad: %v", out)
	}
}
