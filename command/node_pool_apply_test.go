// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func TestNodePoolApplyCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NodePoolApplyCommand{}
}

func TestNodePoolApplyCommand_Run(t *testing.T) {
	ci.Parallel(t)

	// Start test server.
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	// Initialize UI and command.
	ui := cli.NewMockUi()
	cmd := &NodePoolApplyCommand{Meta: Meta{Ui: ui}}

	// Create node pool with HCL file.
	hclTestFile := `
node_pool "dev" {
  description = "dev node pool"
}`
	file, err := os.CreateTemp(t.TempDir(), "node-pool-test-*.hcl")
	must.NoError(t, err)
	_, err = file.WriteString(hclTestFile)
	must.NoError(t, err)

	// Run command.
	args := []string{"-address", url, file.Name()}
	code := cmd.Run(args)
	must.Eq(t, 0, code)

	// Verify node pool was created.
	got, err := srv.Agent.Server().State().NodePoolByName(nil, "dev")
	must.NoError(t, err)
	must.NotNil(t, got)

	// Update node pool.
	file.Truncate(0)
	file.Seek(0, 0)
	hclTestFile = `
node_pool "dev" {
  description = "dev node pool"

  meta {
    test = "true"
  }
}`
	_, err = file.WriteString(hclTestFile)
	must.NoError(t, err)

	// Run command.
	code = cmd.Run(args)
	must.Eq(t, 0, code)

	// Verify node pool was updated.
	got, err = srv.Agent.Server().State().NodePoolByName(nil, "dev")
	must.NoError(t, err)
	must.NotNil(t, got)
	must.NotNil(t, got.Meta)
	must.Eq(t, "true", got.Meta["test"])

	// Create node pool with JSON file.
	jsonTestFile := `
{
  "Name": "prod",
  "Description": "prod node pool"
}`

	file, err = os.CreateTemp(t.TempDir(), "node-pool-test-*.json")
	must.NoError(t, err)
	_, err = file.WriteString(jsonTestFile)
	must.NoError(t, err)

	// Run command.
	args = []string{"-address", url, "-json", file.Name()}
	code = cmd.Run(args)
	must.Eq(t, 0, code)

	// Verify node pool was created.
	got, err = srv.Agent.Server().State().NodePoolByName(nil, "prod")
	must.NoError(t, err)
	must.NotNil(t, got)
}

func TestNodePoolApplyCommand_Run_fail(t *testing.T) {
	ci.Parallel(t)

	// Start test server.
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	waitForNodes(t, client)

	testCases := []struct {
		name           string
		args           []string
		input          string
		expectedOutput string
		expectedCode   int
	}{
		{
			name:           "missing file",
			args:           []string{},
			expectedOutput: "This command takes one argument",
			expectedCode:   1,
		},
		{
			name:           "file doesn't exist",
			args:           []string{"doesn-exist.hcl"},
			expectedOutput: "no such file",
			expectedCode:   1,
		},
		{
			name:           "invalid json",
			args:           []string{"-json", "invalid.json"},
			input:          "not json",
			expectedOutput: "Failed to parse input",
			expectedCode:   1,
		},
		{
			name:           "invalid hcl",
			args:           []string{"invalid.hcl"},
			input:          "not HCL",
			expectedOutput: "Failed to parse input",
			expectedCode:   1,
		},
		{
			name:           "valid json without json flag",
			args:           []string{"valid.json"},
			input:          `{"Name": "dev"}`,
			expectedOutput: "Failed to parse input",
			expectedCode:   1,
		},
		{
			name:           "valid hcl with json flag",
			args:           []string{"-json", "valid.hcl"},
			input:          `node_pool "dev" {}`,
			expectedOutput: "Failed to parse input",
			expectedCode:   1,
		},
		{
			name:           "invalid node pool hcl",
			args:           []string{"invalid.hcl"},
			input:          `not_a_node_pool "dev" {}`,
			expectedOutput: "Failed to parse input",
			expectedCode:   1,
		},
		{
			name:           "invalid node pool",
			args:           []string{"-address", url, "invalid_node_pool.hcl"},
			input:          `node_pool "invalid name" {}`,
			expectedOutput: "Error applying node pool",
			expectedCode:   1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Initialize UI and command.
			ui := cli.NewMockUi()
			cmd := &NodePoolApplyCommand{Meta: Meta{Ui: ui}}

			// Write input to file.
			if tc.input != "" {
				// Split the filename and extension from the last argument to
				// add a "*" between them so os.CreateTemp retains the file
				// extension.
				filename := tc.args[len(tc.args)-1]
				ext := filepath.Ext(filename)
				name, _ := strings.CutSuffix(filename, ext)

				file, err := os.CreateTemp(t.TempDir(), fmt.Sprintf("%s-*%s", name, ext))
				must.NoError(t, err)
				_, err = file.WriteString(tc.input)
				must.NoError(t, err)

				// Update last arg with full test file path.
				tc.args[len(tc.args)-1] = file.Name()
			}

			got := cmd.Run(tc.args)
			test.Eq(t, tc.expectedCode, got)
			test.StrContains(t, ui.ErrorWriter.String(), tc.expectedOutput)
		})
	}
}
