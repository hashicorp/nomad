// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pointer"
)

func TestNodePoolJobsListCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Start test server.
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	// Register test node pool
	dev1 := &api.NodePool{Name: "dev-1", Description: "Pool dev-1"}
	_, err := client.NodePools().Register(dev1, nil)
	must.NoError(t, err)

	// Register a non-default namespace
	ns := &api.Namespace{Name: "system"}
	_, err = client.Namespaces().Register(ns, nil)
	must.NoError(t, err)

	// Register some jobs
	registerJob := func(np, ns, id string) {
		job := testJob(id)
		job.Namespace = pointer.Of(ns)
		job.NodePool = pointer.Of(np)
		_, _, err := client.Jobs().Register(job, nil)
		must.NoError(t, err)
	}

	registerJob("dev-1", "default", "job0")
	registerJob("dev-1", "default", "job1")
	registerJob("dev-1", "system", "job2")
	registerJob("default", "default", "job3")
	registerJob("default", "default", "job4")
	registerJob("default", "system", "job5")
	registerJob("all", "system", "job6")
	registerJob("all", "default", "job7")

	testCases := []struct {
		name         string
		args         []string
		expectedJobs []string
		expectedErr  string
		expectedCode int
	}{
		{
			name:         "missing arg",
			args:         []string{},
			expectedErr:  "This command takes one argument: <node-pool>",
			expectedCode: 1,
		},
		{
			name:         "list with wildcard namespaces",
			args:         []string{"-namespace", "*", "dev-1"},
			expectedJobs: []string{"job0", "job1", "job2"},
			expectedCode: 0,
		},
		{
			name:         "list with specific namespaces",
			args:         []string{"-namespace", "system", "dev-1"},
			expectedJobs: []string{"job2"},
			expectedCode: 0,
		},
		{
			name:         "list with specific namespace in the all pool",
			args:         []string{"-namespace", "system", "all"},
			expectedJobs: []string{"job6"},
			expectedCode: 0,
		},
		{
			name:         "list with filter",
			args:         []string{"-filter", "ID == \"job1\"", "dev-1"},
			expectedJobs: []string{"job1"},
			expectedCode: 0,
		},
		{
			name:         "paginate",
			args:         []string{"-per-page", "2", "-namespace", "*", "dev-1"},
			expectedJobs: []string{"job0", "job1"},
			expectedCode: 0,
		},
		{
			name: "paginate page 2",
			args: []string{
				"-per-page", "2", "-page-token", "job2",
				"-namespace", "*", "dev-1"},
			expectedJobs: []string{"job2"},
			expectedCode: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Initialize UI and command.
			ui := cli.NewMockUi()
			cmd := &NodePoolJobsCommand{Meta: Meta{Ui: ui}}

			// Run command, always in JSON so we can easily parse results
			args := []string{"-address", url, "-json"}
			args = append(args, tc.args...)
			code := cmd.Run(args)

			gotStdout := ui.OutputWriter.String()
			gotStdout = jsonOutputRaftIndexes.ReplaceAllString(gotStdout, "")

			must.Eq(t, tc.expectedCode, code,
				must.Sprintf("got unexpected code with stdout:\n%s\nstderr:\n%s",
					gotStdout, ui.ErrorWriter.String(),
				))

			if tc.expectedCode == 0 {
				var jobs []*api.JobListStub
				err := json.Unmarshal([]byte(gotStdout), &jobs)
				must.NoError(t, err)

				gotJobs := helper.ConvertSlice(jobs,
					func(j *api.JobListStub) string { return j.ID })
				must.Eq(t, tc.expectedJobs, gotJobs,
					must.Sprintf("got unexpected list of jobs:\n%s",
						gotStdout))
			} else {
				test.StrContains(t, ui.ErrorWriter.String(), strings.TrimSpace(tc.expectedErr))
			}
		})
	}
}
