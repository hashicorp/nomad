// +build !windows

package docker

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/testutil"
	tu "github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestDockerDriver_Signal(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task, cfg, _ := dockerTask(t)
	cfg.Command = "/bin/sh"
	cfg.Args = []string{"local/test.sh"}
	require.NoError(t, task.EncodeConcreteDriverConfig(cfg))

	driver := dockerDriverHarness(t, nil)
	cleanup := driver.MkAllocDir(task, true)
	defer cleanup()

	// Copy the image into the task's directory
	copyImage(t, task.TaskDir(), "busybox.tar")

	testFile := filepath.Join(task.TaskDir().LocalDir, "test.sh")
	testData := []byte(`
at_term() {
    echo 'Terminated.' > $NOMAD_TASK_DIR/output
    exit 3
}
trap at_term INT
while true; do
    echo 'sleeping'
    sleep 0.2
done
	`)
	require.NoError(t, ioutil.WriteFile(testFile, testData, 0777))
	_, _, err := driver.StartTask(task)
	require.NoError(t, err)
	defer driver.DestroyTask(task.ID, true)
	require.NoError(t, driver.WaitUntilStarted(task.ID, time.Duration(tu.TestMultiplier()*5)*time.Second))
	handle, ok := driver.Impl().(*Driver).tasks.Get(task.ID)
	require.True(t, ok)

	waitForExist(t, newTestDockerClient(t), handle.container.ID)
	require.NoError(t, handle.Kill(time.Duration(tu.TestMultiplier()*5)*time.Second, os.Interrupt))

	waitCh, err := driver.WaitTask(context.Background(), task.ID)
	require.NoError(t, err)
	select {
	case res := <-waitCh:
		if res.Successful() {
			require.Fail(t, "should err: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		require.Fail(t, "timeout")
	}

	// Check the log file to see it exited because of the signal
	outputFile := filepath.Join(task.TaskDir().LocalDir, "output")
	act, err := ioutil.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Couldn't read expected output: %v", err)
	}

	exp := "Terminated."
	if strings.TrimSpace(string(act)) != exp {
		t.Fatalf("Command outputted %v; want %v", act, exp)
	}
}

func TestDockerDriver_containerBinds(t *testing.T) {
	task, cfg, _ := dockerTask(t)
	driver := dockerDriverHarness(t, nil)
	cleanup := driver.MkAllocDir(task, false)
	defer cleanup()

	binds, err := driver.Impl().(*Driver).containerBinds(task, cfg)
	require.NoError(t, err)
	require.Contains(t, binds, fmt.Sprintf("%s:/alloc", task.TaskDir().SharedAllocDir))
	require.Contains(t, binds, fmt.Sprintf("%s:/local", task.TaskDir().LocalDir))
	require.Contains(t, binds, fmt.Sprintf("%s:/secrets", task.TaskDir().SecretsDir))
}
