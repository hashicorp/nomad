// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package rawexec

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/shoenig/test/must"
)

// TestRawExecDriver_ExecutorKill verifies that killing the executor will stop
// its child processes
func TestRawExecDriver_ExecutorKill(t *testing.T) {
	ci.Parallel(t)

	d := newEnabledRawExecDriver(t)
	harness := dtestutil.NewDriverHarness(t, d)
	t.Cleanup(harness.Kill)

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
	must.NoError(t, harness.WaitUntilStarted(task.ID, 1*time.Second))

	// forcibly kill the executor, not the workload
	must.NotEq(t, taskState.ReattachConfig.Pid, taskState.Pid)
	proc, err := os.FindProcess(taskState.ReattachConfig.Pid)
	must.NoError(t, err)

	taskProc, err := os.FindProcess(taskState.Pid)
	must.NoError(t, err)

	must.NoError(t, proc.Kill())
	t.Logf("killed %d, waiting on %d to stop", taskState.ReattachConfig.Pid, taskState.Pid)

	t.Cleanup(func() {
		if taskProc != nil {
			taskProc.Kill()
		}
	})

	done := make(chan struct{})
	go func() {
		taskProc.Wait()
		close(done)
	}()

	select {
	case <-time.After(5 * time.Second):
		t.Fatal("expected child process to exit")
	case <-done:
	}
}
