package driver

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	lxc "gopkg.in/lxc/go-lxc.v2"
)

func TestLxcDriver_Fingerprint(t *testing.T) {
	if !lxcPresent(t) {
		t.Skip("lxc not present")
	}

	task := &structs.Task{
		Name:      "foo",
		Resources: structs.DefaultResources(),
	}

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewLxcDriver(driverCtx)
	node := &structs.Node{
		Attributes: map[string]string{},
	}
	apply, err := d.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !apply {
		t.Fatalf("should apply")
	}
	if node.Attributes["driver.lxc"] == "" {
		t.Fatalf("missing driver")
	}
}

func TestLxcDriver_Start_Wait(t *testing.T) {
	if !lxcPresent(t) {
		t.Skip("lxc not present")
	}

	task := &structs.Task{
		Name: "foo",
		Config: map[string]interface{}{
			"template": "/usr/share/lxc/templates/lxc-busybox",
		},
		KillTimeout: 10 * time.Second,
		Resources:   structs.DefaultResources(),
	}

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewLxcDriver(driverCtx)

	handle, err := d.Start(execCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}

	lxcHandle, _ := handle.(*lxcDriverHandle)

	// Destroy the container after the test
	defer func() {
		lxcHandle.container.Stop()
		lxcHandle.container.Destroy()
	}()

	testutil.WaitForResult(func() (bool, error) {
		state := lxcHandle.container.State()
		if state == lxc.RUNNING {
			return true, nil
		}
		return false, fmt.Errorf("container in state: %v", state)
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Desroy the container
	if err := handle.Kill(); err != nil {
		t.Fatalf("err: %v", err)
	}

	select {
	case res := <-handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}
}

func TestLxcDriver_Open_Wait(t *testing.T) {
	if !lxcPresent(t) {
		t.Skip("lxc not present")
	}

	task := &structs.Task{
		Name: "foo",
		Config: map[string]interface{}{
			"template": "/usr/share/lxc/templates/lxc-busybox",
		},
		KillTimeout: 10 * time.Second,
		Resources:   structs.DefaultResources(),
	}

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewLxcDriver(driverCtx)

	handle, err := d.Start(execCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}

	// Destroy the container after the test
	if lh, ok := handle.(*lxcDriverHandle); ok {
		defer func() {
			lh.container.Stop()
			lh.container.Destroy()
		}()
	}

	handle2, err := d.Open(execCtx, handle.ID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if handle2 == nil {
		t.Fatalf("missing handle on open")
	}

	lxcHandle, _ := handle2.(*lxcDriverHandle)

	testutil.WaitForResult(func() (bool, error) {
		state := lxcHandle.container.State()
		if state == lxc.RUNNING {
			return true, nil
		}
		return false, fmt.Errorf("container in state: %v", state)
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Desroy the container
	if err := handle2.Kill(); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func lxcPresent(t *testing.T) bool {
	return lxc.Version() != ""
}
