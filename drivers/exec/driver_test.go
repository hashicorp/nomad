package exec

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/hcl2/hcl"
	ctestutils "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/testtask"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/shared"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if !testtask.Run() {
		os.Exit(m.Run())
	}
}

func TestExecDriver_Fingerprint_NonLinux(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	require := require.New(t)
	if runtime.GOOS == "linux" {
		t.Skip("Test only available not on Linux")
	}

	d := NewExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)

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
	t.Parallel()
	require := require.New(t)

	ctestutils.ExecCompatible(t)

	d := NewExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)

	fingerCh, err := harness.Fingerprint(context.Background())
	require.NoError(err)
	select {
	case finger := <-fingerCh:
		require.Equal(drivers.HealthStateHealthy, finger.Health)
		require.Equal("1", finger.Attributes["driver.exec"])
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		require.Fail("timeout receiving fingerprint")
	}
}

func TestExecDriver_StartWait(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctestutils.ExecCompatible(t)

	d := NewExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)
	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "test",
	}

	taskConfig := map[string]interface{}{
		"command": "cat",
		"args":    []string{"/proc/self/cgroup"},
	}
	encodeDriverHelper(require, task, taskConfig)

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()
	fmt.Println(task.AllocDir)

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)
	result := <-ch
	require.Zero(result.ExitCode)
	require.NoError(harness.DestroyTask(task.ID, true))
}

func TestExecDriver_StartWaitStop(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctestutils.ExecCompatible(t)

	d := NewExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)
	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "test",
	}

	taskConfig := map[string]interface{}{
		"command": "/bin/sleep",
		"args":    []string{"5"},
	}
	encodeDriverHelper(require, task, taskConfig)

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result := <-ch
		require.Equal(2, result.Signal)
	}()

	require.NoError(harness.WaitUntilStarted(task.ID, 1*time.Second))

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := harness.StopTask(task.ID, 2*time.Second, "SIGINT")
		require.NoError(err)
	}()

	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		wg.Wait()
	}()

	select {
	case <-waitCh:
		status, err := harness.InspectTask(task.ID)
		require.NoError(err)
		require.Equal(drivers.TaskStateExited, status.State)
	case <-time.After(1 * time.Second):
		require.Fail("timeout waiting for task to shutdown")
	}

	require.NoError(harness.DestroyTask(task.ID, true))
}

func TestExecDriver_StartWaitRecover(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctestutils.ExecCompatible(t)

	d := NewExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)
	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "test",
	}

	taskConfig := map[string]interface{}{
		"command": "/bin/sleep",
		"args":    []string{"5"},
	}
	encodeDriverHelper(require, task, taskConfig)

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := harness.WaitTask(ctx, handle.Config.ID)
	require.NoError(err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result := <-ch
		require.Error(result.Err)
	}()

	require.NoError(harness.WaitUntilStarted(task.ID, 1*time.Second))
	cancel()

	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		wg.Wait()
	}()

	select {
	case <-waitCh:
		status, err := harness.InspectTask(task.ID)
		require.NoError(err)
		require.Equal(drivers.TaskStateRunning, status.State)
	case <-time.After(1 * time.Second):
		require.Fail("timeout waiting for task wait to cancel")
	}

	// Loose task
	d.(*Driver).tasks.Delete(task.ID)
	_, err = harness.InspectTask(task.ID)
	require.Error(err)

	require.NoError(harness.RecoverTask(handle))
	status, err := harness.InspectTask(task.ID)
	require.NoError(err)
	require.Equal(drivers.TaskStateRunning, status.State)

	require.NoError(harness.StopTask(task.ID, 0, ""))
	require.NoError(harness.DestroyTask(task.ID, true))
}

func TestExecDriver_Stats(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctestutils.ExecCompatible(t)

	d := NewExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)
	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "test",
	}

	taskConfig := map[string]interface{}{
		"command": "/bin/sleep",
		"args":    []string{"5"},
	}
	encodeDriverHelper(require, task, taskConfig)

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	require.NoError(err)
	require.NotNil(handle)

	require.NoError(harness.WaitUntilStarted(task.ID, 1*time.Second))
	stats, err := harness.TaskStats(task.ID)
	require.NoError(err)
	require.NotZero(stats.ResourceUsage.MemoryStats.RSS)

	require.NoError(harness.DestroyTask(task.ID, true))
}

