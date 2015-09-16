package driver

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestExecDriver_Fingerprint(t *testing.T) {
	d := NewExecDriver(testDriverContext())
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
	if node.Attributes["driver.exec"] == "" {
		t.Fatalf("missing driver")
	}
}

func TestExecDriver_StartOpen_Wait(t *testing.T) {
	ctx := NewExecContext()
	d := NewExecDriver(testDriverContext())

	task := &structs.Task{
		Config: map[string]string{
			"command": "/bin/sleep",
			"args":    "5",
		},
	}
	if task.Resources == nil {
		task.Resources = &structs.Resources{}
	}
	task.Resources.CPU = 2048
	task.Resources.MemoryMB = 2

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
}

func TestExecDriver_Start_Wait(t *testing.T) {
	ctx := NewExecContext()
	d := NewExecDriver(testDriverContext())

	task := &structs.Task{
		Config: map[string]string{
			"command": "/bin/sleep",
			"args":    "1",
		},
	}
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

func TestExecDriver_Start_Kill_Wait(t *testing.T) {
	ctx := NewExecContext()
	d := NewExecDriver(testDriverContext())

	task := &structs.Task{
		Config: map[string]string{
			"command": "/bin/sleep",
			"args":    "10",
		},
	}
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
			t.Fatalf("should err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout")
	}
}
