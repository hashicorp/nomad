package driver

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"

	ctestutils "github.com/hashicorp/nomad/client/testutil"
)

// The fingerprinter test should always pass, even if QEMU is not installed.
func TestQemuDriver_Fingerprint(t *testing.T) {
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
	if node.Attributes["driver.qemu"] == "" {
		t.Fatalf("Missing Qemu driver")
	}
	if node.Attributes["driver.qemu.version"] == "" {
		t.Fatalf("Missing Qemu driver version")
	}
}

func TestQemuDriver_StartOpen_Wait(t *testing.T) {
	ctestutils.QemuCompatible(t)
	task := &structs.Task{
		Name:   "linux",
		Driver: "qemu",
		Config: map[string]interface{}{
			"image_path":  "linux-0.2.img",
			"accelerator": "tcg",
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
				&structs.NetworkResource{
					ReservedPorts: []structs.Port{{"main", 22000}, {"web", 80}},
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

	if err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("Prestart faild: %v", err)
	}

	handle, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}

	// Ensure that sending a Signal returns an error
	if err := handle.Signal(syscall.SIGINT); err == nil {
		t.Fatalf("Expect an error when signalling")
	}

	// Attempt to open
	handle2, err := d.Open(ctx.ExecCtx, handle.ID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle2 == nil {
		t.Fatalf("missing handle")
	}

	// Clean up
	if err := handle.Kill(); err != nil {
		fmt.Printf("\nError killing Qemu test: %s", err)
	}
}

func TestQemuDriverUser(t *testing.T) {
	ctestutils.QemuCompatible(t)
	task := &structs.Task{
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
		},
		LogConfig: &structs.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		},
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 512,
			Networks: []*structs.NetworkResource{
				&structs.NetworkResource{
					ReservedPorts: []structs.Port{{"main", 22000}, {"web", 80}},
				},
			},
		},
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewQemuDriver(ctx.DriverCtx)

	if err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("Prestart faild: %v", err)
	}

	handle, err := d.Start(ctx.ExecCtx, task)
	if err == nil {
		handle.Kill()
		t.Fatalf("Should've failed")
	}
	msg := "unknown user alice"
	if !strings.Contains(err.Error(), msg) {
		t.Fatalf("Expecting '%v' in '%v'", msg, err)
	}
}
