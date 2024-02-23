// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestOperatorSchedulerGetConfig_Run(t *testing.T) {
	ci.Parallel(t)

	srv, _, addr := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	c := &OperatorSchedulerGetConfig{Meta: Meta{Ui: ui}}

	// Run the command, so we get the default output and test this.
	must.Zero(t, c.Run([]string{"-address=" + addr}))
	s := ui.OutputWriter.String()
	must.StrContains(t, s, "Scheduler Algorithm           = binpack")
	must.StrContains(t, s, "Preemption SysBatch Scheduler = false")
	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()

	// Request JSON output and test.
	must.Zero(t, c.Run([]string{"-address=" + addr, "-json"}))
	s = ui.OutputWriter.String()
	var js api.SchedulerConfiguration
	must.NoError(t, json.Unmarshal([]byte(s), &js))
	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()

	// Request a template output and test.
	must.Zero(t, c.Run([]string{"-address=" + addr, "-t='{{printf \"%s!!!\" .SchedulerConfig.SchedulerAlgorithm}}'"}))
	must.StrContains(t, ui.OutputWriter.String(), "binpack!!!")
	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()

	// Test an unsupported flag.
	must.One(t, c.Run([]string{"-address=" + addr, "-yaml"}))
	must.StrContains(t, ui.OutputWriter.String(), "Usage: nomad operator scheduler get-config")
}
