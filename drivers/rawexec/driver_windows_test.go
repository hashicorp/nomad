// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package rawexec

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/drivers/shared/executor/procstats"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

// TestRawExecDriver_ExecutorKill verifies that killing the executor will stop
// its child processes
func TestRawExecDriver_ExecutorKill(t *testing.T) {
	ci.Parallel(t)

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

	//	_, err := harness.WaitTask(context.Background(), handle.Config.ID)
	//	must.NoError(t, err)
	must.NoError(t, harness.WaitUntilStarted(task.ID, 1*time.Second))

	// we don't know the PID of the executor but we know there are only 3
	// children, so forcibly kill the one that isn't the workload
	children := procstats.List(os.Getpid())
	spew.Dump(children)
	// for _, childPid := range children.Slice() {
	// 	if childPid != taskState.Pid {
	// 		break
	// 	}
	// }
	fmt.Println("--------------")
	time.Sleep(10 * time.Second)

	proc, err := os.FindProcess(taskState.ReattachConfig.Pid)
	must.NoError(t, err)
	must.NoError(t, proc.Kill())
	t.Logf("killed %d", taskState.ReattachConfig.Pid)

	must.NotEq(t, taskState.ReattachConfig.Pid, taskState.Pid)

	// select {
	// case result := <-ch:
	// 	must.ErrorContains(t, result.Err, "executor: error waiting on process")
	// case <-time.After(10 * time.Second):
	// 	t.Fatal("timeout waiting for task to shutdown")
	// }

	t.Cleanup(func() {
		if proc != nil {
			proc.Kill()
		}
	})

	// the child process should be gone as well
	must.Wait(t, wait.InitialSuccess(wait.BoolFunc(func() bool {
		proc, err = os.FindProcess(taskState.Pid)
		return err != nil
	}),
		wait.Timeout(3*time.Second),
		wait.Gap(100*time.Millisecond),
	))

	must.EqError(t, err, "OpenProcess: The parameter is incorrect.")
}
