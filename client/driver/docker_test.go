package driver

import (
	"os/exec"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

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
	d := NewDockerDriver(testDriverContext())
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
	if node.Attributes["driver.docker"] == "" {
		t.Fatalf("Docker not found. The remainder of the docker tests will be skipped.")
	}
	t.Logf("Found docker version %s", node.Attributes["driver.docker.version"])
}

func TestDockerDriver_StartOpen_Wait(t *testing.T) {
	if !dockerLocated() {
		t.SkipNow()
	}
	ctx := NewExecContext()
	d := NewDockerDriver(testDriverContext())

	task := &structs.Task{
		Config: map[string]string{
			"image": "redis",
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
	}
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
	ctx := NewExecContext()
	d := NewDockerDriver(testDriverContext())

	task := &structs.Task{
		Config: map[string]string{
			"image": "redis",
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
	}
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
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestDockerDriver_Start_Kill_Wait(t *testing.T) {
	if !dockerLocated() {
		t.SkipNow()
	}
	ctx := NewExecContext()
	d := NewDockerDriver(testDriverContext())

	task := &structs.Task{
		Config: map[string]string{
			"image": "redis",
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
		},
	}
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
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestDocker_StartTwo(t *testing.T) {
	task1 := &structs.Task{
		Config: map[string]string{
			"image": "redis",
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
			Networks: []*structs.NetworkResource{
				&structs.NetworkResource{
					IP:            "127.0.0.1",
					ReservedPorts: []int{11114},
					DynamicPorts:  []string{"REDIS"},
				},
			},
		},
	}

	task2 := &structs.Task{
		Config: map[string]string{
			"image": "redis",
		},
		Resources: &structs.Resources{
			MemoryMB: 256,
			CPU:      512,
			Networks: []*structs.NetworkResource{
				&structs.NetworkResource{
					IP:            "127.0.0.1",
					ReservedPorts: []int{11115},
					DynamicPorts:  []string{"REDIS"},
				},
			},
		},
	}

	ctx := NewExecContext()
	d := NewDockerDriver(testDriverContext())

	d.Start(ctx, task1)
	d.Start(ctx, task2)
}
