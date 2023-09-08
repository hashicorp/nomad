// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	ctestutils "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/drivers/shared/executor"
	"github.com/hashicorp/nomad/helper/pluginutils/hclutils"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/testtask"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	basePlug "github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if !testtask.Run() {
		os.Exit(m.Run())
	}
}

func testResources(allocID, task string) *drivers.Resources {
	if allocID == "" || task == "" {
		panic("must be set")
	}

	r := &drivers.Resources{
		NomadResources: &structs.AllocatedTaskResources{
			Memory: structs.AllocatedMemoryResources{
				MemoryMB: 128,
			},
			Cpu: structs.AllocatedCpuResources{
				CpuShares: 100,
			},
		},
		LinuxResources: &drivers.LinuxResources{
			MemoryLimitBytes: 134217728,
			CPUShares:        100,
			CpusetCgroupPath: cgroupslib.LinuxResourcesPath(allocID, task),
		},
	}

	return r
}

func TestExecDriver_Fingerprint_NonLinux(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	if runtime.GOOS == "linux" {
		t.Skip("Test only available not on Linux")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewExecDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	fingerCh, err := harness.Fingerprint(context.Background())
	require.NoError(err)
	select {
	case finger := <-fingerCh:
		require.Equal(drivers.HealthStateUndetected, finger.Health)
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		require.Fail("timeout receiving fingerprint")
	}
}

func TestExecDriver_Fingerprint(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	ctestutils.ExecCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewExecDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	fingerCh, err := harness.Fingerprint(context.Background())
	require.NoError(err)
	select {
	case finger := <-fingerCh:
		require.Equal(drivers.HealthStateHealthy, finger.Health)
		require.True(finger.Attributes["driver.exec"].GetBool())
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		require.Fail("timeout receiving fingerprint")
	}
}

func TestExecDriver_StartWait(t *testing.T) {
	ci.Parallel(t)
	ctestutils.ExecCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := testlog.HCLogger(t)

	d := NewExecDriver(ctx, logger)
	harness := dtestutil.NewDriverHarness(t, d)
	allocID := uuid.Generate()
	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      "test",
		Resources: testResources(allocID, "test"),
	}

	tc := &TaskConfig{
		Command: "cat",
		Args:    []string{"/proc/self/cgroup"},
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&tc))

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	require.NoError(t, err)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(t, err)
	result := <-ch
	require.Zero(t, result.ExitCode)
	require.NoError(t, harness.DestroyTask(task.ID, true))
}

func TestExecDriver_StartWaitStopKill(t *testing.T) {
	ci.Parallel(t)
	ctestutils.ExecCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewExecDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)
	allocID := uuid.Generate()
	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      "test",
		Resources: testResources(allocID, "test"),
	}

	tc := &TaskConfig{
		Command: "/bin/bash",
		Args:    []string{"-c", "echo hi; sleep 600"},
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&tc))

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	require.NoError(t, err)
	defer harness.DestroyTask(task.ID, true)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(t, err)

	require.NoError(t, harness.WaitUntilStarted(task.ID, 1*time.Second))

	go func() {
		harness.StopTask(task.ID, 2*time.Second, "SIGINT")
	}()

	select {
	case result := <-ch:
		require.False(t, result.Successful())
	case <-time.After(10 * time.Second):
		require.Fail(t, "timeout waiting for task to shutdown")
	}

	// Ensure that the task is marked as dead, but account
	// for WaitTask() closing channel before internal state is updated
	testutil.WaitForResult(func() (bool, error) {
		status, err := harness.InspectTask(task.ID)
		if err != nil {
			return false, fmt.Errorf("inspecting task failed: %v", err)
		}
		if status.State != drivers.TaskStateExited {
			return false, fmt.Errorf("task hasn't exited yet; status: %v", status.State)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	require.NoError(t, harness.DestroyTask(task.ID, true))
}

func TestExecDriver_StartWaitRecover(t *testing.T) {
	ci.Parallel(t)
	ctestutils.ExecCompatible(t)

	dCtx, dCancel := context.WithCancel(context.Background())
	defer dCancel()

	d := NewExecDriver(dCtx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)
	allocID := uuid.Generate()
	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      "test",
		Resources: testResources(allocID, "test"),
	}

	tc := &TaskConfig{
		Command: "/bin/sleep",
		Args:    []string{"5"},
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&tc))

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := harness.WaitTask(ctx, handle.Config.ID)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result := <-ch
		require.Error(t, result.Err)
	}()

	require.NoError(t, harness.WaitUntilStarted(task.ID, 1*time.Second))
	cancel()

	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		wg.Wait()
	}()

	select {
	case <-waitCh:
		status, err := harness.InspectTask(task.ID)
		require.NoError(t, err)
		require.Equal(t, drivers.TaskStateRunning, status.State)
	case <-time.After(1 * time.Second):
		require.Fail(t, "timeout waiting for task wait to cancel")
	}

	// Loose task
	d.(*Driver).tasks.Delete(task.ID)
	_, err = harness.InspectTask(task.ID)
	require.Error(t, err)

	require.NoError(t, harness.RecoverTask(handle))
	status, err := harness.InspectTask(task.ID)
	require.NoError(t, err)
	require.Equal(t, drivers.TaskStateRunning, status.State)

	require.NoError(t, harness.StopTask(task.ID, 0, ""))
	require.NoError(t, harness.DestroyTask(task.ID, true))
}

