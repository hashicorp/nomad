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

func TestNodePoolInfoCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NodePoolInfoCommand{}
}

func TestNodePoolInfoCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Start test server.
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	// Register test node pools.
	dev1 := &api.NodePool{
		Name:        "dev-1",
		Description: "Test pool",
		Meta: map[string]string{
			"env": "test",
		},
	}
	_, err := client.NodePools().Register(dev1, nil)
	must.NoError(t, err)

	dev1Output := `
Name        = dev-1
Description = Test pool

Metadata
env = test

Scheduler Configuration
No scheduler configuration`

	dev1JsonOutput := `
{
    "Description": "Test pool",
    "Meta": {
        "env": "test"
    },
    "Name": "dev-1",
    "SchedulerConfiguration": null
}`

	// These two node pools are used to test exact prefix match.
	prod1 := &api.NodePool{Name: "prod-1"}
	_, err = client.NodePools().Register(prod1, nil)
	must.NoError(t, err)

	prod12 := &api.NodePool{Name: "prod-12"}
	_, err = client.NodePools().Register(prod12, nil)
	must.NoError(t, err)

	testCases := []struct {
		name         string
		args         []string
		expectedOut  string
		expectedErr  string
		expectedCode int
	}{
		{
			name:         "basic info",
			args:         []string{"dev-1"},
			expectedOut:  dev1Output,
			expectedCode: 0,
		},
		{
			name:         "basic info by prefix",
			args:         []string{"dev"},
			expectedOut:  dev1Output,
			expectedCode: 0,
		},
		{
			name: "exact prefix match",
			args: []string{"prod-1"},
			expectedOut: `
Name        = prod-1
Description = <none>

Metadata
No metadata

Scheduler Configuration
No scheduler configuration`,
			expectedCode: 0,
		},
		{
			name:         "json",
			args:         []string{"-json", "dev"},
			expectedOut:  dev1JsonOutput,
			expectedCode: 0,
		},
		{
			name: "template",
			args: []string{
				"-t", "{{.Name}} -> {{.Meta.env}}",
				"dev-1",
			},
			expectedOut:  "dev-1 -> test",
			expectedCode: 0,
		},
		{
			name:         "fail because of missing node pool arg",
			args:         []string{},
			expectedErr:  "This command takes one argument",
			expectedCode: 1,
		},
		{
			name:         "fail because no match",
			args:         []string{"invalid"},
			expectedErr:  `No node pool with prefix "invalid" found`,
			expectedCode: 1,
		},
		{
			name:         "fail because of multiple matches",
			args:         []string{"de"}, // Matches default and dev-1.
			expectedErr:  "Prefix matched multiple node pools",
			expectedCode: 1,
		},
		{
			name: "fail because of invalid template",
			args: []string{
				"-t", "{{.NotValid}}",
				"dev-1",
			},
			expectedErr:  "Error formatting the data",
			expectedCode: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Initialize UI and command.
			ui := cli.NewMockUi()
			cmd := &NodePoolInfoCommand{Meta: Meta{Ui: ui}}

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
