package driver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"

	ctestutils "github.com/hashicorp/nomad/client/testutil"
)

// The fingerprinter test should always pass, even if QEMU is not installed.
func TestQemuDriver_Fingerprint(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.QemuCompatible(t)
	task := &structs.Task{
		Name:      "foo",
		Driver:    "qemu",
		Resources: structs.DefaultResources(),
	}
	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewQemuDriver(ctx.DriverCtx)

	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	apply, err := d.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !apply {
		t.Fatalf("should apply")
	}
	if node.Attributes[qemuDriverAttr] == "" {
		t.Fatalf("Missing Qemu driver")
	}
	if node.Attributes[qemuDriverVersionAttr] == "" {
		t.Fatalf("Missing Qemu driver version")
	}
}

func TestQemuDriver_StartOpen_Wait(t *testing.T) {
	logger := testLogger()
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.QemuCompatible(t)
	task := &structs.Task{
		Name:   "linux",
		Driver: "qemu",
		Config: map[string]interface{}{
			"image_path":        "linux-0.2.img",
			"accelerator":       "tcg",
			"graceful_shutdown": false,
			"port_map": []map[string]int{{
				"main": 22,
				"web":  8080,
			}},
			"args": []string{"-nodefconfig", "-nodefaults"},
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 512,
			Networks: []*structs.NetworkResource{
				{
					ReservedPorts: []structs.Port{{Label: "main", Value: 22000}, {Label: "web", Value: 80}},
				},
			},
		},
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewQemuDriver(ctx.DriverCtx)

	// Copy the test image into the task's directory
	dst := ctx.ExecCtx.TaskDir.Dir

	copyFile("./test-resources/qemu/linux-0.2.img", filepath.Join(dst, "linux-0.2.img"), t)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("Prestart failed: %v", err)
	}

	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure that sending a Signal returns an error
	if err := resp.Handle.Signal(syscall.SIGINT); err == nil {
		t.Fatalf("Expect an error when signalling")
	}

	// Attempt to open
	handle2, err := d.Open(ctx.ExecCtx, resp.Handle.ID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle2 == nil {
		t.Fatalf("missing handle")
	}

	// Clean up
	if err := resp.Handle.Kill(); err != nil {
		logger.Printf("Error killing Qemu test: %s", err)
	}
}

func TestQemuDriver_GracefulShutdown(t *testing.T) {
	logger := testLogger()
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.QemuCompatible(t)
	ctestutils.RequireRoot(t)
	task := &structs.Task{
		Name:   "linux",
		Driver: "qemu",
		Config: map[string]interface{}{
			"image_path":        "linux-0.2.img",
			"accelerator":       "tcg",
			"graceful_shutdown": true,
			"port_map": []map[string]int{{
				"main": 22,
				"web":  8080,
			}},
			"args": []string{"-nodefconfig", "-nodefaults"},
		},
		// With the use of tcg acceleration, it's very unlikely a qemu instance
		// will boot (and gracefully halt) in a reasonable amount of time, so
		// this timeout is kept low to reduce test execution time.
		KillTimeout: time.Duration(1 * time.Second),
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 512,
			Networks: []*structs.NetworkResource{
				{
					ReservedPorts: []structs.Port{{Label: "main", Value: 22000}, {Label: "web", Value: 80}},
				},
			},
		},
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewQemuDriver(ctx.DriverCtx)

	apply, err := d.Fingerprint(&config.Config{}, ctx.DriverCtx.node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !apply {
		t.Fatalf("should apply")
	}

	dst := ctx.ExecCtx.TaskDir.Dir

	copyFile("./test-resources/qemu/linux-0.2.img", filepath.Join(dst, "linux-0.2.img"), t)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("Prestart failed: %v", err)
	}

	resp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Clean up
	defer func() {
		if err := resp.Handle.Kill(); err != nil {
			logger.Printf("Error killing Qemu test: %s", err)
		}
	}()

	// The monitor socket will not exist immediately, so we'll wait up to
	// 5 seconds for it to become available.
	monitorPath := fmt.Sprintf("%s/linux/%s", ctx.AllocDir.AllocDir, qemuMonitorSocketName)
	monitorPathExists := false
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(monitorPath); !os.IsNotExist(err) {
			logger.Printf("monitor socket exists at %q\n", monitorPath)
			monitorPathExists = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if monitorPathExists == false {
		t.Fatalf("monitor socket did not exist after waiting 20 seconds")
	}

	// userPid supplied in sendQemuShutdown calls is bogus (it's used only
	// for log output)
	if err := sendQemuShutdown(ctx.DriverCtx.logger, "", 0); err == nil {
		t.Fatalf("sendQemuShutdown should return an error if monitorPath parameter is empty")
	}

	if err := sendQemuShutdown(ctx.DriverCtx.logger, "/path/that/does/not/exist", 0); err == nil {
		t.Fatalf("sendQemuShutdown should return an error if file does not exist at monitorPath")
	}

	if err := sendQemuShutdown(ctx.DriverCtx.logger, monitorPath, 0); err != nil {
		t.Fatalf("unexpected error from sendQemuShutdown: %s", err)
	}
}

func TestQemuDriverUser(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	ctestutils.QemuCompatible(t)
	tasks := []*structs.Task{
		{
			Name:   "linux",
			Driver: "qemu",
			User:   "alice",
			Config: map[string]interface{}{
				"image_path":        "linux-0.2.img",
				"accelerator":       "tcg",
				"graceful_shutdown": false,
				"port_map": []map[string]int{{
					"main": 22,
					"web":  8080,
				}},
				"args": []string{"-nodefconfig", "-nodefaults"},
				"msg":  "unknown user alice",
			},
			LogConfig: &structs.LogConfig{
				MaxFiles:      10,
				MaxFileSizeMB: 10,
			},
			Resources: &structs.Resources{
				CPU:      500,
				MemoryMB: 512,
				Networks: []*structs.NetworkResource{
					{
						ReservedPorts: []structs.Port{{Label: "main", Value: 22000}, {Label: "web", Value: 80}},
					},
				},
			},
		},
		{
			Name:   "linux",
			Driver: "qemu",
			User:   "alice",
			Config: map[string]interface{}{
				"image_path":  "linux-0.2.img",
				"accelerator": "tcg",
				"port_map": []map[string]int{{
					"main": 22,
					"web":  8080,
				}},
				"args": []string{"-nodefconfig", "-nodefaults"},
				"msg":  "Qemu memory assignment out of bounds",
			},
			LogConfig: &structs.LogConfig{
				MaxFiles:      10,
				MaxFileSizeMB: 10,
			},
			Resources: &structs.Resources{
				CPU:      500,
				MemoryMB: -1,
				Networks: []*structs.NetworkResource{
					{
						ReservedPorts: []structs.Port{{Label: "main", Value: 22000}, {Label: "web", Value: 80}},
					},
				},
			},
		},
	}

	for _, task := range tasks {
		ctx := testDriverContexts(t, task)
		defer ctx.AllocDir.Destroy()
		d := NewQemuDriver(ctx.DriverCtx)

		if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
			t.Fatalf("Prestart faild: %v", err)
		}

		resp, err := d.Start(ctx.ExecCtx, task)
		if err == nil {
			resp.Handle.Kill()
			t.Fatalf("Should've failed")
		}

		msg := task.Config["msg"].(string)
		if !strings.Contains(err.Error(), msg) {
			t.Fatalf("Expecting '%v' in '%v'", msg, err)
		}
	}
}

func TestQemuDriverGetMonitorPathOldQemu(t *testing.T) {
	task := &structs.Task{
		Name:   "linux",
		Driver: "qemu",
		Config: map[string]interface{}{
			"image_path":        "linux-0.2.img",
			"accelerator":       "tcg",
			"graceful_shutdown": true,
			"port_map": []map[string]int{{
				"main": 22,
				"web":  8080,
			}},
			"args": []string{"-nodefconfig", "-nodefaults"},
		},
		KillTimeout: time.Duration(1 * time.Second),
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 512,
			Networks: []*structs.NetworkResource{
				{
					ReservedPorts: []structs.Port{{Label: "main", Value: 22000}, {Label: "web", Value: 80}},
				},
			},
		},
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()

	// Simulate an older version of qemu which does not support long monitor socket paths
	ctx.DriverCtx.node.Attributes[qemuDriverVersionAttr] = "2.0.0"

	d := &QemuDriver{DriverContext: *ctx.DriverCtx}

	shortPath := strings.Repeat("x", 10)
	_, err := d.getMonitorPath(shortPath)
	if err != nil {
		t.Fatal("Should not have returned an error")
	}

	longPath := strings.Repeat("x", qemuLegacyMaxMonitorPathLen+100)
	_, err = d.getMonitorPath(longPath)
	if err == nil {
		t.Fatal("Should have returned an error")
	}

	// Max length includes the '/' separator and socket name
	maxLengthCount := qemuLegacyMaxMonitorPathLen - len(qemuMonitorSocketName) - 1
	maxLengthLegacyPath := strings.Repeat("x", maxLengthCount)
	_, err = d.getMonitorPath(maxLengthLegacyPath)
	if err != nil {
		t.Fatalf("Should not have returned an error: %s", err)
	}
}

func TestQemuDriverGetMonitorPathNewQemu(t *testing.T) {
	task := &structs.Task{
		Name:   "linux",
		Driver: "qemu",
		Config: map[string]interface{}{
			"image_path":        "linux-0.2.img",
			"accelerator":       "tcg",
			"graceful_shutdown": true,
			"port_map": []map[string]int{{
				"main": 22,
				"web":  8080,
			}},
			"args": []string{"-nodefconfig", "-nodefaults"},
		},
		KillTimeout: time.Duration(1 * time.Second),
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 512,
			Networks: []*structs.NetworkResource{
				{
					ReservedPorts: []structs.Port{{Label: "main", Value: 22000}, {Label: "web", Value: 80}},
				},
			},
		},
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()

	// Simulate a version of qemu which supports long monitor socket paths
	ctx.DriverCtx.node.Attributes[qemuDriverVersionAttr] = "2.99.99"

	d := &QemuDriver{DriverContext: *ctx.DriverCtx}

	shortPath := strings.Repeat("x", 10)
	_, err := d.getMonitorPath(shortPath)
	if err != nil {
		t.Fatal("Should not have returned an error")
	}

	longPath := strings.Repeat("x", qemuLegacyMaxMonitorPathLen+100)
	_, err = d.getMonitorPath(longPath)
	if err != nil {
		t.Fatal("Should not have returned an error")
	}

	maxLengthCount := qemuLegacyMaxMonitorPathLen - len(qemuMonitorSocketName) - 1
	maxLengthLegacyPath := strings.Repeat("x", maxLengthCount)
	_, err = d.getMonitorPath(maxLengthLegacyPath)
	if err != nil {
		t.Fatal("Should not have returned an error")
	}
}
