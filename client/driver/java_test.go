package driver

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"

	ctestutils "github.com/hashicorp/nomad/client/testutil"
)

var (
	osJavaDriverSupport = map[string]bool{
		"linux": true,
	}
)

// javaLocated checks whether java is installed so we can run java stuff.
func javaLocated() bool {
	_, err := exec.Command("java", "-version").CombinedOutput()
	return err == nil
}

// The fingerprinter test should always pass, even if Java is not installed.
func TestJavaDriver_Fingerprint(t *testing.T) {
	ctestutils.JavaCompatible(t)
	task := &structs.Task{
		Name:      "foo",
		Resources: structs.DefaultResources(),
	}
	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewJavaDriver(driverCtx)
	node := &structs.Node{
		Attributes: map[string]string{
			"unique.cgroup.mountpoint": "/sys/fs/cgroups",
		},
	}
	apply, err := d.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if apply != javaLocated() {
		t.Fatalf("Fingerprinter should detect Java when it is installed")
	}
	if node.Attributes["driver.java"] != "1" {
		if v, ok := osJavaDriverSupport[runtime.GOOS]; v && ok {
			t.Fatalf("missing java driver")
		} else {
			t.Skipf("missing java driver, no OS support")
		}
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
		Config: map[string]interface{}{
			"jar_path":    "demoapp.jar",
			"jvm_options": []string{"-Xmx64m", "-Xms32m"},
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

	// Copy the test jar into the task's directory
	dst, _ := execCtx.AllocDir.TaskDirs[task.Name]
	copyFile("./test-resources/java/demoapp.jar", filepath.Join(dst, "demoapp.jar"), t)

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
	if !javaLocated() {
		t.Skip("Java not found; skipping")
	}

	ctestutils.JavaCompatible(t)
	task := &structs.Task{
		Name: "demo-app",
		Config: map[string]interface{}{
			"jar_path": "demoapp.jar",
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	driverCtx, execCtx := testDriverContexts(task)
	d := NewJavaDriver(driverCtx)

	// Copy the test jar into the task's directory
	dst, _ := execCtx.AllocDir.TaskDirs[task.Name]
	copyFile("./test-resources/java/demoapp.jar", filepath.Join(dst, "demoapp.jar"), t)

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

	// Get the stdout of the process and assrt that it's not empty
	stdout := filepath.Join(execCtx.AllocDir.LogDir(), "demo-app.stdout.0")
	fInfo, err := os.Stat(stdout)
	if err != nil {
		t.Fatalf("failed to get stdout of process: %v", err)
	}
	if fInfo.Size() == 0 {
		t.Fatalf("stdout of process is empty")
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
		Config: map[string]interface{}{
			"jar_path": "demoapp.jar",
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

	// Copy the test jar into the task's directory
	dst, _ := execCtx.AllocDir.TaskDirs[task.Name]
	copyFile("./test-resources/java/demoapp.jar", filepath.Join(dst, "demoapp.jar"), t)

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

func TestJavaDriverUser(t *testing.T) {
	if !javaLocated() {
		t.Skip("Java not found; skipping")
	}

	ctestutils.JavaCompatible(t)
	task := &structs.Task{
		Name: "demo-app",
		User: "alice",
		Config: map[string]interface{}{
			"jar_path": "demoapp.jar",
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
	if err == nil {
		handle.Kill()
		t.Fatalf("Should've failed")
	}
	msg := "user alice"
	if !strings.Contains(err.Error(), msg) {
		t.Fatalf("Expecting '%v' in '%v'", msg, err)
	}
}
