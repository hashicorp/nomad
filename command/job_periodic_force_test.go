// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestJobPeriodicForceCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobPeriodicForceCommand{}
}

func TestJobPeriodicForceCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobPeriodicForceCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, code, 1, "expected error")
	out := ui.ErrorWriter.String()
	require.Contains(t, out, commandErrorText(cmd), "expected help output")
	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{"-address=nope", "12"})
	require.Equal(t, code, 1, "expected error")
	out = ui.ErrorWriter.String()
	require.Contains(t, out, "Error querying job prefix", "expected force error")
}

func TestJobPeriodicForceCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobPeriodicForceCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake job, not periodic
	state := srv.Agent.Server().State()
	j := mock.Job()
	require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, j))

	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(complete.Args{Last: j.ID[:len(j.ID)-5]})
	require.Empty(t, res)

	// Create another fake job, periodic
	state = srv.Agent.Server().State()
	j2 := mock.Job()
	j2.Periodic = &structs.PeriodicConfig{
		Enabled:         true,
		Spec:            "spec",
		SpecType:        "cron",
		ProhibitOverlap: true,
		TimeZone:        "test zone",
	}
	require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, nil, j2))

	res = predictor.Predict(complete.Args{Last: j2.ID[:len(j.ID)-5]})
	require.Equal(t, []string{j2.ID}, res)

	res = predictor.Predict(complete.Args{})
	require.Equal(t, []string{j2.ID}, res)
}

