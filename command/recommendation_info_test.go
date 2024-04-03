// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestRecommendationInfoCommand_Run(t *testing.T) {
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
	cmd := &RecommendationInfoCommand{
		RecommendationAutocompleteCommand: RecommendationAutocompleteCommand{
			Meta: Meta{Ui: ui},
		},
	}

	// Perform an initial call, which should return a not found error.
	code := cmd.Run([]string{"-address=" + url, "2c13f001-f5b6-ce36-03a5-e37afe160df5"})
	if srv.Enterprise {
		must.One(t, code)
		out := ui.ErrorWriter.String()
		must.StrContains(t, out, "Recommendation not found")
	} else {
		must.One(t, code)
		must.StrContains(t, ui.ErrorWriter.String(), "Nomad Enterprise only endpoint")
	}

	// Register a test job to write a recommendation against.
	testJob := testJob("recommendation_info")
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
	} else {
		must.ErrorContains(t, err, "Nomad Enterprise only endpoint")
	}

	// Only perform the call if we are running enterprise tests. Otherwise the
	// recResp object will be nil.
	if srv.Enterprise {
		code = cmd.Run([]string{"-address=" + url, recResp.ID})
		must.Zero(t, code)
		out := ui.OutputWriter.String()
		must.StrContains(t, out, "test-meta-entry")
		must.StrContains(t, out, "p13")
		must.StrContains(t, out, "1.13")
		must.StrContains(t, out, recResp.ID)
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
