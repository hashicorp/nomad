// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/cli"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestVolumeClaimListCommand_Run(t *testing.T) {
	ci.Parallel(t)

	config := func(c *agent.Config) {
		c.ACL.Enabled = true
	}

	srv, _, url := testServer(t, true, config)
	state := srv.Agent.Server().State()
	defer srv.Shutdown()

	// get an ACL token
	token := mock.CreatePolicyAndToken(t, state, 999, "good",
		`namespace "*" { capabilities = ["host-volume-read"] }
	     node { policy = "read" }`)
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
		// different Job
		{
			ID:            uuid.Generate(),
			Namespace:     structs.DefaultNamespace,
			JobID:         "bar",
			TaskGroupName: "foo",
			VolumeID:      uuid.Generate(),
			VolumeName:    "foo",
		},
		// different tg
		{
			ID:            uuid.Generate(),
			Namespace:     structs.DefaultNamespace,
			JobID:         "foo",
			TaskGroupName: "bar",
			VolumeID:      uuid.Generate(),
			VolumeName:    "foo",
		},
		// different volume name
		{
			ID:            uuid.Generate(),
			Namespace:     structs.DefaultNamespace,
			JobID:         "foo",
			TaskGroupName: "bar",
			VolumeID:      uuid.Generate(),
			VolumeName:    "bar",
		},
	}

	for _, claim := range existingClaims {
		must.NoError(t, state.UpsertTaskGroupHostVolumeClaim(structs.MsgTypeTestSetup, 1000, claim))
	}

	ui := cli.NewMockUi()
	cmd := &VolumeClaimListCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// List with an invalid token fails
	invalidToken := mock.ACLToken()
	code := cmd.Run([]string{"-address=" + url, "-token=" + invalidToken.SecretID})
	must.One(t, code)

	// List with no token at all
	code = cmd.Run([]string{"-address=" + url})
	must.One(t, code)

	// List with a valid token
	code = cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "-verbose"})
	must.Zero(t, code)
	out := ui.OutputWriter.String()
	must.StrContains(t, out, existingClaims[0].ID)

	// List json
	must.Zero(t, cmd.Run([]string{"-address=" + url, "-token=" + token.SecretID, "-json"}))
	out = ui.OutputWriter.String()
	must.StrContains(t, out, "CreateIndex")

	ui.OutputWriter.Reset()

	// Filter by job "foo" and volume name "foo"
	must.Zero(t, cmd.Run([]string{
		"-address=" + url,
		"-token=" + token.SecretID,
		"-job=" + "foo",
		"-volume-name=" + "foo",
		"-verbose",
	}))
	out = ui.OutputWriter.String()

	// only existingClaims[3] matches this filter
	must.StrContains(t, out, existingClaims[3].ID)
	for _, id := range []string{existingClaims[0].ID, existingClaims[1].ID, existingClaims[2].ID, existingClaims[4].ID} {
		must.StrNotContains(t, out, id, must.Sprintf("did not expect to find %s in %s", id, out))
	}

	ui.OutputWriter.Reset()

	// Prefix list
	must.Zero(t, cmd.Run([]string{
		"-address=" + url,
		"-token=" + token.SecretID,
		"-verbose",
		existingClaims[0].ID[0:2],
	}))
	out = ui.OutputWriter.String()

	must.StrContains(t, out, existingClaims[0].ID)
	for _, id := range []string{existingClaims[1].ID, existingClaims[2].ID, existingClaims[3].ID, existingClaims[4].ID} {
		must.StrNotContains(t, out, id, must.Sprintf("did not expect to find %s in %s", id, out))
	}

	ui.OutputWriter.Reset()
}