func TestJobPeriodicForceCommand_NonPeriodicJob(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		if len(nodes) == 0 {
			return false, fmt.Errorf("missing node")
		}
		if _, ok := nodes[0].Drivers["mock_driver"]; !ok {
			return false, fmt.Errorf("mock_driver not ready")
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// Register a job
	j := testJob("job_not_periodic")

	ui := cli.NewMockUi()
	cmd := &JobPeriodicForceCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	resp, _, err := client.Jobs().Register(j, nil)
	require.NoError(t, err)
	code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
	require.Equal(t, 0, code)

	code = cmd.Run([]string{"-address=" + url, "job_not_periodic"})
	require.Equal(t, 1, code, "expected exit code")
	out := ui.ErrorWriter.String()
	require.Contains(t, out, "No periodic job(s)", "non-periodic error message")
}

func TestJobPeriodicForceCommand_SuccessfulPeriodicForceDetach(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		if len(nodes) == 0 {
			return false, fmt.Errorf("missing node")
		}
		if _, ok := nodes[0].Drivers["mock_driver"]; !ok {
			return false, fmt.Errorf("mock_driver not ready")
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// Register a job
	j := testJob("job1_is_periodic")
	j.Periodic = &api.PeriodicConfig{
		SpecType:        pointer.Of(api.PeriodicSpecCron),
		Spec:            pointer.Of("*/15 * * * * *"),
		ProhibitOverlap: pointer.Of(true),
		TimeZone:        pointer.Of("Europe/Minsk"),
	}

	ui := cli.NewMockUi()
	cmd := &JobPeriodicForceCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	_, _, err := client.Jobs().Register(j, nil)
	require.NoError(t, err)

	code := cmd.Run([]string{"-address=" + url, "-detach", "job1_is_periodic"})
	require.Equal(t, 0, code, "expected no error code")
	out := ui.OutputWriter.String()
	require.Contains(t, out, "Force periodic successful")
	require.Contains(t, out, "Evaluation ID:")
}

func TestJobPeriodicForceCommand_SuccessfulPeriodicForce(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		if len(nodes) == 0 {
			return false, fmt.Errorf("missing node")
		}
		if _, ok := nodes[0].Drivers["mock_driver"]; !ok {
			return false, fmt.Errorf("mock_driver not ready")
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// Register a job
	j := testJob("job2_is_periodic")
	j.Periodic = &api.PeriodicConfig{
		SpecType:        pointer.Of(api.PeriodicSpecCron),
		Spec:            pointer.Of("*/15 * * * * *"),
		ProhibitOverlap: pointer.Of(true),
		TimeZone:        pointer.Of("Europe/Minsk"),
	}

	ui := cli.NewMockUi()
	cmd := &JobPeriodicForceCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	_, _, err := client.Jobs().Register(j, nil)
	require.NoError(t, err)

	code := cmd.Run([]string{"-address=" + url, "job2_is_periodic"})
	require.Equal(t, 0, code, "expected no error code")
	out := ui.OutputWriter.String()
	require.Contains(t, out, "Monitoring evaluation")
	require.Contains(t, out, "finished with status \"complete\"")
}

func TestJobPeriodicForceCommand_SuccessfulIfJobIDEqualsPrefix(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		if len(nodes) == 0 {
			return false, fmt.Errorf("missing node")
		}
		if _, ok := nodes[0].Drivers["mock_driver"]; !ok {
			return false, fmt.Errorf("mock_driver not ready")
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	j1 := testJob("periodic-prefix")
	j1.Periodic = &api.PeriodicConfig{
		SpecType:        pointer.Of(api.PeriodicSpecCron),
		Spec:            pointer.Of("*/15 * * * * *"),
		ProhibitOverlap: pointer.Of(true),
		TimeZone:        pointer.Of("Europe/Minsk"),
	}
	j2 := testJob("periodic-prefix-another-job")
	j2.Periodic = &api.PeriodicConfig{
		SpecType:        pointer.Of(api.PeriodicSpecCron),
		Spec:            pointer.Of("*/15 * * * * *"),
		ProhibitOverlap: pointer.Of(true),
		TimeZone:        pointer.Of("Europe/Minsk"),
	}

	ui := cli.NewMockUi()
	cmd := &JobPeriodicForceCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	_, _, err := client.Jobs().Register(j1, nil)
	require.NoError(t, err)
	_, _, err = client.Jobs().Register(j2, nil)
	require.NoError(t, err)

	code := cmd.Run([]string{"-address=" + url, "periodic-prefix"})
	require.Equal(t, 0, code, "expected no error code")
	out := ui.OutputWriter.String()
	require.Contains(t, out, "Monitoring evaluation")
	require.Contains(t, out, "finished with status \"complete\"")
}

func TestJobPeriodicForceCommand_ACL(t *testing.T) {
	ci.Parallel(t)

	// Start server with ACL enabled.
	srv, client, url := testServer(t, true, func(c *agent.Config) {
		c.ACL.Enabled = true
	})
	defer srv.Shutdown()
	client.SetSecretID(srv.RootToken.SecretID)

	// Create a periodic job.
	jobID := "test_job_periodic_force_acl"
	job := testJob(jobID)
	job.Periodic = &api.PeriodicConfig{
		SpecType: pointer.Of(api.PeriodicSpecCron),
		Spec:     pointer.Of("*/15 * * * * *"),
	}

	rootTokenOpts := &api.WriteOptions{
		AuthToken: srv.RootToken.SecretID,
	}
	_, _, err := client.Jobs().Register(job, rootTokenOpts)
	must.NoError(t, err)

	testCases := []struct {
		name        string
		jobPrefix   bool
		aclPolicy   string
		expectedErr string
	}{
		{
			name:        "no token",
			aclPolicy:   "",
			expectedErr: api.PermissionDeniedErrorContent,
		},
		{
			name: "missing submit-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["list-jobs"]
}
`,
			expectedErr: api.PermissionDeniedErrorContent,
		},
		{
			name: "submit-job allowed but can't monitor eval without read-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["submit-job"]
}
`,
			expectedErr: "No evaluation with id",
		},
		{
			name: "submit-job allowed and can monitor eval with read-job",
			aclPolicy: `
namespace "default" {
	capabilities = ["submit-job", "read-job"]
}
`,
		},
		{
			name:      "job prefix requires list-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["submit-job"]
}
`,
			expectedErr: "job not found",
		},
		{
			name:      "job prefix works with list-job but can't monitor eval without read-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["submit-job", "list-jobs"]
}
`,
			expectedErr: "No evaluation with id",
		},
		{
			name:      "job prefix works with list-job and can monitor eval with read-job",
			jobPrefix: true,
			aclPolicy: `
namespace "default" {
	capabilities = ["read-job", "submit-job", "list-jobs"]
}
`,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &JobPeriodicForceCommand{Meta: Meta{Ui: ui}}
			args := []string{
				"-address", url,
			}

			if tc.aclPolicy != "" {
				state := srv.Agent.Server().State()

				// Create ACL token with test case policy and add it to the
				// command.
				policyName := nonAlphaNum.ReplaceAllString(tc.name, "-")
				token := mock.CreatePolicyAndToken(t, state, uint64(302+i), policyName, tc.aclPolicy)
				args = append(args, "-token", token.SecretID)
			}

			// Add job ID or job ID prefix to the command.
			if tc.jobPrefix {
				args = append(args, jobID[:3])
			} else {
				args = append(args, jobID)
			}

			// Run command.
			code := cmd.Run(args)
			if tc.expectedErr == "" {
				must.Zero(t, code)
			} else {
				must.One(t, code)
				must.StrContains(t, ui.ErrorWriter.String(), tc.expectedErr)
			}
		})
	}
}
