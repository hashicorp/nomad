// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/v2/ci"
	"github.com/shoenig/test/must"
)

func TestOperator_Autopilot_GetConfig_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &OperatorRaftListCommand{}
}

func TestOperatorAutopilotGetConfigCommand(t *testing.T) {
	ci.Parallel(t)
	s, _, addr := testServer(t, false, nil)
	defer s.Shutdown()

	ui := cli.NewMockUi()
	c := &OperatorAutopilotGetCommand{Meta: Meta{Ui: ui}}
	args := []string{"-address=" + addr}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	output := strings.TrimSpace(ui.OutputWriter.String())
	if !strings.Contains(output, "CleanupDeadServers = true") {
		t.Fatalf("bad: %s", output)
	}
}

func TestOperatorAutopilotGetConfigCommand_JSON(t *testing.T) {
	ci.Parallel(t)
	s, _, addr := testServer(t, false, nil)
	defer s.Shutdown()

	ui := cli.NewMockUi()
	c := &OperatorAutopilotGetCommand{Meta: Meta{Ui: ui}}
	args := []string{"-address=" + addr, "-json"}

	code := c.Run(args)
	must.Eq(t, 0, code)

	output := ui.OutputWriter.String()
	var config api.AutopilotConfiguration
	must.NoError(t, json.Unmarshal([]byte(output), &config))
	must.True(t, config.CleanupDeadServers)
}

func TestOperatorAutopilotGetConfigCommand_Template(t *testing.T) {
	ci.Parallel(t)
	s, _, addr := testServer(t, false, nil)
	defer s.Shutdown()

	ui := cli.NewMockUi()
	c := &OperatorAutopilotGetCommand{Meta: Meta{Ui: ui}}
	args := []string{"-address=" + addr, "-t", "{{.CleanupDeadServers}}"}

	code := c.Run(args)
	must.Eq(t, 0, code)

	output := strings.TrimSpace(ui.OutputWriter.String())
	must.Eq(t, "true", output)
}
