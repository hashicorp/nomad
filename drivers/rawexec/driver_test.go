// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package rawexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/client/lib/numalib"
	ctestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/testtask"
	"github.com/hashicorp/nomad/helper/uuid"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	basePlug "github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
	"github.com/stretchr/testify/require"
)

// defaultEnv creates the default environment for raw exec tasks
func defaultEnv() map[string]string {
	m := make(map[string]string)
	return m
}

func testResources(allocID, task string) *drivers.Resources {
	if allocID == "" || task == "" {
		panic("must be set")
	}

	r := &drivers.Resources{
		NomadResources: &nstructs.AllocatedTaskResources{
			Memory: nstructs.AllocatedMemoryResources{
				MemoryMB: 128,
			},
			Cpu: nstructs.AllocatedCpuResources{
				CpuShares: 100,
			},
		},
		LinuxResources: &drivers.LinuxResources{
			MemoryLimitBytes: 134217728,
			CPUShares:        100,
			CpusetCgroupPath: cgroupslib.LinuxResourcesPath(allocID, task, false),
		},
	}

	return r
}

func TestMain(m *testing.M) {
	if !testtask.Run() {
		os.Exit(m.Run())
	}
}

var (
	topology = numalib.Scan(numalib.PlatformScanners())
)

func newEnabledRawExecDriver(t *testing.T) *Driver {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	logger := testlog.HCLogger(t)
	d := NewRawExecDriver(ctx, logger).(*Driver)
	d.config.Enabled = true
	d.nomadConfig = &base.ClientDriverConfig{
		Topology: topology,
	}

	return d
}

func TestRawExecDriver_SetConfig(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := testlog.HCLogger(t)

	d := NewRawExecDriver(ctx, logger)
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	var (
		bconfig = new(basePlug.Config)
		config  = new(Config)
		data    = make([]byte, 0)
	)

	// Default is raw_exec is disabled.
	require.NoError(basePlug.MsgPackEncode(&data, config))
	bconfig.PluginConfig = data
	require.NoError(harness.SetConfig(bconfig))
	require.Exactly(config, d.(*Driver).config)

	// Enable raw_exec, but disable cgroups.
	config.Enabled = true
	data = []byte{}
	require.NoError(basePlug.MsgPackEncode(&data, config))
	bconfig.PluginConfig = data
	require.NoError(harness.SetConfig(bconfig))
	require.Exactly(config, d.(*Driver).config)
}

func TestRawExecDriver_Fingerprint(t *testing.T) {
	ci.Parallel(t)

	fingerprintTest := func(config *Config, expected *drivers.Fingerprint) func(t *testing.T) {
		return func(t *testing.T) {
			require := require.New(t)
			d := newEnabledRawExecDriver(t)
			harness := dtestutil.NewDriverHarness(t, d)
			defer harness.Kill()

			var data []byte
			require.NoError(basePlug.MsgPackEncode(&data, config))
			bconfig := &basePlug.Config{
				PluginConfig: data,
			}
			require.NoError(harness.SetConfig(bconfig))

			fingerCh, err := harness.Fingerprint(context.Background())
			require.NoError(err)
			select {
			case result := <-fingerCh:
				require.Equal(expected, result)
			case <-time.After(time.Duration(testutil.TestMultiplier()) * time.Second):
				require.Fail("timeout receiving fingerprint")
			}
		}
	}

	cases := []struct {
		Name     string
		Conf     Config
		Expected drivers.Fingerprint
	}{
		{
			Name: "Disabled",
			Conf: Config{
				Enabled: false,
			},
			Expected: drivers.Fingerprint{
				Attributes:        nil,
				Health:            drivers.HealthStateUndetected,
				HealthDescription: "disabled",
			},
		},
		{
			Name: "Enabled",
			Conf: Config{
				Enabled: true,
			},
			Expected: drivers.Fingerprint{
				Attributes:        map[string]*pstructs.Attribute{"driver.raw_exec": pstructs.NewBoolAttribute(true)},
				Health:            drivers.HealthStateHealthy,
				HealthDescription: drivers.DriverHealthy,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, fingerprintTest(&tc.Conf, &tc.Expected))
	}
}

func TestRawExecDriver_StartWait(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	d := newEnabledRawExecDriver(t)
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	allocID := uuid.Generate()
	taskName := "test"
	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      taskName,
		Env:       defaultEnv(),
		Resources: testResources(allocID, taskName),
	}

	tc := &TaskConfig{
		Command: testtask.Path(),
		Args:    []string{"sleep", "10ms"},
	}
	require.NoError(task.EncodeConcreteDriverConfig(&tc))
	testtask.SetTaskConfigEnv(task)

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	harness.MakeTaskCgroup(allocID, taskName)

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)

	var result *drivers.ExitResult
	select {
	case result = <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	require.Zero(result.ExitCode)
	require.Zero(result.Signal)
	require.False(result.OOMKilled)
	require.NoError(result.Err)
	require.NoError(harness.DestroyTask(task.ID, true))
}

