package driver

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"

	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	ctestutils "github.com/hashicorp/nomad/client/testutil"
)

func TestRktVersionRegex(t *testing.T) {
	input_rkt := "rkt version 0.8.1"
	input_appc := "appc version 1.2.0"
	expected_rkt := "0.8.1"
	expected_appc := "1.2.0"
	rktMatches := reRktVersion.FindStringSubmatch(input_rkt)
	appcMatches := reAppcVersion.FindStringSubmatch(input_appc)
	if rktMatches[1] != expected_rkt {
		fmt.Printf("Test failed; got %q; want %q\n", rktMatches[1], expected_rkt)
	}
	if appcMatches[1] != expected_appc {
		fmt.Printf("Test failed; got %q; want %q\n", appcMatches[1], expected_appc)
	}
}

func TestRktDriver_Handle(t *testing.T) {
	h := &rktHandle{
		proc:        &os.Process{Pid: 123},
		image:       "foo",
		killTimeout: 5 * time.Nanosecond,
		doneCh:      make(chan struct{}),
		waitCh:      make(chan *cstructs.WaitResult, 1),
	}

	actual := h.ID()
	expected := `Rkt:{"Pid":123,"Image":"foo","KillTimeout":5}`
	if actual != expected {
		t.Errorf("Expected `%s`, found `%s`", expected, actual)
	}
}

// The fingerprinter test should always pass, even if rkt is not installed.
func TestRktDriver_Fingerprint(t *testing.T) {
	ctestutils.RktCompatible(t)
	driverCtx, _ := testDriverContexts(&structs.Task{Name: "foo"})
	d := NewRktDriver(driverCtx)
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
		Config: map[string]interface{}{
			"trust_prefix": "coreos.com/etcd",
			"image":        "coreos.com/etcd:v2.0.4",
			"command":      "/etcd",
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
	}

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewRktDriver(driverCtx)

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
		fmt.Printf("\nError killing Rkt test: %s", err)
	}
}

func TestRktDriver_Start_Wait(t *testing.T) {
	ctestutils.RktCompatible(t)
	task := &structs.Task{
		Name: "etcd",
		Config: map[string]interface{}{
			"trust_prefix": "coreos.com/etcd",
			"image":        "coreos.com/etcd:v2.0.4",
			"command":      "/etcd",
			"args":         []string{"--version"},
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
	}

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewRktDriver(driverCtx)

	handle, err := d.Start(execCtx, task)
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
	case res := <-handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(5 * time.Second):
		break
	}
        err = handle.Kill()
        if err != nil {
                t.Fatalf("Error: %s", err)
        }
}

func TestRktDriver_Start_Wait_Skip_Trust(t *testing.T) {
	ctestutils.RktCompatible(t)
	task := &structs.Task{
		Name: "etcd",
		Config: map[string]interface{}{
			"image":   "coreos.com/etcd:v2.0.4",
			"command": "/etcd",
			"args":    []string{"--version"},
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
	}

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewRktDriver(driverCtx)

	handle, err := d.Start(execCtx, task)
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
	case res := <-handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(5 * time.Second):
		break
	}
        err = handle.Kill()
        if err != nil {
                t.Fatalf("Error: %s", err)
        }
}

func TestRktDriver_Start_Wait_Logs(t *testing.T) {
	ctestutils.RktCompatible(t)
        t.Skip("Logs not implemented for rkt")
	task := &structs.Task{
		Name: "etcd",
		Config: map[string]interface{}{
			"trust_prefix": "coreos.com/etcd",
			"image":        "coreos.com/etcd:v2.0.4",
			"command":      "/etcd",
			"args":         []string{"--version"},
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
	}

	driverCtx, execCtx := testDriverContexts(task)
	defer execCtx.AllocDir.Destroy()
	d := NewRktDriver(driverCtx)

	handle, err := d.Start(execCtx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}
	defer handle.Kill()

	select {
	case res := <-handle.WaitCh():
		if !res.Successful() {
			t.Fatalf("err: %v", res)
		}
	case <-time.After(5 * time.Second):
	        break
	}

	taskDir, ok := execCtx.AllocDir.TaskDirs[task.Name]
	if !ok {
		t.Fatalf("Could not find task directory for task: %v", task)
	}
	stdout := filepath.Join(taskDir, allocdir.TaskLocal, fmt.Sprintf("%v.stdout", task.Name))
	data, err := ioutil.ReadFile(stdout)
	if err != nil {
		t.Fatalf("Failed to read tasks stdout: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Task's stdout is empty")
	}
}
