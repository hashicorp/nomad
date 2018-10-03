package rawexec

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	ctestutil "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/testtask"
	"github.com/hashicorp/nomad/helper/uuid"
	basePlug "github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if !testtask.Run() {
		os.Exit(m.Run())
	}
}

func TestRawExecDriver_SetConfig(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	d := NewRawExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)

	// Disable raw exec.
	config := &Config{}

	var data []byte
	require.NoError(basePlug.MsgPackEncode(&data, config))
	require.NoError(harness.SetConfig(data))
	require.Exactly(config, d.(*RawExecDriver).config)

	config.Enabled = true
	config.NoCgroups = true
	data = []byte{}
	require.NoError(basePlug.MsgPackEncode(&data, config))
	require.NoError(harness.SetConfig(data))
	require.Exactly(config, d.(*RawExecDriver).config)

	config.NoCgroups = false
	data = []byte{}
	require.NoError(basePlug.MsgPackEncode(&data, config))
	require.NoError(harness.SetConfig(data))
	require.Exactly(config, d.(*RawExecDriver).config)
}

func TestRawExecDriver_Fingerprint(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	d := NewRawExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)

	// Disable raw exec.
	config := &Config{}

	var data []byte
	require.NoError(basePlug.MsgPackEncode(&data, config))
	require.NoError(harness.SetConfig(data))

	fingerCh, err := harness.Fingerprint(context.Background())
	require.NoError(err)
	select {
	case finger := <-fingerCh:
		require.Equal(drivers.HealthStateUndetected, finger.Health)
		require.Empty(finger.Attributes["driver.raw_exec"])
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		require.Fail("timeout receiving fingerprint")
	}

	// Enable raw exec
	config.Enabled = true
	data = []byte{}
	require.NoError(basePlug.MsgPackEncode(&data, config))
	require.NoError(harness.SetConfig(data))

FINGER_LOOP:
	for {
		select {
		case finger := <-fingerCh:
			if finger.Health == drivers.HealthStateHealthy {
				break FINGER_LOOP
			}
		case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
			require.Fail("timeout receiving fingerprint")
			break FINGER_LOOP
		}
	}
}

func TestRawExecDriver_StartWait(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	d := NewRawExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)
	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "test",
	}
	task.EncodeDriverConfig(&TaskConfig{
		Command: "go",
		Args:    []string{"version"},
	})
	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	handle, err := harness.StartTask(task)
	require.NoError(err)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)
	result := <-ch
	require.Zero(result.ExitCode)
	require.NoError(harness.DestroyTask(task.ID, true))
}

func TestRawExecDriver_StartWaitStop(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	d := NewRawExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)
	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "test",
	}
	task.EncodeDriverConfig(&TaskConfig{
		Command: testtask.Path(),
		Args:    []string{"sleep", "100s"},
	})
	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	handle, err := harness.StartTask(task)
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

func TestRawExecDriver_StartWaitRecoverWaitStop(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	d := NewRawExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)
	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "sleep",
	}
	task.EncodeDriverConfig(&TaskConfig{
		Command: testtask.Path(),
		Args:    []string{"sleep", "100s"},
	})
	testtask.SetTaskConfigEnv(task)
	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	handle, err := harness.StartTask(task)
	require.NoError(err)

	ch, err := harness.WaitTask(context.Background(), task.ID)
	require.NoError(err)

	var waitDone bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result := <-ch
		require.Error(result.Err)
		waitDone = true
	}()

	originalStatus, err := d.InspectTask(task.ID)
	require.NoError(err)

	d.(*RawExecDriver).tasks.Delete(task.ID)

	wg.Wait()
	require.True(waitDone)
	_, err = d.InspectTask(task.ID)
	require.Equal(drivers.ErrTaskNotFound, err)

	err = d.RecoverTask(handle)
	require.NoError(err)

	status, err := d.InspectTask(task.ID)
	require.NoError(err)
	require.Exactly(originalStatus, status)

	ch, err = harness.WaitTask(context.Background(), task.ID)
	require.NoError(err)

	wg.Add(1)
	waitDone = false
	go func() {
		defer wg.Done()
		result := <-ch
		require.NoError(result.Err)
		require.NotZero(result.ExitCode)
		require.Equal(9, result.Signal)
		waitDone = true
	}()

	time.Sleep(300 * time.Millisecond)
	require.NoError(d.StopTask(task.ID, 0, "SIGKILL"))
	wg.Wait()
	require.NoError(d.DestroyTask(task.ID, false))
	require.True(waitDone)

}