func TestRawExecDriver_Start_Wait_AllocDir(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	d := newEnabledRawExecDriver(t)
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	allocID := uuid.Generate()
	taskName := "sleep"
	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      taskName,
		Env:       defaultEnv(),
		Resources: testResources(allocID, taskName),
	}

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	harness.MakeTaskCgroup(allocID, taskName)

	exp := []byte("win")
	file := "output.txt"
	outPath := fmt.Sprintf(`%s/%s`, task.TaskDir().SharedAllocDir, file)

	tc := &TaskConfig{
		Command: testtask.Path(),
		Args:    []string{"sleep", "1s", "write", string(exp), outPath},
	}
	require.NoError(task.EncodeConcreteDriverConfig(&tc))
	testtask.SetTaskConfigEnv(task)

	_, _, err := harness.StartTask(task)
	require.NoError(err)

	// Task should terminate quickly
	waitCh, err := harness.WaitTask(context.Background(), task.ID)
	require.NoError(err)

	select {
	case res := <-waitCh:
		require.NoError(res.Err)
		require.True(res.Successful())
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		require.Fail("WaitTask timeout")
	}

	// Check that data was written to the shared alloc directory.
	outputFile := filepath.Join(task.TaskDir().SharedAllocDir, file)
	act, err := os.ReadFile(outputFile)
	require.NoError(err)
	require.Exactly(exp, act)
	require.NoError(harness.DestroyTask(task.ID, true))
}

// This test creates a process tree such that without cgroups tracking the
// processes cleanup of the children would not be possible. Thus the test
// asserts that the processes get killed properly when using cgroups.
func TestRawExecDriver_Start_Kill_Wait_Cgroup(t *testing.T) {
	ci.Parallel(t)
	ctestutil.ExecCompatible(t)

	pidFile := "pid"

	d := newEnabledRawExecDriver(t)
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	allocID := uuid.Generate()
	taskName := "sleep"
	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      taskName,
		User:      "root",
		Env:       defaultEnv(),
		Resources: testResources(allocID, taskName),
	}

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	harness.MakeTaskCgroup(allocID, taskName)

	tc := &TaskConfig{
		Command: testtask.Path(),
		Args:    []string{"fork/exec", pidFile, "pgrp", "0", "sleep", "7s"},
	}
	must.NoError(t, task.EncodeConcreteDriverConfig(&tc))
	testtask.SetTaskConfigEnv(task)

	_, _, err := harness.StartTask(task)
	must.NoError(t, err)

	// Find the process
	var pidData []byte

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			data, err := os.ReadFile(filepath.Join(task.TaskDir().Dir, pidFile))
			if err != nil {
				return err
			}
			if len(bytes.TrimSpace(data)) == 0 {
				return errors.New("pidFile empty")
			}
			pidData = data
			return nil
		}),
		wait.Gap(1*time.Second),
		wait.Timeout(3*time.Second),
	))

	pid, err := strconv.Atoi(string(pidData))
	must.NoError(t, err)

	// Check the pid is up
	process, err := os.FindProcess(pid)
	must.NoError(t, err)
	must.NoError(t, process.Signal(syscall.Signal(0)))

	// Stop the task
	must.NoError(t, harness.StopTask(task.ID, 0, ""))

	// Task should terminate quickly
	waitCh, err := harness.WaitTask(context.Background(), task.ID)
	must.NoError(t, err)
	select {
	case res := <-waitCh:
		must.False(t, res.Successful())
	case <-time.After(10 * time.Second):
		must.Unreachable(t, must.Sprint("exceeded wait timeout"))
	}

	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(func() bool {
			return process.Signal(syscall.Signal(0)) == nil
		}),
		wait.Gap(1*time.Second),
		wait.Timeout(3*time.Second),
	))

	must.NoError(t, harness.DestroyTask(task.ID, true))
}

