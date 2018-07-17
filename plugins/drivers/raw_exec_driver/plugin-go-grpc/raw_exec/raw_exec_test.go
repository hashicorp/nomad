package raw_exec

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/env"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testtask"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

func TestRawExecDriver_Fingerprint(t *testing.T) {
	t.Parallel()
	task := &structs.Task{
		Name:      "foo",
		Driver:    "raw_exec",
		Resources: structs.DefaultResources(),
	}
	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRawExecDriver(ctx.DriverCtx)
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	// Disable raw exec.
	cfg := &config.Config{Options: map[string]string{rawExecEnableOption: "false"}}

	request := &cstructs.FingerprintRequest{Config: cfg, Node: node}
	var response cstructs.FingerprintResponse
	err := d.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if response.Attributes["driver.raw_exec"] != "" {
		t.Fatalf("driver incorrectly enabled")
	}

	// Enable raw exec.
	request.Config.Options[rawExecEnableOption] = "true"
	err = d.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	if response.Attributes["driver.raw_exec"] != "1" {
		t.Fatalf("driver not enabled")
	}
}

func TestRawExecDriver_StartOpen_Wait(t *testing.T) {
	t.Parallel()
	task := &structs.Task{
		Name:   "sleep",
		Driver: "raw_exec",
		Config: map[string]interface{}{
			"command": testtask.Path(),
			"args":    []string{"sleep", "1s"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}
	testtask.SetTaskEnv(task)
	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRawExecDriver(ctx.DriverCtx)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("prestart err: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Attempt to open
	handle2, err := d.Open(ctx.ExecCtx, resp.Handle.ID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle2 == nil {
		t.Fatalf("missing handle")
	}

	// Task should terminate quickly
	select {
	case <-handle2.WaitCh():
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}
	resp.Handle.Kill()
	handle2.Kill()
}

func TestRawExecDriver_Start_Wait(t *testing.T) {
	t.Parallel()
	task := &structs.Task{
		Name:   "sleep",
		Driver: "raw_exec",
		Config: map[string]interface{}{
			"command": testtask.Path(),
			"args":    []string{"sleep", "1s"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}
	testtask.SetTaskEnv(task)
	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRawExecDriver(ctx.DriverCtx)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("prestart err: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update should be a no-op
	err = resp.Handle.Update(task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Task should terminate quickly
	select {
	case res := <-resp.Handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}
}

func TestRawExecDriver_Start_Wait_AllocDir(t *testing.T) {
	t.Parallel()
	exp := []byte("win")
	file := "output.txt"
	outPath := fmt.Sprintf(`${%s}/%s`, env.AllocDir, file)
	task := &structs.Task{
		Name:   "sleep",
		Driver: "raw_exec",
		Config: map[string]interface{}{
			"command": testtask.Path(),
			"args": []string{
				"sleep", "1s",
				"write", string(exp), outPath,
			},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}
	testtask.SetTaskEnv(task)

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRawExecDriver(ctx.DriverCtx)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("prestart err: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Task should terminate quickly
	select {
	case res := <-resp.Handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}

	// Check that data was written to the shared alloc directory.
	outputFile := filepath.Join(ctx.AllocDir.SharedDir, file)
	act, err := ioutil.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Couldn't read expected output: %v", err)
	}

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("Command outputted %v; want %v", act, exp)
	}
}

func TestRawExecDriver_Start_Kill_Wait(t *testing.T) {
	t.Parallel()
	task := &structs.Task{
		Name:   "sleep",
		Driver: "raw_exec",
		Config: map[string]interface{}{
			"command": testtask.Path(),
			"args":    []string{"sleep", "45s"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}
	testtask.SetTaskEnv(task)

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRawExecDriver(ctx.DriverCtx)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("prestart err: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	go func() {
		time.Sleep(1 * time.Second)
		err := resp.Handle.Kill()

		// Can't rely on the ordering between wait and kill on travis...
		if !testutil.IsTravis() && err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	// Task should terminate quickly
	select {
	case res := <-resp.Handle.WaitCh():
		if res.Successful() {
			t.Fatal("should err")
		}
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}
}

//// This test creates a process tree such that without cgroups tracking the
//// processes cleanup of the children would not be possible. Thus the test
//// asserts that the processes get killed properly when using cgroups.
//func TestRawExecDriver_Start_Kill_Wait_Cgroup(t *testing.T) {
//	tu.ExecCompatible(t)
//	t.Parallel()
//	pidFile := "pid"
//	task := &structs.Task{
//		Name:   "sleep",
//		Driver: "raw_exec",
//		Config: map[string]interface{}{
//			"command": testtask.Path(),
//			"args":    []string{"fork/exec", pidFile, "pgrp", "0", "sleep", "20s"},
//		},
//		LogConfig: &structs.LogConfig{
//			MaxFiles:      10,
//			MaxFileSizeMB: 10,
//		},
//		Resources: basicResources,
//		User:      "root",
//	}
//	testtask.SetTaskEnv(task)
//
//	ctx := testDriverContexts(t, task)
//	ctx.DriverCtx.node.Attributes["unique.cgroup.mountpoint"] = "foo" // Enable cgroups
//	defer ctx.AllocDir.Destroy()
//	d := NewRawExecDriver(ctx.DriverCtx)
//
//	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
//		t.Fatalf("prestart err: %v", err)
//	}
//	resp, err := d.Start(ctx.ExecCtx, task)
//	if err != nil {
//		t.Fatalf("err: %v", err)
//	}
//
//	// Find the process
//	var pidData []byte
//	testutil.WaitForResult(func() (bool, error) {
//		var err error
//		pidData, err = ioutil.ReadFile(filepath.Join(ctx.AllocDir.AllocDir, "sleep", pidFile))
//		if err != nil {
//			return false, err
//		}
//
//		return true, nil
//	}, func(err error) {
//		t.Fatalf("err: %v", err)
//	})
//
//	pid, err := strconv.Atoi(string(pidData))
//	if err != nil {
//		t.Fatalf("failed to convert pid: %v", err)
//	}
//
//	// Check the pid is up
//	process, err := os.FindProcess(pid)
//	if err != nil {
//		t.Fatalf("failed to find process")
//	}
//	if err := process.Signal(syscall.Signal(0)); err != nil {
//		t.Fatalf("process doesn't exist: %v", err)
//	}
//
//	go func() {
//		time.Sleep(1 * time.Second)
//		err := resp.Handle.Kill()
//
//		// Can't rely on the ordering between wait and kill on travis...
//		if !testutil.IsTravis() && err != nil {
//			t.Fatalf("err: %v", err)
//		}
//	}()
//
//	// Task should terminate quickly
//	select {
//	case res := <-resp.Handle.WaitCh():
//		if res.Successful() {
//			t.Fatal("should err")
//		}
//	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
//		t.Fatalf("timeout")
//	}
//
//	testutil.WaitForResult(func() (bool, error) {
//		if err := process.Signal(syscall.Signal(0)); err == nil {
//			return false, fmt.Errorf("process should not exist: %v", pid)
//		}
//
//		return true, nil
//	}, func(err error) {
//		t.Fatalf("err: %v", err)
//	})
//}

func TestRawExecDriver_HandlerExec(t *testing.T) {
	t.Parallel()
	task := &structs.Task{
		Name:   "sleep",
		Driver: "raw_exec",
		Config: map[string]interface{}{
			"command": testtask.Path(),
			"args":    []string{"sleep", "9000s"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}
	testtask.SetTaskEnv(task)
	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewRawExecDriver(ctx.DriverCtx)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("prestart err: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exec a command that should work
	out, code, err := resp.Handle.Exec(context.TODO(), "/usr/bin/stat", []string{"/tmp"})
	if err != nil {
		t.Fatalf("error exec'ing stat: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected `stat /alloc` to succeed but exit code was: %d", code)
	}
	if expected := 100; len(out) < expected {
		t.Fatalf("expected at least %d bytes of output but found %d:\n%s", expected, len(out), out)
	}

	// Exec a command that should fail
	out, code, err = resp.Handle.Exec(context.TODO(), "/usr/bin/stat", []string{"lkjhdsaflkjshowaisxmcvnlia"})
	if err != nil {
		t.Fatalf("error exec'ing stat: %v", err)
	}
	if code == 0 {
		t.Fatalf("expected `stat` to fail but exit code was: %d", code)
	}
	if expected := "No such file or directory"; !bytes.Contains(out, []byte(expected)) {
		t.Fatalf("expected output to contain %q but found: %q", expected, out)
	}

	select {
	case res := <-resp.Handle.WaitCh():
		t.Fatalf("Shouldn't be exited: %v", res.String())
	default:
	}

	if err := resp.Handle.Kill(); err != nil {
		t.Fatalf("error killing exec handle: %v", err)
	}
}
