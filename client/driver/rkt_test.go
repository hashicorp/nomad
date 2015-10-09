package driver

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"

	ctestutils "github.com/hashicorp/nomad/client/testutil"
)

func TestRktDriver_Handle(t *testing.T) {
	h := &rktHandle{
		proc:   &os.Process{Pid: 123},
		name:   "foo",
		doneCh: make(chan struct{}),
		waitCh: make(chan error, 1),
	}

	actual := h.ID()
	expected := `Rkt:{"Pid":123,"Name":"foo"}`
	if actual != expected {
		t.Errorf("Expected `%s`, found `%s`", expected, actual)
	}
}

// The fingerprinter test should always pass, even if rkt is not installed.
func TestRktDriver_Fingerprint(t *testing.T) {
	ctestutils.RktCompatible(t)
	d := NewRktDriver(testDriverContext(""))
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
	if node.Attributes["driver.rkt"] != "1" {
		t.Fatalf("Missing Rkt driver")
	}
	if node.Attributes["driver.rkt.version"] == "" {
		t.Fatalf("Missing Rkt driver version")
	}
	if node.Attributes["driver.rkt.appc.version"] == "" {
		t.Fatalf("Missing appc version for the Rkt driver")
	}
}

func TestRktDriver_Start(t *testing.T) {
	ctestutils.RktCompatible(t)
	// TODO: use test server to load from a fixture
	task := &structs.Task{
		Name: "etcd",
		Config: map[string]string{
			"trust_prefix": "coreos.com/etcd",
			"name":         "coreos.com/etcd:v2.0.4",
			"exec":         "/etcd",
		},
	}

	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	d := NewRktDriver(driverCtx)
	defer ctx.AllocDir.Destroy()

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
		fmt.Printf("\nError killing Rkt test: %s", err)
	}
}

func TestRktDriver_Start_Wait(t *testing.T) {
	ctestutils.RktCompatible(t)
	task := &structs.Task{
		Name: "etcd",
		Config: map[string]string{
			"trust_prefix": "coreos.com/etcd",
			"name":         "coreos.com/etcd:v2.0.4",
			"exec":         "/etcd",
			"args":         "--version",
		},
	}

	driverCtx := testDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	d := NewRktDriver(driverCtx)
	defer ctx.AllocDir.Destroy()

	handle, err := d.Start(ctx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}
	defer handle.Kill()

	// Update should be a no-op
	err = handle.Update(task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	select {
	case err := <-handle.WaitCh():
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout")
	}
}
