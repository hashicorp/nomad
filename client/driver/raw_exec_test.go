package driver

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/environment"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestRawExecDriver_Fingerprint(t *testing.T) {
	d := NewRawExecDriver(testDriverContext(""))
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
	task := &structs.Task{
		Name: "sleep",
		Config: map[string]interface{}{
			"command": "/bin/sleep",
			"args":    "1",
		},
		Resources: basicResources,
	}
	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()

	d := NewRawExecDriver(driverCtx)
	handle, err := d.Start(ctx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}

	// Attempt to open
	handle2, err := d.Open(ctx, handle.ID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle2 == nil {
		t.Fatalf("missing handle")
	}

	// Task should terminate quickly
	select {
	case <-handle2.WaitCh():
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestRawExecDriver_Start_Artifact_basic(t *testing.T) {
	var file, checksum string
	switch runtime.GOOS {
	case "darwin":
		file = "hi_darwin_amd64"
		checksum = "md5:d7f2fdb13b36dcb7407721d78926b335"
	default:
		file = "hi_linux_amd64"
		checksum = "md5:a9b14903a8942748e4f8474e11f795d3"
	}

	task := &structs.Task{
		Name: "sleep",
		Config: map[string]interface{}{
			"artifact_source": fmt.Sprintf("https://dl.dropboxusercontent.com/u/47675/jar_thing/%s", file),
			"command":         filepath.Join("$NOMAD_TASK_DIR", file),
			"checksum":        checksum,
		},
		Resources: basicResources,
	}
	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()

	d := NewRawExecDriver(driverCtx)
	handle, err := d.Start(ctx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}

	// Attempt to open
	handle2, err := d.Open(ctx, handle.ID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle2 == nil {
		t.Fatalf("missing handle")
	}

	// Task should terminate quickly
	select {
	case <-handle2.WaitCh():
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestRawExecDriver_Start_Artifact_expanded(t *testing.T) {
	var file string
	switch runtime.GOOS {
	case "darwin":
		file = "hi_darwin_amd64"
	default:
		file = "hi_linux_amd64"
	}

	task := &structs.Task{
		Name: "sleep",
		Config: map[string]interface{}{
			"artifact_source": fmt.Sprintf("https://dl.dropboxusercontent.com/u/47675/jar_thing/%s", file),
			"command":         "/bin/bash",
			"args":            fmt.Sprintf("-c '/bin/sleep 1 && %s'", filepath.Join("$NOMAD_TASK_DIR", file)),
		},
		Resources: basicResources,
	}
	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()

	d := NewRawExecDriver(driverCtx)
	handle, err := d.Start(ctx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}

	// Attempt to open
	handle2, err := d.Open(ctx, handle.ID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle2 == nil {
		t.Fatalf("missing handle")
	}

	// Task should terminate quickly
	select {
	case <-handle2.WaitCh():
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestRawExecDriver_Start_Wait(t *testing.T) {
	task := &structs.Task{
		Name: "sleep",
		Config: map[string]interface{}{
			"command": "/bin/sleep",
			"args":    "1",
		},
		Resources: basicResources,
	}

	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()

	d := NewRawExecDriver(driverCtx)
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

func TestRawExecDriver_Start_Wait_AllocDir(t *testing.T) {
	exp := []byte{'w', 'i', 'n'}
	file := "output.txt"
	task := &structs.Task{
		Name: "sleep",
		Config: map[string]interface{}{
			"command": "/bin/bash",
			"args":    fmt.Sprintf(`-c "sleep 1; echo -n %s > $%s/%s"`, string(exp), environment.AllocDir, file),
		},
		Resources: basicResources,
	}

	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()

	d := NewRawExecDriver(driverCtx)
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

func TestRawExecDriver_Start_Kill_Wait(t *testing.T) {
	task := &structs.Task{
		Name: "sleep",
		Config: map[string]interface{}{
			"command": "/bin/sleep",
			"args":    "1",
		},
		Resources: basicResources,
	}

	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()

	d := NewRawExecDriver(driverCtx)
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
