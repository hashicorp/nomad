// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

func TestJobAllocsCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobAllocsCommand{}
}

func TestJobAllocsCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobAllocsCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	outerr := ui.ErrorWriter.String()
	must.One(t, code)
	must.StrContains(t, outerr, commandErrorText(cmd))

	ui.ErrorWriter.Reset()

	// Bad address
	code = cmd.Run([]string{"-address=nope", "foo"})
	outerr = ui.ErrorWriter.String()
	must.One(t, code)
	must.StrContains(t, outerr, "Error querying job prefix")

	ui.ErrorWriter.Reset()

	// Bad job name
	code = cmd.Run([]string{"-address=" + url, "foo"})
	outerr = ui.ErrorWriter.String()
	must.One(t, code)
	must.StrContains(t, outerr, "No job(s) with prefix or ID \"foo\" found")

	ui.ErrorWriter.Reset()
}

func TestJobAllocsCommand_Run(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobAllocsCommand{Meta: Meta{Ui: ui}}

	// Create a job without an allocation
	job := mock.Job()
	state := srv.Agent.Server().State()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job))

	// Should display no match if the job doesn't have allocations
	code := cmd.Run([]string{"-address=" + url, job.ID})
	out := ui.OutputWriter.String()
	must.Zero(t, code)
	must.StrContains(t, out, "No allocations placed")

	ui.OutputWriter.Reset()

	// Inject an allocation
	a := mock.Alloc()
	a.Job = job
	a.JobID = job.ID
	a.TaskGroup = job.TaskGroups[0].Name
	a.Metrics = &structs.AllocMetric{}
	a.DesiredStatus = structs.AllocDesiredStatusRun
	a.ClientStatus = structs.AllocClientStatusRunning
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 200, []*structs.Allocation{a}))

	// Should now display the alloc
	code = cmd.Run([]string{"-address=" + url, "-verbose", job.ID})
	out = ui.OutputWriter.String()
	outerr := ui.ErrorWriter.String()
	must.Zero(t, code)
	must.Eq(t, "", outerr)
	must.StrContains(t, out, a.ID)

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}

func TestJobAllocsCommand_Template(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobAllocsCommand{Meta: Meta{Ui: ui}}

	// Create a job
	job := mock.Job()
	state := srv.Agent.Server().State()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job))

	// Inject a running allocation
	a := mock.Alloc()
	a.Job = job
	a.JobID = job.ID
	a.TaskGroup = job.TaskGroups[0].Name
	a.Metrics = &structs.AllocMetric{}
	a.DesiredStatus = structs.AllocDesiredStatusRun
	a.ClientStatus = structs.AllocClientStatusRunning
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 200, []*structs.Allocation{a}))

	// Inject a pending allocation
	b := mock.Alloc()
	b.Job = job
	b.JobID = job.ID
	b.TaskGroup = job.TaskGroups[0].Name
	b.Metrics = &structs.AllocMetric{}
	b.DesiredStatus = structs.AllocDesiredStatusRun
	b.ClientStatus = structs.AllocClientStatusPending
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 300, []*structs.Allocation{b}))

	// Should display an AllocacitonListStub object
	code := cmd.Run([]string{"-address=" + url, "-t", "'{{printf \"%#+v\" .}}'", job.ID})
	out := ui.OutputWriter.String()
	outerr := ui.ErrorWriter.String()

	must.Zero(t, code)
	must.Eq(t, "", outerr)
	must.StrContains(t, out, "api.AllocationListStub")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Should display only the running allocation ID
	code = cmd.Run([]string{"-address=" + url, "-t", "'{{ range . }}{{ if eq .ClientStatus \"running\" }}{{ println .ID }}{{ end }}{{ end }}'", job.ID})
	out = ui.OutputWriter.String()
	outerr = ui.ErrorWriter.String()

	must.Zero(t, code)
	must.Eq(t, "", outerr)
	must.StrContains(t, out, a.ID)
	must.StrNotContains(t, out, b.ID)

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}

func TestJobAllocsCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobAllocsCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake job
	state := srv.Agent.Server().State()
	j := mock.Job()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, j))

	prefix := j.ID[:len(j.ID)-5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.SliceLen(t, 1, res)
	must.Eq(t, j.ID, res[0])
}

func TestJobAllocsCommand_ACL(t *testing.T) {
	ci.Parallel(t)

	// Start server with ACL enabled.
	srv, _, url := testServer(t, true, func(c *agent.Config) {
		c.ACL.Enabled = true
	})
	defer srv.Shutdown()

	// Create a job with an alloc.
	job := mock.Job()
	state := srv.Agent.Server().State()
	err := state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job)
	must.NoError(t, err)

	a := mock.Alloc()
	a.Job = job
	a.JobID = job.ID
	a.TaskGroup = job.TaskGroups[0].Name
	a.Metrics = &structs.AllocMetric{}
	a.DesiredStatus = structs.AllocDesiredStatusRun
	a.ClientStatus = structs.AllocClientStatusRunning
	err = state.UpsertAllocs(structs.MsgTypeTestSetup, 200, []*structs.Allocation{a})
	must.NoError(t, err)

	testCases := []struct {
		name        string
		jobPrefix   bool
		aclPolicy   string
		expectedErr string
		expectedOut string
	}{
		{
			name:        "no token",
			aclPolicy:   "",
			expectedErr: api.PermissionDeniedErrorContent,
		},
		{
			name: "missing read-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["alloc-lifecycle"]
}
`,
			expectedErr: api.PermissionDeniedErrorContent,
		},
		{
			name: "read-job allowed",
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job"]
}
`,
		},
		{
			name:      "job prefix requires list-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job"]
}
`,
			expectedOut: "No allocations",
		},
		{
			name:      "job prefix works with list-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job", "list-jobs"]
}
`,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &JobAllocsCommand{Meta: Meta{Ui: ui}}
			args := []string{
				"-address", url,
			}

			if tc.aclPolicy != "" {
				// Create ACL token with test case policy and add it to the
				// command.
				policyName := nonAlphaNum.ReplaceAllString(tc.name, "-")
				token := mock.CreatePolicyAndToken(t, state, uint64(302+i), policyName, tc.aclPolicy)
				args = append(args, "-token", token.SecretID)
			}

			// Add job ID or job ID prefix to the command.
			if tc.jobPrefix {
				args = append(args, job.ID[:3])
			} else {
				args = append(args, job.ID)
			}

			// Run command.
			code := cmd.Run(args)
			if tc.expectedErr == "" {
				must.Zero(t, code)
			} else {
				must.One(t, code)
				must.StrContains(t, ui.ErrorWriter.String(), tc.expectedErr)
			}
			if tc.expectedOut != "" {
				must.StrContains(t, ui.OutputWriter.String(), tc.expectedOut)
			}
		})
	}
}
