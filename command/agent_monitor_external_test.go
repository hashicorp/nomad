// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestMonitorExternalCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &MonitorExternalCommand{}
}

func TestMonitorExternalCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &MonitorExternalCommand{Meta: Meta{Ui: ui}}

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

	ui.ErrorWriter.Reset()

	// Fails on passing a log-include-location flag which cannot be parsed.
	code = cmd.Run([]string{"-address=" + url, "-follow=maybe"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, `invalid boolean value "maybe" for -follow`)

	// Fails on passing a log-include-location flag which cannot be parsed.
	code = cmd.Run([]string{"-address=" + url, "-on-disk=maybe"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, `invalid boolean value "maybe" for -on-disk`)
}
