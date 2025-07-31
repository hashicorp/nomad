// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestACLPolicySelfCommand_ViaEnvVar(t *testing.T) {
	const policyName = "nw"

	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}
	srv, _, url := testServer(t, true, config)
	t.Cleanup(srv.Shutdown)

	createPolicy := func(t *testing.T, srv *agent.TestAgent, token *structs.ACLToken, job *structs.Job) {
		args := structs.ACLPolicyUpsertRequest{
			Policies: []*structs.ACLPolicy{
				{
					Name:        policyName,
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
		reply := structs.GenericResponse{}
		must.NoError(t, srv.RPC("ACL.UpsertPolicies", &args, &reply))
	}

	runCommand := func(t *testing.T, url, token string) string {
		ui := cli.NewMockUi()
		cmd := &ACLPolicySelfCommand{Meta: Meta{Ui: ui, flagAddress: url}}
		t.Setenv("NOMAD_TOKEN", token)
		must.Zero(t, cmd.Run([]string{"-address=" + url}))
		return ui.OutputWriter.String()
	}

	rootToken := srv.RootToken

	t.Run("SelfPolicy returns correct output for management token", func(t *testing.T) {
		createPolicy(t, srv, rootToken, mock.MinJob())

		out := runCommand(t, url, rootToken.SecretID)
		must.StrContains(t, out, "This is a management token. No individual policies are assigned.")
	})

	t.Run("SelfPolicy returns correct output for client token", func(t *testing.T) {
		job := mock.MinJob()
		createPolicy(t, srv, rootToken, job)

		clientToken := mock.ACLToken()
		clientToken.Policies = []string{policyName}
		must.NoError(t, srv.Agent.Server().State().UpsertACLTokens(
			structs.MsgTypeTestSetup, 1, []*structs.ACLToken{clientToken},
		))

		out := runCommand(t, url, clientToken.SecretID)
		must.StrContains(t, out, policyName)
		must.StrContains(t, out, job.ID)
	})
}