func TestRawExecDriver_Start_Wait_AllocDir(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	d := NewRawExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "sleep",
	}

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	exp := []byte("win")
	file := "output.txt"
	outPath := fmt.Sprintf(`%s/%s`, task.TaskDir().SharedAllocDir, file)
	task.EncodeDriverConfig(&TaskConfig{
		Command: testtask.Path(),
		Args: []string{
			"sleep", "1s", "write",
			string(exp), outPath,
		},
	})

	testtask.SetTaskConfigEnv(task)

	_, err := harness.StartTask(task)
	require.NoError(err)

	// Task should terminate quickly
	waitCh, err := harness.WaitTask(context.Background(), task.ID)
	select {
	case res := <-waitCh:
		require.NoError(res.Err)
		require.True(res.Successful())
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		require.Fail("WaitTask timeout")
	}

	// Check that data was written to the shared alloc directory.
	outputFile := filepath.Join(task.TaskDir().SharedAllocDir, file)
	act, err := ioutil.ReadFile(outputFile)
	require.NoError(err)
	require.Exactly(exp, act)
	require.NoError(harness.DestroyTask(task.ID, true))
}

// This test creates a process tree such that without cgroups tracking the
// processes cleanup of the children would not be possible. Thus the test
// asserts that the processes get killed properly when using cgroups.
func TestRawExecDriver_Start_Kill_Wait_Cgroup(t *testing.T) {
	ctestutil.ExecCompatible(t)
	t.Parallel()
	require := require.New(t)
	pidFile := "pid"

	d := NewRawExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "sleep",
		User: "root",
	}

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	task.EncodeDriverConfig(&TaskConfig{
		Command: testtask.Path(),
		Args:    []string{"fork/exec", pidFile, "pgrp", "0", "sleep", "20s"},
	})

	testtask.SetTaskConfigEnv(task)

	_, err := harness.StartTask(task)
	require.NoError(err)

	// Find the process
	var pidData []byte
	testutil.WaitForResult(func() (bool, error) {
		var err error
		pidData, err = ioutil.ReadFile(filepath.Join(task.TaskDir().Dir, pidFile))
		if err != nil {
			return false, err
		}

		if len(pidData) == 0 {
			return false, fmt.Errorf("pidFile empty")
		}

		return true, nil
	}, func(err error) {
		require.NoError(err)
	})

	pid, err := strconv.Atoi(string(pidData))
	require.NoError(err, "failed to read pidData: %s", string(pidData))

	// Check the pid is up
	process, err := os.FindProcess(pid)
	require.NoError(err)
	require.NoError(process.Signal(syscall.Signal(0)))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(1 * time.Second)
		err := harness.StopTask(task.ID, 0, "")

		// Can't rely on the ordering between wait and kill on travis...
		if !testutil.IsTravis() {
			require.NoError(err)
		}
	}()

	// Task should terminate quickly
	waitCh, err := harness.WaitTask(context.Background(), task.ID)
	require.NoError(err)
	select {
	case res := <-waitCh:
		require.False(res.Successful())
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		require.Fail("WaitTask timeout")
	}

	testutil.WaitForResult(func() (bool, error) {
		if err := process.Signal(syscall.Signal(0)); err == nil {
			return false, fmt.Errorf("process should not exist: %v", pid)
		}

		return true, nil
	}, func(err error) {
		require.NoError(err)
	})

	wg.Wait()
	require.NoError(harness.DestroyTask(task.ID, true))
}

func TestRawExecDriver_Exec(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	d := NewRawExecDriver(testlog.HCLogger(t))
	harness := drivers.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "sleep",
	}

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	task.EncodeDriverConfig(&TaskConfig{
		Command: testtask.Path(),
		Args:    []string{"sleep", "9000s"},
	})

	testtask.SetTaskConfigEnv(task)

	_, err := harness.StartTask(task)
	require.NoError(err)

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

	require.NoError(harness.DestroyTask(task.ID, true))
}
