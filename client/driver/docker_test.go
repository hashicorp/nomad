package driver

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

var dockerLocated bool = true

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

func TestDockerDriver_Fingerprint(t *testing.T) {
	d := NewDockerDriver(testLogger(), testConfig())
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
		dockerLocated = false
		t.Fatalf("Docker not found. The remainder of the docker tests will be skipped.")
	}
	t.Logf("Found docker version %s", node.Attributes["driver.docker.version"])
}

func TestDockerDriver_StartOpen_Wait(t *testing.T) {
	if !dockerLocated {
		t.SkipNow()
	}
	ctx := NewExecContext()
	d := NewDockerDriver(testLogger(), testConfig())

	task := &structs.Task{
		Config: map[string]string{
			"image": "cbednarski/python-demo",
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
	if !dockerLocated {
		t.SkipNow()
	}
	ctx := NewExecContext()
	d := NewDockerDriver(testLogger(), testConfig())

	task := &structs.Task{
		Config: map[string]string{
			"image": "cbednarski/python-demo",
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
	if !dockerLocated {
		t.SkipNow()
	}
	ctx := NewExecContext()
	d := NewDockerDriver(testLogger(), testConfig())

	task := &structs.Task{
		Config: map[string]string{
			"image": "cbednarski/python-demo",
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
