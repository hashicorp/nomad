// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func TestNodePoolListCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NodePoolListCommand{}
}

func TestNodePoolListCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Start test server.
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	// Register test node pools.
	dev1 := &api.NodePool{Name: "dev-1", Description: "Pool dev-1"}
	_, err := client.NodePools().Register(dev1, nil)
	must.NoError(t, err)

	prod1 := &api.NodePool{Name: "prod-1"}
	_, err = client.NodePools().Register(prod1, nil)
	must.NoError(t, err)

	prod2 := &api.NodePool{Name: "prod-2", Description: "Pool prod-2"}
	_, err = client.NodePools().Register(prod2, nil)
	must.NoError(t, err)

	testCases := []struct {
		name         string
		args         []string
		expectedOut  string
		expectedErr  string
		expectedCode int
	}{
		{
			name: "list all",
			args: []string{},
			expectedOut: `
Name     Description
all      Node pool with all nodes in the cluster.
default  Default node pool.
dev-1    Pool dev-1
prod-1   <none>
prod-2   Pool prod-2`,
			expectedCode: 0,
		},
		{
			name: "filter",
			args: []string{
				"-filter", `Name contains "prod"`,
			},
			expectedOut: `
Name    Description
prod-1  <none>
prod-2  Pool prod-2`,
			expectedCode: 0,
		},
		{
			name: "paginate",
			args: []string{
				"-per-page", "2",
			},
			expectedOut: `
Name     Description
all      Node pool with all nodes in the cluster.
default  Default node pool.`,
			expectedCode: 0,
		},
		{
			name: "paginate page 2",
			args: []string{
				"-per-page", "2",
				"-page-token", "dev-1",
			},
			expectedOut: `
Name    Description
dev-1   Pool dev-1
prod-1  <none>`,
			expectedCode: 0,
		},
		{
			name: "json",
			args: []string{
				"-json",
				"-filter", `Name == "prod-1"`,
			},
			expectedOut: `
[
    {
        "Description": "",
        "Meta": null,
        "Name": "prod-1",
        "SchedulerConfiguration": null
    }
]`,
			expectedCode: 0,
		},
		{
			name: "template",
			args: []string{
				"-t", "{{range .}}{{.Name}} {{end}}",
			},
			expectedOut:  "all default dev-1 prod-1 prod-2",
			expectedCode: 0,
		},
		{
			name:         "fail because of arg",
			args:         []string{"invalid"},
			expectedErr:  "This command takes no arguments",
			expectedCode: 1,
		},
		{
			name: "fail because of invalid template",
			args: []string{
				"-t", "{{.NotValid}}",
			},
			expectedErr:  "Error formatting the data",
			expectedCode: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Initialize UI and command.
			ui := cli.NewMockUi()
			cmd := &NodePoolListCommand{Meta: Meta{Ui: ui}}

			// Run command.
			args := []string{"-address", url}
			args = append(args, tc.args...)
			code := cmd.Run(args)

			gotStdout := ui.OutputWriter.String()
			gotStdout = jsonOutputRaftIndexes.ReplaceAllString(gotStdout, "")

			test.Eq(t, tc.expectedCode, code)
			test.StrContains(t, gotStdout, strings.TrimSpace(tc.expectedOut))
			test.StrContains(t, ui.ErrorWriter.String(), strings.TrimSpace(tc.expectedErr))
		})
	}
}
