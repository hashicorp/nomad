// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/helper/snapshot"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestOperatorSnapshotSave_Works(t *testing.T) {
	ci.Parallel(t)

	tmpDir := t.TempDir()

	srv, _, url := testServer(t, false, func(c *agent.Config) {
		c.DevMode = false
		c.DataDir = filepath.Join(tmpDir, "server")

		c.AdvertiseAddrs.HTTP = "127.0.0.1"
		c.AdvertiseAddrs.RPC = "127.0.0.1"
		c.AdvertiseAddrs.Serf = "127.0.0.1"
	})

	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &OperatorSnapshotSaveCommand{Meta: Meta{Ui: ui}}

	dest := filepath.Join(tmpDir, "backup.snap")
	code := cmd.Run([]string{
		"--address=" + url,
		dest,
	})
	must.Zero(t, code)
	must.StrContains(t, ui.OutputWriter.String(), "State file written to "+dest)

	f, err := os.Open(dest)
	must.NoError(t, err)

	meta, err := snapshot.Verify(f)
	must.NoError(t, err)
	must.Positive(t, meta.Index)
}

func TestOperatorSnapshotSave_Fails(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	cmd := &OperatorSnapshotSaveCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), commandErrorText(cmd))
	ui.ErrorWriter.Reset()

	// Fails when specified file does not exist
	code = cmd.Run([]string{"/unicorns/leprechauns"})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "no such file")
}
