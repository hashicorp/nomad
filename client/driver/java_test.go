package driver

import (
	"os/exec"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"

	ctestutils "github.com/hashicorp/nomad/client/testutil"
)

// javaLocated checks whether java is installed so we can run java stuff.
func javaLocated() bool {
	_, err := exec.Command("java", "-version").CombinedOutput()
	return err == nil
}

// The fingerprinter test should always pass, even if Java is not installed.
func TestJavaDriver_Fingerprint(t *testing.T) {
	t.Parallel()
	ctestutils.JavaCompatible(t)
	driverCtx, _ := testDriverContexts(&structs.Task{Name: "foo"})
	d := NewJavaDriver(driverCtx)
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
	t.Parallel()
	if !javaLocated() {
		t.Skip("Java not found; skipping")
	}

	ctestutils.JavaCompatible(t)
	task := &structs.Task{
		Name: "demo-app",
		Config: map[string]interface{}{
			"artifact_source": "https://dl.dropboxusercontent.com/u/47675/jar_thing/demoapp.jar",
			"jvm_options":     []string{"-Xmx64m", "-Xms32m"},
			"checksum":        "sha256:58d6e8130308d32e197c5108edd4f56ddf1417408f743097c2e662df0f0b17c8",
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewJavaDriver(driverCtx)

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

	time.Sleep(2 * time.Second)

	// There is a race condition between the handle waiting and killing. One
	// will return an error.
	handle.Kill()
	handle2.Kill()
}

func TestJavaDriver_Start_Wait(t *testing.T) {
	t.Parallel()
	if !javaLocated() {
		t.Skip("Java not found; skipping")
	}

	ctestutils.JavaCompatible(t)
	task := &structs.Task{
		Name: "demo-app",
		Config: map[string]interface{}{
			"artifact_source": "https://dl.dropboxusercontent.com/u/47675/jar_thing/demoapp.jar",
			"checksum":        "sha256:58d6e8130308d32e197c5108edd4f56ddf1417408f743097c2e662df0f0b17c8",
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewJavaDriver(driverCtx)

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
	t.Parallel()
	if !javaLocated() {
		t.Skip("Java not found; skipping")
	}

	ctestutils.JavaCompatible(t)
	task := &structs.Task{
		Name: "demo-app",
		Config: map[string]interface{}{
			"artifact_source": "https://dl.dropboxusercontent.com/u/47675/jar_thing/demoapp.jar",
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewJavaDriver(driverCtx)

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

		// Need to kill long lived process
		if err = handle.Kill(); err != nil {
			t.Fatalf("Error: %s", err)
		}
	}
}
