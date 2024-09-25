// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestJobTagCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobTagCommand{}
}
func TestJobTagApplyCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobTagApplyCommand{}
}

// Top-level, nomad job tag doesn't do anything on its own but list subcommands.
func TestJobTagCommand_Help(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobTagCommand{Meta: Meta{Ui: ui}}
	code := cmd.Run([]string{})
	must.Eq(t, -18511, code)
}

func TestJobTagApplyCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Create a job with multiple versions
	v0 := mock.Job()

	state := srv.Agent.Server().State()

	v0.ID = "test-job-applyer"
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, v0))

	v1 := v0.Copy()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, v1))

	v2 := v0.Copy()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1002, nil, v2))

	v3 := v0.Copy()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1003, nil, v3))

	ui := cli.NewMockUi()
	cmd := &JobTagApplyCommand{Meta: Meta{Ui: ui}}

	// not passing a name errors
	code := cmd.Run([]string{"-address=" + url, v0.ID})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "name is required")
	ui.ErrorWriter.Reset()

	// passing a non-integer version errors
	code = cmd.Run([]string{"-address=" + url, "-name=test", "-version=abc", v0.ID})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "Error parsing version")
	ui.ErrorWriter.Reset()

	// passing a specific version is fine and tags that version
	code = cmd.Run([]string{"-address=" + url, "-name=test", "-version=0", v0.ID})
	must.Zero(t, code)
	must.StrContains(t, ui.OutputWriter.String(), "Job version 0 tagged with name \"test\"")
	ui.OutputWriter.Reset()

	// passing no version is fine and defaults to the latest version of the job
	code = cmd.Run([]string{"-address=" + url, "-name=test2", v0.ID})
	apiJob, _, err := client.Jobs().Info(v0.ID, nil)
	must.NoError(t, err)
	must.NotNil(t, apiJob.VersionTag)
	must.Eq(t, "test2", apiJob.VersionTag.Name)
	must.StrContains(t, ui.OutputWriter.String(), "Job version 3 tagged with name \"test2\"")
	must.Zero(t, code)
	ui.OutputWriter.Reset()

	// passing a jobname that doesn't exist errors
	code = cmd.Run([]string{"-address=" + url, "-name=test", "non-existent-job"})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "No job(s) with prefix or ID \"non-existent-job\" found")
	ui.ErrorWriter.Reset()

	// passing a version that doesn't exist errors
	code = cmd.Run([]string{"-address=" + url, "-name=test3", "-version=999", v0.ID})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "version 999 not found")
	ui.ErrorWriter.Reset()

	// passing a version with the same name and different version fails
	code = cmd.Run([]string{"-address=" + url, "-name=test", "-version=1", v0.ID})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "tag \"test\" already exists on a different version of job")
	ui.ErrorWriter.Reset()
}

func TestJobTagUnsetCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobTagUnsetCommand{Meta: Meta{Ui: ui}}

	// Create a job with multiple versions
	v0 := mock.Job()

	state := srv.Agent.Server().State()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, v0)
	must.NoError(t, err)

	v0.ID = "test-job-unsetter"
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, v0))

	v1 := v0.Copy()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1001, nil, v1))

	v2 := v0.Copy()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1002, nil, v2))

	v3 := v0.Copy()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1003, nil, v3))

	// not passing a name errors
	code := cmd.Run([]string{"-address=" + url, v0.ID})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "name is required")
	ui.ErrorWriter.Reset()

	// passing a jobname that doesn't exist errors
	code = cmd.Run([]string{"-address=" + url, "-name=test", "non-existent-job"})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "No job(s) with prefix or ID")
	ui.ErrorWriter.Reset()

	// passing a name that doesn't exist errors
	code = cmd.Run([]string{"-address=" + url, "-name=non-existent-tag", v0.ID})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "tag \"non-existent-tag\" not found")
	ui.ErrorWriter.Reset()

	// successfully unsetting a tag
	_, err = client.Jobs().TagVersion(v0.ID, 0, "test-tag", "Test description", nil)
	must.NoError(t, err)

	code = cmd.Run([]string{"-address=" + url, "-name=test-tag", v0.ID})
	must.Zero(t, code)
	// check the output
	must.StrContains(t, ui.OutputWriter.String(), "Tag \"test-tag\" removed from job \"test-job-unsetter\"")
	ui.OutputWriter.Reset()

	// attempting to unset a tag that was just unset
	code = cmd.Run([]string{"-address=" + url, "-name=test-tag", v0.ID})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "tag \"test-tag\" not found")
	ui.ErrorWriter.Reset()
}
