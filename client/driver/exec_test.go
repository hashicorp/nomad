package driver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"

	ctestutils "github.com/hashicorp/nomad/client/testutil"
)

func TestExecDriver_Fingerprint(t *testing.T) {
	ctestutils.ExecCompatible(t)
	task := &structs.Task{
		Name:      "foo",
		Resources: structs.DefaultResources(),
	}
	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewExecDriver(driverCtx)
	node := &structs.Node{
		Attributes: map[string]string{
			"unique.cgroup.mountpoint": "/sys/fs/cgroup",
		},
	}
	apply, err := d.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !apply {
		t.Fatalf("should apply")
	}
	if node.Attributes["driver.exec"] == "" {
		t.Fatalf("missing driver")
	}
}

func TestExecDriver_StartOpen_Wait(t *testing.T) {
	ctestutils.ExecCompatible(t)
	task := &structs.Task{
		Name: "sleep",
		Config: map[string]interface{}{
			"command": "/bin/sleep",
			"args":    []string{"5"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewExecDriver(driverCtx)

	handle, err := d.Start(execCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}

	// Attempt to open
	handle2, err := d.Open(execCtx, handle.ID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle2 == nil {
		t.Fatalf("missing handle")
	}

	handle.Kill()
	handle2.Kill()
}

func TestExecDriver_KillUserPid_OnPluginReconnectFailure(t *testing.T) {
	ctestutils.ExecCompatible(t)
	task := &structs.Task{
		Name: "sleep",
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

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewExecDriver(driverCtx)

	handle, err := d.Start(execCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}
	defer handle.Kill()

	id := &execId{}
	if err := json.Unmarshal([]byte(handle.ID()), id); err != nil {
		t.Fatalf("Failed to parse handle '%s': %v", handle.ID(), err)
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
	handle2, err := d.Open(execCtx, handle.ID())
	if err == nil {
		t.Fatalf("expected error")
	}
	if handle2 != nil {
		handle2.Kill()
		t.Fatalf("expected handle2 to be nil")
	}
	// Test if the userpid is still present
	userProc, err := os.FindProcess(id.UserPid)

	err = userProc.Signal(syscall.Signal(0))

	if err == nil {
		t.Fatalf("expected user process to die")
	}
}

func TestExecDriver_Start_Wait(t *testing.T) {
	ctestutils.ExecCompatible(t)
	task := &structs.Task{
		Name: "sleep",
		Config: map[string]interface{}{
			"command": "/bin/sleep",
			"args":    []string{"2"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewExecDriver(driverCtx)

	handle, err := d.Start(execCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}

	// Update should be a no-op
	err = handle.Update(task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Task should terminate quickly
	select {
	case res := <-handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}
}

func TestExecDriver_Start_Wait_AllocDir(t *testing.T) {
	ctestutils.ExecCompatible(t)

	exp := []byte{'w', 'i', 'n'}
	file := "output.txt"
	task := &structs.Task{
		Name: "sleep",
		Config: map[string]interface{}{
			"command": "/bin/bash",
			"args": []string{
				"-c",
				fmt.Sprintf(`sleep 1; echo -n %s > ${%s}/%s`, string(exp), env.AllocDir, file),
			},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewExecDriver(driverCtx)

	handle, err := d.Start(execCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}

	// Task should terminate quickly
	select {
	case res := <-handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}

	// Check that data was written to the shared alloc directory.
	outputFile := filepath.Join(execCtx.AllocDir.SharedDir, file)
	act, err := ioutil.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Couldn't read expected output: %v", err)
	}

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("Command outputted %v; want %v", act, exp)
	}
}

func TestExecDriver_Start_Kill_Wait(t *testing.T) {
	ctestutils.ExecCompatible(t)
	task := &structs.Task{
		Name: "sleep",
		Config: map[string]interface{}{
			"command": "/bin/sleep",
			"args":    []string{"100"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources:   basicResources,
		KillTimeout: 10 * time.Second,
	}

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewExecDriver(driverCtx)

	handle, err := d.Start(execCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		err := handle.Kill()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	// Task should terminate quickly
	select {
	case res := <-handle.WaitCh():
		if res.Successful() {
			t.Fatal("should err")
		}
	case <-time.After(time.Duration(testutil.TestMultiplier()*10) * time.Second):
		t.Fatalf("timeout")
	}
}

func TestExecDriverUser(t *testing.T) {
	ctestutils.ExecCompatible(t)
	task := &structs.Task{
		Name: "sleep",
		User: "alice",
		Config: map[string]interface{}{
			"command": "/bin/sleep",
			"args":    []string{"100"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources:   basicResources,
		KillTimeout: 10 * time.Second,
	}

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewExecDriver(driverCtx)

	handle, err := d.Start(execCtx, task)
	if err == nil {
		handle.Kill()
		t.Fatalf("Should've failed")
	}
	msg := "user alice"
	if !strings.Contains(err.Error(), msg) {
		t.Fatalf("Expecting '%v' in '%v'", msg, err)
	}
}
