// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestAgentInfoCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &AgentInfoCommand{}
}

func TestAgentInfoCommand_Run(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AgentInfoCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{"-address=" + url})
	must.Zero(t, code)
}

func TestAgentInfoCommand_Run_JSON(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AgentInfoCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{"-address=" + url, "-json"})
	must.Zero(t, code)

	out := ui.OutputWriter.String()
	must.StrContains(t, out, `"config"`)
}

func TestAgentInfoCommand_Run_Gotemplate(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AgentInfoCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{"-address=" + url, "-t", "{{.Stats.raft}}"})
	must.Zero(t, code)

	out := ui.OutputWriter.String()
	must.StrContains(t, out, "last_log_index")
}

func TestAgentInfoCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &AgentInfoCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	must.One(t, code)

	out := ui.ErrorWriter.String()
	must.StrContains(t, out, commandErrorText(cmd))

	ui.ErrorWriter.Reset()

	// Fails on connection failure
	code = cmd.Run([]string{"-address=nope"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "Error querying agent info")
}
