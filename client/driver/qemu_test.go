package driver

import (
	"fmt"
	"path/filepath"
	"strings"
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
		Resources: structs.DefaultResources(),
	}
	driverCtx, _ := testDriverContexts(task)
	d := NewQemuDriver(driverCtx)
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
		Name: "linux",
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

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewQemuDriver(driverCtx)

	// Copy the test image into the task's directory
	dst, _ := execCtx.AllocDir.TaskDirs[task.Name]
	copyFile("./test-resources/qemu/linux-0.2.img", filepath.Join(dst, "linux-0.2.img"), t)

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

	// Clean up
	if err := handle.Kill(); err != nil {
		fmt.Printf("\nError killing Qemu test: %s", err)
	}
}

func TestQemuDriverUser(t *testing.T) {
	ctestutils.QemuCompatible(t)
	task := &structs.Task{
		Name: "linux",
		User: "alice",
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

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewQemuDriver(driverCtx)

	handle, err := d.Start(execCtx, task)
	if err == nil {
		handle.Kill()
		t.Fatalf("Should've failed")
	}
	msg := "unknown user alice"
	if !strings.Contains(err.Error(), msg) {
		t.Fatalf("Expecting '%v' in '%v'", msg, err)
	}
}
