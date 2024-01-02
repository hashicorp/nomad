// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/testutil"
)

func TestRecommendationInfoCommand_Run(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
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
		t.Fatalf("err: %s", err)
	})

	ui := cli.NewMockUi()
	cmd := &RecommendationInfoCommand{
		RecommendationAutocompleteCommand: RecommendationAutocompleteCommand{
			Meta: Meta{Ui: ui},
		},
	}

	// Perform an initial call, which should return a not found error.
	code := cmd.Run([]string{"-address=" + url, "2c13f001-f5b6-ce36-03a5-e37afe160df5"})
	if srv.Enterprise {
		require.Equal(1, code)
		out := ui.ErrorWriter.String()
		require.Contains(out, "Recommendation not found")
	} else {
		require.Equal(1, code)
		require.Contains(ui.ErrorWriter.String(), "Nomad Enterprise only endpoint")
	}

	// Register a test job to write a recommendation against.
	testJob := testJob("recommendation_info")
	regResp, _, err := client.Jobs().Register(testJob, nil)
	require.NoError(err)
	registerCode := waitForSuccess(ui, client, fullId, t, regResp.EvalID)
	require.Equal(0, registerCode)

	// Write a recommendation.
	rec := api.Recommendation{
		JobID:    *testJob.ID,
		Group:    *testJob.TaskGroups[0].Name,
		Task:     testJob.TaskGroups[0].Tasks[0].Name,
		Resource: "CPU",
		Value:    1050,
		Meta:     map[string]interface{}{"test-meta-entry": "test-meta-value"},
		Stats:    map[string]float64{"p13": 1.13},
	}
	recResp, _, err := client.Recommendations().Upsert(&rec, nil)
	if srv.Enterprise {
		require.NoError(err)
	} else {
		require.Error(err, "Nomad Enterprise only endpoint")
	}

	// Only perform the call if we are running enterprise tests. Otherwise the
	// recResp object will be nil.
	if srv.Enterprise {
		code = cmd.Run([]string{"-address=" + url, recResp.ID})
		require.Equal(0, code)
		out := ui.OutputWriter.String()
		require.Contains(out, "test-meta-entry")
		require.Contains(out, "p13")
		require.Contains(out, "1.13")
		require.Contains(out, recResp.ID)
	}
}

func TestRecommendationInfoCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, client, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := RecommendationInfoCommand{
		RecommendationAutocompleteCommand: RecommendationAutocompleteCommand{
			Meta: Meta{
				Ui:          ui,
				flagAddress: url,
			},
		},
	}
	testRecommendationAutocompleteCommand(t, client, srv, &cmd.RecommendationAutocompleteCommand)
}
