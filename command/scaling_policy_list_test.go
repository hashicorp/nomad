// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestScalingPolicyListCommand_Run(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	srv, client, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &ScalingPolicyListCommand{Meta: Meta{Ui: ui}}

	// Perform an initial list, which should return zero results.
	code := cmd.Run([]string{"-address=" + url})
	require.Equal(0, code)
	out := ui.OutputWriter.String()
	require.Contains(out, "No policies found")

	// Generate two test jobs.
	jobs := []*api.Job{testJob("scaling_policy_list_1"), testJob("scaling_policy_list_2")}

	// Generate an example scaling policy.
	scalingPolicy := api.ScalingPolicy{
		Type:    api.ScalingPolicyTypeHorizontal,
		Enabled: pointer.Of(true),
		Min:     pointer.Of(int64(1)),
		Max:     pointer.Of(int64(1)),
	}

	// Iterate the jobs, add the scaling policy and register.
	for _, job := range jobs {
		job.TaskGroups[0].Scaling = &scalingPolicy
		_, _, err := client.Jobs().Register(job, nil)
		require.NoError(err)
	}

	// Perform a new list which should yield results..
	code = cmd.Run([]string{"-address=" + url})
	require.Equal(0, code)
	out = ui.OutputWriter.String()
	require.Contains(out, "ID")
	require.Contains(out, "Enabled")
	require.Contains(out, "Type")
	require.Contains(out, "Target")
	require.Contains(out, "scaling_policy_list_1")
	require.Contains(out, "scaling_policy_list_2")
}
