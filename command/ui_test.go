// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestCommand_Ui(t *testing.T) {

	type testCaseSetupFn func(*testing.T)

	cases := []struct {
		Name        string
		SetupFn     testCaseSetupFn
		Args        []string
		ExpectedURL string
	}{
		{
			Name:        "default values",
			ExpectedURL: "http://127.0.0.1:4646",
		},
		{
			Name:        "set namespace via flag",
			Args:        []string{"-namespace=dev"},
			ExpectedURL: "http://127.0.0.1:4646?namespace=dev",
		},
		{
			Name:        "set region via flag",
			Args:        []string{"-region=earth"},
			ExpectedURL: "http://127.0.0.1:4646?region=earth",
		},
		{
			Name:        "set region and namespace via flag",
			Args:        []string{"-region=earth", "-namespace=dev"},
			ExpectedURL: "http://127.0.0.1:4646?namespace=dev&region=earth",
		},
		{
			Name: "set namespace via env var",
			SetupFn: func(t *testing.T) {
				t.Setenv("NOMAD_NAMESPACE", "dev")
			},
			ExpectedURL: "http://127.0.0.1:4646?namespace=dev",
		},
		{
			Name: "set region via env var",
			SetupFn: func(t *testing.T) {
				t.Setenv("NOMAD_REGION", "earth")
			},
			ExpectedURL: "http://127.0.0.1:4646?region=earth",
		},
		{
			Name: "set region and namespace via env var",
			SetupFn: func(t *testing.T) {
				t.Setenv("NOMAD_REGION", "earth")
				t.Setenv("NOMAD_NAMESPACE", "dev")
			},
			ExpectedURL: "http://127.0.0.1:4646?namespace=dev&region=earth",
		},
		{
			Name: "set region and namespace via env var",
			SetupFn: func(t *testing.T) {
				t.Setenv("NOMAD_REGION", "earth")
				t.Setenv("NOMAD_NAMESPACE", "dev")
			},
			ExpectedURL: "http://127.0.0.1:4646?namespace=dev&region=earth",
		},
		{
			Name: "flags have higher precedence",
			SetupFn: func(t *testing.T) {
				t.Setenv("NOMAD_REGION", "earth")
				t.Setenv("NOMAD_NAMESPACE", "dev")
			},
			Args: []string{
				"-region=mars",
				"-namespace=prod",
			},
			ExpectedURL: "http://127.0.0.1:4646?namespace=prod&region=mars",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			// Make sure environment variables are clean.
			t.Setenv("NOMAD_NAMESPACE", "")
			t.Setenv("NOMAD_REGION", "")

			// Setup fake CLI UI and test case
			ui := cli.NewMockUi()
			cmd := &UiCommand{Meta: Meta{Ui: ui}}

			if tc.SetupFn != nil {
				tc.SetupFn(t)
			}

			// Don't try to open a browser.
			args := append(tc.Args, "-show-url")

			if code := cmd.Run(args); code != 0 {
				require.Equal(t, 0, code, "expected exit code 0, got %d", code)
			}

			got := ui.OutputWriter.String()
			expected := fmt.Sprintf("URL for web UI: %s", tc.ExpectedURL)
			require.Equal(t, expected, strings.TrimSpace(got))
		})
	}
}
