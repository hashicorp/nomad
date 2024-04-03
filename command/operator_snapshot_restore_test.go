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
	"github.com/shoenig/test/must"
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
		must.Zero(t, code)
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
	must.NoError(t, err)
	must.Nil(t, j)

	ui := cli.NewMockUi()
	cmd := &OperatorSnapshotRestoreCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{"--address=" + url, snapshotPath})
	must.Eq(t, "", ui.ErrorWriter.String())
	must.Zero(t, code)
	must.StrContains(t, ui.OutputWriter.String(), "Snapshot Restored")

	foundJob, err := srv.Agent.Server().State().JobByID(nil, structs.DefaultNamespace, "snapshot-test-job")
	must.NoError(t, err)
	must.Eq(t, "snapshot-test-job", foundJob.ID)
}

func TestOperatorSnapshotRestore_Fails(t *testing.T) {
	ci.Parallel(t)

	ui := cli.NewMockUi()
	cmd := &OperatorSnapshotRestoreCommand{Meta: Meta{Ui: ui}}

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
