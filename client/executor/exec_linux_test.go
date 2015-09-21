package executor

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	constraint = &structs.Resources{
		CPU:      0.5,
		MemoryMB: 256,
		Networks: []*structs.NetworkResource{
			&structs.NetworkResource{
				MBits:        50,
				DynamicPorts: 1,
			},
		},
	}
)

func TestExecutorLinux_Start_Invalid(t *testing.T) {
	invalid := "/bin/foobar"
	e := Command(invalid, "1")

	if err := e.Limit(constraint); err != nil {
		t.Fatalf("Limit() failed: %v", err)
	}

	if err := e.Start(); err == nil {
		t.Fatalf("Start(%v) should have failed", invalid)
	}
}

func TestExecutorLinux_Start_Wait_Failure_Code(t *testing.T) {
	e := Command("/bin/date", "-invalid")

	if err := e.Limit(constraint); err != nil {
		t.Fatalf("Limit() failed: %v", err)
	}

	if err := e.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if err := e.Wait(); err == nil {
		t.Fatalf("Wait() should have failed")
	}
}

func TestExecutorLinux_Start_Wait(t *testing.T) {
	path, err := ioutil.TempDir("", "TestExecutorLinux_Start_Wait")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	// Make the file writable to everyone.
	os.Chmod(path, 0777)

	expected := "hello world"
	filePath := filepath.Join(path, "output")
	cmd := fmt.Sprintf("%v \"%v\" > %v", "sleep 1 ; echo -n", expected, filePath)
	e := Command("/bin/bash", "-c", cmd)

	if err := e.Limit(constraint); err != nil {
		t.Fatalf("Limit() failed: %v", err)
	}

	if err := e.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if err := e.Wait(); err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	output, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Couldn't read file %v", filePath)
	}

	act := string(output)
	if act != expected {
		t.Fatalf("Command output incorrectly: want %v; got %v", expected, act)
	}
}

func TestExecutorLinux_Start_Kill(t *testing.T) {
	path, err := ioutil.TempDir("", "TestExecutorLinux_Start_Kill")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	// Make the file writable to everyone.
	os.Chmod(path, 0777)

	filePath := filepath.Join(path, "test")
	e := Command("/bin/bash", "-c", "sleep 1 ; echo \"failure\" > "+filePath)

	// This test can only be run if cgroups are enabled.
	if !e.(*LinuxExecutor).cgroupEnabled {
		t.SkipNow()
	}

	if err := e.Limit(constraint); err != nil {
		t.Fatalf("Limit() failed: %v", err)
	}

	if err := e.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if err := e.Shutdown(); err != nil {
		t.Fatalf("Shutdown() failed: %v", err)
	}

	time.Sleep(1500 * time.Millisecond)

	// Check that the file doesn't exist.
	if _, err := os.Stat(filePath); err == nil {
		t.Fatalf("Stat(%v) should have failed: task not killed", filePath)
	}
}

func TestExecutorLinux_Open(t *testing.T) {
	path, err := ioutil.TempDir("", "TestExecutorLinux_Open")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	// Make the file writable to everyone.
	os.Chmod(path, 0777)

	filePath := filepath.Join(path, "test")
	e := Command("/bin/bash", "-c", "sleep 1 ; echo \"failure\" > "+filePath)

	// This test can only be run if cgroups are enabled.
	if !e.(*LinuxExecutor).cgroupEnabled {
		t.SkipNow()
	}

	if err := e.Limit(constraint); err != nil {
		t.Fatalf("Limit() failed: %v", err)
	}

	if err := e.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	id, err := e.ID()
	if err != nil {
		t.Fatalf("ID() failed: %v", err)
	}

	if _, err := OpenId(id); err == nil {
		t.Fatalf("Open(%v) should have failed", id)
	}

	time.Sleep(1500 * time.Millisecond)

	// Check that the file doesn't exist, open should have killed the process.
	if _, err := os.Stat(filePath); err == nil {
		t.Fatalf("Stat(%v) should have failed: task not killed", filePath)
	}
}
