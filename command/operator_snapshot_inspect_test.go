// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestOperatorSnapshotInspect_Works(t *testing.T) {
	ci.Parallel(t)

	snapPath := generateSnapshotFile(t, nil)

	ui := cli.NewMockUi()
	cmd := &OperatorSnapshotInspectCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{snapPath})
	require.Zero(t, code)

	output := ui.OutputWriter.String()
	for _, key := range []string{
		"ID",
		"Size",
		"Index",
		"Term",
		"Version",
	} {
		require.Contains(t, output, key)
	}
}

func TestOperatorSnapshotInspect_HandlesFailure(t *testing.T) {
	ci.Parallel(t)

	tmpDir := t.TempDir()

	err := os.WriteFile(
		filepath.Join(tmpDir, "invalid.snap"),
		[]byte("invalid data"),
		0600)
	require.NoError(t, err)

	t.Run("not found", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &OperatorSnapshotInspectCommand{Meta: Meta{Ui: ui}}

		code := cmd.Run([]string{filepath.Join(tmpDir, "foo")})
		require.NotZero(t, code)
		require.Contains(t, ui.ErrorWriter.String(), "no such file")
	})

	t.Run("invalid file", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &OperatorSnapshotInspectCommand{Meta: Meta{Ui: ui}}

		code := cmd.Run([]string{filepath.Join(tmpDir, "invalid.snap")})
		require.NotZero(t, code)
		require.Contains(t, ui.ErrorWriter.String(), "Error verifying snapshot")
	})
}

func generateSnapshotFile(t *testing.T, prepare func(srv *agent.TestAgent, client *api.Client, url string)) string {
	tmpDir := t.TempDir()

	srv, api, url := testServer(t, false, func(c *agent.Config) {
		c.DevMode = false
		c.DataDir = filepath.Join(tmpDir, "server")

		c.AdvertiseAddrs.HTTP = "127.0.0.1"
		c.AdvertiseAddrs.RPC = "127.0.0.1"
		c.AdvertiseAddrs.Serf = "127.0.0.1"
	})

	defer srv.Shutdown()

	if prepare != nil {
		prepare(srv, api, url)
	}

	ui := cli.NewMockUi()
	cmd := &OperatorSnapshotSaveCommand{Meta: Meta{Ui: ui}}

	dest := filepath.Join(tmpDir, "backup.snap")
	code := cmd.Run([]string{
		"--address=" + url,
		dest,
	})
	require.Zero(t, code)

	return dest
}
