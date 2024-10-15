// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package rawexec

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/shoenig/test/must"
)

// TestRawExecDriver_ExecutorKill verifies that killing the executor will stop
// its child processes
func TestRawExecDriver_ExecutorKill(t *testing.T) {
	//	ci.Parallel(t)

	d := newEnabledRawExecDriver(t)
	harness := dtestutil.NewDriverHarness(t, d)
	t.Cleanup(func() { harness.Kill() })

	config := &Config{Enabled: true}
	var data []byte
	must.NoError(t, base.MsgPackEncode(&data, config))
	bconfig := &base.Config{
		PluginConfig: data,
		AgentConfig: &base.AgentConfig{
			Driver: &base.ClientDriverConfig{
				Topology: d.nomadConfig.Topology,
			},
		},
	}
	must.NoError(t, harness.SetConfig(bconfig))

	allocID := uuid.Generate()
	taskName := "test"
	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      taskName,
		Resources: testResources(allocID, taskName),
	}

	taskConfig := map[string]interface{}{}
	taskConfig["command"] = "Powershell.exe"
	taskConfig["args"] = []string{"sleep", "100s"}

	must.NoError(t, task.EncodeConcreteDriverConfig(&taskConfig))

	cleanup := harness.MkAllocDir(task, false)
	t.Cleanup(cleanup)

	handle, _, err := harness.StartTask(task)
	must.NoError(t, err)

	var taskState TaskState
	must.NoError(t, handle.GetDriverState(&taskState))

	childPid := taskState.Pid

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	must.NoError(t, err)
	must.NoError(t, harness.WaitUntilStarted(task.ID, 1*time.Second))
	harness.Kill()

	select {
	case result := <-ch:
		must.EqError(t, result.Err, "rpc error: code = Unavailable desc = error reading from server: EOF")
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for task to shutdown")
	}

	// the child process should be gone as well
	proc, err := os.FindProcess(childPid)
	t.Cleanup(func() {
		if proc != nil {
			proc.Kill()
		}
	})
	must.EqError(t, err, "OpenProcess: The parameter is incorrect.")
}
