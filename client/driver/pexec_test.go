package driver

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/environment"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestPexecDriver_Fingerprint(t *testing.T) {
	d := NewPrivilegedExecDriver(testDriverContext(""))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	// Disable privileged exec.
	cfg := &config.Config{Options: map[string]string{pexecConfigOption: "false"}}

	apply, err := d.Fingerprint(cfg, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if apply {
		t.Fatalf("should not apply")
	}
	if node.Attributes["driver.pexec"] != "" {
		t.Fatalf("driver incorrectly enabled")
	}

	// Enable privileged exec.
	cfg.Options[pexecConfigOption] = "true"
	apply, err = d.Fingerprint(cfg, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !apply {
		t.Fatalf("should apply")
	}
	if node.Attributes["driver.pexec"] != "1" {
		t.Fatalf("driver not enabled")
	}
}

func TestPexecDriver_StartOpen_Wait(t *testing.T) {
	task := &structs.Task{
		Name: "sleep",
		Config: map[string]string{
			"command": "/bin/sleep",
			"args":    "2",
		},
	}
	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()

	d := NewPrivilegedExecDriver(driverCtx)
	handle, err := d.Start(ctx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}

	// Attempt to open
	handle2, err := d.Open(ctx, handle.ID())
	handle2.(*pexecHandle).waitCh = make(chan error, 1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle2 == nil {
		t.Fatalf("missing handle")
	}

	// Task should terminate quickly
	select {
	case err := <-handle2.WaitCh():
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestPexecDriver_Start_Wait(t *testing.T) {
	task := &structs.Task{
		Name: "sleep",
		Config: map[string]string{
			"command": "/bin/sleep",
			"args":    "1",
		},
	}

	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()

	d := NewPrivilegedExecDriver(driverCtx)
	handle, err := d.Start(ctx, task)
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
	case err := <-handle.WaitCh():
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestPexecDriver_Start_Wait_AllocDir(t *testing.T) {
	exp := []byte{'w', 'i', 'n'}
	file := "output.txt"
	task := &structs.Task{
		Name: "sleep",
		Config: map[string]string{
			"command": "/bin/bash",
			"args":    fmt.Sprintf(`-c "sleep 1; echo -n %s > $%s/%s"`, string(exp), environment.AllocDir, file),
		},
	}

	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()

	d := NewPrivilegedExecDriver(driverCtx)
	handle, err := d.Start(ctx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}

	// Task should terminate quickly
	select {
	case err := <-handle.WaitCh():
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	case <-time.After(2 * time.Second):
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

func TestPexecDriver_Start_Kill_Wait(t *testing.T) {
	task := &structs.Task{
		Name: "sleep",
		Config: map[string]string{
			"command": "/bin/sleep",
			"args":    "1",
		},
	}

	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()

	d := NewPrivilegedExecDriver(driverCtx)
	handle, err := d.Start(ctx, task)
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
	case err := <-handle.WaitCh():
		if err == nil {
			t.Fatal("should err")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout")
	}
}
