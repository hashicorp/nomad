// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func TestServiceInfoCommand_Run(t *testing.T) {
	ci.Parallel(t)

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait until our test node is ready.
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
		must.NoError(t, err)
	})

	ui := cli.NewMockUi()
	cmd := &ServiceInfoCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: url,
		},
	}

	// Run the command without any arguments to ensure we are performing this
	// check.
	must.One(t, cmd.Run([]string{"-address=" + url}))
	must.StrContains(t, ui.ErrorWriter.String(),
		"This command takes one argument: <service_name>")
	ui.ErrorWriter.Reset()

	// Create a test job with a Nomad service.
	testJob := testJob("service-discovery-nomad-info")
	testJob.TaskGroups[0].Services = []*api.Service{
		{Name: "service-discovery-nomad-info", Provider: "nomad", PortLabel: "9999", Tags: []string{"foo", "bar"}}}

	// Register that job.
	regResp, _, err := client.Jobs().Register(testJob, nil)
	must.NoError(t, err)
	registerCode := waitForSuccess(ui, client, fullId, t, regResp.EvalID)
	must.Zero(t, registerCode)

	// Reset the output writer, otherwise we will have additional information here.
	ui.OutputWriter.Reset()

	// Job register doesn't assure the service registration has completed. It
	// therefore needs this wrapper to account for eventual service
	// registration. One this has completed, we can perform lookups without
	// similar wraps.
	//
	// TODO(shoenig) clean this up
	require.Eventually(t, func() bool {

		defer ui.OutputWriter.Reset()

		// Perform a standard lookup.
		if code := cmd.Run([]string{"-address=" + url, "service-discovery-nomad-info"}); code != 0 {
			return false
		}

		// Test each header and data entry.
		s := ui.OutputWriter.String()
		if !strings.Contains(s, "Job ID") {
			return false
		}
		if !strings.Contains(s, "Address") {
			return false
		}
		if !strings.Contains(s, "Node ID") {
			return false
		}
		if !strings.Contains(s, "Alloc ID") {
			return false
		}
		if !strings.Contains(s, "service-discovery-nomad-info") {
			return false
		}
		if !strings.Contains(s, ":9999") {
			return false
		}
		if !strings.Contains(s, "[foo,bar]") {
			return false
		}
		return true
	}, 5*time.Second, 100*time.Millisecond)

	// Perform a verbose lookup.
	code := cmd.Run([]string{"-address=" + url, "-verbose", "service-discovery-nomad-info"})
	must.Zero(t, code)

	// Test KV entries.
	s := ui.OutputWriter.String()
	must.StrContains(t, s, "Service Name = service-discovery-nomad-info")
	must.StrContains(t, s, "Namespace    = default")
	must.StrContains(t, s, "Job ID       = service-discovery-nomad-info")
	must.StrContains(t, s, "Datacenter   = dc1")
	must.StrContains(t, s, "Address      = :9999")
	must.StrContains(t, s, "Tags         = [foo,bar]")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}

func Test_argsWithNewPageToken(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		inputOsArgs    []string
		inputNextToken string
		expectedOutput string
		name           string
	}{
		{
			inputOsArgs:    []string{"nomad", "service", "info", "-page-token=abcdef", "example-cache"},
			inputNextToken: "ghijkl",
			expectedOutput: "nomad service info -page-token=ghijkl example-cache",
			name:           "page token with equals sign",
		},
		{
			inputOsArgs:    []string{"nomad", "service", "info", "-page-token", "abcdef", "example-cache"},
			inputNextToken: "ghijkl",
			expectedOutput: "nomad service info -page-token ghijkl example-cache",
			name:           "page token with whitespace gap",
		},
		{
			inputOsArgs:    []string{"nomad", "service", "info", "-per-page", "3", "-page-token", "abcdef", "example-cache"},
			inputNextToken: "ghijkl",
			expectedOutput: "nomad service info -per-page 3 -page-token ghijkl example-cache",
			name:           "per page and page token",
		},
		{
			inputOsArgs:    []string{"nomad", "service", "info", "-page-token", "abcdef", "-per-page", "3", "example-cache"},
			inputNextToken: "ghijkl",
			expectedOutput: "nomad service info -page-token ghijkl -per-page 3 example-cache",
			name:           "page token and per page",
		},
		{
			inputOsArgs:    []string{"nomad", "service", "info", "-page-token", "abcdef", "-per-page=3", "example-cache"},
			inputNextToken: "ghijkl",
			expectedOutput: "nomad service info -page-token ghijkl -per-page=3 example-cache",
			name:           "page token and per page with equal",
		},
		{
			inputOsArgs:    []string{"nomad", "service", "info", "-verbose", "-page-token", "abcdef", "-per-page=3", "example-cache"},
			inputNextToken: "ghijkl",
			expectedOutput: "nomad service info -verbose -page-token ghijkl -per-page=3 example-cache",
			name:           "page token per page with verbose",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := argsWithNewPageToken(tc.inputOsArgs, tc.inputNextToken)
			must.Eq(t, tc.expectedOutput, actualOutput)
		})
	}
}
