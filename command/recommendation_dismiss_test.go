// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

func TestRecommendationDismissCommand_Run(t *testing.T) {
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
		t.Fatalf("err: %s", err)
	})

	ui := cli.NewMockUi()
	cmd := &RecommendationDismissCommand{
		RecommendationAutocompleteCommand: RecommendationAutocompleteCommand{
			Meta: Meta{
				Ui:          ui,
				flagAddress: url,
			},
		},
	}

	// Register a test job to write a recommendation against.
	testJob := testJob("recommendation_dismiss")
	regResp, _, err := client.Jobs().Register(testJob, nil)
	must.NoError(t, err)
	registerCode := waitForSuccess(ui, client, fullId, t, regResp.EvalID)
	must.Zero(t, registerCode)

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
		must.NoError(t, err)

		// Read the recommendation out to ensure it is there as a control on
		// later tests.
		recInfo, _, err := client.Recommendations().Info(recResp.ID, nil)
		must.NoError(t, err)
		must.NotNil(t, recInfo)
	} else {
		must.ErrorContains(t, err, "Nomad Enterprise only endpoint")
	}

	// Only perform the call if we are running enterprise tests. Otherwise the
	// recResp object will be nil.
	if !srv.Enterprise {
		return
	}
	code := cmd.Run([]string{"-address=" + url, recResp.ID})
	must.Zero(t, code)
	out := ui.OutputWriter.String()
	must.StrContains(t, out, "Successfully dismissed recommendation")

	// Perform an info call on the recommendation which should return not
	// found.
	recInfo, _, err := client.Recommendations().Info(recResp.ID, nil)
	must.ErrorContains(t, err, "not found")
	must.Nil(recInfo)
}

func TestRecommendationDismissCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &RecommendationDismissCommand{
		RecommendationAutocompleteCommand: RecommendationAutocompleteCommand{
			Meta: Meta{
				Ui:          ui,
				flagAddress: url,
			},
		},
	}

	testRecommendationAutocompleteCommand(t, client, srv, &cmd.RecommendationAutocompleteCommand)
}

func testRecommendationAutocompleteCommand(t *testing.T, client *api.Client, srv *agent.TestAgent, cmd *RecommendationAutocompleteCommand) {
	// Register a test job to write a recommendation against.
	testJob := testJob("recommendation_autocomplete")
	_, _, err := client.Jobs().Register(testJob, nil)
	must.NoError(t, err)

	// Write a recommendation.
	rec := &api.Recommendation{
		JobID:    *testJob.ID,
		Group:    *testJob.TaskGroups[0].Name,
		Task:     testJob.TaskGroups[0].Tasks[0].Name,
		Resource: "CPU",
		Value:    1050,
		Meta:     map[string]interface{}{"test-meta-entry": "test-meta-value"},
		Stats:    map[string]float64{"p13": 1.13},
	}
	rec, _, err = client.Recommendations().Upsert(rec, nil)
	if srv.Enterprise {
		must.NoError(t, err)
	} else {
		must.ErrorContains(t, err, "Nomad Enterprise only endpoint")
		return
	}

	prefix := rec.ID[:5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	must.Len(t, 1, res)
	must.Eq(t, rec.ID, res[0])
}
