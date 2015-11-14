package driver

import (
	"os/exec"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"

	ctestutils "github.com/hashicorp/nomad/client/testutil"
)

// javaLocated checks whether java is installed so we can run java stuff.
func javaLocated() bool {
	_, err := exec.Command("java", "-version").CombinedOutput()
	return err == nil
}

// The fingerprinter test should always pass, even if Java is not installed.
func TestJavaDriver_Fingerprint(t *testing.T) {
	ctestutils.JavaCompatible(t)
	d := NewJavaDriver(testDriverContext(""))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	apply, err := d.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if apply != javaLocated() {
		t.Fatalf("Fingerprinter should detect Java when it is installed")
	}
	if node.Attributes["driver.java"] != "1" {
		t.Fatalf("missing driver")
	}
	for _, key := range []string{"driver.java.version", "driver.java.runtime", "driver.java.vm"} {
		if node.Attributes[key] == "" {
			t.Fatalf("missing driver key (%s)", key)
		}
	}
}

func TestJavaDriver_StartOpen_Wait(t *testing.T) {
	if !javaLocated() {
		t.Skip("Java not found; skipping")
	}

	ctestutils.JavaCompatible(t)
	task := &structs.Task{
		Name: "demo-app",
		Config: map[string]string{
			"artifact_source": "https://dl.dropboxusercontent.com/u/47675/jar_thing/demoapp.jar",
			"jvm_options":     "-Xmx2048m -Xms256m",
			"checksum":        "sha256:58d6e8130308d32e197c5108edd4f56ddf1417408f743097c2e662df0f0b17c8",
		},
		Resources: basicResources,
	}

	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()
	d := NewJavaDriver(driverCtx)

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

	time.Sleep(2 * time.Second)
	// need to kill long lived process
	err = handle.Kill()
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
}

func TestJavaDriver_Start_Wait(t *testing.T) {
	if !javaLocated() {
		t.Skip("Java not found; skipping")
	}

	ctestutils.JavaCompatible(t)
	task := &structs.Task{
		Name: "demo-app",
		Config: map[string]string{
			"artifact_source": "https://dl.dropboxusercontent.com/u/47675/jar_thing/demoapp.jar",
			"jvm_options":     "-Xmx2048m -Xms256m",
			"checksum":        "sha256:58d6e8130308d32e197c5108edd4f56ddf1417408f743097c2e662df0f0b17c8",
		},
		Resources: basicResources,
	}

	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()
	d := NewJavaDriver(driverCtx)

	handle, err := d.Start(ctx, task)
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
	case <-time.After(2 * time.Second):
		// expect the timeout b/c it's a long lived process
		break
	}

	// need to kill long lived process
	err = handle.Kill()
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
}

func TestJavaDriver_Start_Kill_Wait(t *testing.T) {
	if !javaLocated() {
		t.Skip("Java not found; skipping")
	}

	ctestutils.JavaCompatible(t)
	task := &structs.Task{
		Name: "demo-app",
		Config: map[string]string{
			"artifact_source": "https://dl.dropboxusercontent.com/u/47675/jar_thing/demoapp.jar",
		},
		Resources: basicResources,
	}

	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()
	d := NewJavaDriver(driverCtx)

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
	case res := <-handle.WaitCh():
		if res.Successful() {
			t.Fatal("should err")
		}
	case <-time.After(8 * time.Second):
		t.Fatalf("timeout")
	}

	// need to kill long lived process
	err = handle.Kill()
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
}
