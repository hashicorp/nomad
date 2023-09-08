// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestNodePoolDeleteCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NodePoolDeleteCommand{}
}

func TestNodePoolDeleteCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Start test server.
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	// Register test node pools.
	dev1 := &api.NodePool{Name: "dev-1"}
	_, err := client.NodePools().Register(dev1, nil)
	must.NoError(t, err)

	// Initialize UI and command.
	ui := cli.NewMockUi()
	cmd := &NodePoolDeleteCommand{Meta: Meta{Ui: ui}}

	// Delete test node pool.
	args := []string{"-address", url, dev1.Name}
	code := cmd.Run(args)
	must.Eq(t, 0, code)
	must.StrContains(t, ui.OutputWriter.String(), "Successfully deleted")

	// Verify node pool was delete.
	got, _, err := client.NodePools().Info(dev1.Name, nil)
	must.ErrorContains(t, err, "404")
	must.Nil(t, got)
}

func TestNodePoolDeleteCommand_Run_fail(t *testing.T) {
	ci.Parallel(t)

	// Start test server.
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	testCases := []struct {
		name         string
		args         []string
		expectedErr  string
		expectedCode int
	}{
		{
			name:         "missing pool",
			args:         []string{"-address", url},
			expectedCode: 1,
			expectedErr:  "This command takes one argument",
		},
		{
			name:         "invalid pool",
			args:         []string{"-address", url, "invalid"},
			expectedCode: 1,
			expectedErr:  "not found",
		},
		{
			name:         "built-in pool",
			args:         []string{"-address", url, "all"},
			expectedCode: 1,
			expectedErr:  "not allowed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Initialize UI and command.
			ui := cli.NewMockUi()
			cmd := &NodePoolDeleteCommand{Meta: Meta{Ui: ui}}

			// Run command.
			code := cmd.Run(tc.args)
			must.Eq(t, tc.expectedCode, code)
			must.StrContains(t, ui.ErrorWriter.String(), tc.expectedErr)
		})
	}
}
