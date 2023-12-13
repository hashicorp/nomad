// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestOperatorSnapshotRestore_Works(t *testing.T) {
	ci.Parallel(t)

	tmpDir := t.TempDir()

	snapshotPath := generateSnapshotFile(t, func(srv *agent.TestAgent, client *api.Client, url string) {
		sampleJob := `
job "snapshot-test-job" {
	type = "service"
	datacenters = [ "dc1" ]
	group "group1" {
		count = 1
		task "task1" {
			driver = "exec"
			resources {
				cpu = 1000
				memory = 512
			}
		}
	}

}`

		ui := cli.NewMockUi()
		cmd := &JobRunCommand{Meta: Meta{Ui: ui}}
		cmd.JobGetter.testStdin = strings.NewReader(sampleJob)

		code := cmd.Run([]string{"--address=" + url, "-detach", "-"})
		require.Zero(t, code)
	})

	srv, _, url := testServer(t, false, func(c *agent.Config) {
		c.DevMode = false
		c.DataDir = filepath.Join(tmpDir, "server1")

		c.AdvertiseAddrs.HTTP = "127.0.0.1"
		c.AdvertiseAddrs.RPC = "127.0.0.1"
		c.AdvertiseAddrs.Serf = "127.0.0.1"
	})

	defer srv.Shutdown()

	// job is not found before restore
	j, err := srv.Agent.Server().State().JobByID(nil, structs.DefaultNamespace, "snapshot-test-job")
	require.NoError(t, err)
	require.Nil(t, j)

	ui := cli.NewMockUi()
	cmd := &OperatorSnapshotRestoreCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{"--address=" + url, snapshotPath})
	require.Empty(t, ui.ErrorWriter.String())
	require.Zero(t, code)
	require.Contains(t, ui.OutputWriter.String(), "Snapshot Restored")

	foundJob, err := srv.Agent.Server().State().JobByID(nil, structs.DefaultNamespace, "snapshot-test-job")
	require.NoError(t, err)
	require.Equal(t, "snapshot-test-job", foundJob.ID)
}

func TestOperatorSnapshotRestore_Fails(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	cmd := &OperatorSnapshotRestoreCommand{Meta: Meta{Ui: ui}}

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