// TestExecDriver_NoOrphans asserts that when the main
// task dies, the orphans in the PID namespaces are killed by the kernel
func TestExecDriver_NoOrphans(t *testing.T) {
	ci.Parallel(t)
	ctestutils.ExecCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewExecDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	config := &Config{
		NoPivotRoot:    false,
		DefaultModePID: executor.IsolationModePrivate,
		DefaultModeIPC: executor.IsolationModePrivate,
	}

	var data []byte
	require.NoError(t, basePlug.MsgPackEncode(&data, config))
	baseConfig := &basePlug.Config{PluginConfig: data}
	require.NoError(t, harness.SetConfig(baseConfig))

	allocID := uuid.Generate()
	taskName := "test"
	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      taskName,
		Resources: testResources(allocID, taskName),
	}

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	taskConfig := map[string]interface{}{}
	taskConfig["command"] = "/bin/sh"
	// print the child PID in the task PID namespace, then sleep for 5 seconds to give us a chance to examine processes
	taskConfig["args"] = []string{"-c", fmt.Sprintf(`sleep 3600 & sleep 20`)}
	require.NoError(t, task.EncodeConcreteDriverConfig(&taskConfig))

	handle, _, err := harness.StartTask(task)
	require.NoError(t, err)
	defer harness.DestroyTask(task.ID, true)

	waitCh, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(t, err)

	require.NoError(t, harness.WaitUntilStarted(task.ID, 1*time.Second))

	var childPids []int
	taskState := TaskState{}
	testutil.WaitForResult(func() (bool, error) {
		require.NoError(t, handle.GetDriverState(&taskState))
		if taskState.Pid == 0 {
			return false, fmt.Errorf("task PID is zero")
		}

		children, err := os.ReadFile(fmt.Sprintf("/proc/%d/task/%d/children", taskState.Pid, taskState.Pid))
		if err != nil {
			return false, fmt.Errorf("error reading /proc for children: %v", err)
		}
		pids := strings.Fields(string(children))
		if len(pids) < 2 {
			return false, fmt.Errorf("error waiting for two children, currently %d", len(pids))
		}
		for _, cpid := range pids {
			p, err := strconv.Atoi(cpid)
			if err != nil {
				return false, fmt.Errorf("error parsing child pids from /proc: %s", cpid)
			}
			childPids = append(childPids, p)
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	select {
	case result := <-waitCh:
		require.True(t, result.Successful(), "command failed: %#v", result)
	case <-time.After(30 * time.Second):
		require.Fail(t, "timeout waiting for task to shutdown")
	}

	// isProcessRunning returns an error if process is not running
	isProcessRunning := func(pid int) error {
		process, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("failed to find process: %s", err)
		}

		err = process.Signal(syscall.Signal(0))
		if err != nil {
			return fmt.Errorf("failed to signal process: %s", err)
		}

		return nil
	}

	// task should be dead
	require.Error(t, isProcessRunning(taskState.Pid))

	// all children should eventually be killed by OS
	testutil.WaitForResult(func() (bool, error) {
		for _, cpid := range childPids {
			err := isProcessRunning(cpid)
			if err == nil {
				return false, fmt.Errorf("child process %d is still running", cpid)
			}
			if !strings.Contains(err.Error(), "failed to signal process") {
				return false, fmt.Errorf("unexpected error: %v", err)
			}
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

func TestExecDriver_Stats(t *testing.T) {
	ci.Parallel(t)
	ctestutils.ExecCompatible(t)

	dctx, dcancel := context.WithCancel(context.Background())
	defer dcancel()

	d := NewExecDriver(dctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	allocID := uuid.Generate()
	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      "test",
		Resources: testResources(allocID, "test"),
	}

	tc := &TaskConfig{
		Command: "/bin/sleep",
		Args:    []string{"5"},
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&tc))

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	require.NoError(t, err)
	require.NotNil(t, handle)

	require.NoError(t, harness.WaitUntilStarted(task.ID, 1*time.Second))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	statsCh, err := harness.TaskStats(ctx, task.ID, time.Second*10)
	require.NoError(t, err)
	select {
	case stats := <-statsCh:
		require.NotEmpty(t, stats.ResourceUsage.MemoryStats.Measured)
		require.NotZero(t, stats.Timestamp)
		require.WithinDuration(t, time.Now(), time.Unix(0, stats.Timestamp), time.Second)
	case <-time.After(time.Second):
		require.Fail(t, "timeout receiving from channel")
	}

	require.NoError(t, harness.DestroyTask(task.ID, true))
}

func TestExecDriver_Start_Wait_AllocDir(t *testing.T) {
	ci.Parallel(t)
	ctestutils.ExecCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewExecDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)
	allocID := uuid.Generate()
	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      "sleep",
		Resources: testResources(allocID, "test"),
	}
	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	exp := []byte{'w', 'i', 'n'}
	file := "output.txt"
	tc := &TaskConfig{
		Command: "/bin/bash",
		Args: []string{
			"-c",
			fmt.Sprintf(`sleep 1; echo -n %s > /alloc/%s`, string(exp), file),
		},
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&tc))

	handle, _, err := harness.StartTask(task)
	require.NoError(t, err)
	require.NotNil(t, handle)

	// Task should terminate quickly
	waitCh, err := harness.WaitTask(context.Background(), task.ID)
	require.NoError(t, err)
	select {
	case res := <-waitCh:
		require.True(t, res.Successful(), "task should have exited successfully: %v", res)
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		require.Fail(t, "timeout waiting for task")
	}

	// Check that data was written to the shared alloc directory.
	outputFile := filepath.Join(task.TaskDir().SharedAllocDir, file)
	act, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	require.Exactly(t, exp, act)

	require.NoError(t, harness.DestroyTask(task.ID, true))
}

func TestExecDriver_User(t *testing.T) {
	ci.Parallel(t)
	ctestutils.ExecCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewExecDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)
	allocID := uuid.Generate()
	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      "sleep",
		User:      "alice",
		Resources: testResources(allocID, "sleep"),
	}
	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	tc := &TaskConfig{
		Command: "/bin/sleep",
		Args:    []string{"100"},
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&tc))

	handle, _, err := harness.StartTask(task)
	require.Error(t, err)
	require.Nil(t, handle)

	msg := "user alice"
	if !strings.Contains(err.Error(), msg) {
		t.Fatalf("Expecting '%v' in '%v'", msg, err)
	}
}

