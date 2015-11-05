package driver

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/environment"
	"github.com/hashicorp/nomad/nomad/structs"
)

func testDockerDriverContext(task string) *DriverContext {
	cfg := testConfig()
	cfg.DevMode = true
	return NewDriverContext(task, cfg, cfg.Node, testLogger())
}

// dockerLocated looks to see whether docker is available on this system before
// we try to run tests. We'll keep it simple and just check for the CLI.
func dockerLocated() bool {
	_, err := exec.Command("docker", "-v").CombinedOutput()
	return err == nil
}

func TestDockerDriver_Handle(t *testing.T) {
	h := &dockerHandle{
		imageID:     "imageid",
		containerID: "containerid",
		doneCh:      make(chan struct{}),
		waitCh:      make(chan error, 1),
	}

	actual := h.ID()
	expected := `DOCKER:{"ImageID":"imageid","ContainerID":"containerid"}`
	if actual != expected {
		t.Errorf("Expected `%s`, found `%s`", expected, actual)
	}
}

// The fingerprinter test should always pass, even if Docker is not installed.
func TestDockerDriver_Fingerprint(t *testing.T) {
	d := NewDockerDriver(testDockerDriverContext(""))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	apply, err := d.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if apply != dockerLocated() {
		t.Fatalf("Fingerprinter should detect Docker when it is installed")
	}
	if node.Attributes["driver.docker"] != "1" {
		t.Log("Docker not found. The remainder of the docker tests will be skipped.")
	}
	t.Logf("Found docker version %s", node.Attributes["driver.docker.version"])
}

func TestDockerDriver_StartOpen_Wait(t *testing.T) {
	if !dockerLocated() {
		t.SkipNow()
	}

	task := &structs.Task{
		Name: "redis-demo",
		Config: map[string]string{
			"image": "redis",
		},
		Resources: basicResources,
	}

	driverCtx := testDockerDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()
	d := NewDockerDriver(driverCtx)

	handle, err := d.Start(ctx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}
	defer handle.Kill()

	// Attempt to open
	handle2, err := d.Open(ctx, handle.ID())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle2 == nil {
		t.Fatalf("missing handle")
	}
}

