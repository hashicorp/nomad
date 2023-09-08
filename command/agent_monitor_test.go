// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestMonitorCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &MonitorCommand{}
}

func TestMonitorCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &MonitorCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	must.One(t, code)

	out := ui.ErrorWriter.String()
	must.StrContains(t, out, commandErrorText(cmd))

	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{"-address=nope"})
	must.One(t, code)

	// Fails on nonexistent node
	code = cmd.Run([]string{"-address=" + url, "-node-id=12345678-abcd-efab-cdef-123456789abc"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, "No node(s) with prefix")
}
