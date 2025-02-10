// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package executor

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/fsisolation"
	"github.com/shoenig/test/must"
)

// testExecutorCommand sets up a test task environment.
func testExecutorCommand(t *testing.T) *testExecCmd {
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	taskEnv := taskenv.NewBuilder(mock.Node(), alloc, task, "global").Build()

	allocDir := allocdir.NewAllocDir(testlog.HCLogger(t), t.TempDir(), t.TempDir(), alloc.ID)
	must.NoError(t, allocDir.Build())
	t.Cleanup(func() { allocDir.Destroy() })

	must.NoError(t, allocDir.NewTaskDir(task).Build(fsisolation.None, nil, task.User))
	td := allocDir.TaskDirs[task.Name]
	cmd := &ExecCommand{
		Env:     taskEnv.List(),
		TaskDir: td.Dir,
		Resources: &drivers.Resources{
			NomadResources: &structs.AllocatedTaskResources{
				Cpu: structs.AllocatedCpuResources{
					CpuShares: 500,
				},
				Memory: structs.AllocatedMemoryResources{
					MemoryMB: 256,
				},
			},
		},
	}

	testCmd := &testExecCmd{
		command:  cmd,
		allocDir: allocDir,
	}
	configureTLogging(t, testCmd)
	return testCmd
}

func TestExecutor_ProcessExit(t *testing.T) {
	ci.Parallel(t)

	topology := numalib.Scan(numalib.PlatformScanners())
	compute := topology.Compute()

	cmd := testExecutorCommand(t)
	cmd.command.Cmd = "Powershell.exe"
	cmd.command.Args = []string{"sleep", "30"}
	executor := NewExecutor(testlog.HCLogger(t), compute)

	t.Cleanup(func() { executor.Shutdown("SIGKILL", 0) })

	childPs, err := executor.Launch(cmd.command)
	must.NoError(t, err)
	must.NonZero(t, childPs.Pid)

	proc, err := os.FindProcess(childPs.Pid)
	must.NoError(t, err)
	must.NoError(t, proc.Kill())

	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	t.Cleanup(cancel)
	waitPs, err := executor.Wait(ctx)
	must.NoError(t, err)
	must.Eq(t, 1, waitPs.ExitCode)
	must.Eq(t, childPs.Pid, waitPs.Pid)
}
