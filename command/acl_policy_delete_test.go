package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

func TestACLPolicyDeleteCommand(t *testing.T) {
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
	assert.Nil(state.UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{policy}))

	ui := cli.NewMockUi()
	cmd := &ACLPolicyDeleteCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Delete the policy without a valid token fails
	invalidToken := mock.ACLToken()
	code := cmd.Run([]string{"-address=" + url, "-token=" + invalidToken.SecretID, policy.Name})
	assert.Equal(1, code)

	// Delete the policy with a valid management token
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, policy.Name})
	assert.Equal(0, code)

	// Check the output
	out := ui.OutputWriter.String()
	if !strings.Contains(out, fmt.Sprintf("Successfully deleted %s policy", policy.Name)) {
		t.Fatalf("bad: %v", out)
	}
}
