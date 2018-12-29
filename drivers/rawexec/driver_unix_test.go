// +build !windows

package rawexec

import (
	"context"
	"runtime"
	"testing"

	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/testtask"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/drivers"
	dtestutil "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestRawExecDriver_User(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("Linux only test")
	}
	require := require.New(t)

	d := NewRawExecDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "sleep",
		User: "alice",
	}

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	taskConfig := map[string]interface{}{}
	taskConfig["command"] = testtask.Path()
	taskConfig["args"] = []string{"sleep", "45s"}

	encodeDriverHelper(require, task, taskConfig)
	testtask.SetTaskConfigEnv(task)

	_, _, err := harness.StartTask(task)
	require.Error(err)
	msg := "unknown user alice"
	require.Contains(err.Error(), msg)
}

func TestRawExecDriver_Signal(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("Linux only test")
	}
	require := require.New(t)

	d := NewRawExecDriver(testlog.HCLogger(t))
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "signal",
	}

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	taskConfig := map[string]interface{}{}
	taskConfig["command"] = "/bin/bash"
	taskConfig["args"] = []string{"test.sh"}

	encodeDriverHelper(require, task, taskConfig)
	testtask.SetTaskConfigEnv(task)

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

	testtask.SetTaskConfigEnv(task)
	_, _, err := harness.StartTask(task)
	require.NoError(err)

	go func() {
		time.Sleep(100 * time.Millisecond)
		require.NoError(harness.SignalTask(task.ID, "SIGUSR1"))
	}()

	// Task should terminate quickly
	waitCh, err := harness.WaitTask(context.Background(), task.ID)
	require.NoError(err)
	select {
	case res := <-waitCh:
		require.False(res.Successful())
		require.Equal(3, res.ExitCode)
	case <-time.After(time.Duration(testutil.TestMultiplier()*6) * time.Second):
		require.Fail("WaitTask timeout")
	}

	// Check the log file to see it exited because of the signal
	outputFile := filepath.Join(task.TaskDir().LogDir, "signal.stdout.0")
	exp := "Terminated."
	testutil.WaitForResult(func() (bool, error) {
		act, err := ioutil.ReadFile(outputFile)
		if err != nil {
			return false, fmt.Errorf("Couldn't read expected output: %v", err)
		}

		if strings.TrimSpace(string(act)) != exp {
			t.Logf("Read from %v", outputFile)
			return false, fmt.Errorf("Command outputted %v; want %v", act, exp)
		}
		return true, nil
	}, func(err error) { require.NoError(err) })
}
