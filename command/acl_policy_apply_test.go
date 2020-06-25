package command

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

func TestACLPolicyApplyCommand(t *testing.T) {
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
	cmd := &ACLPolicyApplyCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a test policy
	policy := mock.ACLPolicy()

	// Get a file
	f, err := ioutil.TempFile("", "nomad-test")
	assert.Nil(err)
	defer os.Remove(f.Name())

	// Write the policy to the file
	err = ioutil.WriteFile(f.Name(), []byte(policy.Rules), 0700)
	assert.Nil(err)

	// Attempt to apply a policy without a valid management token
	code := cmd.Run([]string{"-address=" + url, "-token=foo", "test-policy", f.Name()})
	assert.Equal(1, code)

	// Apply a policy with a valid management token
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "test-policy", f.Name()})
	assert.Equal(0, code)

	// Check the output
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "Successfully wrote") {
		t.Fatalf("bad: %v", out)
	}
}
