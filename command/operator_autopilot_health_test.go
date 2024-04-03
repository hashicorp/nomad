// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
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