// TestExecDriver_HandlerExec ensures the exec driver's handle properly
// executes commands inside the container.
func TestExecDriver_HandlerExec(t *testing.T) {
	ci.Parallel(t)
	ctestutils.ExecCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewExecDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)
	allocID := uuid.Generate()
	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      "sleep",
		Resources: testResources(allocID, "sleep"),
	}
	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	tc := &TaskConfig{
		Command: "/bin/sleep",
		Args:    []string{"9000"},
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&tc))

	handle, _, err := harness.StartTask(task)
	require.NoError(t, err)
	require.NotNil(t, handle)

	// Assert cgroup membership
	res, err := harness.ExecTask(task.ID, []string{"/bin/cat", "/proc/self/cgroup"}, time.Second)
	require.NoError(t, err)
	require.True(t, res.ExitResult.Successful())
	stdout := strings.TrimSpace(string(res.Stdout))
	switch cgroupslib.GetMode() {
	case cgroupslib.CG1:
		for _, line := range strings.Split(stdout, "\n") {
			// skip empty lines
			if line == "" {
				continue
			}
			// skip rdma & misc subsystems
			if strings.Contains(line, ":rdma:") || strings.Contains(line, ":misc:") || strings.Contains(line, "::") {
				continue
			}
			// assert we are in a nomad cgroup
			if !strings.Contains(line, ":/nomad/") {
				t.Fatalf("not a member of the allocs nomad cgroup: %q", line)
			}
		}
	default:
		require.True(t, strings.HasSuffix(stdout, ".scope"), "actual stdout %q", stdout)
	}

	// Exec a command that should fail
	res, err = harness.ExecTask(task.ID, []string{"/usr/bin/stat", "lkjhdsaflkjshowaisxmcvnlia"}, time.Second)
	require.NoError(t, err)
	require.False(t, res.ExitResult.Successful())
	if expected := "No such file or directory"; !bytes.Contains(res.Stdout, []byte(expected)) {
		t.Fatalf("expected output to contain %q but found: %q", expected, res.Stdout)
	}

	require.NoError(t, harness.DestroyTask(task.ID, true))
}

