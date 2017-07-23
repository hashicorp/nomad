package driver

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
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
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.JavaCompatible(t)
	task := &structs.Task{
		Name:      "foo",
		Driver:    "java",
		Resources: structs.DefaultResources(),
	}
	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewJavaDriver(ctx.DriverCtx)
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
	if !testutil.IsTravis() {
		t.Parallel()
	}
	if !javaLocated() {
		t.Skip("Java not found; skipping")
	}

	ctestutils.JavaCompatible(t)
	task := &structs.Task{
		Name:   "demo-app",
		Driver: "java",
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

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewJavaDriver(ctx.DriverCtx)

	// Copy the test jar into the task's directory
	dst := ctx.ExecCtx.TaskDir.Dir
	copyFile("./test-resources/java/demoapp.jar", filepath.Join(dst, "demoapp.jar"), t)

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

	time.Sleep(2 * time.Second)

	// There is a race condition between the handle waiting and killing. One
	// will return an error.
	resp.Handle.Kill()
	handle2.Kill()
}

func TestJavaDriver_Start_Wait(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	if !javaLocated() {
		t.Skip("Java not found; skipping")
	}

	ctestutils.JavaCompatible(t)
	task := &structs.Task{
		Name:   "demo-app",
		Driver: "java",
		Config: map[string]interface{}{
			"jar_path": "demoapp.jar",
			"args":     []string{"1"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewJavaDriver(ctx.DriverCtx)

	// Copy the test jar into the task's directory
	dst := ctx.ExecCtx.TaskDir.Dir
	copyFile("./test-resources/java/demoapp.jar", filepath.Join(dst, "demoapp.jar"), t)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("prestart err: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Task should terminate after 1 seconds
	select {
	case res := <-resp.Handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout")
	}

	// Get the stdout of the process and assrt that it's not empty
	stdout := filepath.Join(ctx.ExecCtx.TaskDir.LogDir, "demo-app.stdout.0")
	fInfo, err := os.Stat(stdout)
	if err != nil {
		t.Fatalf("failed to get stdout of process: %v", err)
	}
	if fInfo.Size() == 0 {
		t.Fatalf("stdout of process is empty")
	}

	// need to kill long lived process
	err = resp.Handle.Kill()
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
}

func TestJavaDriver_Start_Kill_Wait(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	if !javaLocated() {
		t.Skip("Java not found; skipping")
	}

	ctestutils.JavaCompatible(t)
	task := &structs.Task{
		Name:   "demo-app",
		Driver: "java",
		Config: map[string]interface{}{
			"jar_path": "demoapp.jar",
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewJavaDriver(ctx.DriverCtx)

	// Copy the test jar into the task's directory
	dst := ctx.ExecCtx.TaskDir.Dir
	copyFile("./test-resources/java/demoapp.jar", filepath.Join(dst, "demoapp.jar"), t)

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

		// Need to kill long lived process
		if err = resp.Handle.Kill(); err != nil {
			t.Fatalf("Error: %s", err)
		}
	}
}

func TestJavaDriver_Signal(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	if !javaLocated() {
		t.Skip("Java not found; skipping")
	}

	ctestutils.JavaCompatible(t)
	task := &structs.Task{
		Name:   "demo-app",
		Driver: "java",
		Config: map[string]interface{}{
			"jar_path": "demoapp.jar",
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewJavaDriver(ctx.DriverCtx)

	// Copy the test jar into the task's directory
	dst := ctx.ExecCtx.TaskDir.Dir
	copyFile("./test-resources/java/demoapp.jar", filepath.Join(dst, "demoapp.jar"), t)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("prestart err: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		err := resp.Handle.Signal(syscall.SIGHUP)
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

		// Need to kill long lived process
		if err = resp.Handle.Kill(); err != nil {
			t.Fatalf("Error: %s", err)
		}
	}
}

func TestJavaDriver_User(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	if !javaLocated() {
		t.Skip("Java not found; skipping")
	}
	if runtime.GOOS != "linux" {
		t.Skip("Linux only test")
	}

	ctestutils.JavaCompatible(t)
	task := &structs.Task{
		Name:   "demo-app",
		Driver: "java",
		User:   "alice",
		Config: map[string]interface{}{
			"jar_path": "demoapp.jar",
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewJavaDriver(ctx.DriverCtx)

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

func TestJavaDriver_Start_Wait_Class(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	if !javaLocated() {
		t.Skip("Java not found; skipping")
	}

	ctestutils.JavaCompatible(t)
	task := &structs.Task{
		Name:   "demo-app",
		Driver: "java",
		Config: map[string]interface{}{
			"class_path": "${NOMAD_TASK_DIR}",
			"class":      "Hello",
			"args":       []string{"1"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: basicResources,
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewJavaDriver(ctx.DriverCtx)

	// Copy the test jar into the task's directory
	dst := ctx.ExecCtx.TaskDir.LocalDir
	copyFile("./test-resources/java/Hello.class", filepath.Join(dst, "Hello.class"), t)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("prestart err: %v", err)
	}
	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Task should terminate after 1 seconds
	select {
	case res := <-resp.Handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout")
	}

	// Get the stdout of the process and assrt that it's not empty
	stdout := filepath.Join(ctx.ExecCtx.TaskDir.LogDir, "demo-app.stdout.0")
	fInfo, err := os.Stat(stdout)
	if err != nil {
		t.Fatalf("failed to get stdout of process: %v", err)
	}
	if fInfo.Size() == 0 {
		t.Fatalf("stdout of process is empty")
	}

	// need to kill long lived process
	if err := resp.Handle.Kill(); err != nil {
		t.Fatalf("Error: %s", err)
	}
}
