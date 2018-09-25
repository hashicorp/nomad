package raw_exec

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/drivers/base"
	"github.com/stretchr/testify/require"
)

func TestRawExecDriver_StartWait(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	d := NewRawExecDriver(testlog.HCLogger(t))
	harness := base.NewDriverHarness(t, d)
	task := &base.TaskConfig{
		ID:   uuid.Generate(),
		Name: "test",
	}
	task.EncodeDriverConfig(&TaskConfig{
		Command: "go",
		Args:    []string{"version"},
	})
	cleanup := harness.MkAllocDir(task)
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
	harness := base.NewDriverHarness(t, d)
	task := &base.TaskConfig{
		ID:   uuid.Generate(),
		Name: "test",
	}
	task.EncodeDriverConfig(&TaskConfig{
		Command: "/bin/bash",
		Args:    []string{"test.sh"},
	})
	cleanup := harness.MkAllocDir(task)
	defer cleanup()

	testFile := filepath.Join(task.TaskDir().Dir, "test.sh")
	testData := []byte(`
at_term() {
    echo 'Terminated.'
    exit 3
}
trap at_term USR1
while true; do
    sleep 1
done
	`)
	require.NoError(ioutil.WriteFile(testFile, testData, 0777))

	handle, err := harness.StartTask(task)
	require.NoError(err)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)

	var waitDone bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result := <-ch
		require.Equal(3, result.ExitCode)
		waitDone = true
	}()

	time.Sleep(100 * time.Millisecond)

	var stopDone bool
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := harness.StopTask(task.ID, 1*time.Second, "SIGUSR1")
		require.NoError(err)
		stopDone = true
	}()

	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		wg.Wait()
	}()

	time.Sleep(100 * time.Millisecond)

	status, err := harness.InspectTask(task.ID)
	require.NoError(err)
	require.Equal(base.TaskStateRunning, status.State)
	require.False(waitDone)
	require.False(stopDone)

	select {
	case <-waitCh:
		status, err := harness.InspectTask(task.ID)
		require.NoError(err)
		require.Equal(base.TaskStateExited, status.State)
	case <-time.After(1 * time.Second):
		require.Fail("timeout waiting for task to shutdown")
	}

	require.NoError(harness.DestroyTask(task.ID, true))
}

func TestRawExecDriver_StartWaitRecoverWaitStop(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	d := NewRawExecDriver(testlog.HCLogger(t))
	harness := base.NewDriverHarness(t, d)
	task := &base.TaskConfig{
		ID:   uuid.Generate(),
		Name: "test",
	}
	task.EncodeDriverConfig(&TaskConfig{
		Command: "sleep",
		Args:    []string{"1000"},
	})
	cleanup := harness.MkAllocDir(task)
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
	require.Equal(base.ErrTaskNotFound, err)

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
		require.Equal(9, result.Signal)
		waitDone = true
	}()

	require.NoError(d.StopTask(task.ID, 0, ""))
	require.NoError(d.DestroyTask(task.ID, false))
	wg.Wait()
	require.True(waitDone)

}
