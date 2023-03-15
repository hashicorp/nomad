package command

import (
	"testing"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestSentinelListCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &SentinelListCommand{}
}

func TestSentinelListCommand_Run(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, false, func(c *agent.Config) {
		c.ACL.Enabled = true
	})
	defer srv.Shutdown()

	state := srv.Agent.Server().State()

	// Wait for the server to start fully and ensure we have a bootstrap token.
	testutil.WaitForLeader(t, srv.Agent.RPC)
	rootACLToken := srv.RootToken
	must.NotNil(t, rootACLToken)

	ui := cli.NewMockUi()
	//cmd := &SentinelListCommand{Meta: Meta{Ui: ui}}

	// Create a test ACLPolicy
	policy := &structs.ACLPolicy{
		Name:  "testPolicy",
		Rules: acl.PolicyWrite,
	}

	policy.SetHash()
	err := state.UpsertACLPolicies(structs.MsgTypeTestSetup, 1000, []*structs.ACLPolicy{policy})
	must.NoError(t, err)

	cmd := &SentinelListCommand{Meta: Meta{Ui: ui}}
	code := cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID})
	must.Zero(t, code)

	out := ui.OutputWriter.String()
	must.StrContains(t, out, `Name         Scope       Enforcement Level  Description`)
	must.StrContains(t, out, "testPolicy")

	ui.OutputWriter.Reset()
}