func TestExecDriver_DevicesAndMounts(t *testing.T) {
	ci.Parallel(t)
	ctestutils.ExecCompatible(t)

	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "testfile"), []byte("from-host"), 600)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewExecDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)
	allocID := uuid.Generate()
	task := &drivers.TaskConfig{
		ID:         uuid.Generate(),
		Name:       "test",
		User:       "root", // need permission to read mounts paths
		Resources:  testResources(allocID, "test"),
		StdoutPath: filepath.Join(tmpDir, "task-stdout"),
		StderrPath: filepath.Join(tmpDir, "task-stderr"),
		Devices: []*drivers.DeviceConfig{
			{
				TaskPath:    "/dev/inserted-random",
				HostPath:    "/dev/random",
				Permissions: "rw",
			},
		},
		Mounts: []*drivers.MountConfig{
			{
				TaskPath: "/tmp/task-path-rw",
				HostPath: tmpDir,
				Readonly: false,
			},
			{
				TaskPath: "/tmp/task-path-ro",
				HostPath: tmpDir,
				Readonly: true,
			},
		},
	}

	require.NoError(t, os.WriteFile(task.StdoutPath, []byte{}, 660))
	require.NoError(t, os.WriteFile(task.StderrPath, []byte{}, 660))

	tc := &TaskConfig{
		Command: "/bin/bash",
		Args: []string{"-c", `
export LANG=en.UTF-8
echo "mounted device /inserted-random: $(stat -c '%t:%T' /dev/inserted-random)"
echo "reading from ro path: $(cat /tmp/task-path-ro/testfile)"
echo "reading from rw path: $(cat /tmp/task-path-rw/testfile)"
touch /tmp/task-path-rw/testfile && echo 'overwriting file in rw succeeded'
touch /tmp/task-path-rw/testfile-from-rw && echo from-exec >  /tmp/task-path-rw/testfile-from-rw && echo 'writing new file in rw succeeded'
touch /tmp/task-path-ro/testfile && echo 'overwriting file in ro succeeded'
touch /tmp/task-path-ro/testfile-from-ro && echo from-exec >  /tmp/task-path-ro/testfile-from-ro && echo 'writing new file in ro succeeded'
exit 0
`},
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&tc))

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	require.NoError(t, err)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(t, err)
	result := <-ch
	require.NoError(t, harness.DestroyTask(task.ID, true))

	stdout, err := os.ReadFile(task.StdoutPath)
	require.NoError(t, err)
	require.Equal(t, `mounted device /inserted-random: 1:8
reading from ro path: from-host
reading from rw path: from-host
overwriting file in rw succeeded
writing new file in rw succeeded`, strings.TrimSpace(string(stdout)))

	stderr, err := os.ReadFile(task.StderrPath)
	require.NoError(t, err)
	require.Equal(t, `touch: cannot touch '/tmp/task-path-ro/testfile': Read-only file system
touch: cannot touch '/tmp/task-path-ro/testfile-from-ro': Read-only file system`, strings.TrimSpace(string(stderr)))

	// testing exit code last so we can inspect output first
	require.Zero(t, result.ExitCode)

	fromRWContent, err := os.ReadFile(filepath.Join(tmpDir, "testfile-from-rw"))
	require.NoError(t, err)
	require.Equal(t, "from-exec", strings.TrimSpace(string(fromRWContent)))
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

