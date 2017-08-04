// +build !windows

package driver

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestRawExecDriver_Signal(t *testing.T) {
	t.Parallel()
	task := &structs.Task{
		Name:   "signal",
		Driver: "raw_exec",
		Config: map[string]interface{}{
			"command": "/bin/bash",
			"args":    []string{"test.sh"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources:   basicResources,
		KillTimeout: 10 * time.Second,
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRawExecDriver(ctx.DriverCtx)

	testFile := filepath.Join(ctx.ExecCtx.TaskDir.Dir, "test.sh")
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
	if err := ioutil.WriteFile(testFile, testData, 0777); err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("prestart err: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		err := resp.Handle.Signal(syscall.SIGUSR1)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	// Task should terminate quickly
	select {
	case res := <-resp.Handle.WaitCh():
		if res.Successful() {
			t.Fatal("should err")
		}
	case <-time.After(time.Duration(testutil.TestMultiplier()*6) * time.Second):
		t.Fatalf("timeout")
	}

	// Check the log file to see it exited because of the signal
	outputFile := filepath.Join(ctx.ExecCtx.TaskDir.LogDir, "signal.stdout.0")
	act, err := ioutil.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Couldn't read expected output: %v", err)
	}

	exp := "Terminated."
	if strings.TrimSpace(string(act)) != exp {
		t.Logf("Read from %v", outputFile)
		t.Fatalf("Command outputted %v; want %v", act, exp)
	}
}
