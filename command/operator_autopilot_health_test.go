// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestOperator_Autopilot_State_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &OperatorAutopilotHealthCommand{}
}

func TestOperatorAutopilotStateCommand(t *testing.T) {
	ci.Parallel(t)
	s, _, addr := testServer(t, false, nil)
	defer s.Shutdown()

	ui := cli.NewMockUi()
	c := &OperatorAutopilotHealthCommand{Meta: Meta{Ui: ui}}
	args := []string{"-address=" + addr}

	code := c.Run(args)
	must.Eq(t, 0, code, must.Sprintf("got error for exit code: %v", ui.ErrorWriter.String()))

	out := ui.OutputWriter.String()
	must.StrContains(t, out, "Healthy")
}

func TestOperatorAutopilotStateCommand_JSON(t *testing.T) {
	ci.Parallel(t)
	s, _, addr := testServer(t, false, nil)
	defer s.Shutdown()

	ui := cli.NewMockUi()
	c := &OperatorAutopilotHealthCommand{Meta: Meta{Ui: ui}}
	args := []string{"-address=" + addr, "-json"}

	code := c.Run(args)
	must.Eq(t, 0, code, must.Sprintf("got error for exit code: %v", ui.ErrorWriter.String()))

	// Attempt to unmarshal the data which tests that the output is JSON and
	// peak into the data, checking that healthy is an expected and no-default
	// value.
	operatorHealthyReply := api.OperatorHealthReply{}

	must.NoError(t, json.Unmarshal(ui.OutputWriter.Bytes(), &operatorHealthyReply))
	must.True(t, operatorHealthyReply.Healthy)
}
