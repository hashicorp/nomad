// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
)

func TestNamespaceApplyCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NamespaceApplyCommand{}
}

func TestNamespaceApplyCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &NamespaceApplyCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	if code := cmd.Run([]string{"-address=nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("name required error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestNamespaceApplyCommand_Good(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &NamespaceApplyCommand{Meta: Meta{Ui: ui}}

	// Create a namespace
	name, desc := "foo", "bar"
	if code := cmd.Run([]string{"-address=" + url, "-description=" + desc, name}); code != 0 {
		t.Fatalf("expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	}

	namespaces, _, err := client.Namespaces().List(nil)
	assert.Nil(t, err)
	assert.Len(t, namespaces, 2)
}

func TestNamespaceApplyCommand_parseNamesapceSpec(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name     string
		input    string
		expected *api.Namespace
	}{
		{
			name: "valid namespace",
			input: `
name        = "test-namespace"
description = "Test namespace"
quota       = "test"

capabilities {
  enabled_task_drivers  = ["exec", "docker"]
  disabled_task_drivers = ["raw_exec"]
}

node_pool_config {
  default = "dev"
  allowed = ["prod*"]
}

meta {
  dept = "eng"
}`,
			expected: &api.Namespace{
				Name:        "test-namespace",
				Description: "Test namespace",
				Quota:       "test",
				Capabilities: &api.NamespaceCapabilities{
					EnabledTaskDrivers:  []string{"exec", "docker"},
					DisabledTaskDrivers: []string{"raw_exec"},
				},
				NodePoolConfiguration: &api.NamespaceNodePoolConfiguration{
					Default: "dev",
					Allowed: []string{"prod*"},
				},
				Meta: map[string]string{
					"dept": "eng",
				},
			},
		},
		{
			name:  "minimal",
			input: `name = "test-small"`,
			expected: &api.Namespace{
				Name: "test-small",
			},
		},
		{
			name:     "empty",
			input:    "",
			expected: &api.Namespace{},
		},
		{
			name: "lists in node pool config are nil if not provided",
			input: `
name = "nil-lists"

node_pool_config {
  default = "default"
}
`,
			expected: &api.Namespace{
				Name: "nil-lists",
				NodePoolConfiguration: &api.NamespaceNodePoolConfiguration{
					Default: "default",
					Allowed: nil,
					Denied:  nil,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseNamespaceSpec([]byte(tc.input))
			must.NoError(t, err)
			must.Eq(t, tc.expected, got)
		})
	}
}