func TestDockerDriver_Start_Wait(t *testing.T) {
	if !dockerLocated() {
		t.SkipNow()
	}

	task := &structs.Task{
		Name: "redis-demo",
		Config: map[string]string{
			"image":       "redis",
			"command":     "redis-server",
			"args":        "-v",
			"priviledged": "true",
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
	}

	driverCtx := testDockerDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()
	d := NewDockerDriver(driverCtx)

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

func TestDockerDriver_Start_Wait_AllocDir(t *testing.T) {
	if !dockerLocated() {
		t.SkipNow()
	}

	exp := []byte{'w', 'i', 'n'}
	file := "output.txt"
	task := &structs.Task{
		Name: "redis-demo",
		Config: map[string]string{
			"image":   "redis",
			"command": "/bin/bash",
			"args":    fmt.Sprintf(`-c "sleep 1; echo -n %s > $%s/%s"`, string(exp), environment.AllocDir, file),
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
	}

	driverCtx := testDockerDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()
	d := NewDockerDriver(driverCtx)

	handle, err := d.Start(ctx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}
	defer handle.Kill()

	select {
	case err := <-handle.WaitCh():
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout")
	}

	// Check that data was written to the shared alloc directory.
	outputFile := filepath.Join(ctx.AllocDir.SharedDir, file)
	act, err := ioutil.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Couldn't read expected output: %v", err)
	}

	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("Command outputted %v; want %v", act, exp)
	}
}

func TestDockerDriver_Start_Kill_Wait(t *testing.T) {
	if !dockerLocated() {
		t.SkipNow()
	}

	task := &structs.Task{
		Name: "redis-demo",
		Config: map[string]string{
			"image":   "redis",
			"command": "/bin/sleep",
			"args":    "10",
		},
		Resources: basicResources,
	}

	driverCtx := testDockerDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()
	d := NewDockerDriver(driverCtx)

	handle, err := d.Start(ctx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}
	defer handle.Kill()

	go func() {
		time.Sleep(100 * time.Millisecond)
		err := handle.Kill()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	select {
	case err := <-handle.WaitCh():
		if err == nil {
			t.Fatalf("should err: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout")
	}
}

func taskTemplate() *structs.Task {
	return &structs.Task{
		Name: "redis-demo",
		Config: map[string]string{
			"image": "redis",
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
			Networks: []*structs.NetworkResource{
				&structs.NetworkResource{
					IP:            "127.0.0.1",
					ReservedPorts: []int{11110},
					DynamicPorts:  []string{"REDIS"},
				},
			},
		},
	}
}

func TestDocker_StartN(t *testing.T) {
	if !dockerLocated() {
		t.SkipNow()
	}

	task1 := taskTemplate()
	task1.Resources.Networks[0].ReservedPorts[0] = 11111

	task2 := taskTemplate()
	task2.Resources.Networks[0].ReservedPorts[0] = 22222

	task3 := taskTemplate()
	task3.Resources.Networks[0].ReservedPorts[0] = 33333

	taskList := []*structs.Task{task1, task2, task3}

	handles := make([]DriverHandle, len(taskList))

	t.Logf("==> Starting %d tasks", len(taskList))

	// Let's spin up a bunch of things
	var err error
	for idx, task := range taskList {
		driverCtx := testDockerDriverContext(task.Name)
		ctx := testDriverExecContext(task, driverCtx)
		defer ctx.AllocDir.Destroy()
		d := NewDockerDriver(driverCtx)

		handles[idx], err = d.Start(ctx, task)
		if err != nil {
			t.Errorf("Failed starting task #%d: %s", idx+1, err)
		}
	}

	t.Log("==> All tasks are started. Terminating...")

	for idx, handle := range handles {
		if handle == nil {
			t.Errorf("Bad handle for task #%d", idx+1)
			continue
		}

		err := handle.Kill()
		if err != nil {
			t.Errorf("Failed stopping task #%d: %s", idx+1, err)
		}
	}

	t.Log("==> Test complete!")
}

func TestDocker_StartNVersions(t *testing.T) {
	if !dockerLocated() {
		t.SkipNow()
	}

	task1 := taskTemplate()
	task1.Config["image"] = "redis"
	task1.Resources.Networks[0].ReservedPorts[0] = 11111

	task2 := taskTemplate()
	task2.Config["image"] = "redis:latest"
	task2.Resources.Networks[0].ReservedPorts[0] = 22222

	task3 := taskTemplate()
	task3.Config["image"] = "redis:3.0"
	task3.Resources.Networks[0].ReservedPorts[0] = 33333

	taskList := []*structs.Task{task1, task2, task3}

	handles := make([]DriverHandle, len(taskList))

	t.Logf("==> Starting %d tasks", len(taskList))

	// Let's spin up a bunch of things
	var err error
	for idx, task := range taskList {
		driverCtx := testDockerDriverContext(task.Name)
		ctx := testDriverExecContext(task, driverCtx)
		defer ctx.AllocDir.Destroy()
		d := NewDockerDriver(driverCtx)

		handles[idx], err = d.Start(ctx, task)
		if err != nil {
			t.Errorf("Failed starting task #%d: %s", idx+1, err)
		}
	}

	t.Log("==> All tasks are started. Terminating...")

	for idx, handle := range handles {
		if handle == nil {
			t.Errorf("Bad handle for task #%d", idx+1)
			continue
		}

		err := handle.Kill()
		if err != nil {
			t.Errorf("Failed stopping task #%d: %s", idx+1, err)
		}
	}

	t.Log("==> Test complete!")
}

func TestDockerHostNet(t *testing.T) {
	if !dockerLocated() {
		t.SkipNow()
	}

	task := &structs.Task{
		Name: "redis-demo",
		Config: map[string]string{
			"image":        "redis",
			"network_mode": "host",
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
	}
	driverCtx := testDockerDriverContext(task.Name)
	ctx := testDriverExecContext(task, driverCtx)
	defer ctx.AllocDir.Destroy()
	d := NewDockerDriver(driverCtx)

	handle, err := d.Start(ctx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}
	defer handle.Kill()
}