func TestRawExecDriver_ParentCgroup(t *testing.T) {
	t.Skip("TODO: seth will fix this during the cpuset partitioning work")

	ci.Parallel(t)
	ctestutil.ExecCompatible(t)
	ctestutil.CgroupsCompatibleV2(t)

	d := newEnabledRawExecDriver(t)
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	allocID := uuid.Generate()
	taskName := "sleep"
	task := &drivers.TaskConfig{
		AllocID: allocID,
		ID:      uuid.Generate(),
		Name:    taskName,
		Env: map[string]string{
			"NOMAD_PARENT_CGROUP": "custom.slice",
		},
	}

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	harness.MakeTaskCgroup(allocID, taskName)

	// run sleep task
	tc := &TaskConfig{
		Command: testtask.Path(),
		Args:    []string{"sleep", "9000s"},
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&tc))
	testtask.SetTaskConfigEnv(task)
	_, _, err := harness.StartTask(task)
	require.NoError(t, err)

	// inspect environment variable
	res, execErr := harness.ExecTask(task.ID, []string{"/usr/bin/env"}, 1*time.Second)
	require.NoError(t, execErr)
	require.True(t, res.ExitResult.Successful())
	require.Contains(t, string(res.Stdout), "custom.slice")

	// inspect /proc/self/cgroup
	res2, execErr2 := harness.ExecTask(task.ID, []string{"cat", "/proc/self/cgroup"}, 1*time.Second)
	require.NoError(t, execErr2)
	require.True(t, res2.ExitResult.Successful())
	require.Contains(t, string(res2.Stdout), "custom.slice")

	// kill the sleep task
	require.NoError(t, harness.DestroyTask(task.ID, true))
}

func TestRawExecDriver_Exec(t *testing.T) {
	ci.Parallel(t)
	ctestutil.ExecCompatible(t)

	require := require.New(t)

	d := newEnabledRawExecDriver(t)
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	allocID := uuid.Generate()
	taskName := "sleep"
	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      taskName,
		Env:       defaultEnv(),
		Resources: testResources(allocID, taskName),
	}

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	harness.MakeTaskCgroup(allocID, taskName)

	tc := &TaskConfig{
		Command: testtask.Path(),
		Args:    []string{"sleep", "9000s"},
	}
	require.NoError(task.EncodeConcreteDriverConfig(&tc))
	testtask.SetTaskConfigEnv(task)

	_, _, err := harness.StartTask(task)
	require.NoError(err)

	if runtime.GOOS == "windows" {
		// Exec a command that should work
		res, err := harness.ExecTask(task.ID, []string{"cmd.exe", "/c", "echo", "hello"}, 1*time.Second)
		require.NoError(err)
		require.True(res.ExitResult.Successful())
		require.Equal(string(res.Stdout), "hello\r\n")

		// Exec a command that should fail
		res, err = harness.ExecTask(task.ID, []string{"cmd.exe", "/c", "stat", "notarealfile123abc"}, 1*time.Second)
		require.NoError(err)
		require.False(res.ExitResult.Successful())
		require.Contains(string(res.Stdout), "not recognized")
	} else {
		// Exec a command that should work
		res, err := harness.ExecTask(task.ID, []string{"/usr/bin/stat", "/tmp"}, 1*time.Second)
		require.NoError(err)
		require.True(res.ExitResult.Successful())
		require.True(len(res.Stdout) > 100)

		// Exec a command that should fail
		res, err = harness.ExecTask(task.ID, []string{"/usr/bin/stat", "notarealfile123abc"}, 1*time.Second)
		require.NoError(err)
		require.False(res.ExitResult.Successful())
		require.Contains(string(res.Stdout), "No such file or directory")
	}

	require.NoError(harness.DestroyTask(task.ID, true))
}

func TestConfig_ParseAllHCL(t *testing.T) {
	ci.Parallel(t)

	cfgStr := `
config {
  command = "/bin/bash"
  args = ["-c", "echo hello"]
}`

	expected := &TaskConfig{
		Command: "/bin/bash",
		Args:    []string{"-c", "echo hello"},
	}

	var tc *TaskConfig
	hclutils.NewConfigParser(taskConfigSpec).ParseHCL(t, cfgStr, &tc)

	require.EqualValues(t, expected, tc)
}

func TestRawExecDriver_Disabled(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	d := newEnabledRawExecDriver(t)
	d.config.Enabled = false

	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	allocID := uuid.Generate()
	taskName := "test"
	task := &drivers.TaskConfig{
		AllocID: allocID,
		ID:      uuid.Generate(),
		Name:    taskName,
		Env:     defaultEnv(),
	}

	handle, _, err := harness.StartTask(task)
	require.Error(err)
	require.Contains(err.Error(), errDisabledDriver.Error())
	require.Nil(handle)
}
