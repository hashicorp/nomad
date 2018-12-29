package driver

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"

	ctestutils "github.com/hashicorp/nomad/client/testutil"
)

func TestExecDriver_Fingerprint(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.ExecCompatible(t)
	task := &structs.Task{
		Name:      "foo",
		Driver:    "exec",
		Resources: structs.DefaultResources(),
	}
	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewExecDriver(ctx.DriverCtx)
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
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.ExecCompatible(t)
	task := &structs.Task{
		Name:   "sleep",
		Driver: "exec",
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

	// Attempt to open
	handle2, err := d.Open(ctx.ExecCtx, resp.Handle.ID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle2 == nil {
		t.Fatalf("missing handle")
	}

	resp.Handle.Kill()
	handle2.Kill()
}

func TestExecDriver_Start_Wait(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.ExecCompatible(t)
	task := &structs.Task{
		Name:   "sleep",
		Driver: "exec",
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

func TestExecDriver_Start_Wait_AllocDir(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.ExecCompatible(t)

	exp := []byte{'w', 'i', 'n'}
	file := "output.txt"
	task := &structs.Task{
		Name:   "sleep",
		Driver: "exec",
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

func TestExecDriver_Start_Kill_Wait(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.ExecCompatible(t)
	task := &structs.Task{
		Name:   "sleep",
		Driver: "exec",
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

	go func() {
		time.Sleep(100 * time.Millisecond)
		err := resp.Handle.Kill()
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
	case <-time.After(time.Duration(testutil.TestMultiplier()*10) * time.Second):
		t.Fatalf("timeout")
	}
}

func TestExecDriverUser(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.ExecCompatible(t)
	task := &structs.Task{
		Name:   "sleep",
		Driver: "exec",
		User:   "alice",
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

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewExecDriver(ctx.DriverCtx)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("prestart err: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err == nil {
		resp.Handle.Kill()
		t.Fatalf("Should've failed")
	}
	msg := "user alice"
	if !strings.Contains(err.Error(), msg) {
		t.Fatalf("Expecting '%v' in '%v'", msg, err)
	}
}

// TestExecDriver_HandlerExec ensures the exec driver's handle properly
// executes commands inside the container.
func TestExecDriver_HandlerExec(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.ExecCompatible(t)
	task := &structs.Task{
		Name:   "sleep",
		Driver: "exec",
		Config: map[string]interface{}{
			"command": "/bin/sleep",
			"args":    []string{"9000"},
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
	handle := resp.Handle

	// Exec a command that should work and dump the environment
	out, code, err := handle.Exec(context.Background(), "/bin/sh", []string{"-c", "env | grep NOMAD"})
	if err != nil {
		t.Fatalf("error exec'ing stat: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected `stat /alloc` to succeed but exit code was: %d", code)
	}

	// Assert exec'd commands are run in a task-like environment
	scriptEnv := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(string(line), "=", 2)
		if len(parts) != 2 {
			t.Fatalf("Invalid env var: %q", line)
		}
		scriptEnv[parts[0]] = parts[1]
	}
	if v, ok := scriptEnv["NOMAD_SECRETS_DIR"]; !ok || v != "/secrets" {
		t.Errorf("Expected NOMAD_SECRETS_DIR=/secrets but found=%t value=%q", ok, v)
	}
	if v, ok := scriptEnv["NOMAD_ALLOC_ID"]; !ok || v != ctx.DriverCtx.allocID {
		t.Errorf("Expected NOMAD_SECRETS_DIR=%q but found=%t value=%q", ctx.DriverCtx.allocID, ok, v)
	}

	// Assert cgroup membership
	out, code, err = handle.Exec(context.Background(), "/bin/cat", []string{"/proc/self/cgroup"})
	if err != nil {
		t.Fatalf("error exec'ing cat /proc/self/cgroup: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected `cat /proc/self/cgroup` to succeed but exit code was: %d", code)
	}
	found := false
	for _, line := range strings.Split(string(out), "\n") {
		// Every cgroup entry should be /nomad/$ALLOC_ID
		if line == "" {
			continue
		}
		if !strings.Contains(line, ":/nomad/") {
			t.Errorf("Not a member of the alloc's cgroup: expected=...:/nomad/... -- found=%q", line)
			continue
		}
		found = true
	}
	if !found {
		t.Errorf("exec'd command isn't in the task's cgroup")
	}

	// Exec a command that should fail
	out, code, err = handle.Exec(context.Background(), "/usr/bin/stat", []string{"lkjhdsaflkjshowaisxmcvnlia"})
	if err != nil {
		t.Fatalf("error exec'ing stat: %v", err)
	}
	if code == 0 {
		t.Fatalf("expected `stat` to fail but exit code was: %d", code)
	}
	if expected := "No such file or directory"; !bytes.Contains(out, []byte(expected)) {
		t.Fatalf("expected output to contain %q but found: %q", expected, out)
	}

	if err := handle.Kill(); err != nil {
		t.Fatalf("error killing exec handle: %v", err)
	}
}
