package driver

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"

	ctestutils "github.com/hashicorp/nomad/client/testutil"
)

// The fingerprinter test should always pass, even if QEMU is not installed.
func TestQemuDriver_Fingerprint(t *testing.T) {
	ctestutils.QemuCompatible(t)
	d := NewQemuDriver(testDriverContext(""))
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
	// TODO: use test server to load from a fixture
	task := &structs.Task{
		Name: "linux",
		Config: map[string]interface{}{
			"artifact_source": "https://dl.dropboxusercontent.com/u/47675/jar_thing/linux-0.2.img",
			"checksum":        "sha256:a5e836985934c3392cbbd9b26db55a7d35a8d7ae1deb7ca559dd9c0159572544",
			"accelerator":     "tcg",
			"guest_ports":     "22,8080",
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

	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()
	d := NewQemuDriver(driverCtx)

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

	// Clean up
	if err := handle.Kill(); err != nil {
		fmt.Printf("\nError killing Qemu test: %s", err)
	}
}

func TestQemuDriver_RequiresMemory(t *testing.T) {
	ctestutils.QemuCompatible(t)
	// TODO: use test server to load from a fixture
	task := &structs.Task{
		Name: "linux",
		Config: map[string]interface{}{
			"artifact_source": "https://dl.dropboxusercontent.com/u/47675/jar_thing/linux-0.2.img",
			"accelerator":     "tcg",
			"host_port":       "8080",
			"guest_port":      "8081",
			"checksum":        "sha256:a5e836985934c3392cbbd9b26db55a7d35a8d7ae1deb7ca559dd9c0159572544",
			// ssh u/p would be here
		},
	}

	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()
	d := NewQemuDriver(driverCtx)

	_, err := d.Start(ctx, task)
	if err == nil {
		t.Fatalf("Expected error when not specifying memory")
	}
}
