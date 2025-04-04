// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestACLPolicySelfCommand_ViaEnvVar(t *testing.T) {
	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	defer srv.Shutdown()

	state := srv.Agent.Server().State()

	// Bootstrap an initial ACL token
	token := srv.RootToken
	must.NotNil(t, token)

	// Create a minimal job
	job := mock.MinJob()

	// Add a job policy
	polArgs := structs.ACLPolicyUpsertRequest{
		Policies: []*structs.ACLPolicy{
			{
				Name:        "nw",
				Description: "test job can write to nodes",
				Rules:       `node { policy = "write" }`,
				JobACL: &structs.JobACL{
					Namespace: job.Namespace,
					JobID:     job.ID,
				},
			},
		},
		WriteRequest: structs.WriteRequest{
			Region:    job.Region,
			AuthToken: token.SecretID,
			Namespace: job.Namespace,
		},
	}
	polReply := structs.GenericResponse{}
	must.NoError(t, srv.RPC("ACL.UpsertPolicies", &polArgs, &polReply))
	must.NonZero(t, polReply.WriteMeta.Index)

	ui := cli.NewMockUi()
	cmd := &ACLPolicySelfCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	allocs := testutil.WaitForRunningWithToken(t, srv.RPC, job, token.SecretID)
	must.Len(t, 1, allocs)

	alloc, err := state.AllocByID(nil, allocs[0].ID)
	must.NoError(t, err)
	must.MapContainsKey(t, alloc.SignedIdentities, "t")
	wid := alloc.SignedIdentities["t"]

	// Fetch info on policies with a JWT
	t.Setenv("NOMAD_TOKEN", wid)
	code := cmd.Run([]string{"-address=" + url})
	must.Zero(t, code)

	// Check the output
	out := ui.OutputWriter.String()
	must.StrContains(t, out, polArgs.Policies[0].Name)

	// make sure we put the job ACLs in there, too
	must.StrContains(t, out, polArgs.Policies[0].JobACL.JobID)
}
