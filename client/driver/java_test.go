package driver

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestJavaDriver_Fingerprint(t *testing.T) {
	d := NewJavaDriver(testLogger())
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
	if node.Attributes["driver.java"] != "1" {
		t.Fatalf("missing driver")
	}
	for _, key := range []string{"driver.java.version", "driver.java.runtime", "driver.java.vm"} {
		if node.Attributes[key] == "" {
			t.Fatalf("missing driver key (%s)", key)
		}
	}
}

func TestJavaDriver_StartOpen_Wait(t *testing.T) {
	ctx := NewExecContext()
	ctx.AllocDir = os.TempDir()
	d := NewJavaDriver(testLogger())

	task := &structs.Task{
		Config: map[string]string{
			"jar_source": "https://dl.dropboxusercontent.com/u/47675/jar_thing/demoapp.jar",
			// "jar_source": "https://s3-us-west-2.amazonaws.com/java-jar-thing/demoapp.jar",
			// "args": "-d64",
		},
	}
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

	time.Sleep(2 * time.Second)
	// need to kill long lived process
	err = handle.Kill()
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
}

func TestJavaDriver_Start_Wait(t *testing.T) {
	ctx := NewExecContext()
	ctx.AllocDir = os.TempDir()
	d := NewJavaDriver(testLogger())

	task := &structs.Task{
		Config: map[string]string{
			"jar_source": "https://dl.dropboxusercontent.com/u/47675/jar_thing/demoapp.jar",
			// "jar_source": "https://s3-us-west-2.amazonaws.com/java-jar-thing/demoapp.jar",
			// "args": "-d64",
		},
	}
	handle, err := d.Start(ctx, task)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if handle == nil {
		t.Fatalf("missing handle")
	}

	// Task should terminate quickly
	select {
	case err := <-handle.WaitCh():
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	case <-time.After(2 * time.Second):
		// expect the timeout b/c it's a long lived process
		break
	}

	// need to kill long lived process
	err = handle.Kill()
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
}

func TestJavaDriver_Start_Kill_Wait(t *testing.T) {
	ctx := NewExecContext()
	ctx.AllocDir = os.TempDir()
	d := NewJavaDriver(testLogger())

	task := &structs.Task{
		Config: map[string]string{
			"jar_source": "https://dl.dropboxusercontent.com/u/47675/jar_thing/demoapp.jar",
			// "jar_source": "https://s3-us-west-2.amazonaws.com/java-jar-thing/demoapp.jar",
			// "args": "-d64",
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

	// need to kill long lived process
	err = handle.Kill()
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
}

func cleanupFile(path string) error {
	return nil
}