func TestExecDriver_NoPivotRoot(t *testing.T) {
	ci.Parallel(t)
	ctestutils.ExecCompatible(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := NewExecDriver(ctx, testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	config := &Config{
		NoPivotRoot:    true,
		DefaultModePID: executor.IsolationModePrivate,
		DefaultModeIPC: executor.IsolationModePrivate,
	}

	var data []byte
	require.NoError(t, basePlug.MsgPackEncode(&data, config))
	bconfig := &basePlug.Config{PluginConfig: data}
	require.NoError(t, harness.SetConfig(bconfig))

	allocID := uuid.Generate()
	task := &drivers.TaskConfig{
		AllocID:   allocID,
		ID:        uuid.Generate(),
		Name:      "sleep",
		Resources: testResources(allocID, "sleep"),
	}
	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	tc := &TaskConfig{
		Command: "/bin/sleep",
		Args:    []string{"100"},
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&tc))

	handle, _, err := harness.StartTask(task)
	require.NoError(t, err)
	require.NotNil(t, handle)
	require.NoError(t, harness.DestroyTask(task.ID, true))
}

func TestDriver_Config_validate(t *testing.T) {
	ci.Parallel(t)
	t.Run("pid/ipc", func(t *testing.T) {
		for _, tc := range []struct {
			pidMode, ipcMode string
			exp              error
		}{
			{pidMode: "host", ipcMode: "host", exp: nil},
			{pidMode: "private", ipcMode: "host", exp: nil},
			{pidMode: "host", ipcMode: "private", exp: nil},
			{pidMode: "private", ipcMode: "private", exp: nil},
			{pidMode: "other", ipcMode: "private", exp: errors.New(`default_pid_mode must be "private" or "host", got "other"`)},
			{pidMode: "private", ipcMode: "other", exp: errors.New(`default_ipc_mode must be "private" or "host", got "other"`)},
		} {
			require.Equal(t, tc.exp, (&Config{
				DefaultModePID: tc.pidMode,
				DefaultModeIPC: tc.ipcMode,
			}).validate())
		}
	})

	t.Run("allow_caps", func(t *testing.T) {
		for _, tc := range []struct {
			ac  []string
			exp error
		}{
			{ac: []string{}, exp: nil},
			{ac: []string{"all"}, exp: nil},
			{ac: []string{"chown", "sys_time"}, exp: nil},
			{ac: []string{"CAP_CHOWN", "cap_sys_time"}, exp: nil},
			{ac: []string{"chown", "not_valid", "sys_time"}, exp: errors.New("allow_caps configured with capabilities not supported by system: not_valid")},
		} {
			require.Equal(t, tc.exp, (&Config{
				DefaultModePID: "private",
				DefaultModeIPC: "private",
				AllowCaps:      tc.ac,
			}).validate())
		}
	})
}

func TestDriver_TaskConfig_validate(t *testing.T) {
	ci.Parallel(t)
	t.Run("pid/ipc", func(t *testing.T) {
		for _, tc := range []struct {
			pidMode, ipcMode string
			exp              error
		}{
			{pidMode: "host", ipcMode: "host", exp: nil},
			{pidMode: "host", ipcMode: "private", exp: nil},
			{pidMode: "host", ipcMode: "", exp: nil},
			{pidMode: "host", ipcMode: "other", exp: errors.New(`ipc_mode must be "private" or "host", got "other"`)},

			{pidMode: "host", ipcMode: "host", exp: nil},
			{pidMode: "private", ipcMode: "host", exp: nil},
			{pidMode: "", ipcMode: "host", exp: nil},
			{pidMode: "other", ipcMode: "host", exp: errors.New(`pid_mode must be "private" or "host", got "other"`)},
		} {
			require.Equal(t, tc.exp, (&TaskConfig{
				ModePID: tc.pidMode,
				ModeIPC: tc.ipcMode,
			}).validate())
		}
	})

	t.Run("cap_add", func(t *testing.T) {
		for _, tc := range []struct {
			adds []string
			exp  error
		}{
			{adds: nil, exp: nil},
			{adds: []string{"chown"}, exp: nil},
			{adds: []string{"CAP_CHOWN"}, exp: nil},
			{adds: []string{"chown", "sys_time"}, exp: nil},
			{adds: []string{"chown", "not_valid", "sys_time"}, exp: errors.New("cap_add configured with capabilities not supported by system: not_valid")},
		} {
			require.Equal(t, tc.exp, (&TaskConfig{
				CapAdd: tc.adds,
			}).validate())
		}
	})

	t.Run("cap_drop", func(t *testing.T) {
		for _, tc := range []struct {
			drops []string
			exp   error
		}{
			{drops: nil, exp: nil},
			{drops: []string{"chown"}, exp: nil},
			{drops: []string{"CAP_CHOWN"}, exp: nil},
			{drops: []string{"chown", "sys_time"}, exp: nil},
			{drops: []string{"chown", "not_valid", "sys_time"}, exp: errors.New("cap_drop configured with capabilities not supported by system: not_valid")},
		} {
			require.Equal(t, tc.exp, (&TaskConfig{
				CapDrop: tc.drops,
			}).validate())
		}
	})
}
