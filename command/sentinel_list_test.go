package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/uuid"
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

	srv, cl, url := testServer(t, true, func(c *agent.Config) {
		c.ACL.Enabled = true
	})
	defer srv.Shutdown()

	// Wait for the server to start fully and ensure we have a bootstrap token.
	testutil.WaitForLeader(t, srv.Agent.RPC)

	rootACLToken := srv.RootToken
	must.NotNil(t, rootACLToken)

	ui := cli.NewMockUi()

	apHandle := cl.SentinelPolicies()

	uuid1 := uuid.Generate()
	sp1 := &api.SentinelPolicy{
		Name:        fmt.Sprintf("sent-policy-%s", uuid1),
		Description: "Super cool policy!",
	}

	_, err := apHandle.Upsert(sp1, nil)
	must.NoError(t, err)

	uuid2 := uuid.Generate()
	sp2 := &api.SentinelPolicy{
		Name:        fmt.Sprintf("sent-policy-%s", uuid2),
		Description: "Super cool policy!",
	}

	_, err = apHandle.Upsert(sp2, nil)
	must.NoError(t, err)

	cmd := &SentinelListCommand{Meta: Meta{Ui: ui}}
	code := cmd.Run([]string{"-address=" + url, "-token=" + rootACLToken.SecretID})
	must.Zero(t, code)

	out := ui.OutputWriter.String()
	must.StrContains(t, out, "Super cool policy")
	must.StrContains(t, out, fmt.Sprintf("sent-policy-%s", uuid1))
	must.StrContains(t, out, fmt.Sprintf("sent-policy-%s", uuid2))

	ui.OutputWriter.Reset()
}
