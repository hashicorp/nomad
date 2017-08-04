//+build linux,lxc

package driver

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	lxc "gopkg.in/lxc/go-lxc.v2"
)

func TestLxcDriver_Fingerprint(t *testing.T) {
	t.Parallel()
	if !lxcPresent(t) {
		t.Skip("lxc not present")
	}

	task := &structs.Task{
		Name:      "foo",
		Driver:    "lxc",
		Resources: structs.DefaultResources(),
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewLxcDriver(ctx.DriverCtx)

	node := &structs.Node{
		Attributes: map[string]string{},
	}
	apply, err := d.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !apply {
		t.Fatalf("should apply by default")
	}

	apply, err = d.Fingerprint(&config.Config{Options: map[string]string{lxcConfigOption: "0"}}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if apply {
		t.Fatalf("should not apply with config")
	}
	if node.Attributes["driver.lxc"] == "" {
		t.Fatalf("missing driver")
	}
}

func TestLxcDriver_Start_Wait(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	if !lxcPresent(t) {
		t.Skip("lxc not present")
	}

	task := &structs.Task{
		Name:   "foo",
		Driver: "lxc",
		Config: map[string]interface{}{
			"template": "/usr/share/lxc/templates/lxc-busybox",
		},
		KillTimeout: 10 * time.Second,
		Resources:   structs.DefaultResources(),
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewLxcDriver(ctx.DriverCtx)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("prestart err: %v", err)
	}
	sresp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	lxcHandle, _ := sresp.Handle.(*lxcDriverHandle)

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

	// Look for mounted directories in their proper location
	containerName := fmt.Sprintf("%s-%s", task.Name, ctx.DriverCtx.allocID)
	for _, mnt := range []string{"alloc", "local", "secrets"} {
		fullpath := filepath.Join(lxcHandle.lxcPath, containerName, "rootfs", mnt)
		stat, err := os.Stat(fullpath)
		if err != nil {
			t.Fatalf("err %v", err)
		}
		if !stat.IsDir() {
			t.Fatalf("expected %q to be a dir", fullpath)
		}
	}

	// Desroy the container
	if err := sresp.Handle.Kill(); err != nil {
		t.Fatalf("err: %v", err)
	}

	select {
	case res := <-sresp.Handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		t.Fatalf("timeout")
	}
}

func TestLxcDriver_Open_Wait(t *testing.T) {
	if !testutil.IsTravis() {
		t.Parallel()
	}
	if !lxcPresent(t) {
		t.Skip("lxc not present")
	}

	task := &structs.Task{
		Name:   "foo",
		Driver: "lxc",
		Config: map[string]interface{}{
			"template": "/usr/share/lxc/templates/lxc-busybox",
		},
		KillTimeout: 10 * time.Second,
		Resources:   structs.DefaultResources(),
	}

	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewLxcDriver(ctx.DriverCtx)

	if _, err := d.Prestart(ctx.ExecCtx, task); err != nil {
		t.Fatalf("prestart err: %v", err)
	}
	sresp, err := d.Start(ctx.ExecCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Destroy the container after the test
	lh := sresp.Handle.(*lxcDriverHandle)
	defer func() {
		lh.container.Stop()
		lh.container.Destroy()
	}()

	handle2, err := d.Open(ctx.ExecCtx, lh.ID())
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