func TestExecDriver_Start_Wait_AllocDir(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctestutils.ExecCompatible(t)

	d := NewExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)
	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "sleep",
	}
	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	exp := []byte{'w', 'i', 'n'}
	file := "output.txt"
	taskConfig := map[string]interface{}{
		"command": "/bin/bash",
		"args": []string{
			"-c",
			fmt.Sprintf(`sleep 1; echo -n %s > /alloc/%s`, string(exp), file),
		},
	}
	encodeDriverHelper(require, task, taskConfig)

	handle, _, err := harness.StartTask(task)
	require.NoError(err)
	require.NotNil(handle)

	// Task should terminate quickly
	waitCh, err := harness.WaitTask(context.Background(), task.ID)
	require.NoError(err)
	select {
	case res := <-waitCh:
		require.True(res.Successful(), "task should have exited successfully: %v", res)
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		require.Fail("timeout waiting for task")
	}

	// Check that data was written to the shared alloc directory.
	outputFile := filepath.Join(task.TaskDir().SharedAllocDir, file)
	act, err := ioutil.ReadFile(outputFile)
	require.NoError(err)
	require.Exactly(exp, act)

	require.NoError(harness.DestroyTask(task.ID, true))
}

func TestExecDriver_User(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctestutils.ExecCompatible(t)

	d := NewExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)
	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "sleep",
		User: "alice",
	}
	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	taskConfig := map[string]interface{}{
		"command": "/bin/sleep",
		"args":    []string{"100"},
	}
	encodeDriverHelper(require, task, taskConfig)

	handle, _, err := harness.StartTask(task)
	require.Error(err)
	require.Nil(handle)

	msg := "user alice"
	if !strings.Contains(err.Error(), msg) {
		t.Fatalf("Expecting '%v' in '%v'", msg, err)
	}
}

// TestExecDriver_HandlerExec ensures the exec driver's handle properly
// executes commands inside the container.
func TestExecDriver_HandlerExec(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	ctestutils.ExecCompatible(t)

	d := NewExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)
	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "sleep",
	}
	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	taskConfig := map[string]interface{}{
		"command": "/bin/sleep",
		"args":    []string{"9000"},
	}
	encodeDriverHelper(require, task, taskConfig)

	handle, _, err := harness.StartTask(task)
	require.NoError(err)
	require.NotNil(handle)

	// Exec a command that should work and dump the environment
	// TODO: enable section when exec env is fully loaded
	/*res, err := harness.ExecTask(task.ID, []string{"/bin/sh", "-c", "env | grep ^NOMAD"}, time.Second)
	require.NoError(err)
	require.True(res.ExitResult.Successful())

	// Assert exec'd commands are run in a task-like environment
	scriptEnv := make(map[string]string)
	for _, line := range strings.Split(string(res.Stdout), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(string(line), "=", 2)
		if len(parts) != 2 {
			t.Fatalf("Invalid env var: %q", line)
		}
		scriptEnv[parts[0]] = parts[1]
	}
	if v, ok := scriptEnv["NOMAD_SECRETS_DIR"]; !ok || v != "/secrets" {
		t.Errorf("Expected NOMAD_SECRETS_DIR=/secrets but found=%t value=%q", ok, v)
	}*/

	// Assert cgroup membership
	res, err := harness.ExecTask(task.ID, []string{"/bin/cat", "/proc/self/cgroup"}, time.Second)
	require.NoError(err)
	require.True(res.ExitResult.Successful())
	found := false
	for _, line := range strings.Split(string(res.Stdout), "\n") {
		// Every cgroup entry should be /nomad/$ALLOC_ID
		if line == "" {
			continue
		}
		// Skip systemd cgroup
		if strings.HasPrefix(line, "1:name=systemd") {
			continue
		}
		if !strings.Contains(line, ":/nomad/") {
			t.Errorf("Not a member of the alloc's cgroup: expected=...:/nomad/... -- found=%q", line)
			continue
		}
		found = true
	}
	require.True(found, "exec'd command isn't in the task's cgroup")

	// Exec a command that should fail
	res, err = harness.ExecTask(task.ID, []string{"/usr/bin/stat", "lkjhdsaflkjshowaisxmcvnlia"}, time.Second)
	require.NoError(err)
	require.False(res.ExitResult.Successful())
	if expected := "No such file or directory"; !bytes.Contains(res.Stdout, []byte(expected)) {
		t.Fatalf("expected output to contain %q but found: %q", expected, res.Stdout)
	}

	require.NoError(harness.DestroyTask(task.ID, true))
}

func encodeDriverHelper(require *require.Assertions, task *drivers.TaskConfig, taskConfig map[string]interface{}) {
	evalCtx := &hcl.EvalContext{
		Functions: shared.GetStdlibFuncs(),
	}
	spec, diag := hclspec.Convert(taskConfigSpec)
	require.False(diag.HasErrors())
	taskConfigCtyVal, diag := shared.ParseHclInterface(taskConfig, spec, evalCtx)
	require.False(diag.HasErrors())
	err := task.EncodeDriverConfig(taskConfigCtyVal)
	require.Nil(err)
}
