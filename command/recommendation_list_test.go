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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecommendationListCommand_Run(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
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
		require.Equal(0, code)
		out := ui.OutputWriter.String()
		require.Contains(out, "No recommendations found")
	} else {
		require.Equal(1, code)
		require.Contains(ui.ErrorWriter.String(), "Nomad Enterprise only endpoint")
	}

	// Register a test job to write a recommendation against.
	testJob := testJob("recommendation_list")
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
	_, _, err = client.Recommendations().Upsert(&rec, nil)
	if srv.Enterprise {
		require.NoError(err)
	} else {
		require.Error(err, "Nomad Enterprise only endpoint")
	}

	// Perform a new list which should yield results.
	code = cmd.Run([]string{"-address=" + url})
	if srv.Enterprise {
		require.Equal(0, code)
		out := ui.OutputWriter.String()
		require.Contains(out, "ID")
		require.Contains(out, "Job")
		require.Contains(out, "Group")
		require.Contains(out, "Task")
		require.Contains(out, "Resource")
		require.Contains(out, "Value")
		require.Contains(out, "CPU")
	} else {
		require.Equal(1, code)
		require.Contains(ui.ErrorWriter.String(), "Nomad Enterprise only endpoint")
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
			assert.Equal(t, tc.expectedOutputList, sortedRecs.r, tc.name)
		})
	}
}
