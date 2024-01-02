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
	"github.com/stretchr/testify/require"
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
	require.Zero(t, code)
	require.Contains(t, ui.OutputWriter.String(), "State file written to "+dest)

	f, err := os.Open(dest)
	require.NoError(t, err)

	meta, err := snapshot.Verify(f)
	require.NoError(t, err)
	require.NotZero(t, meta.Index)
}

func TestOperatorSnapshotSave_Fails(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	cmd := &OperatorSnapshotSaveCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, 1, code)
	require.Contains(t, ui.ErrorWriter.String(), commandErrorText(cmd))
	ui.ErrorWriter.Reset()

	// Fails when specified file does not exist
	code = cmd.Run([]string{"/unicorns/leprechauns"})
	require.Equal(t, 1, code)
	require.Contains(t, ui.ErrorWriter.String(), "no such file")
}
