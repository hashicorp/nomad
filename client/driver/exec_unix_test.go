// +build !windows

package driver

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"

	ctestutils "github.com/hashicorp/nomad/client/testutil"
)

func TestExecDriver_KillUserPid_OnPluginReconnectFailure(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.ExecCompatible(t)
	task := &structs.Task{
		Name:   "sleep",
		Driver: "exec",
		Config: map[string]interface{}{
			"command": "/bin/sleep",
			"args":    []string{"1000000"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewExecDriver(ctx.DriverCtx)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("prestart err: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer resp.Handle.Kill()

	id := &execId{}
	if err := json.Unmarshal([]byte(resp.Handle.ID()), id); err != nil {
		t.Fatalf("Failed to parse handle '%s': %v", resp.Handle.ID(), err)
	}
	pluginPid := id.PluginConfig.Pid
	proc, err := os.FindProcess(pluginPid)
	if err != nil {
		t.Fatalf("can't find plugin pid: %v", pluginPid)
	}
	if err := proc.Kill(); err != nil {
		t.Fatalf("can't kill plugin pid: %v", err)
	}

	// Attempt to open
	handle2, err := d.Open(ctx.ExecCtx, resp.Handle.ID())
	if err == nil {
		t.Fatalf("expected error")
	}
	if handle2 != nil {
		handle2.Kill()
		t.Fatalf("expected handle2 to be nil")
	}

	// Test if the userpid is still present
	userProc, _ := os.FindProcess(id.UserPid)

	for retry := 3; retry > 0; retry-- {
		if err = userProc.Signal(syscall.Signal(0)); err != nil {
			// Process is gone as expected; exit
			return
		}

		// Killing processes is async; wait and check again
		time.Sleep(time.Second)
	}
	if err = userProc.Signal(syscall.Signal(0)); err == nil {
		t.Fatalf("expected user process to die")
	}
}

func TestExecDriver_Signal(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.ExecCompatible(t)
	task := &structs.Task{
		Name:   "signal",
		Driver: "exec",
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
	d := NewExecDriver(ctx.DriverCtx)

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
