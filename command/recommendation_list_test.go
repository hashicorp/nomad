// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"sort"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestRecommendationListCommand_Run(t *testing.T) {
	ci.Parallel(t)

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	testutil.WaitForLeader(t, srv.Agent.RPC)
	clientID := srv.Agent.Client().NodeID()
	testutil.WaitForClient(t, srv.Agent.Client().RPC, clientID, srv.Agent.Client().Region())

	ui := cli.NewMockUi()
	cmd := &RecommendationListCommand{Meta: Meta{Ui: ui}}

	// Perform an initial list, which should return zero results.
	code := cmd.Run([]string{"-address=" + url})
	if srv.Enterprise {
		must.Zero(t, code)
		out := ui.OutputWriter.String()
		must.StrContains(out, "No recommendations found")
	} else {
		must.One(t, code)
		must.StrContains(t, ui.ErrorWriter.String(), "Nomad Enterprise only endpoint")
	}

	// Register a test job to write a recommendation against.
	testJob := testJob("recommendation_list")
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
	_, _, err = client.Recommendations().Upsert(&rec, nil)
	if srv.Enterprise {
		must.NoError(t, err)
	} else {
		must.ErrorContains(t, err, "Nomad Enterprise only endpoint")
	}

	// Perform a new list which should yield results.
	code = cmd.Run([]string{"-address=" + url})
	if srv.Enterprise {
		must.Zero(t, code)
		out := ui.OutputWriter.String()
		must.StrContains(t, out, "ID")
		must.StrContains(t, out, "Job")
		must.StrContains(t, out, "Group")
		must.StrContains(t, out, "Task")
		must.StrContains(t, out, "Resource")
		must.StrContains(t, out, "Value")
		must.StrContains(t, out, "CPU")
	} else {
		must.One(t, code)
		must.StrContains(t, ui.ErrorWriter.String(), "Nomad Enterprise only endpoint")
	}
}

func TestRecommendationListCommand_Sort(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		inputRecommendationList []*api.Recommendation
		expectedOutputList      []*api.Recommendation
		name                    string
	}{
		{
			inputRecommendationList: []*api.Recommendation{
				{Namespace: "default", JobID: "example", Group: "cache", Task: "redis", Resource: "MemoryMB"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "redis", Resource: "CPU"},
			},
			expectedOutputList: []*api.Recommendation{
				{Namespace: "default", JobID: "example", Group: "cache", Task: "redis", Resource: "CPU"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "redis", Resource: "MemoryMB"},
			},
			name: "single job with both resources",
		},
		{
			inputRecommendationList: []*api.Recommendation{
				{Namespace: "default", JobID: "example", Group: "cache", Task: "redis", Resource: "MemoryMB"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "redis", Resource: "CPU"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "mongodb", Resource: "MemoryMB"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "mongodb", Resource: "CPU"},
			},
			expectedOutputList: []*api.Recommendation{
				{Namespace: "default", JobID: "example", Group: "cache", Task: "mongodb", Resource: "CPU"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "mongodb", Resource: "MemoryMB"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "redis", Resource: "CPU"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "redis", Resource: "MemoryMB"},
			},
			name: "single job with multiple groups",
		},
		{
			inputRecommendationList: []*api.Recommendation{
				{Namespace: "default", JobID: "example", Group: "cache", Task: "redis", Resource: "MemoryMB"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "redis", Resource: "CPU"},
				{Namespace: "default", JobID: "distro", Group: "cache", Task: "redis", Resource: "MemoryMB"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "mongodb", Resource: "MemoryMB"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "mongodb", Resource: "CPU"},
				{Namespace: "default", JobID: "distro", Group: "cache", Task: "redis", Resource: "CPU"},
			},
			expectedOutputList: []*api.Recommendation{
				{Namespace: "default", JobID: "distro", Group: "cache", Task: "redis", Resource: "CPU"},
				{Namespace: "default", JobID: "distro", Group: "cache", Task: "redis", Resource: "MemoryMB"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "mongodb", Resource: "CPU"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "mongodb", Resource: "MemoryMB"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "redis", Resource: "CPU"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "redis", Resource: "MemoryMB"},
			},
			name: "multiple jobs",
		},
		{
			inputRecommendationList: []*api.Recommendation{
				{Namespace: "default", JobID: "example", Group: "cache", Task: "redis", Resource: "MemoryMB"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "redis", Resource: "CPU"},
				{Namespace: "cefault", JobID: "distro", Group: "cache", Task: "redis", Resource: "MemoryMB"},
				{Namespace: "default", JobID: "distro", Group: "cache", Task: "redis", Resource: "MemoryMB"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "mongodb", Resource: "MemoryMB"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "mongodb", Resource: "CPU"},
				{Namespace: "default", JobID: "distro", Group: "cache", Task: "redis", Resource: "CPU"},
				{Namespace: "cefault", JobID: "distro", Group: "cache", Task: "redis", Resource: "CPU"},
			},
			expectedOutputList: []*api.Recommendation{
				{Namespace: "cefault", JobID: "distro", Group: "cache", Task: "redis", Resource: "CPU"},
				{Namespace: "cefault", JobID: "distro", Group: "cache", Task: "redis", Resource: "MemoryMB"},
				{Namespace: "default", JobID: "distro", Group: "cache", Task: "redis", Resource: "CPU"},
				{Namespace: "default", JobID: "distro", Group: "cache", Task: "redis", Resource: "MemoryMB"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "mongodb", Resource: "CPU"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "mongodb", Resource: "MemoryMB"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "redis", Resource: "CPU"},
				{Namespace: "default", JobID: "example", Group: "cache", Task: "redis", Resource: "MemoryMB"},
			},
			name: "multiple namespaces",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sortedRecs := recommendationList{r: tc.inputRecommendationList}
			sort.Sort(sortedRecs)
			must.Eq(t, tc.expectedOutputList, sortedRecs.r)
		})
	}
}
