// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"os"
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/shoenig/test/must"
)

func TestMonitorExportCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &MonitorExportCommand{}
}

func TestMonitorExportCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	const expectedText = "log log log log log"

	testFile, err := os.CreateTemp("", "nomadtests")
	must.NoError(t, err)

	_, err = testFile.Write([]byte(expectedText))
	must.NoError(t, err)
	inlineFilePath := testFile.Name()
	config := func(c *agent.Config) {
		c.LogFile = inlineFilePath
	}
	srv, _, url := testServer(t, false, config)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &MonitorExportCommand{Meta: Meta{Ui: ui}}

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

	// Fails on passing a non boolean value to -follow.
	code = cmd.Run([]string{"-address=" + url, "-follow=maybe"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, `invalid boolean value "maybe" for -follow`)

	ui.ErrorWriter.Reset()

	// Fails on passing non boolean value to -on-disk.
	code = cmd.Run([]string{"-address=" + url, "-on-disk=maybe"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, `invalid boolean value "maybe" for -on-disk`)

	ui.ErrorWriter.Reset()

	// Fails on passing both on-disk and service-name
	code = cmd.Run([]string{"-address=" + url, "-on-disk=true", "-service-name=nomad"})
	must.One(t, code)

	out = ui.ErrorWriter.String()
	must.StrContains(t, out, `journalctl and nomad log file simultaneously`)
}
