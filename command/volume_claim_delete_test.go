// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestVolumeClaimDeleteCommand_Run(t *testing.T) {
	ci.Parallel(t)

	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	state := srv.Agent.Server().State()
	defer srv.Shutdown()

	// get an ACL token
	token := mock.CreatePolicyAndToken(t, state, 999, "good",
		`namespace "*" { capabilities = ["host-volume-write"] }
	     node { policy = "write" }`)
	must.NotNil(t, token)

	// Create some test claims
	existingClaims := []*structs.TaskGroupHostVolumeClaim{
		{
			ID:            uuid.Generate(),
			Namespace:     structs.DefaultNamespace,
			JobID:         "foo",
			TaskGroupName: "foo",
			VolumeID:      uuid.Generate(),
			VolumeName:    "bar",
		},
		// different NS
		{
			ID:            uuid.Generate(),
			Namespace:     "foo",
			JobID:         "foo",
			TaskGroupName: "foo",
			VolumeID:      uuid.Generate(),
			VolumeName:    "foo",
		},
	}

	for _, claim := range existingClaims {
		must.NoError(t, state.UpsertTaskGroupHostVolumeClaim(structs.MsgTypeTestSetup, 1000, claim))
	}

	ui := cli.NewMockUi()
	cmd := &VolumeClaimDeleteCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Delete with an invalid token fails
	invalidToken := mock.ACLToken()
	must.One(t, cmd.Run([]string{"-address=" + url, "-token=" + invalidToken.SecretID, existingClaims[0].ID}))

	// Delete with a valid token
	must.Zero(t, cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, existingClaims[0].ID}))
	out := ui.OutputWriter.String()
	must.StrContains(t, out, "successfully deleted")

	ui.OutputWriter.Reset()

	// List and make sure there are no tokens left (we have no permissions to read foo ns)
	listCmd := &VolumeClaimListCommand{Meta: Meta{Ui: ui, flagAddress: url}}
	must.Zero(t, listCmd.Run([]string{
		"-address=" + url,
		"-token=" + token.SecretID,
	}))
	out = ui.OutputWriter.String()

	must.StrContains(t, out, "No task group host volume claims found")

	ui.OutputWriter.Reset()
}
