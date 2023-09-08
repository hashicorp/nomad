// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestOperatorSchedulerGetConfig_Run(t *testing.T) {
	ci.Parallel(t)

	srv, _, addr := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	c := &OperatorSchedulerGetConfig{Meta: Meta{Ui: ui}}

	// Run the command, so we get the default output and test this.
	require.EqualValues(t, 0, c.Run([]string{"-address=" + addr}))
	s := ui.OutputWriter.String()
	require.Contains(t, s, "Scheduler Algorithm           = binpack")
	require.Contains(t, s, "Preemption SysBatch Scheduler = false")
	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()

	// Request JSON output and test.
	require.EqualValues(t, 0, c.Run([]string{"-address=" + addr, "-json"}))
	s = ui.OutputWriter.String()
	var js api.SchedulerConfiguration
	require.NoError(t, json.Unmarshal([]byte(s), &js))
	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()

	// Request a template output and test.
	require.EqualValues(t, 0, c.Run([]string{"-address=" + addr, "-t='{{printf \"%s!!!\" .SchedulerConfig.SchedulerAlgorithm}}'"}))
	require.Contains(t, ui.OutputWriter.String(), "binpack!!!")
	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()

	// Test an unsupported flag.
	require.EqualValues(t, 1, c.Run([]string{"-address=" + addr, "-yaml"}))
	require.Contains(t, ui.OutputWriter.String(), "Usage: nomad operator scheduler get-config")
}
