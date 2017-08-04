package driver

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/env"
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
	cfg := &config.Config{Options: map[string]string{rawExecConfigOption: "false"}}

	apply, err := d.Fingerprint(cfg, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if apply {
		t.Fatalf("should not apply")
	}
	if node.Attributes["driver.raw_exec"] != "" {
		t.Fatalf("driver incorrectly enabled")
	}

	// Enable raw exec.
	cfg.Options[rawExecConfigOption] = "true"
	apply, err = d.Fingerprint(cfg, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !apply {
		t.Fatalf("should apply")
	}
	if node.Attributes["driver.raw_exec"] != "1" {
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

func TestRawExecDriverUser(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("Linux only test")
	}
	task := &structs.Task{
		Name:   "sleep",
		Driver: "raw_exec",
		User:   "alice",
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
	if err == nil {
		resp.Handle.Kill()
		t.Fatalf("Should've failed")
	}
	msg := "unknown user alice"
	if !strings.Contains(err.Error(), msg) {
		t.Fatalf("Expecting '%v' in '%v'", msg, err)
	}
}

func TestRawExecDriver_HandlerExec(t *testing.T) {
	t.Parallel()
	task := &structs.Task{
		Name:   "sleep",
		Driver: "raw_exec",
		Config: map[string]interface{}{
			"command": testtask.Path(),
			"args":    []string{"sleep", "9000"},
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

	if err := resp.Handle.Kill(); err != nil {
		t.Fatalf("error killing exec handle: %v", err)
	}
}
