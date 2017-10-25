// +build !windows

package driver

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/nomad/structs"
	tu "github.com/hashicorp/nomad/testutil"
)

func TestDockerDriver_Signal(t *testing.T) {
	if !tu.IsTravis() {
		t.Parallel()
	}
	if !testutil.DockerIsConnected(t) {
		t.Skip("Docker not connected")
	}

	task := &structs.Task{
		Name:   "redis-demo",
		Driver: "docker",
		Config: map[string]interface{}{
			"image":   "busybox",
			"load":    "busybox.tar",
			"command": "/bin/sh",
			"args":    []string{"local/test.sh"},
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
	}

	ctx := testDockerDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewDockerDriver(ctx.DriverCtx)

	// Copy the image into the task's directory
	copyImage(t, ctx.ExecCtx.TaskDir, "busybox.tar")

	testFile := filepath.Join(ctx.ExecCtx.TaskDir.LocalDir, "test.sh")
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
	if err := ioutil.WriteFile(testFile, testData, 0777); err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	_, err := d.Prestart(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("error in prestart: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer resp.Handle.Kill()

	waitForExist(t, resp.Handle.(*DockerHandle).client, resp.Handle.(*DockerHandle))

	time.Sleep(1 * time.Second)
	if err := resp.Handle.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("Signal returned an error: %v", err)
	}

	select {
	case res := <-resp.Handle.WaitCh():
		if res.Successful() {
			t.Fatalf("should err: %v", res)
		}
	case <-time.After(time.Duration(tu.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}

	// Check the log file to see it exited because of the signal
	outputFile := filepath.Join(ctx.ExecCtx.TaskDir.LocalDir, "output")
	act, err := ioutil.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Couldn't read expected output: %v", err)
	}

	exp := "Terminated."
	if strings.TrimSpace(string(act)) != exp {
		t.Fatalf("Command outputted %v; want %v", act, exp)
	}
}
